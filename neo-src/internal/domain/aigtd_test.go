package domain

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTaskStatus_Valid(t *testing.T) {
	cases := []struct {
		s  string
		ok bool
	}{
		{"inbox", true},
		{"active", true},
		{"done", true},
		{"archived", true},
		{"rejected", true},
		{"bogus", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsValidTaskStatus(c.s); got != c.ok {
			t.Errorf("IsValidTaskStatus(%q) = %v, want %v", c.s, got, c.ok)
		}
	}
}

// TestTaskRow_JSONRoundTrip_AllScoreFields 验证 v11 新增字段
// (Version / PriorityScore / UrgencyScore / EnergyRequired / ContextTag /
// BlockedBy)参与 JSON 序列化与反序列化,且 BlockedBy 切片能完整往返。
func TestTaskRow_JSONRoundTrip_AllScoreFields(t *testing.T) {
	tr := TaskRow{
		ID:             1,
		Title:          "x",
		Version:        7,
		PriorityScore:  4,
		UrgencyScore:   8,
		EnergyRequired: 3,
		ContextTag:     "编码",
		BlockedBy:      []int64{2, 3},
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var back TaskRow
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Version != 7 || back.PriorityScore != 4 || back.UrgencyScore != 8 ||
		back.EnergyRequired != 3 || back.ContextTag != "编码" ||
		!reflect.DeepEqual(back.BlockedBy, []int64{2, 3}) {
		t.Errorf("round-trip 丢失字段: %+v", back)
	}
}
