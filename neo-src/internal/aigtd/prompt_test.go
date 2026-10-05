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
