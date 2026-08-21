// Package handlers —— Timer handler 测试。
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// newTestApp 为集成式 handler 测试构建带真实 SQLite + SettingsManager 的
// 真 App。
func newTestApp(t *testing.T) *app.App {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

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

	return app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		nil, // backup
		dbPath,
	)
}

// setupTestRouter 构建只注册 timer 路由的 gin.Engine。
func setupTestRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})

	r.GET("/api/state", TimerState)
	r.GET("/api/timer/progress", TimerProgress)
	r.POST("/api/start", TimerStart)
	r.POST("/api/pause", TimerPause)
	r.POST("/api/reset", TimerReset)
	r.POST("/api/finish", TimerFinish)
	r.POST("/api/mode", TimerMode)
	r.POST("/api/timer/rest", TimerStartRest)
	r.GET("/api/timer/config", TimerConfig)
	r.POST("/api/timer/config", TimerUpdateConfig)

	return r
}

func TestTimer_GetState_NoActiveSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/state: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if isRunning, ok := got["is_running"].(bool); !ok || isRunning {
		t.Errorf("is_running = %v, want false (no active session)", isRunning)
	}

	if _, ok := got["timezone"]; !ok {
		t.Errorf("missing timezone field in response")
	}

	if _, ok := got["habit_id"]; ok {
		t.Errorf("habit_id should not be present without active session")
	}
}

func TestTimer_GetState_TimezonePresent(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	r.ServeHTTP(w, req)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	tz, ok := got["timezone"]
	if !ok {
		t.Fatal("timezone field missing")
	}
	if _, ok := tz.(float64); !ok {
		t.Errorf("timezone = %T, want number", tz)
	}
}

func TestTimer_GetProgress_NoActiveSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["session_id"] != nil {
		t.Errorf("session_id = %v, want nil (no active session)", got["session_id"])
	}

	if got["habit_id"] != nil {
		t.Errorf("habit_id = %v, want nil (no active session)", got["habit_id"])
	}

	for _, key := range []string{"mode", "is_running", "is_paused", "is_finished", "elapsed_seconds", "remaining_seconds", "in_rest"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in progress response", key)
		}
	}
}

func TestTimer_GetProgress_WithActiveSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	sessionID, err := a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Unlock()
	if err != nil {
		t.Fatalf("CreateTimerSession: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	gotSessionID := int64(got["session_id"].(float64))
	if gotSessionID != sessionID {
		t.Errorf("session_id = %d, want %d", gotSessionID, sessionID)
	}
}

func TestTimer_Start_FreshStart(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// 省略 habit_id 是因为测试 DB 里没有 habits —— foreign_keys=ON 会拒绝
	// 悬空外键。
	body := bytes.NewBufferString(`{"mode": "countdown", "work_duration": 1500, "rest_duration": 300, "loop_count": 4}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started", got["status"])
	}
	if got["session_id"] == nil {
		t.Error("missing session_id in response")
	}
}

func TestTimer_Start_AlreadyRunning_Paused(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	sessionID, _ := a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserPauseTimerEvent{})
	a.SaveProgressLocked()
	a.Unlock()

	body := bytes.NewBufferString(`{"habit_id": 456}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (paused): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started (resumed)", got["status"])
	}
	gotSessionID := int64(got["session_id"].(float64))
	if gotSessionID != sessionID {
		t.Errorf("session_id = %d, want original %d", gotSessionID, sessionID)
	}
}

func TestTimer_Start_AlreadyRunning_Finished(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserFinishTimerEvent{})
	a.SaveProgressLocked()
	a.Unlock()

	body := bytes.NewBufferString(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (finished): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started (new session)", got["status"])
	}
}

func TestTimer_Start_InvalidJSON(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (invalid json): code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started (defaults)", got["status"])
	}
}

func TestTimer_Start_MissingFields_UsesDefaults(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (empty): code = %d", w.Code)
	}

	a.RLock()
	sessionID := a.CurrentTimerSessionID
	a.RUnlock()

	if sessionID == nil {
		t.Fatal("session not created")
	}

	row, err := a.SQLite.Timers().GetTimerSessionByID(*sessionID)
	if err != nil {
		t.Fatalf("GetTimerSessionByID: %v", err)
	}

	if row.Mode != "stopwatch" {
		t.Errorf("mode = %s, want stopwatch (default)", row.Mode)
	}
	// CreateTimerSession 在插入时写 elapsed_seconds=0；请求的工作时长被
	// 记录到 work_duration。
	if row.WorkDuration != 25*60 {
		t.Errorf("work_duration = %d, want %d (default)", row.WorkDuration, 25*60)
	}
}

func TestTimer_Pause_Normal(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pause", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/pause: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "paused" {
		t.Errorf("status = %v, want paused", got["status"])
	}

	state := a.Clock.Update()
	if !state.IsPaused() {
		t.Error("clock state is not paused after pause")
	}
}

func TestTimer_Reset_Normal(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/reset", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/reset: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "reset" {
		t.Errorf("status = %v, want reset", got["status"])
	}

	a.RLock()
	sessionID := a.CurrentTimerSessionID
	habitID := a.CurrentHabitID
	a.RUnlock()

	if sessionID != nil {
		t.Errorf("session_id = %v, want nil after reset", sessionID)
	}
	if habitID != nil {
		t.Errorf("habit_id = %v, want nil after reset", habitID)
	}
}

func TestTimer_Finish_Success(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	a.CreateTimerSession(int64Ptr(123), "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/finish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/finish: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "finished" {
		t.Errorf("status = %v, want finished", got["status"])
	}
	if got["elapsed_seconds"] == nil {
		t.Error("missing elapsed_seconds in response")
	}
}

func TestTimer_Finish_FallbackPath(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	a.Lock()
	a.CreateTimerSession(int64Ptr(456), "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	// 模拟时间流逝
	time.Sleep(100 * time.Millisecond)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/finish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/finish: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "finished" {
		t.Errorf("status = %v, want finished", got["status"])
	}
}

func TestTimer_Finish_NoHabit_NoSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/finish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/finish (no session): code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "finished" {
		t.Errorf("status = %v, want finished", got["status"])
	}
}

func TestTimer_ModeSwitch_Countdown(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{"mode": "countdown"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode countdown: code = %d", w.Code)
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

func TestTimer_ModeSwitch_Stopwatch(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{"mode": "stopwatch"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode stopwatch: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "mode_changed" {
		t.Errorf("status = %v, want mode_changed", got["status"])
	}
	if got["new_mode"] != "stopwatch" {
		t.Errorf("new_mode = %v, want stopwatch", got["new_mode"])
	}
}

func TestTimer_ModeSwitch_InvalidMode(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{"mode": "invalid"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (invalid): code = %d", w.Code)
	}

	got := strings.TrimSpace(w.Body.String())
	if got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

func TestTimer_ModeSwitch_EmptyBody(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(``)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (empty): code = %d", w.Code)
	}

	got := strings.TrimSpace(w.Body.String())
	if got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

func TestTimer_ModeSwitch_JSONParseFailure(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (parse fail): code = %d", w.Code)
	}

	got := strings.TrimSpace(w.Body.String())
	if got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

func TestTimer_StartRest_Normal(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/timer/rest", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/timer/rest: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "rest_started" {
		t.Errorf("status = %v, want rest_started", got["status"])
	}
	if got["rest_seconds"] != float64(5*60) {
		t.Errorf("rest_seconds = %v, want %d", got["rest_seconds"], 5*60)
	}

	state := a.Clock.Update()
	if state.GetMode() != domain.CountdownMode {
		t.Errorf("mode = %v, want countdown", state.GetMode())
	}
}

func TestTimer_Config_Get(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/timer/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/config: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	for _, key := range []string{"default_mode", "countdown", "stopwatch"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in config response", key)
		}
	}

	countdown, ok := got["countdown"].(map[string]any)
	if !ok {
		t.Fatal("countdown field is not an object")
	}
	for _, key := range []string{"duration_seconds", "loop", "loop_count", "loop_interval_seconds"} {
		if _, ok := countdown[key]; !ok {
			t.Errorf("missing countdown key %q", key)
		}
	}

	stopwatch, ok := got["stopwatch"].(map[string]any)
	if !ok {
		t.Fatal("stopwatch field is not an object")
	}
	if _, ok := stopwatch["max_seconds"]; !ok {
		t.Error("missing stopwatch.max_seconds")
	}
}

func TestTimer_UpdateConfig_ValidPartial(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// 省略 default_mode 是因为 ClockTaskConfig.DefaultMode 是 ModeEnum
	// （int）而非字符串 —— 把 "countdown" 当字符串传会让严格 JSON bind 失败。
	body := bytes.NewBufferString(`{"countdown": {"duration_seconds": 1800}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/timer/config", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/timer/config: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "config_updated" {
		t.Errorf("status = %v, want config_updated", got["status"])
	}
}

func TestTimer_UpdateConfig_InvalidJSON(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/timer/config", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/timer/config (invalid): code = %d, want 400", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["error"] != "invalid json" {
		t.Errorf("error = %v, want 'invalid json'", got["error"])
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
