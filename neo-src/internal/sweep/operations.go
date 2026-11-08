// Package sweep 是 GTD Daily Sweep / Review 的用例层(issue #66)。
//
// 职责:
//   - Adopt:幂等前置 → 事务 → 版本闸门(恰好一次)→ 审计行 → commit;
//   - Reject:仅校验入参(不写 reject 审计,YAGNI);
//   - Start:为每个顶层 task 入队一个 review job(best-effort);
//   - Today:拉当日 review 提议,附 ai_job_id / 当前 version / 是否已采纳。
//
// 事务边界、乐观锁、幂等 token 都在本包;HTTP handler 只做 JSON 绑定与
// sweep.Error → 状态码映射。
package sweep

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// Error 是 sweep 域的类型化错误。
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// Is 让同 Code 的错误互相匹配(带 detail 的副本仍能被 errors.Is 命中哨兵)。
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e.Code == t.Code
}

var (
	// ErrVersionConflict 表示版本闸门命中 0 行 → 409,提示客户端刷新。
	ErrVersionConflict = &Error{Code: "version_conflict", Message: "version conflict"}
	// ErrInvalidProposal 覆盖所有入参不合法:无效 action、ai_job 不存在、
	// proposal.task_id 与 job 不符、reschedule 无时间。
	ErrInvalidProposal = &Error{Code: "sweep.invalid_proposal", Message: "invalid proposal"}
	// ErrAdoptionDuplicate 表示并发竞态下审计行已被写入;caller 按幂等处理。
	ErrAdoptionDuplicate = &Error{Code: "sweep.adoption_duplicate", Message: "adoption already recorded"}
)

// Store 是本包依赖的持久化能力集合。
type Store interface {
	DB() *sql.DB
	ListTopLevel(status string, limit, offset int) ([]domain.TaskRow, error)
	GetTask(id int64) (*domain.TaskRow, []domain.SubtaskRow, error)
	GetTaskVersion(id int64) (int64, error)
	UpdateTaskTx(tx *sql.Tx, id int64, fields storage.TaskUpdateFields, version int64) error
	BumpTaskVersionTx(tx *sql.Tx, id int64, version int64) error
	GetSubtaskStatusTx(tx *sql.Tx, id int64) (domain.SubtaskStatus, int64, int64, error)
	UpdateSubtaskTx(tx *sql.Tx, id int64, fields storage.SubtaskUpdateFields, version int64) error
	AppendSubtasksTx(tx *sql.Tx, taskID int64, subs []storage.SubtaskInput) error
	DeleteSubtaskTx(tx *sql.Tx, id int64) error
	EnqueueJob(taskID int64, provider, model, reqPayload string, mode domain.AIJobMode) (int64, error)
	FindAdoption(aiJobID int64, changeID string) (*domain.Adoption, error)
	RecordAdoptionTx(tx *sql.Tx, a domain.Adoption) (int64, error)
	GetSweepLog(date string) (*domain.DailySweepLog, error)
	LogSweep(date, source string) error
	// RecentReviewJobs 返回近 1 天内 status='success' 的 review job 原始载荷。
	RecentReviewJobs() ([]ReviewJobPayload, error)
}

// ReviewJobPayload 是一条成功 review job 的原始载荷。
type ReviewJobPayload struct {
	JobID   int64
	TaskID  int64
	Payload string
}

// AdoptInput 是 Adopt 的入参。
type AdoptInput struct {
	AIJobID  int64
	ChangeID string
	Version  int64
	Proposal domain.ReviewProposal
}

// AdoptResult 是 Adopt 的返回。Idempotent 为 true 表示这次调用命中幂等路径,
// 没有写任何新行。
type AdoptResult struct {
	TaskID       int64
	VersionAfter int64
	Idempotent   bool
}

// TodayProposal 是 Today 返回的一条提议。
type TodayProposal struct {
	AIJobID   int64
	Proposal  domain.ReviewProposal
	Version   int64
	IsAdopted bool
}

// TodayResult 是 Today 的返回。
type TodayResult struct {
	Proposals []TodayProposal
	Log       *domain.DailySweepLog
}

// Operations 是 daily sweep 用例层。
type Operations struct {
	store  Store
	config func() (provider, model string)
}

// New 构造 Operations。config 为 nil 时用默认 provider/model。
func New(store Store, config func() (provider, model string)) *Operations {
	if config == nil {
		config = DefaultConfig
	}
	return &Operations{store: store, config: config}
}

// DefaultConfig 返回 settings 缺省时的 LLM provider/model。
func DefaultConfig() (provider, model string) {
	return "openai_compat", "deepseek-chat"
}

// Adopt 采纳一条 review 提议。
//
// 流程:
//  0. proposal.TaskID != 0 且 action 合法,否则 ErrInvalidProposal;
//  1. reschedule 必须至少给一个时间(否则无字段可改会静默不 bump,审计撒谎);
//  2. 以 ai_jobs.task_id 为权威 task id,与 proposal.task_id 不符 → ErrInvalidProposal;
//  3. 幂等前置命中 → Idempotent=true,不重复改;
//  4. 开事务,版本闸门恰好一次(reschedule/drop 走 UpdateTaskTx 自带 CAS+bump;
//     modify_subtasks 走 applySubChangesTx + BumpTaskVersionTx;no_op 只 bump);
//  5. RecordAdoptionTx(version_after = version+1);
//  6. commit。任一版本冲突 → ErrVersionConflict(整事务回滚)。
//
// 不双 bump:reschedule/drop 走 UpdateTaskTx 已 bump,不能再调 BumpTaskVersionTx。
func (o *Operations) Adopt(in AdoptInput) (AdoptResult, error) {
	if in.Proposal.TaskID == 0 || !aigtd.IsValidAction(in.Proposal.Action) {
		return AdoptResult{}, ErrInvalidProposal
	}
	if in.Proposal.Action == "reschedule" &&
		in.Proposal.SuggestedStart == nil && in.Proposal.SuggestedEnd == nil {
		return AdoptResult{}, ErrInvalidProposal
	}
	realTaskID, err := o.jobTaskID(in.AIJobID)
	if err != nil {
		return AdoptResult{}, ErrInvalidProposal
	}
	if realTaskID != in.Proposal.TaskID {
		return AdoptResult{}, ErrInvalidProposal
	}

	// 1. 幂等前置
	if ex, _ := o.store.FindAdoption(in.AIJobID, in.ChangeID); ex != nil {
		return AdoptResult{TaskID: ex.TaskID, VersionAfter: ex.VersionAfter, Idempotent: true}, nil
	}

	tx, err := o.store.DB().Begin()
	if err != nil {
		return AdoptResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	taskID := in.Proposal.TaskID
	version := in.Version

	// 2. 版本闸门(恰好一次)
	switch in.Proposal.Action {
	case "reschedule":
		if err := o.store.UpdateTaskTx(tx, taskID, storage.TaskUpdateFields{
			ScheduledStart: in.Proposal.SuggestedStart,
			ScheduledEnd:   in.Proposal.SuggestedEnd,
		}, version); err != nil {
			return AdoptResult{}, mapStoreError(err)
		}
	case "drop":
		archived := domain.TaskStatusArchived
		if err := o.store.UpdateTaskTx(tx, taskID,
			storage.TaskUpdateFields{Status: &archived}, version); err != nil {
			return AdoptResult{}, mapStoreError(err)
		}
	case "modify_subtasks":
		if err := o.applySubChangesTx(tx, in.Proposal, taskID); err != nil {
			return AdoptResult{}, mapStoreError(err)
		}
		if err := o.store.BumpTaskVersionTx(tx, taskID, version); err != nil {
			return AdoptResult{}, mapStoreError(err)
		}
	case "no_op":
		if err := o.store.BumpTaskVersionTx(tx, taskID, version); err != nil {
			return AdoptResult{}, mapStoreError(err)
		}
	default:
		return AdoptResult{}, ErrInvalidProposal
	}

	// 3. 审计行
	if _, err := o.store.RecordAdoptionTx(tx, domain.Adoption{
		TaskID:        taskID,
		AIJobID:       in.AIJobID,
		ChangeID:      in.ChangeID,
		VersionBefore: version,
		VersionAfter:  version + 1,
	}); err != nil {
		if errors.Is(err, storage.ErrAdoptionDuplicate) {
			// 并发兜底 —— 重查返幂等
			if ex, ferr := o.store.FindAdoption(in.AIJobID, in.ChangeID); ferr == nil && ex != nil {
				return AdoptResult{TaskID: ex.TaskID, VersionAfter: ex.VersionAfter, Idempotent: true}, nil
			}
		}
		return AdoptResult{}, err
	}

	// 4. commit
	if err := tx.Commit(); err != nil {
		return AdoptResult{}, err
	}
	return AdoptResult{TaskID: taskID, VersionAfter: version + 1}, nil
}

// applySubChangesTx 按 op 派发子任务变更;目标非 active 或跨 task → 跳过
// (不是错误)。
//
// op 集:add / delete / modify_title / mark_done。subVer 由
// GetSubtaskStatusTx 在 tx 内取。
func (o *Operations) applySubChangesTx(tx *sql.Tx, p domain.ReviewProposal, taskID int64) error {
	for _, sc := range p.SuggestedSubtaskChanges {
		switch sc.Op {
		case "add":
			if sc.NewTitle == nil {
				continue
			}
			if err := o.store.AppendSubtasksTx(tx, taskID, []storage.SubtaskInput{
				{Title: *sc.NewTitle, EstimatedMinutes: 25},
			}); err != nil {
				return err
			}
		case "delete":
			if sc.SubtaskID == nil {
				continue
			}
			status, parentID, _, err := o.store.GetSubtaskStatusTx(tx, *sc.SubtaskID)
			if err != nil || status != domain.SubtaskStatusActive || parentID != taskID {
				continue // 非 active 或跨 task → 跳过(防误改他 task 的子任务)
			}
			if err := o.store.DeleteSubtaskTx(tx, *sc.SubtaskID); err != nil {
				return err
			}
		case "modify_title":
			if sc.SubtaskID == nil || sc.NewTitle == nil {
				continue
			}
			status, parentID, ver, err := o.store.GetSubtaskStatusTx(tx, *sc.SubtaskID)
			if err != nil || status != domain.SubtaskStatusActive || parentID != taskID {
				continue
			}
			if err := o.store.UpdateSubtaskTx(tx, *sc.SubtaskID,
				storage.SubtaskUpdateFields{Title: sc.NewTitle}, ver); err != nil {
				return err
			}
		case "mark_done":
			if sc.SubtaskID == nil {
				continue
			}
			status, parentID, ver, err := o.store.GetSubtaskStatusTx(tx, *sc.SubtaskID)
			if err != nil || status != domain.SubtaskStatusActive || parentID != taskID {
				continue
			}
			done := domain.SubtaskStatusDone
			if err := o.store.UpdateSubtaskTx(tx, *sc.SubtaskID,
				storage.SubtaskUpdateFields{Status: &done}, ver); err != nil {
				return err
			}
		default:
			// 未知 op 或被丢弃 —— 跳过(非错误)。
		}
	}
	return nil
}

// jobTaskID 查 ai_jobs 行返回其 task_id。job 不存在 → 错误。
func (o *Operations) jobTaskID(jobID int64) (int64, error) {
	db := o.store.DB()
	if db == nil {
		return 0, errors.New("sweep: database not connected")
	}
	var t int64
	err := db.QueryRow(`SELECT task_id FROM ai_jobs WHERE id = ?;`, jobID).Scan(&t)
	if err != nil {
		return 0, err
	}
	return t, nil
}

// Reject 拒绝一条提议:只校验入参,不写审计(Δ4.4 — YAGNI)。
func (o *Operations) Reject(changeID string) error {
	if strings.TrimSpace(changeID) == "" {
		return ErrInvalidProposal
	}
	return nil
}

// Start 为所有顶层非 archived/rejected 的 task 各入队一个 review job。
//
// 同一 task 已有 queued/running job 则跳过;单个 task 入队失败不阻塞其余
// (best-effort)。
func (o *Operations) Start(ctx context.Context) ([]int64, error) {
	_ = ctx
	tasks, err := o.store.ListTopLevel("", 100, 0)
	if err != nil {
		return nil, err
	}
	provider, model := o.config()
	jobIDs := []int64{}
	for _, t := range tasks {
		id, err := o.store.EnqueueJob(t.ID, provider, model, `{}`, domain.AIJobModeReview)
		if err != nil {
			continue // best-effort(含 ErrJobInFlight)
		}
		jobIDs = append(jobIDs, id)
	}
	return jobIDs, nil
}

// Log 写一条 sweep 日志(cron / 手动触发各自决定是否写)。
func (o *Operations) Log(date, source string) error {
	return o.store.LogSweep(date, source)
}

// Today 拉近 1 天内成功的 review job 的提议,附 ai_job_id / 当前 version /
// 是否已采纳;today 用于取当日 sweep 日志(YYYY-MM-DD)。
func (o *Operations) Today(today string) (TodayResult, error) {
	rows, err := o.store.RecentReviewJobs()
	if err != nil {
		return TodayResult{}, err
	}
	out := make([]TodayProposal, 0, len(rows))
	for _, r := range rows {
		if r.Payload == "" {
			continue
		}
		props, _, _ := aigtd.ParseReviewResult(r.TaskID, r.JobID, r.Payload)
		ver, _ := o.store.GetTaskVersion(r.TaskID)
		for _, p := range props {
			adopted := false
			if ex, _ := o.store.FindAdoption(r.JobID, p.ChangeID); ex != nil {
				adopted = true
			}
			out = append(out, TodayProposal{
				AIJobID: r.JobID, Proposal: p, Version: ver, IsAdopted: adopted,
			})
		}
	}
	result := TodayResult{Proposals: out}
	log, _ := o.store.GetSweepLog(today)
	result.Log = log
	return result, nil
}

// sqliteStore 把 storage.SqliteManager 适配成 Store。SQL 细节(近 1 天窗口)
// 留在用例层,避免 adapter 直接拼查询。
type sqliteStore struct {
	sqlite *storage.SqliteManager
}

func (s sqliteStore) DB() *sql.DB { return s.sqlite.DB() }

func (s sqliteStore) ListTopLevel(status string, limit, offset int) ([]domain.TaskRow, error) {
	return s.sqlite.AITasks().ListTopLevel(status, limit, offset)
}

func (s sqliteStore) GetTask(id int64) (*domain.TaskRow, []domain.SubtaskRow, error) {
	return s.sqlite.AITasks().GetTask(id)
}

func (s sqliteStore) GetTaskVersion(id int64) (int64, error) {
	return s.sqlite.AITasks().GetTaskVersion(id)
}

func (s sqliteStore) UpdateTaskTx(tx *sql.Tx, id int64, fields storage.TaskUpdateFields, version int64) error {
	return s.sqlite.AITasks().UpdateTaskTx(tx, id, fields, version)
}

func (s sqliteStore) BumpTaskVersionTx(tx *sql.Tx, id int64, version int64) error {
	return s.sqlite.AITasks().BumpTaskVersionTx(tx, id, version)
}

func (s sqliteStore) GetSubtaskStatusTx(tx *sql.Tx, id int64) (domain.SubtaskStatus, int64, int64, error) {
	return s.sqlite.AITasks().GetSubtaskStatusTx(tx, id)
}

func (s sqliteStore) UpdateSubtaskTx(tx *sql.Tx, id int64, fields storage.SubtaskUpdateFields, version int64) error {
	return s.sqlite.AITasks().UpdateSubtaskTx(tx, id, fields, version)
}

func (s sqliteStore) AppendSubtasksTx(tx *sql.Tx, taskID int64, subs []storage.SubtaskInput) error {
	return s.sqlite.AITasks().AppendSubtasksTx(tx, taskID, subs)
}

func (s sqliteStore) DeleteSubtaskTx(tx *sql.Tx, id int64) error {
	return s.sqlite.AITasks().DeleteSubtaskTx(tx, id)
}

func (s sqliteStore) EnqueueJob(taskID int64, provider, model, reqPayload string, mode domain.AIJobMode) (int64, error) {
	return s.sqlite.AITasks().EnqueueJob(taskID, provider, model, reqPayload, mode)
}

func (s sqliteStore) FindAdoption(aiJobID int64, changeID string) (*domain.Adoption, error) {
	return s.sqlite.Adoptions().FindAdoption(aiJobID, changeID)
}

func (s sqliteStore) RecordAdoptionTx(tx *sql.Tx, a domain.Adoption) (int64, error) {
	return s.sqlite.Adoptions().RecordAdoptionTx(tx, a)
}

func (s sqliteStore) GetSweepLog(date string) (*domain.DailySweepLog, error) {
	return s.sqlite.DailySweep().GetLog(date)
}

func (s sqliteStore) LogSweep(date, source string) error {
	return s.sqlite.DailySweep().LogSweep(date, source)
}

// RecentReviewJobs 查近 1 天成功 review job。
func (s sqliteStore) RecentReviewJobs() ([]ReviewJobPayload, error) {
	db := s.sqlite.DB()
	if db == nil {
		return nil, errors.New("sweep: database not connected")
	}
	rows, err := db.Query(
		`SELECT id, task_id, response_payload FROM ai_jobs
		 WHERE mode = 'review' AND status = 'success'
		   AND created_at >= datetime('now','-1 day')
		 ORDER BY created_at DESC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReviewJobPayload
	for rows.Next() {
		var jobID, taskID int64
		var payload sql.NullString
		if err := rows.Scan(&jobID, &taskID, &payload); err != nil {
			continue
		}
		if !payload.Valid || payload.String == "" {
			continue
		}
		out = append(out, ReviewJobPayload{JobID: jobID, TaskID: taskID, Payload: payload.String})
	}
	return out, rows.Err()
}

// NewForSqlite 用真实 SqliteManager 构造 Operations(生产接线入口)。
// 放在本包内,避免 adapter 层 import internal/storage。
func NewForSqlite(sqlite *storage.SqliteManager, config func() (provider, model string)) *Operations {
	return New(sqliteStore{sqlite: sqlite}, config)
}

// ReadLLMProviderModel 从 settings id=1 读 llm_provider / llm_model(带回退)。
func ReadLLMProviderModel(db *sql.DB) (provider, model string) {
	provider, model = DefaultConfig()
	if db == nil {
		return provider, model
	}
	_ = db.QueryRow(
		`SELECT llm_provider, llm_model FROM settings WHERE id = 1;`,
	).Scan(&provider, &model)
	if provider == "" {
		provider = "openai_compat"
	}
	if model == "" {
		model = "deepseek-chat"
	}
	return provider, model
}

// mapStoreError 把 storage 哨兵翻译成域错误。
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, storage.ErrVersionConflict) {
		return ErrVersionConflict
	}
	return err
}
