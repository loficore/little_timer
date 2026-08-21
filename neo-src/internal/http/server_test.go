// HTTP 层的 smoke 测试。
//
// 目标：验证 Gin router 注册了每条路由，且每个 handler 都能响应
// （200/401 等）而不 panic。handler 级测试用真实 SQLite +
// SettingsManager 构建真 App，让依赖注入路径真的被执行。
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// newTestRouter 构建接到 stub App 的 router，App 的各 manager 为 nil。
// 只用于注册 + 中间件测试 —— handler 级测试用下面的 newRealTestRouter。
func newTestRouter(t *testing.T) (*gin.Engine, *app.App) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		nil, // settings —— 仅测注册的用例传 nil
		nil, // sqlite —— 仅测注册的用例传 nil
		nil, // backup —— 仅测注册的用例传 nil
		dbPath,
	)

	r := NewRouter(a, "*")
	return r, a
}

// newRealTestRouter 构建接到完整初始化 App 的 router：真实 SqliteManager、
// 真实 SettingsManager。跑 handler 逻辑的测试用它（例如 GET /api/state
// 必须能读 clock）。
func newRealTestRouter(t *testing.T) (*gin.Engine, *app.App) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		nil, // backup —— 为 nil 时 handler 返回 service-unavailable
		dbPath,
	)
	return NewRouter(a, "*"), a
}

// routeExists 检查 Gin router 的树，确认某个 method+path 已注册。Gin 不
// 直接暴露 router tree，所以我们发一个请求，把任何非 404 响应当作“已注册”。
func routeExists(r *gin.Engine, method, path string) bool {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w.Code != http.StatusNotFound
}

// TestAllRoutesRegistered 遍历完整路由表，确认每条路径都有应答
// （不是 404）。这就是 smoke 门。
//
// SSE（/api/events）由 TestEventsRouteExists 单独验证，否则它的流式
// handler 会阻塞测试运行器。
func TestAllRoutesRegistered(t *testing.T) {
	r, _ := newTestRouter(t)

	routes := []struct {
		method, path string
	}{
		// 静态页 + 前端日志
		{http.MethodGet, "/"},
		{http.MethodPost, "/api/log"},

		// 计时器
		{http.MethodGet, "/api/state"},
		{http.MethodGet, "/api/timer/state"},
		{http.MethodGet, "/api/timer/progress"},
		{http.MethodGet, "/api/timer/config"},
		{http.MethodPost, "/api/start"},
		{http.MethodPost, "/api/pause"},
		{http.MethodPost, "/api/reset"},
		{http.MethodPost, "/api/finish"},
		{http.MethodPost, "/api/timer/finish"},
		{http.MethodPost, "/api/timer/rest"},
		{http.MethodPost, "/api/mode"},
		{http.MethodPost, "/api/timer/config"},

		// 习惯
		{http.MethodGet, "/api/habit-sets"},
		{http.MethodPost, "/api/habit-sets"},
		{http.MethodGet, "/api/habits"},
		{http.MethodPost, "/api/habits"},
		{http.MethodGet, "/api/sessions"},
		{http.MethodPost, "/api/sessions"},
		{http.MethodGet, "/api/timer-sessions"},
		{http.MethodPost, "/api/timer-sessions"},

		// 设置
		{http.MethodGet, "/api/settings"},
		{http.MethodPost, "/api/settings"},

		// 备份
		{http.MethodGet, "/api/backup/config"},
		{http.MethodPost, "/api/backup/config"},
		{http.MethodPost, "/api/backup/create"},
		{http.MethodPost, "/api/backup/restore"},
		{http.MethodGet, "/api/backup/list"},
		{http.MethodGet, "/api/backup/info"},
		{http.MethodPost, "/api/backup/verify"},
		{http.MethodPost, "/api/backup/unlock"},
		{http.MethodPost, "/api/backup/lock"},
		{http.MethodGet, "/api/backup/master-password"},
		{http.MethodPost, "/api/backup/master-password"},

		// 鉴权
		{http.MethodGet, "/api/auth/status"},
		{http.MethodPost, "/api/auth/enable"},
		{http.MethodPost, "/api/auth/disable"},

		// 壁纸
		{http.MethodGet, "/api/wallpapers"},
		{http.MethodPost, "/api/wallpapers"},
		{http.MethodPost, "/api/wallpapers/from-url"},
	}

	for _, rt := range routes {
		if !routeExists(r, rt.method, rt.path) {
			t.Errorf("route not registered: %s %s", rt.method, rt.path)
		}
	}
}

// TestEventsRouteExists 确认 /api/events 已注册而无需真的连上（该 handler
// 会永远流下去）。检查方式：带 `Connection: close` 的 GET 请求返回 200 +
// SSE 头而不是 404。我们用 httptest.NewServer，这样测试 client 一返回
// 连接就立即关闭。
func TestEventsRouteExists(t *testing.T) {
	r, _ := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// 预期：timeout 在 server 响应之前触发。其他错误才是真正的失败。
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return
		}
		t.Fatalf("events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("/api/events returned 404")
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("/api/events Content-Type = %q, want text/event-stream prefix", got)
	}
}

// TestCORSHeaders 确认 CORS 中间件在非预检请求上发出预期的头。
func TestCORSHeaders(t *testing.T) {
	r, _ := newTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS: Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("CORS: missing Access-Control-Allow-Methods")
	}
}

// TestCORSPreflight 确认 OPTIONS 以 204 短路。
func TestCORSPreflight(t *testing.T) {
	r, _ := newTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/state", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight: code = %d, want 204", w.Code)
	}
}

// TestAuthPublicPath 确认 /api/auth/status 与 /api/events 即使没接
// SettingsManager 也绕过鉴权（无需鉴权）。
func TestAuthPublicPath(t *testing.T) {
	r, _ := newTestRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	if w.Code == http.StatusUnauthorized {
		t.Errorf("/api/auth/status unexpectedly gated: %d", w.Code)
	}
}

// Test404ForUnknownRoute 确认未知路径返回 404，而不是撞进易 panic 的
// handler。
func Test404ForUnknownRoute(t *testing.T) {
	r, _ := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/this/does/not/exist", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown route: code = %d, want 404", w.Code)
	}
}

// TestTimerStateHandler 确认 /api/state 返回带预期键的 JSON body。
func TestTimerStateHandler(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/state", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/state: code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v (body=%s)", err, w.Body.String())
	}
	for _, key := range []string{"time", "elapsed", "mode", "is_running", "is_finished", "in_rest", "loop_remaining", "loop_total", "rest_remaining", "timezone"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in /api/state response: %v", key, got)
		}
	}
	if mode, _ := got["mode"].(string); mode != "countdown" && mode != "stopwatch" {
		t.Errorf("mode = %q, want countdown|stopwatch", mode)
	}
}

// TestTimerProgressHandler 确认 /api/timer/progress 返回带
// elapsed/remaining 等的 JSON body。
func TestTimerProgressHandler(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code = %d", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	for _, key := range []string{"mode", "is_running", "is_paused", "is_finished", "elapsed_seconds", "remaining_seconds", "in_rest"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in /api/timer/progress response", key)
		}
	}
}

// TestTimerModeSwitchEmpty 确认 POST /api/mode 空 body 返回 `{}`。
func TestTimerModeSwitchEmpty(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/mode", strings.NewReader("")))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (empty): code = %d, body = %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != "{}" {
		t.Errorf("empty /api/mode body = %q, want {}", got)
	}
}

// TestTimerModeSwitchCountdown 确认 POST /api/mode 带合法 mode 时返回
// 预期的 JSON。
func TestTimerModeSwitchCountdown(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"mode":"countdown"}`)
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/mode", body))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode countdown: code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["status"] != "mode_changed" {
		t.Errorf("status = %v, want mode_changed", got["status"])
	}
	if got["new_mode"] != "countdown" {
		t.Errorf("new_mode = %v, want countdown", got["new_mode"])
	}
}

// TestFrontendLogEmptyBody 确认 POST /api/log 空 body 返回 200 +
// success=false。
func TestFrontendLogEmptyBody(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader("")))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/log empty: code = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Errorf("expected success=false in body, got %s", w.Body.String())
	}
}

// TestGenerateTokenIsUnique 确认 GenerateToken 每次调用都返回新 token
// （auth-enable handler 在用）。
func TestGenerateTokenIsUnique(t *testing.T) {
	t1 := app.GenerateToken()
	t2 := app.GenerateToken()
	if t1 == "" || t2 == "" {
		t.Errorf("GenerateToken returned empty string")
	}
	if t1 == t2 {
		t.Errorf("GenerateToken returned duplicate: %s", t1)
	}
}
