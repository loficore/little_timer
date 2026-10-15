package aigtd

import (
	"strings"
	"testing"

	"little-timer/internal/domain"
)

func TestReviewPrompt_IncludesTasksAndMarkDone(t *testing.T) {
	p := ReviewPrompt("2026-10-08", []TaskSummary{
		{
			ID: 1, Title: "写报告", Status: "active",
			SubtaskTitles:   []string{"起草", "定稿"},
			SubtaskStatuses: []string{"active", "done"},
		},
	})
	for _, kw := range []string{"写报告", "起草", "定稿", "mark_done", "delete", "modify_title", "change_id", "reschedule", "drop", "modify_subtasks", "no_op"} {
		if !strings.Contains(p, kw) {
			t.Errorf("ReviewPrompt 缺 %q", kw)
		}
	}
}

func TestBuildUserPrompt_IncludesExistingSubtasks(t *testing.T) {
	out := BuildUserPrompt("写周报", []SubtaskHint{
		{Title: "收集数据", Status: "done"},
		{Title: "撰写初稿", Status: "active"},
	}, domain.AIJobModeAppend)
	if !strings.Contains(out, "收集数据") || !strings.Contains(out, "撰写初稿") {
		t.Fatalf("prompt 未包含已有子任务: %s", out)
	}
	if !strings.Contains(out, "done") || !strings.Contains(out, "active") {
		t.Fatalf("prompt 未包含子任务状态: %s", out)
	}
}

func TestBuildUserPrompt_NoExistingOmitsHintBlock(t *testing.T) {
	plain := BuildUserPrompt("写周报", nil, domain.AIJobModeAppend)
	if strings.Contains(plain, "已有子任务") {
		t.Fatalf("无 existing 时不应有已有清单段: %s", plain)
	}
}

// TestBuildUserPrompt_ReplaceOmitsHintsAndAsksFull 锁定 M1 修复:
// replace 模式即使有 existing 也不得带"不要重复"清单(否则 LLM 省略未完成
// 子任务 → worker 删库),而应给出"全量重新分解"指令。
func TestBuildUserPrompt_ReplaceOmitsHintsAndAsksFull(t *testing.T) {
	out := BuildUserPrompt("写周报", []SubtaskHint{
		{Title: "收集数据", Status: "active"},
	}, domain.AIJobModeReplace)
	if strings.Contains(out, "收集数据") {
		t.Fatalf("replace 模式不应包含已有子任务清单: %s", out)
	}
	if strings.Contains(out, "不要重复") || strings.Contains(out, "只输出尚未覆盖") {
		t.Fatalf("replace 模式不应有'只新增'指令: %s", out)
	}
	if !strings.Contains(out, "重新分解") && !strings.Contains(out, "replace") {
		t.Fatalf("replace 模式应给出全量重分解指令: %s", out)
	}
}

func TestSystemPrompt_MentionsIncrementalRule(t *testing.T) {
	sp := SystemPrompt("2026-10-05")
	if !strings.Contains(sp, "新增") {
		t.Fatalf("system prompt 未提示增量语义: %s", sp)
	}
}

// TestSystemPrompt_SubtaskCountAllowsOne 锁定 #9:系统提示词必须允许
// subtasks 个数为 1(append 模式只新增 1 个子任务的边界),不得再写"0 或 2-4"。
func TestSystemPrompt_SubtaskCountAllowsOne(t *testing.T) {
	sp := SystemPrompt("2026-10-05")
	if !strings.Contains(sp, "0 或 1-4") {
		t.Fatalf("system prompt 应允许 subtasks 个数为 0 或 1-4: %s", sp)
	}
	if strings.Contains(sp, "0 或 2-4") {
		t.Fatalf("system prompt 仍禁止 1 个子任务(0 或 2-4): %s", sp)
	}
}

// TestSystemPrompt_MentionsScoring 锁定 task 7 契约:
// SystemPrompt 必须明确告知 LLM 输出 4 维评分字段及其范围 / 枚举(spec §5.3)。
// 缺一个就 fail —— 防止 LLM 输出形态漂移。
func TestSystemPrompt_MentionsScoring(t *testing.T) {
	sp := SystemPrompt("2026-10-05")
	for _, kw := range []string{
		"priority_score",
		"urgency_score",
		"energy_required",
		"context_tag",
	} {
		if !strings.Contains(sp, kw) {
			t.Errorf("system prompt 缺评分字段 %q", kw)
		}
	}
	// 同时应包含分值范围 + context_tag 枚举(spec §5.3)。
	if !strings.Contains(sp, "1-5") {
		t.Errorf("system prompt 应指明 priority_score 范围 1-5")
	}
	if !strings.Contains(sp, "1-10") {
		t.Errorf("system prompt 应指明 urgency_score 范围 1-10")
	}
	if !strings.Contains(sp, "1-3") {
		t.Errorf("system prompt 应指明 energy_required 范围 1-3")
	}
}

// TestRefineReviewPrompt_IncludesAllInputs 验证 RefineReviewPrompt 把
// currentDate / task 摘要 / 原始 proposal / chip_hint / user_message /
// priorRefined 全部喂进 prompt,LLM 拿得到精化所需的所有上下文。
func TestRefineReviewPrompt_IncludesAllInputs(t *testing.T) {
	ts := TaskSummary{
		ID: 42, Title: "周报", Status: "active",
		SubtaskTitles:   []string{"草稿", "送审"},
		SubtaskStatuses: []string{"active", "done"},
	}
	end := int64(1735689600)
	prop := domain.ReviewProposal{
		ChangeID: "prop-1", TaskID: 42, Action: "reschedule",
		SuggestedEnd: &end, Reason: "AI 原建议",
	}
	prior := []string{"prev-1", "prev-2"}
	out := RefineReviewPrompt("2026-10-15", ts, prop, "改期", "推迟到下周五", prior)

	for _, kw := range []string{
		"2026-10-15", // currentDate
		"周报",         // task title
		"草稿", "送审",   // subtask titles
		"prop-1",                                         // original change_id
		"改期",                                             // chip_hint
		"推迟到下周五",                                         // user_message
		"reschedule", "drop", "modify_subtasks", "no_op", // action 枚举
		"delete", "modify_title", "add", "mark_done", // op 枚举
		"prev-1", "prev-2", // priorRefined
	} {
		if !strings.Contains(out, kw) {
			t.Errorf("RefineReviewPrompt 缺 %q\n--- 输出 ---\n%s", kw, out)
		}
	}
}
