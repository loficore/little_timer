//go:build embed_ui && !android

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx "little-timer/internal/http"
	httpapp "little-timer/internal/http/app"
)

// 这个测试是打包门禁的一部分：没有先 stage 前端就直接 `go test -tags embed_ui`
// 会在这里失败，避免把占位页打进发布产物。
func TestEmbeddedUIIsServed(t *testing.T) {
	ui := embeddedUIFS()
	if ui == nil {
		t.Fatal("embeddedUIFS() = nil, want embedded assets")
	}

	app := httpapp.NewApp(nil, nil, nil, nil, "")
	srv := httptest.NewServer(httpx.NewRouterWithUI(app, "*", ui))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "<title>Little Timer</title>") {
		t.Fatalf("GET / did not return the built frontend (body starts %q)", truncate(string(body), 120))
	}

	resp, err = http.Get(srv.URL + "/wails/runtime.js")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "export const Call") {
		t.Fatalf("GET /wails/runtime.js = %d %q", resp.StatusCode, truncate(string(body), 120))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
