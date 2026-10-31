package webview

import (
	"bytes"
	"testing"
)

func TestWriteProfLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "[CAD] idle avgMs=32.0", want: "[CAD] idle avgMs=32.0\n"},
		{name: "trailing newlines stripped", in: "[PERF] x\n\n", want: "[PERF] x\n"},
		{name: "embedded newline kept", in: "[CAD] a\n[PERF] b", want: "[CAD] a\n[PERF] b\n"},
		{name: "empty", in: "", want: ""},
		{name: "whitespace only", in: "  \n", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeProfLine(&buf, tc.in); err != nil {
				t.Fatalf("writeProfLine: %v", err)
			}
			if buf.String() != tc.want {
				t.Fatalf("got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}

func TestProfBindingNameIsStable(t *testing.T) {
	// cadence.js 以字面量调用 window.ltProfLog;改名必须两侧同步,这里钉住一侧。
	if profBindingName != "ltProfLog" {
		t.Fatalf("profBindingName = %q, cadence.js 期望 %q", profBindingName, "ltProfLog")
	}
}
