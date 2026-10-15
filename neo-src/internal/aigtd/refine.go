package aigtd

import (
	"database/sql"
	"encoding/json"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// ParseRefineRequestPayload 解析 review_refine 模式 ai_jobs.request_payload。
//
// 调用方:worker.process 的 review_refine 分支。
// 失败:返 error,worker 走 MarkJobFailed + SetAIStatus(error) 收尾。
func ParseRefineRequestPayload(payload string) (domain.RefineJobRequestPayload, error) {
	var p domain.RefineJobRequestPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return p, err
	}
	return p, nil
}

// FindOriginalReviewProposal 在该 task 最近一次 success 的 review job 的
// response_payload 中找 changeID 对应的原始 ReviewProposal。
//
// 返回:
//   - (prop, nil): 找到
//   - (nil, nil):  没找到匹配的 review job 或 proposal(上层按业务错误处理,
//     例如 "原始 proposal 已丢失" 提示给用户)
//   - (nil, err):  DB / parse 错误
//
// 调用方:worker.process 的 review_refine 分支(把原始 proposal 喂进
// RefineReviewPrompt,让 LLM 知道用户对哪条建议不满意)。
func FindOriginalReviewProposal(
	aiTasks *storage.AITasksCrud,
	taskID int64,
	changeID string,
) (*domain.ReviewProposal, error) {
	job, err := aiTasks.LatestSuccessfulReviewJob(taskID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	proposals, _, err := ParseReviewResult(taskID, job.ID, job.ResponsePayload)
	if err != nil {
		return nil, err
	}
	for i := range proposals {
		if proposals[i].ChangeID == changeID {
			return &proposals[i], nil
		}
	}
	return nil, nil
}
