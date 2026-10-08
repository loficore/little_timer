// Package handlers —— /api/daily-sweep/*:GTD Review 的触发与提议查询。
//
//   - StartDailySweep 是被 cron 与手动端点共享的入队逻辑(只入队,不写日志);
//   - DailySweepStart 手动触发:入队 + 写 manual 日志(不查 enabled、不查同日去重);
//   - DailySweepToday 拉当日(近 1 天)成功 review job 的 proposals,附 ai_job_id
//     与当前 task version 供前端 adopt。
//
// adopt / reject 在 Task 6 的 daily_sweep_adopt.go 中实现。
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/storage"
)

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
