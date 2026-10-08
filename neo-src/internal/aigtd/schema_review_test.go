package aigtd_test

import (
	"strings"
	"testing"

	"little-timer/internal/aigtd"
)

// TestValidateReviewResult_OK 验证合法 proposals 全部通过,无 warning。
// 覆盖 op 集 {delete, modify_title, add, mark_done} 与 action=modify_subtasks。
func TestValidateReviewResult_OK(t *testing.T) {
	raw := `{"proposals":[
      {"change_id":"11111111-1111-4111-8111-111111111111","task_id":1,"action":"modify_subtasks",
       "suggested_subtask_changes":[
         {"subtask_id":7,"op":"delete","reason":"stale"},
         {"subtask_id":8,"op":"mark_done","reason":"已完成"},
         {"subtask_id":9,"op":"modify_title","new_title":"改名","reason":"更清晰"},
         {"op":"add","new_title":"补写摘要"}],
       "reason":"review"}]}`
	props, warns, err := aigtd.ValidateReviewResult(1, 42, raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(props) != 1 {
		t.Fatalf("props=%d, want 1", len(props))
	}
	if len(warns) != 0 {
		t.Errorf("warns=%v, want empty", warns)
	}
	if got := props[0].SuggestedSubtaskChanges; len(got) != 4 {
		t.Errorf("sub_changes=%d, want 4", len(got))
	}
}

// TestValidateReviewResult_BadActionAndOp_Flagged 验证非法 action 与非法 op
// 都进入 warning(不丢弃 proposal,交给 handler/worker 决定)。
func TestValidateReviewResult_BadActionAndOp_Flagged(t *testing.T) {
	raw := `{"proposals":[
      {"change_id":"x","task_id":1,"action":"BOGUS","reason":""},
      {"change_id":"y","task_id":1,"action":"modify_subtasks",
       "suggested_subtask_changes":[{"subtask_id":1,"op":"reorder"}],"reason":""}]}`
	_, warns, err := aigtd.ValidateReviewResult(1, 42, raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	joined := strings.Join(warns, "|")
	if !strings.Contains(joined, "action") {
		t.Errorf("warns 缺 action: %v", warns)
	}
	if !strings.Contains(joined, "op") {
		t.Errorf("warns 缺 op: %v", warns)
	}
}

// TestValidateReviewResult_ChangeIDFallback 验证缺失/重复的 change_id 被
// 重写为 "taskID-jobID-index" 兜底(Δ2),并产生 warning。
func TestValidateReviewResult_ChangeIDFallback(t *testing.T) {
	raw := `{"proposals":[
      {"task_id":1,"action":"no_op","reason":""},
      {"task_id":1,"action":"no_op","reason":""}]}`
	props, warns, err := aigtd.ValidateReviewResult(1, 42, raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(props) != 2 {
		t.Fatalf("props=%d, want 2", len(props))
	}
	if props[0].ChangeID != "1-42-0" || props[1].ChangeID != "1-42-1" {
		t.Errorf("change_id = %q,%q, want 1-42-0,1-42-1", props[0].ChangeID, props[1].ChangeID)
	}
	if len(warns) == 0 {
		t.Error("应产生 change_id 兜底 warning")
	}
}

// TestValidateReviewResult_DuplicateChangeID_Rewritten 验证同结果内重复的
// LLM change_id 也被重写(仅第一条保留)。
func TestValidateReviewResult_DuplicateChangeID_Rewritten(t *testing.T) {
	raw := `{"proposals":[
      {"change_id":"dup","task_id":1,"action":"no_op","reason":""},
      {"change_id":"dup","task_id":1,"action":"no_op","reason":""}]}`
	props, warns, err := aigtd.ValidateReviewResult(7, 9, raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if props[0].ChangeID != "dup" || props[1].ChangeID != "7-9-1" {
		t.Errorf("change_id = %q,%q, want dup,7-9-1", props[0].ChangeID, props[1].ChangeID)
	}
	if len(warns) == 0 {
		t.Error("应产生重复 change_id warning")
	}
}

// TestValidateReviewResult_InvalidJSON_Error 验证整体 JSON 不可解析时返回 error。
func TestValidateReviewResult_InvalidJSON_Error(t *testing.T) {
	if _, _, err := aigtd.ValidateReviewResult(1, 1, `{not json`); err == nil {
		t.Fatal("非法 JSON 应返回 error")
	}
}
