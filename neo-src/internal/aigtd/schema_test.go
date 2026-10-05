package aigtd_test

import (
	"strings"
	"testing"

	"little-timer/internal/aigtd"
)

// TestValidateDecomposition_ScoresBounds 锁定 task 7 契约:
// ValidateDecomposition 必须对 LLM 输出的 4 维评分做范围 / 枚举校验,
// 越界 / 非枚举值 → 落默认值并向 errs 追加警告(Review Focus #3/#5);
// 完全缺失则视为缺省,静默默认(spec §12:LLM 评分失败 → 默认值 + 警告)。
//
// 同时覆盖 spec §5.3 的嵌套 `_scores` 输出形态 + 兼容顶层键的旧形态。
func TestValidateDecomposition_ScoresBounds(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		wantPriority int
		wantUrgency  int
		wantEnergy   int
		wantContext  string
		wantWarn     string // 子串匹配 errs;空=不期望警告
	}{
		{
			name:         "priority=0 越界 → 默认 + 警告",
			raw:          `{"title":"x","priority_score":0,"urgency_score":5,"energy_required":2,"context_tag":"general","subtasks":[],"is_complex":false,"estimated_minutes":30}`,
			wantPriority: 3, wantUrgency: 5, wantEnergy: 2, wantContext: "general",
			wantWarn: "priority_score",
		},
		{
			name:         "urgency=15 越界 → 默认 + 警告",
			raw:          `{"title":"x","priority_score":4,"urgency_score":15,"energy_required":2,"context_tag":"general","subtasks":[],"is_complex":false,"estimated_minutes":30}`,
			wantPriority: 4, wantUrgency: 5, wantEnergy: 2, wantContext: "general",
			wantWarn: "urgency_score",
		},
		{
			name:         "energy=4 越界 → 默认 + 警告",
			raw:          `{"title":"x","priority_score":4,"urgency_score":5,"energy_required":4,"context_tag":"general","subtasks":[],"is_complex":false,"estimated_minutes":30}`,
			wantPriority: 4, wantUrgency: 5, wantEnergy: 2, wantContext: "general",
			wantWarn: "energy_required",
		},
		{
			name:         "context_tag 非枚举 → general + 警告",
			raw:          `{"title":"x","priority_score":4,"urgency_score":5,"energy_required":2,"context_tag":"写代码","subtasks":[],"is_complex":false,"estimated_minutes":30}`,
			wantPriority: 4, wantUrgency: 5, wantEnergy: 2, wantContext: "general",
			wantWarn: "context_tag",
		},
		{
			name:         "全部合法值 → 原样通过,无警告",
			raw:          `{"title":"x","priority_score":5,"urgency_score":10,"energy_required":3,"context_tag":"编码","subtasks":[],"is_complex":false,"estimated_minutes":30}`,
			wantPriority: 5, wantUrgency: 10, wantEnergy: 3, wantContext: "编码",
			wantWarn: "",
		},
		{
			name:         "_scores 完全缺失 → 全部默认 + 无警告",
			raw:          `{"title":"x","subtasks":[],"is_complex":false,"estimated_minutes":30}`,
			wantPriority: 3, wantUrgency: 5, wantEnergy: 2, wantContext: "general",
			wantWarn: "",
		},
		{
			name:         "嵌套 _scores 形态 + 越界 → 默认 + 警告",
			raw:          `{"title":"x","is_complex":false,"estimated_minutes":30,"subtasks":[],"_scores":{"priority_score":99,"urgency_score":5,"energy_required":2,"context_tag":"general"}}`,
			wantPriority: 3, wantUrgency: 5, wantEnergy: 2, wantContext: "general",
			wantWarn: "priority_score",
		},
		{
			name:         "嵌套 _scores 全部合法 → 原样通过",
			raw:          `{"title":"x","is_complex":false,"estimated_minutes":30,"subtasks":[],"_scores":{"priority_score":2,"urgency_score":7,"energy_required":1,"context_tag":"写作"}}`,
			wantPriority: 2, wantUrgency: 7, wantEnergy: 1, wantContext: "写作",
			wantWarn: "",
		},
		{
			name:         "嵌套 _scores 优先级高于顶层,缺则回顶层",
			raw:          `{"title":"x","is_complex":false,"estimated_minutes":30,"subtasks":[],"priority_score":1,"urgency_score":2,"_scores":{"priority_score":4,"urgency_score":9,"energy_required":3,"context_tag":"学习"}}`,
			wantPriority: 4, wantUrgency: 9, wantEnergy: 3, wantContext: "学习",
			wantWarn: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, errs, err := aigtd.ValidateDecomposition(c.raw)
			if err != nil {
				t.Fatalf("ValidateDecomposition 解析失败: %v", err)
			}
			if res == nil {
				t.Fatal("ValidateDecomposition 返回 nil res")
			}
			if res.PriorityScore != c.wantPriority {
				t.Errorf("priority_score = %d, want %d", res.PriorityScore, c.wantPriority)
			}
			if res.UrgencyScore != c.wantUrgency {
				t.Errorf("urgency_score = %d, want %d", res.UrgencyScore, c.wantUrgency)
			}
			if res.EnergyRequired != c.wantEnergy {
				t.Errorf("energy_required = %d, want %d", res.EnergyRequired, c.wantEnergy)
			}
			if res.ContextTag != c.wantContext {
				t.Errorf("context_tag = %q, want %q", res.ContextTag, c.wantContext)
			}
			if c.wantWarn == "" {
				// 不应出现 _scores 相关警告(spec §12:缺省静默)。
				for _, e := range errs {
					if strings.Contains(e, "priority_score") ||
						strings.Contains(e, "urgency_score") ||
						strings.Contains(e, "energy_required") ||
						strings.Contains(e, "context_tag") {
						t.Errorf("不应出现评分警告: %v", errs)
					}
				}
				return
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e, c.wantWarn) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("errs 中缺含 %q 的警告,实际 errs = %v", c.wantWarn, errs)
			}
		})
	}
}
