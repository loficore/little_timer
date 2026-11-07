package tasks

import (
	"errors"
	"testing"
)

// TestPatchScores_RangeValidation：越界评分 → typed 错误,携带字段与区间。
func TestPatchScores_RangeValidation(t *testing.T) {
	cases := []struct {
		field string
		value int
		min   int
		max   int
	}{
		{"priority_score", 0, 1, 5},
		{"priority_score", 6, 1, 5},
		{"urgency_score", 0, 1, 10},
		{"urgency_score", 11, 1, 10},
		{"energy_required", 0, 1, 3},
		{"energy_required", 4, 1, 3},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			ops := New(newStore(t))
			id := mustCreate(t, ops, "t")

			v := tc.value
			patch := ScoresPatch{}
			switch tc.field {
			case "priority_score":
				patch.PriorityScore = &v
			case "urgency_score":
				patch.UrgencyScore = &v
			case "energy_required":
				patch.EnergyRequired = &v
			}
			_, _, err := ops.PatchScores(id, patch, getRow(t, ops, id).Version)
			var oe *ScoreRangeError
			if !errors.As(err, &oe) {
				t.Fatalf("err=%v, want ScoreRangeError", err)
			}
			if oe.Field != tc.field || oe.Value != tc.value || oe.Min != tc.min || oe.Max != tc.max {
				t.Errorf("got %+v, want %s=%d in [%d,%d]", oe, tc.field, tc.value, tc.min, tc.max)
			}
			if oe.Code() != "scores_out_of_range" {
				t.Errorf("code=%q, want scores_out_of_range", oe.Code())
			}
		})
	}
}

// TestPatchScores_ContextTagFallback：非枚举 context_tag → general + warning。
func TestPatchScores_ContextTagFallback(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "t")

	tag := "写代码"
	row, warning, err := ops.PatchScores(id, ScoresPatch{ContextTag: &tag}, getRow(t, ops, id).Version)
	if err != nil {
		t.Fatalf("patch scores: %v", err)
	}
	if row.ContextTag != "general" {
		t.Errorf("context_tag=%q, want general", row.ContextTag)
	}
	if warning == "" {
		t.Error("非枚举 context_tag 应产生 warning")
	}

	// 合法值不该有 warning。
	ok := "编码"
	row, warning, err = ops.PatchScores(id, ScoresPatch{ContextTag: &ok}, row.Version)
	if err != nil {
		t.Fatalf("patch scores: %v", err)
	}
	if row.ContextTag != "编码" || warning != "" {
		t.Errorf("context_tag=%q warning=%q, want 编码 / 空", row.ContextTag, warning)
	}
}

// TestPatchScores_VersionConflict：stale version → ErrVersionConflict。
func TestPatchScores_VersionConflict(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "t")

	p := 4
	_, _, err := ops.PatchScores(id, ScoresPatch{PriorityScore: &p}, 99)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}
}

// TestPatchScores_PartialFields：只改传入的字段。
func TestPatchScores_PartialFields(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "t")

	p := 5
	row, _, err := ops.PatchScores(id, ScoresPatch{PriorityScore: &p}, getRow(t, ops, id).Version)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	// urgency_score 保持 DB 默认 5(未传)。
	if row.PriorityScore != 5 || row.UrgencyScore != 5 {
		t.Errorf("scores=%d/%d, want 5/5", row.PriorityScore, row.UrgencyScore)
	}
}
