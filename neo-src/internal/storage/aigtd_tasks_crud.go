// Package storage —— AITasksCrud:tasks 表 + 子任务(tasks.parent_id 关联)
// + ai_jobs 表的统一 CRUD。
//
// 设计要点：
//
//   - tasks 与 ai_jobs 是同一表簇,共享迁移与索引;
//   - 子任务通过 parent_id 关联到顶层 task(自引用树,非独立子表);
//   - ai_jobs.status 状态机由 worker 维护;本 CRUD 只负责持久化;
//   - 字段更新走指针语义(nil = 不动),避免零值覆盖;
package storage

import (
	"database/sql"
	"fmt"

	"little-timer/internal/domain"
)

// TaskUpdateFields 携带可选的更新字段。指针为 nil 表示不动;Clear* 标志
// 为 true 表示把对应列显式置 NULL(覆盖之前的值)。
type TaskUpdateFields struct {
	Title                *string
	RawText              *string
	Notes                *string
	DueDate              *string
	ClearDueDate         bool
	Status               *domain.TaskStatus
	ParentID             *int64
	ClearParentID        bool
	ScheduledStart       *int64
	ClearScheduledStart  bool
	ScheduledEnd         *int64
	ClearScheduledEnd    bool
	Pinned               *bool
	OrderIndex           *int64
	EstimatedMinutes     *int64
}

// SubtaskUpdateFields 是子任务的更新字段集合。
type SubtaskUpdateFields struct {
	Title            *string
	Status           *domain.SubtaskStatus
	EstimatedMinutes *int64
	OrderIndex       *int64
}

// AITasksCrud 操作 tasks 表与 ai_jobs 表。
type AITasksCrud struct {
	db *sql.DB
}

// NewAITasksCrud 返回空的 AITasksCrud。SetDB 由 SqliteManager 调用。
func NewAITasksCrud() *AITasksCrud { return &AITasksCrud{} }

// SetDB 接入 *sql.DB。
func (c *AITasksCrud) SetDB(db *sql.DB) { c.db = db }

// CreateTask 建一条顶层 task(raw_text 入库,其余列取默认值)。
func (c *AITasksCrud) CreateTask(rawText, source string) (int64, error) {
	res, err := c.db.Exec(
		`INSERT INTO tasks (raw_text, source, status, ai_status) VALUES (?, ?, 'inbox', 'pending');`,
		rawText, source,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrHabitInsertFailed, err)
	}
	return res.LastInsertId()
}

// GetTask 按 ID 取一行任务 + 它的全部子任务(按 order_index, id 排序)。
func (c *AITasksCrud) GetTask(id int64) (*domain.TaskRow, []domain.SubtaskRow, error) {
	task, err := scanTask(c.db.QueryRow(
		`SELECT `+taskCols+` FROM tasks WHERE id = ?;`, id,
	))
	if err != nil {
		return nil, nil, err
	}
	rows, err := c.db.Query(
		`SELECT `+subtaskCols+` FROM tasks
		 WHERE parent_id = ?
		 ORDER BY order_index ASC, id ASC;`, id,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var subs []domain.SubtaskRow
	for rows.Next() {
		s, err := scanSubtaskRow(rows)
		if err != nil {
			return nil, nil, err
		}
		subs = append(subs, s)
	}
	return task, subs, rows.Err()
}

// ListTopLevel 列顶层任务(parent_id IS NULL)。
//
// `status` 为空表示不过滤;默认隐藏 archived/rejected。
func (c *AITasksCrud) ListTopLevel(status string, limit, offset int) ([]domain.TaskRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT ` + taskCols + ` FROM tasks WHERE parent_id IS NULL`
	args := []any{}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	} else {
		q += ` AND status NOT IN ('archived','rejected')`
	}
	q += ` ORDER BY created_at DESC LIMIT ? OFFSET ?;`
	args = append(args, limit, offset)

	rows, err := c.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHabitQueryFailed, err)
	}
	defer rows.Close()
	var out []domain.TaskRow
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// ListSubtasks 取指定 task 的全部子任务。
func (c *AITasksCrud) ListSubtasks(taskID int64) ([]domain.SubtaskRow, error) {
	rows, err := c.db.Query(
		`SELECT `+subtaskCols+` FROM tasks WHERE parent_id = ? ORDER BY order_index, id;`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SubtaskRow
	for rows.Next() {
		s, err := scanSubtaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateTask 按指针语义更新 fields 中显式给出的列。空操作。
func (c *AITasksCrud) UpdateTask(id int64, fields TaskUpdateFields) error {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		sets = append(sets, col+" = ?")
		args = append(args, v)
	}
	if fields.Title != nil {
		add("title", *fields.Title)
	}
	if fields.RawText != nil {
		add("raw_text", *fields.RawText)
	}
	if fields.Notes != nil {
		add("notes", *fields.Notes)
	}
	if fields.ClearDueDate {
		add("due_date", nil)
	} else if fields.DueDate != nil {
		add("due_date", *fields.DueDate)
	}
	if fields.Status != nil {
		add("status", string(*fields.Status))
	}
	if fields.ClearParentID {
		add("parent_id", nil)
	} else if fields.ParentID != nil {
		add("parent_id", *fields.ParentID)
	}
	if fields.ClearScheduledStart {
		add("scheduled_start", nil)
	} else if fields.ScheduledStart != nil {
		add("scheduled_start", *fields.ScheduledStart)
	}
	if fields.ClearScheduledEnd {
		add("scheduled_end", nil)
	} else if fields.ScheduledEnd != nil {
		add("scheduled_end", *fields.ScheduledEnd)
	}
	if fields.Pinned != nil {
		add("pinned", boolToInt(*fields.Pinned))
	}
	if fields.OrderIndex != nil {
		add("order_index", *fields.OrderIndex)
	}
	if fields.EstimatedMinutes != nil {
		add("estimated_minutes", *fields.EstimatedMinutes)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)
	_, err := c.db.Exec(
		"UPDATE tasks SET "+joinComma(sets)+" WHERE id = ?;", args...,
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHabitUpdateFailed, err)
	}
	return nil
}

// SetAIStatus 只更新 ai_status / ai_error / updated_at。
func (c *AITasksCrud) SetAIStatus(taskID int64, status domain.AIStatus, errMsg string) error {
	_, err := c.db.Exec(
		`UPDATE tasks SET ai_status = ?, ai_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`,
		string(status), errMsg, taskID,
	)
	return err
}

// SetTaskStatus 更新 status 与 updated_at。
func (c *AITasksCrud) SetTaskStatus(id int64, status domain.TaskStatus) error {
	_, err := c.db.Exec(
		`UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`,
		string(status), id,
	)
	return err
}

// DeleteTask 删除一条任务(级联到子任务与 ai_jobs)。
func (c *AITasksCrud) DeleteTask(id int64) error {
	_, err := c.db.Exec(`DELETE FROM tasks WHERE id = ?;`, id)
	return err
}

// MarkParsedAt 设置 parsed_at。
func (c *AITasksCrud) MarkParsedAt(taskID int64) error {
	_, err := c.db.Exec(
		`UPDATE tasks SET parsed_at = CURRENT_TIMESTAMP WHERE id = ?;`, taskID,
	)
	return err
}

// CreateSubtask 建一条子任务(parent_id 指向顶层 task)。
func (c *AITasksCrud) CreateSubtask(taskID int64, title string, estMin int, dueDate *string, orderIndex int) (int64, error) {
	var due any
	if dueDate != nil {
		due = *dueDate
	}
	res, err := c.db.Exec(
		`INSERT INTO tasks (parent_id, title, estimated_minutes, due_date, order_index, source, status, ai_status)
		 VALUES (?, ?, ?, ?, ?, 'ai', 'active', 'done');`,
		taskID, title, estMin, due, orderIndex,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrHabitInsertFailed, err)
	}
	return res.LastInsertId()
}

// UpdateSubtask 按指针语义更新子任务。
func (c *AITasksCrud) UpdateSubtask(id int64, fields SubtaskUpdateFields) error {
	sets := []string{}
	args := []any{}
	if fields.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *fields.Title)
	}
	if fields.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, string(*fields.Status))
	}
	if fields.EstimatedMinutes != nil {
		sets = append(sets, "estimated_minutes = ?")
		args = append(args, *fields.EstimatedMinutes)
	}
	if fields.OrderIndex != nil {
		sets = append(sets, "order_index = ?")
		args = append(args, *fields.OrderIndex)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)
	_, err := c.db.Exec(
		"UPDATE tasks SET "+joinComma(sets)+" WHERE id = ? AND parent_id IS NOT NULL;", args...,
	)
	return err
}

// DeleteSubtask 删除一条子任务(必须 parent_id 非空,误删顶层有 FK 兜底)。
func (c *AITasksCrud) DeleteSubtask(id int64) error {
	_, err := c.db.Exec(`DELETE FROM tasks WHERE id = ? AND parent_id IS NOT NULL;`, id)
	return err
}

// EnqueueJob 入队一个 ai_jobs 行(状态 queued)。
func (c *AITasksCrud) EnqueueJob(taskID int64, provider, model, reqPayload string) (int64, error) {
	res, err := c.db.Exec(
		`INSERT INTO ai_jobs (task_id, provider, model, status, request_payload)
		 VALUES (?, ?, ?, 'queued', ?);`,
		taskID, provider, model, reqPayload,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ClaimNextQueuedJob 原子地取出最早一个 queued 任务,标记为 running。
// 用 SQLite 的 UPDATE...RETURNING(3.35+) 单条 SQL 实现。attempts 不在此处
// 自增 —— 重试语义由 worker 在 RequeueJob 里增加,避免"claim = 一次尝试"的双计。
func (c *AITasksCrud) ClaimNextQueuedJob() (*domain.AIJobRow, error) {
	row := c.db.QueryRow(
		`UPDATE ai_jobs
		 SET status = 'running', started_at = CURRENT_TIMESTAMP
		 WHERE id = (
		   SELECT id FROM ai_jobs WHERE status = 'queued'
		   ORDER BY created_at ASC LIMIT 1
		 )
		 RETURNING `+aiJobCols+`;`,
	)
	return scanAIJobRow(row)
}

// RequeueJob 把 running 的 job 退回 queued,attempts 自增,started_at 清空。
// 供 Worker 在遇到可重试错误(ErrRetryable)且未达上限时调用。
func (c *AITasksCrud) RequeueJob(jobID int64) error {
	_, err := c.db.Exec(
		`UPDATE ai_jobs
		 SET status='queued', started_at=NULL, attempts=attempts+1
		 WHERE id=? AND status='running';`,
		jobID,
	)
	return err
}

// MarkJobRunning 显式标记 running(当前实现由 ClaimNextQueuedJob 自动置位,
// 保留给其他调用方)。
func (c *AITasksCrud) MarkJobRunning(jobID int64) error {
	_, err := c.db.Exec(
		`UPDATE ai_jobs SET status='running', started_at=CURRENT_TIMESTAMP WHERE id=?;`, jobID,
	)
	return err
}

// MarkJobSuccess 写入响应 + 终态。
func (c *AITasksCrud) MarkJobSuccess(jobID int64, respPayload string, inTok, outTok int) error {
	_, err := c.db.Exec(
		`UPDATE ai_jobs
		 SET status='success', finished_at=CURRENT_TIMESTAMP,
		     response_payload=?, input_tokens=?, output_tokens=?
		 WHERE id=?;`,
		respPayload, inTok, outTok, jobID,
	)
	return err
}

// MarkJobFailed 写入失败原因 + 终态。
func (c *AITasksCrud) MarkJobFailed(jobID int64, errMsg string) error {
	_, err := c.db.Exec(
		`UPDATE ai_jobs SET status='failed', finished_at=CURRENT_TIMESTAMP, error_message=?
		 WHERE id=?;`,
		errMsg, jobID,
	)
	return err
}

// ReclaimStuckRunningJobs 把超过 thresholdSeconds 秒还卡在 running 的 job
// 退回 queued,attempts 自增(started_at 也清空)。attempts 累加与 RequeueJob
// 共用同一上限(maxJobAttempts,见 worker.go),防止 worker panic/OOM 后
// 一直 reclaim 绕开重试上限。
func (c *AITasksCrud) ReclaimStuckRunningJobs(thresholdSeconds int) (int, error) {
	res, err := c.db.Exec(
		fmt.Sprintf(
			`UPDATE ai_jobs
			 SET status='queued', started_at=NULL, attempts=attempts+1
			 WHERE status='running' AND started_at < datetime('now', '-%d seconds');`,
			thresholdSeconds,
		),
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// LatestJobForTask 返回某 task 的最近一条 ai_job(按 id DESC)。
func (c *AITasksCrud) LatestJobForTask(taskID int64) (*domain.AIJobRow, error) {
	row := c.db.QueryRow(
		`SELECT `+aiJobCols+` FROM ai_jobs WHERE task_id = ? ORDER BY id DESC LIMIT 1;`,
		taskID,
	)
	return scanAIJobRow(row)
}

// ----- 内部辅助 -----

const taskCols = `id, parent_id, raw_text, title, notes, status, due_date,
	scheduled_start, scheduled_end, pinned, estimated_minutes, order_index,
	source, ai_status, ai_error, created_at, updated_at, parsed_at`

const subtaskCols = `id, parent_id, title, estimated_minutes, order_index, status, created_at`

const aiJobCols = `id, task_id, provider, model, status, request_payload,
	response_payload, error_message, input_tokens, output_tokens, attempts,
	started_at, finished_at, created_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (*domain.TaskRow, error) {
	var t domain.TaskRow
	var parent sql.NullInt64
	var rawText, notes sql.NullString
	var status string
	var dueDate sql.NullString
	var schedStart, schedEnd sql.NullInt64
	var pinned int
	var aiStatus, aiError sql.NullString
	var parsedAt sql.NullTime
	err := row.Scan(
		&t.ID, &parent, &rawText, &t.Title, &notes, &status, &dueDate,
		&schedStart, &schedEnd, &pinned, &t.EstimatedMinutes, &t.OrderIndex,
		&t.Source, &aiStatus, &aiError, &t.CreatedAt, &t.UpdatedAt, &parsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHabitQueryFailed, err)
	}
	if parent.Valid {
		v := parent.Int64
		t.ParentID = &v
	}
	if rawText.Valid {
		t.RawText = rawText.String
	}
	if notes.Valid {
		t.Notes = notes.String
	}
	t.Status = domain.TaskStatus(status)
	if dueDate.Valid {
		v := dueDate.String
		t.DueDate = &v
	}
	if schedStart.Valid {
		v := schedStart.Int64
		t.ScheduledStart = &v
	}
	if schedEnd.Valid {
		v := schedEnd.Int64
		t.ScheduledEnd = &v
	}
	t.Pinned = pinned != 0
	t.AIStatus = domain.AIStatus(aiStatus.String)
	if aiError.Valid {
		t.AIError = aiError.String
	}
	if parsedAt.Valid {
		v := parsedAt.Time
		t.ParsedAt = &v
	}
	return &t, nil
}

func scanTaskRow(rows *sql.Rows) (*domain.TaskRow, error) { return scanTask(rows) }
func scanSubtaskRow(rows rowScanner) (domain.SubtaskRow, error) {
	var s domain.SubtaskRow
	var parent sql.NullInt64
	var status string
	if err := rows.Scan(
		&s.ID, &parent, &s.Title, &s.EstimatedMinutes, &s.OrderIndex,
		&status, &s.CreatedAt,
	); err != nil {
		return s, fmt.Errorf("%w: %v", ErrHabitQueryFailed, err)
	}
	if parent.Valid {
		s.TaskID = parent.Int64
	}
	s.Status = domain.SubtaskStatus(status)
	return s, nil
}

func scanAIJobRow(row rowScanner) (*domain.AIJobRow, error) {
	var j domain.AIJobRow
	var status string
	var resp, errMsg sql.NullString
	var inTok, outTok sql.NullInt64
	var started, finished sql.NullTime
	if err := row.Scan(
		&j.ID, &j.TaskID, &j.Provider, &j.Model, &status,
		&j.RequestPayload, &resp, &errMsg, &inTok, &outTok, &j.Attempts,
		&started, &finished, &j.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrHabitQueryFailed, err)
	}
	j.Status = domain.AIJobStatus(status)
	if resp.Valid {
		j.ResponsePayload = resp.String
	}
	if errMsg.Valid {
		j.ErrorMessage = errMsg.String
	}
	if inTok.Valid {
		v := int(inTok.Int64)
		j.InputTokens = &v
	}
	if outTok.Valid {
		v := int(outTok.Int64)
		j.OutputTokens = &v
	}
	if started.Valid {
		v := started.Time
		j.StartedAt = &v
	}
	if finished.Valid {
		v := finished.Time
		j.FinishedAt = &v
	}
	return &j, nil
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
