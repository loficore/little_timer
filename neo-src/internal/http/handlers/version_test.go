package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/app"
)

// TestVersionGet_ReportsBuildInfo 验证 /api/version 把链接期打戳的事实来源
// 原样透给前端（UI 的版本号只能来自这里，不能自己写死）。
func TestVersionGet_ReportsBuildInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/version", VersionGet)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/version", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var got struct {
		Version   string `json:"version"`
		GitCommit string `json:"git_commit"`
		BuildTime string `json:"build_time"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, w.Body.String())
	}

	if got.Version != app.Version {
		t.Errorf("version: got %q, want %q", got.Version, app.Version)
	}
	if got.GitCommit == "" {
		t.Error("git_commit 不应为空")
	}
	if got.BuildTime == "" {
		t.Error("build_time 不应为空")
	}
}
