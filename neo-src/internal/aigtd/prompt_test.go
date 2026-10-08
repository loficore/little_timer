package aigtd

import (
	"strings"
	"testing"

	"little-timer/internal/domain"
)

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
