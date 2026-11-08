// Package handlers —— /api/daily-sweep/*:GTD Review 的触发与提议查询。
//
// 本文件只做 transport 绑定与 sweep.Error → 状态码映射;幂等 token、事务
// 边界、版本闸门全在 internal/sweep(issue #66)。
//
//   - DailySweepStart 手动触发:入队 + 写 manual 日志(不查 enabled、不查同日去重);
//   - DailySweepToday 拉当日(近 1 天)成功 review job 的 proposals,附 ai_job_id
//     与当前 task version 供前端 adopt;
//   - DailySweepAdopt / DailySweepReject 用户逐条采纳 / 拒绝。
package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/sweep"
)

// adoptRequest 是 POST /api/daily-sweep/adopt 的入参。
type adoptRequest struct {
	ChangeID string                `json:"change_id"`
	AIJobID  int64                 `json:"ai_job_id"`
	Version  int64                 `json:"version"`
	Proposal domain.ReviewProposal `json:"proposal"`
}

// sweepOpsFor 构造 daily sweep 用例层。config 回调每次入队时读 settings,
// 使设置页改动无需重建。
func sweepOpsFor(a *app.App) *sweep.Operations {
	return sweep.NewForSqlite(a.SQLite, func() (provider, model string) {
		return sweep.ReadLLMProviderModel(a.SQLite.DB())
	})
}

// DailySweepAdopt POST /api/daily-sweep/adopt
//
// 幂等前置 → 事务 → 版本闸门(恰好一次)→ 审计行 → commit;
// 版本冲突 → 409;sweep.Error 之外的错误 → 500。
func DailySweepAdopt(c *gin.Context) {
	a := appFromCtx(c)
	var body adoptRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	res, err := sweepOpsFor(a).Adopt(sweep.AdoptInput{
		AIJobID:  body.AIJobID,
		ChangeID: body.ChangeID,
		Version:  body.Version,
		Proposal: body.Proposal,
	})
	if err != nil {
		respondSweepAdoptError(c, err, body)
		return
	}
	if res.Idempotent {
		c.JSON(http.StatusOK, gin.H{
			"task_id":       res.TaskID,
			"version_after": res.VersionAfter,
			"idempotent":    true,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id":       res.TaskID,
		"version_after": res.VersionAfter,
	})
}

// respondSweepAdoptError 是 adopt 路径 sweep.Error → HTTP 状态码的唯一映射点。
func respondSweepAdoptError(c *gin.Context, err error, body adoptRequest) {
	var de *sweep.Error
	if errors.As(err, &de) {
		switch {
		case errors.Is(err, sweep.ErrVersionConflict):
			// 历史契约:adopt 的 409 不带 code,带 success / task_id。
			respondVersionConflict(c, "", false, gin.H{
				"success": false, "task_id": body.Proposal.TaskID,
			})
			return
		case errors.Is(err, sweep.ErrInvalidProposal):
			// 区分历史文案:proposal 本身不合法 vs ai_job 不存在/不匹配。
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": invalidProposalMessage(body)})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
}

// invalidProposalMessage 保持既有 400 文案(handler 测试与前端都依赖它)。
func invalidProposalMessage(body adoptRequest) string {
	if body.Proposal.TaskID == 0 {
		return "invalid proposal"
	}
	return "proposal.task_id does not match ai_job.task_id"
}

// DailySweepReject POST /api/daily-sweep/reject
//
// 不写 reject 审计(Δ4.4 — YAGNI);仅返回 204。
func DailySweepReject(c *gin.Context) {
	var body struct {
		ChangeID string `json:"change_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	if err := sweepOpsFor(appFromCtx(c)).Reject(body.ChangeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "change_id required"})
		return
	}
	c.Status(http.StatusNoContent)
}

// StartDailySweep 为所有顶层非 archived/rejected 的 task 各入队一个 review job,
// 返回新建 job id。同一 task 已有 queued/running job 则跳过。
//
// 只入队、不写 sweep 日志 —— cron 与手动端点各自决定是否 LogSweep。
// cmd/server 把它注入 cron 作为 SweepStarter。
func StartDailySweep(ctx context.Context, a *app.App) ([]int64, error) {
	return sweepOpsFor(a).Start(ctx)
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
	res, err := sweepOpsFor(a).Today(today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "query failed"})
		return
	}
	out := make([]dailySweepProposalDTO, 0, len(res.Proposals))
	for _, p := range res.Proposals {
		out = append(out, dailySweepProposalDTO{
			AIJobID: p.AIJobID, Proposal: p.Proposal, Version: p.Version, IsAdopted: p.IsAdopted,
		})
	}
	resp := gin.H{"proposals": out}
	if res.Log != nil {
		resp["swept_at"] = res.Log.SweptAt
		resp["source"] = res.Log.Source
	}
	c.JSON(http.StatusOK, resp)
}

// DailySweepStart POST /api/daily-sweep/start
//
// 手动触发:不检查 settings.daily_sweep_enabled(用户显式动作),不检查同日
// 去重(用户主动想再做一次);入队后写 manual 日志。返回 202 + job_ids。
func DailySweepStart(c *gin.Context) {
	a := appFromCtx(c)
	ops := sweepOpsFor(a)
	ids, err := ops.Start(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	today := time.Now().In(time.Local).Format("2006-01-02")
	_ = ops.Log(today, "manual")
	c.JSON(http.StatusAccepted, gin.H{"job_ids": ids})
}
