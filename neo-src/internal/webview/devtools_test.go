package webview

import (
	"os"
	"testing"
)

func TestDevtoolsEnabled(t *testing.T) {
	cases := []struct {
		name  string
		set   bool
		value string
		want  bool
	}{
		{name: "unset", set: false, want: false},
		{name: "empty", set: true, value: "", want: false},
		{name: "zero", set: true, value: "0", want: false},
		{name: "off", set: true, value: "off", want: false},
		{name: "yes", set: true, value: "yes", want: false},
		{name: "one", set: true, value: "1", want: true},
		{name: "true", set: true, value: "true", want: true},
		{name: "TRUE", set: true, value: "TRUE", want: true},
		{name: "padded on", set: true, value: " on ", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// t.Setenv registers restoration; Unsetenv after it yields the unset case.
			t.Setenv(devtoolsEnvVar, tc.value)
			if !tc.set {
				if err := os.Unsetenv(devtoolsEnvVar); err != nil {
					t.Fatalf("unset %s: %v", devtoolsEnvVar, err)
				}
			}
			if got := devtoolsEnabled(); got != tc.want {
				t.Fatalf("devtoolsEnabled() = %v, want %v (LT_WEBKIT_DEBUG=%q set=%v)", got, tc.want, tc.value, tc.set)
			}
		})
	}
}
