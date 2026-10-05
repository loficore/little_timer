package aigtd

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt_IncludesExistingSubtasks(t *testing.T) {
	out := BuildUserPrompt("写周报", []SubtaskHint{
		{Title: "收集数据", Status: "done"},
		{Title: "撰写初稿", Status: "active"},
	})
	if !strings.Contains(out, "收集数据") || !strings.Contains(out, "撰写初稿") {
		t.Fatalf("prompt 未包含已有子任务: %s", out)
	}
	if !strings.Contains(out, "done") || !strings.Contains(out, "active") {
		t.Fatalf("prompt 未包含子任务状态: %s", out)
	}
}

func TestBuildUserPrompt_NoExistingOmitsHintBlock(t *testing.T) {
	plain := BuildUserPrompt("写周报", nil)
	if strings.Contains(plain, "已有子任务") {
		t.Fatalf("无 existing 时不应有已有清单段: %s", plain)
	}
}

func TestSystemPrompt_MentionsIncrementalRule(t *testing.T) {
	sp := SystemPrompt("2026-10-05")
	if !strings.Contains(sp, "新增") {
		t.Fatalf("system prompt 未提示增量语义: %s", sp)
	}
}