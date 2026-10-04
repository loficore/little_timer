package domain

import "testing"

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
