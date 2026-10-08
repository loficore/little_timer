// Package handlers —— /api/daily-sweep/*:GTD Review 的触发与提议查询。
//
//   - StartDailySweep 是被 cron 与手动端点共享的入队逻辑(只入队,不写日志);
//   - DailySweepStart 手动触发:入队 + 写 manual 日志(不查 enabled、不查同日去重);
//   - DailySweepToday 拉当日(近 1 天)成功 review job 的 proposals,附 ai_job_id
//     与当前 task version 供前端 adopt;
//   - DailySweepAdopt / DailySweepReject 用户逐条采纳 / 拒绝(adopt 幂等 +
//     mark_done + 事务内版本闸门)。
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/storage"
)

// adoptRequest 是 POST /api/daily-sweep/adopt 的入参。
type adoptRequest struct {
	ChangeID string                 `json:"change_id"`
	AIJobID  int64                  `json:"ai_job_id"`
	Version  int64                  `json:"version"`
	Proposal domain.ReviewProposal  `json:"proposal"`
}

// DailySweepAdopt POST /api/daily-sweep/adopt
//
// 流程(Δ2 + Δ3 + Δ5.4):
//   0. bind body; proposal.TaskID != 0 且 Action ∈ validActions,否则 400;
//   1. 幂等前置命中 — FindAdoption(aiJobID, changeID) → 200 {idempotent:true} 不重复改;
//   2. 开 tx;defer rollback;
//   3. 版本闸门(恰好一次):
//        reschedule/drop → UpdateTaskTx(tx, ..., body.Version)(自带 CAS + bump);
//        modify_subtasks → applySubChangesTx + BumpTaskVersionTx;
//        no_op → BumpTaskVersionTx(只闸门 + bump);
//   4. RecordAdoptionTx(…version_after=body.Version+1);
//   5. commit → 200 {task_id, version_after};ErrVersionConflict → 409;
//      RecordAdoptionTx 命中 UNIQUE(idx_adoptions_idempotent) 并发兜底
//      → 重查 FindAdoption 返 200 idempotent。
//
// 重要:不双 bump。reschedule/drop 走 UpdateTaskTx 已 bump 版本,不能再调
// BumpTaskVersionTx,否则 version_after = +2 破坏后续 adopt 的 409 语义。
func DailySweepAdopt(c *gin.Context) {
	a := appFromCtx(c)
	var body adoptRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	if body.Proposal.TaskID == 0 || !aigtd.IsValidAction(body.Proposal.Action) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid proposal"})
		return
	}
	// M-1:reschedule 必须至少给一个时间 — 否则 UpdateTaskTx 无字段可改会静默不 bump,
	// 导致响应/审计的 version_after 与实际 task.version 不一致(撒谎)。
	if body.Proposal.Action == "reschedule" &&
		body.Proposal.SuggestedStart == nil && body.Proposal.SuggestedEnd == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "reschedule requires suggested_scheduled_start or suggested_scheduled_end",
		})
		return
	}
	// I-4:以 ai_jobs.task_id 为权威 — 不信任 body.proposal.task_id(可能是 LLM 幻觉
	// 或客户端篡改)。ai_job 不存在或与 body 不符 → 400,后续 gate/audit 全用 realTaskID。
	realTaskID, err := jobTaskID(a, body.AIJobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "unknown ai_job_id"})
		return
	}
	if realTaskID != body.Proposal.TaskID {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "proposal.task_id does not match ai_job.task_id",
		})
		return
	}

	// 1. 幂等前置
	if ex, _ := a.SQLite.Adoptions().FindAdoption(body.AIJobID, body.ChangeID); ex != nil {
		c.JSON(http.StatusOK, gin.H{
			"task_id":       ex.TaskID,
			"version_after": ex.VersionAfter,
			"idempotent":    true,
		})
		return
	}

	// 2. tx
	tx, err := a.SQLite.DB().Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "begin tx"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	taskID := body.Proposal.TaskID
	version := body.Version

	// 3. 版本闸门(恰好一次)
	switch body.Proposal.Action {
	case "reschedule":
		if err := a.SQLite.AITasks().UpdateTaskTx(tx, taskID,
			storage.TaskUpdateFields{
				ScheduledStart: body.Proposal.SuggestedStart,
				ScheduledEnd:   body.Proposal.SuggestedEnd,
			}, version); err != nil {
			respondAdoptVersionOrError(c, a, err, body)
			return
		}
	case "drop":
		archived := domain.TaskStatusArchived
		if err := a.SQLite.AITasks().UpdateTaskTx(tx, taskID,
			storage.TaskUpdateFields{Status: &archived}, version); err != nil {
			respondAdoptVersionOrError(c, a, err, body)
			return
		}
	case "modify_subtasks":
		if err := applySubChangesTx(tx, a, body.Proposal, taskID); err != nil {
			respondAdoptVersionOrError(c, a, err, body)
			return
		}
		if err := a.SQLite.AITasks().BumpTaskVersionTx(tx, taskID, version); err != nil {
			respondAdoptVersionOrError(c, a, err, body)
			return
		}
	case "no_op":
		if err := a.SQLite.AITasks().BumpTaskVersionTx(tx, taskID, version); err != nil {
			respondAdoptVersionOrError(c, a, err, body)
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid action"})
		return
	}

	// 4. adoptions
	if _, err := a.SQLite.Adoptions().RecordAdoptionTx(tx, domain.Adoption{
		TaskID:        taskID,
		AIJobID:       body.AIJobID,
		ChangeID:      body.ChangeID,
		VersionBefore: version,
		VersionAfter:  version + 1,
	}); err != nil {
		if errors.Is(err, storage.ErrAdoptionDuplicate) {
			// 并发兜底 — 重查返 200 idempotent
			if ex, ferr := a.SQLite.Adoptions().FindAdoption(body.AIJobID, body.ChangeID); ferr == nil && ex != nil {
				c.JSON(http.StatusOK, gin.H{
					"task_id":       ex.TaskID,
					"version_after": ex.VersionAfter,
					"idempotent":    true,
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "record adoption"})
		return
	}

	// 5. commit
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "commit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id":       taskID,
		"version_after": version + 1,
	})
}

// respondAdoptVersionOrError 把 tx 路径上的错误映射为 HTTP 响应并回滚。
//   - ErrVersionConflict → 409 + 提示 refresh;
//   - 其它 → 500。
func respondAdoptVersionOrError(c *gin.Context, a *app.App, err error, body adoptRequest) {
	if errors.Is(err, storage.ErrVersionConflict) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "version conflict", "task_id": body.Proposal.TaskID})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
}

// applySubChangesTx 按 op 派发子任务变更(delta Δ3);目标非 active → 跳过(非错误)。
//
// op 集:
//   - add:AppendSubtasksTx(必须 NewTitle != nil);EstimatedMinutes 缺省 25
//   - delete:仅 active,DeleteSubtaskTx
//   - modify_title:仅 active,UpdateSubtaskTx(subVer)
//   - mark_done:仅 active,UpdateSubtaskTx(Status:&Done, subVer)
//   - 其它 / nil subtask_id / nil NewTitle:跳过
//
// subVer 由 GetSubtaskStatusTx 在 tx 内取,跨 op 复用一次即可。
func applySubChangesTx(tx *sql.Tx, a *app.App, p domain.ReviewProposal, taskID int64) error {
	for i, sc := range p.SuggestedSubtaskChanges {
		switch sc.Op {
		case "add":
			if sc.NewTitle == nil {
				continue
			}
			if err := a.SQLite.AITasks().AppendSubtasksTx(tx, taskID, []storage.SubtaskInput{
				{Title: *sc.NewTitle, EstimatedMinutes: 25},
			}); err != nil {
				return err
			}
		case "delete":
			if sc.SubtaskID == nil {
				continue
			}
			status, parentID, _, err := a.SQLite.AITasks().GetSubtaskStatusTx(tx, *sc.SubtaskID)
			if err != nil || status != domain.SubtaskStatusActive || parentID != taskID {
				continue // 非 active 或跨 task → 跳过(防误改他 task 的子任务)
			}
			if err := a.SQLite.AITasks().DeleteSubtaskTx(tx, *sc.SubtaskID); err != nil {
				return err
			}
		case "modify_title":
			if sc.SubtaskID == nil || sc.NewTitle == nil {
				continue
			}
			status, parentID, ver, err := a.SQLite.AITasks().GetSubtaskStatusTx(tx, *sc.SubtaskID)
			if err != nil || status != domain.SubtaskStatusActive || parentID != taskID {
				continue
			}
			if err := a.SQLite.AITasks().UpdateSubtaskTx(tx, *sc.SubtaskID,
				storage.SubtaskUpdateFields{Title: sc.NewTitle}, ver); err != nil {
				return err
			}
		case "mark_done":
			if sc.SubtaskID == nil {
				continue
			}
			status, parentID, ver, err := a.SQLite.AITasks().GetSubtaskStatusTx(tx, *sc.SubtaskID)
			if err != nil || status != domain.SubtaskStatusActive || parentID != taskID {
				continue
			}
			done := domain.SubtaskStatusDone
			if err := a.SQLite.AITasks().UpdateSubtaskTx(tx, *sc.SubtaskID,
				storage.SubtaskUpdateFields{Status: &done}, ver); err != nil {
				return err
			}
		default:
			// 未知 op 或被丢弃 —— 跳过(非错误)。
			_ = i
		}
	}
	return nil
}

// jobTaskID 查 ai_jobs 行返回其 task_id。job 不存在 → ErrNotFound。
// handler 用它把 body 里的 ai_job_id 解析为权威 task_id,避免信任 LLM
// proposals JSON 里的 task_id(可能被幻觉或被客户端篡改)。
var errJobNotFound = errors.New("storage: ai_job not found")

func jobTaskID(a *app.App, jobID int64) (int64, error) {
	if a == nil || a.SQLite == nil || a.SQLite.DB() == nil {
		return 0, errJobNotFound
	}
	var t int64
	err := a.SQLite.DB().QueryRow(`SELECT task_id FROM ai_jobs WHERE id = ?;`, jobID).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errJobNotFound
	}
	if err != nil {
		return 0, err
	}
	return t, nil
}

// DailySweepReject POST /api/daily-sweep/reject
//
// 不写 reject 审计(Δ4.4 — YAGNI);仅返回 204。后续想补 reject audit 时,
// 加 adoptions(op='reject', change_id) 行即可,索引同样兜底。
func DailySweepReject(c *gin.Context) {
	var body struct {
		ChangeID string `json:"change_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	if strings.TrimSpace(body.ChangeID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "change_id required"})
		return
	}
	c.Status(http.StatusNoContent)
}

// StartDailySweep 为所有顶层非 archived/rejected 的 task 各入队一个 review job,
// 返回新建 job id。同一 task 已有 queued/running job(ErrJobInFlight)则跳过。
// 只入队、不写 sweep 日志 —— cron 与手动端点各自决定是否 LogSweep。
func StartDailySweep(ctx context.Context, a *app.App) ([]int64, error) {
	_ = ctx
	tasks, err := a.SQLite.AITasks().ListTopLevel("", 100, 0)
	if err != nil {
		return nil, err
	}
	provider, model := readLLMProviderModel(a)
	jobIDs := []int64{}
	for _, t := range tasks {
		id, err := a.SQLite.AITasks().EnqueueJob(t.ID, provider, model, `{}`, domain.AIJobModeReview)
		if err != nil {
			if errors.Is(err, storage.ErrJobInFlight) {
				continue // 已有 in-flight job,跳过
			}
			continue // best-effort:单个 task 入队失败不阻塞其余
		}
		jobIDs = append(jobIDs, id)
	}
	return jobIDs, nil
}

// readLLMProviderModel 从 settings id=1 读 llm_provider / llm_model(带回退)。
func readLLMProviderModel(a *app.App) (provider, model string) {
	provider, model = "openai_compat", "deepseek-chat"
	if a == nil || a.SQLite == nil || a.SQLite.DB() == nil {
		return provider, model
	}
	_ = a.SQLite.DB().QueryRow(
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

// DailySweepStart POST /api/daily-sweep/start
//
// 手动触发:不检查 settings.daily_sweep_enabled(用户显式动作),不检查同日
// 去重(用户主动想再做一次);入队后写 manual 日志。返回 202 + job_ids。
func DailySweepStart(c *gin.Context) {
	a := appFromCtx(c)
	ids, err := StartDailySweep(c.Request.Context(), a)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	today := time.Now().In(time.Local).Format("2006-01-02")
	_ = a.SQLite.DailySweep().LogSweep(today, "manual")
	c.JSON(http.StatusAccepted, gin.H{"job_ids": ids})
}

// dailySweepProposalDTO 是 today 出参里的一条提议:proposal 本体 + 来源 job +
// 当前 task version(供前端 adopt 的乐观锁 token)+ 是否已采纳。
type dailySweepProposalDTO struct {
	AIJobID   int64                 `json:"ai_job_id"`
	Proposal  domain.ReviewProposal `json:"proposal"`
	Version   int64                 `json:"version"`
	IsAdopted bool                  `json:"is_adopted"`
}

// DailySweepToday GET /api/daily-sweep/today
//
// 拉近 1 天内 status='success' 的 review job,解析 response_payload 里的
// proposals;每条附 ai_job_id / 当前 version / is_adopted。
func DailySweepToday(c *gin.Context) {
	a := appFromCtx(c)
	today := time.Now().In(time.Local).Format("2006-01-02")
	log, _ := a.SQLite.DailySweep().GetLog(today)

	rows, err := a.SQLite.DB().Query(
		`SELECT id, task_id, response_payload FROM ai_jobs
		 WHERE mode = 'review' AND status = 'success'
		   AND created_at >= datetime('now','-1 day')
		 ORDER BY created_at DESC;`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "query failed"})
		return
	}
	defer rows.Close()

	out := []dailySweepProposalDTO{}
	for rows.Next() {
		var jobID, taskID int64
		var payload sql.NullString
		if err := rows.Scan(&jobID, &taskID, &payload); err != nil {
			continue
		}
		if !payload.Valid || payload.String == "" {
			continue
		}
		props, _, _ := aigtd.ParseReviewResult(taskID, jobID, payload.String)
		ver, _ := a.SQLite.AITasks().GetTaskVersion(taskID)
		for _, p := range props {
			adopted := false
			if ex, _ := a.SQLite.Adoptions().FindAdoption(jobID, p.ChangeID); ex != nil {
				adopted = true
			}
			out = append(out, dailySweepProposalDTO{
				AIJobID: jobID, Proposal: p, Version: ver, IsAdopted: adopted,
			})
		}
	}

	resp := gin.H{"proposals": out}
	if log != nil {
		resp["swept_at"] = log.SweptAt
		resp["source"] = log.Source
	}
	c.JSON(http.StatusOK, resp)
}
