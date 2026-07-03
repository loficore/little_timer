// Integration tests for the HTTP layer.
//
// Unlike server_test.go (which uses httptest.NewRecorder + r.ServeHTTP),
// this file uses httptest.NewServer so the tests exercise the full
// client → router → handler → storage pipeline the way a real browser
// or curl invocation would.  That's the level the spec calls for:
//
//   - TimerStartStop: state transitions via the HTTP surface.
//   - SettingsRoundTrip: POST /api/settings then GET /api/settings.
//   - HabitCRUD: full habit-set / habit lifecycle via REST.
//   - BackupFlow: create / list / restore via the local adapter.
//
// No mocks — every test wires the same real App that cmd/server would
// at boot.
package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// integrationFixture bundles a running httptest.Server with the
// underlying App + Storage + Settings.  Tests get a `*integrationFixture`
// via `newIntegrationServer(t)` and call methods against it.
type integrationFixture struct {
	server  *httptest.Server
	app     *app.App
	sqlite  *storage.SqliteManager
	settings *settings.SettingsManager
	backup  *backup.BackupManager
	backupDir string
	dbPath  string
}

func (f *integrationFixture) URL(path string) string {
	return f.server.URL + path
}

func (f *integrationFixture) Close() {
	f.server.Close()
}

// newIntegrationServer wires every layer with a real SQLite file and
// a real LocalAdapter for backups.  All temp files live under
// t.TempDir() so the test is hermetic.
func newIntegrationServer(t *testing.T) *integrationFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lt.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}

	bm, err := backup.NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		bm,
		dbPath,
	)

	srv := httptest.NewServer(NewRouter(a, "*"))
	t.Cleanup(func() {
		srv.Close()
		_ = bm // adapter holds no resources to release
		_ = sm.Close()
	})

	return &integrationFixture{
		server:    srv,
		app:       a,
		sqlite:    sqlite,
		settings:  sm,
		backup:    bm,
		backupDir: backupDir,
		dbPath:    dbPath,
	}
}

// httpDo is a small helper: send `method path body`, parse the JSON
// response into `out` (if non-nil), return status + body for assertions.
func (f *integrationFixture) httpDo(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, f.URL(path), reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, raw
}

// -----------------------------------------------------------------------------
// /api/start / pause / reset — the full state-machine round-trip.
// -----------------------------------------------------------------------------

func TestAPI_TimerStartStop(t *testing.T) {
	f := newIntegrationServer(t)

	// Initial state — should be idle / paused / not running.
	state := f.fetchState(t)
	if running, _ := state["is_running"].(bool); running {
		t.Errorf("initial state is_running = true, want false; state=%v", state)
	}
	if mode, _ := state["mode"].(string); mode != "countdown" && mode != "stopwatch" {
		t.Errorf("initial mode = %q, want countdown|stopwatch", mode)
	}

	// Start the timer.
	code, body := f.httpDo(t, http.MethodPost, "/api/start", map[string]any{
		"mode":          "stopwatch",
		"work_duration": 1500,
	})
	if code != http.StatusOK {
		t.Fatalf("POST /api/start: code=%d body=%s", code, body)
	}
	var startResp map[string]any
	if err := json.Unmarshal(body, &startResp); err != nil {
		t.Fatalf("start response not JSON: %v body=%s", err, body)
	}
	if status, _ := startResp["status"].(string); status != "started" && status != "already_running" {
		t.Errorf("start status = %q, want started|already_running", status)
	}

	// /api/state has no is_paused; /api/timer/progress does.
	progress := f.fetchProgress(t)
	if paused, _ := progress["is_paused"].(bool); paused {
		t.Errorf("after start is_paused = true, want false; progress=%v", progress)
	}
	if running, _ := progress["is_running"].(bool); !running {
		t.Errorf("after start is_running = false, want true; progress=%v", progress)
	}

	// Pause.
	code, body = f.httpDo(t, http.MethodPost, "/api/pause", nil)
	if code != http.StatusOK {
		t.Fatalf("POST /api/pause: code=%d body=%s", code, body)
	}

	progress = f.fetchProgress(t)
	if running, _ := progress["is_running"].(bool); running {
		t.Errorf("after pause is_running = true, want false")
	}
	if paused, _ := progress["is_paused"].(bool); !paused {
		t.Errorf("after pause is_paused = false, want true")
	}

	// Reset.
	code, body = f.httpDo(t, http.MethodPost, "/api/reset", nil)
	if code != http.StatusOK {
		t.Fatalf("POST /api/reset: code=%d body=%s", code, body)
	}

	state = f.fetchState(t)
	if running, _ := state["is_running"].(bool); running {
		t.Errorf("after reset is_running = true, want false")
	}
	if finished, _ := state["is_finished"].(bool); finished {
		t.Errorf("after reset is_finished = true, want false")
	}
}

func (f *integrationFixture) fetchProgress(t *testing.T) map[string]any {
	t.Helper()
	code, body := f.httpDo(t, http.MethodGet, "/api/timer/progress", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code=%d body=%s", code, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("/api/timer/progress not JSON: %v body=%s", err, body)
	}
	return out
}

// fetchState hits /api/state and parses the JSON body.  Fails the
// test on any non-200 or parse error.
func (f *integrationFixture) fetchState(t *testing.T) map[string]any {
	t.Helper()
	code, body := f.httpDo(t, http.MethodGet, "/api/state", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/state: code=%d body=%s", code, body)
	}
	var state map[string]any
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("/api/state not JSON: %v body=%s", err, body)
	}
	return state
}

// -----------------------------------------------------------------------------
// /api/settings round-trip.
// -----------------------------------------------------------------------------

func TestAPI_SettingsRoundTrip(t *testing.T) {
	f := newIntegrationServer(t)

	state := f.fetchSettings(t)
	basic, _ := state["basic"].(map[string]any)
	if basic == nil {
		t.Fatalf("GET /api/settings missing basic: %v", state)
	}
	if basic["timezone"] == nil {
		t.Errorf("GET /api/settings basic.timezone missing: %v", basic)
	}

	code, body := f.httpDo(t, http.MethodPost, "/api/settings", map[string]any{
		"basic": map[string]any{
			"timezone": -5,
			"language": "EN",
		},
	})
	if code != http.StatusOK {
		t.Fatalf("POST /api/settings: code=%d body=%s", code, body)
	}

	state = f.fetchSettings(t)
	basic, _ = state["basic"].(map[string]any)
	if basic == nil {
		t.Fatalf("post-update GET /api/settings missing basic: %v", state)
	}
	if got, _ := basic["language"].(string); got != "EN" {
		t.Errorf("after update basic.language = %q, want EN", got)
	}
	if got, ok := basic["timezone"].(float64); !ok || got != -5 {
		t.Errorf("after update basic.timezone = %v (%T), want -5",
			basic["timezone"], basic["timezone"])
	}
}

func (f *integrationFixture) fetchSettings(t *testing.T) map[string]any {
	t.Helper()
	code, body := f.httpDo(t, http.MethodGet, "/api/settings", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/settings: code=%d body=%s", code, body)
	}
	var state map[string]any
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("/api/settings not JSON: %v body=%s", err, body)
	}
	return state
}

// -----------------------------------------------------------------------------
// Habit CRUD: create habit-set → create habit → update → delete.
// -----------------------------------------------------------------------------

func TestAPI_HabitCRUD(t *testing.T) {
	f := newIntegrationServer(t)

	code, body := f.httpDo(t, http.MethodPost, "/api/habit-sets", map[string]any{
		"name":        "Daily Reading",
		"description": "books",
		"color":       "#abcdef",
	})
	if code != http.StatusOK {
		t.Fatalf("POST /api/habit-sets: code=%d body=%s", code, body)
	}
	var setResp map[string]any
	_ = json.Unmarshal(body, &setResp)
	setID := int64(setResp["id"].(float64))
	if setID <= 0 {
		t.Fatalf("habit set id = %v, want > 0", setResp["id"])
	}

	code, body = f.httpDo(t, http.MethodPost, "/api/habits", map[string]any{
		"set_id":       setID,
		"name":         "Read 30 min",
		"goal_seconds": 1800,
		"color":        "#123456",
	})
	if code != http.StatusOK {
		t.Fatalf("POST /api/habits: code=%d body=%s", code, body)
	}
	var habitResp map[string]any
	_ = json.Unmarshal(body, &habitResp)
	habitID := int64(habitResp["id"].(float64))
	if habitID <= 0 {
		t.Fatalf("habit id = %v, want > 0", habitResp["id"])
	}

	code, body = f.httpDo(t, http.MethodGet,
		fmt.Sprintf("/api/habits?set_id=%d", setID), nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/habits: code=%d body=%s", code, body)
	}
	var listed []map[string]any
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("habits list not JSON: %v body=%s", err, body)
	}
	if len(listed) != 1 {
		t.Fatalf("habits list len = %d, want 1", len(listed))
	}
	if listed[0]["name"] != "Read 30 min" {
		t.Errorf("listed habit name = %v, want Read 30 min", listed[0]["name"])
	}

	code, body = f.httpDo(t, http.MethodPut,
		fmt.Sprintf("/api/habits/%d", habitID),
		map[string]any{
			"name":         "Read 45 min",
			"goal_seconds": 2700,
			"color":        "#654321",
		})
	if code != http.StatusOK {
		t.Fatalf("PUT /api/habits/%d: code=%d body=%s", habitID, code, body)
	}

	code, body = f.httpDo(t, http.MethodGet,
		fmt.Sprintf("/api/habits/%d/detail", habitID), nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/habits/%d/detail: code=%d body=%s", habitID, code, body)
	}
	var detail map[string]any
	_ = json.Unmarshal(body, &detail)
	if detail["name"] != "Read 45 min" {
		t.Errorf("detail name = %v, want Read 45 min", detail["name"])
	}
	if goal := int64(detail["goal_seconds"].(float64)); goal != 2700 {
		t.Errorf("detail goal_seconds = %d, want 2700", goal)
	}

	code, body = f.httpDo(t, http.MethodDelete,
		fmt.Sprintf("/api/habits/%d", habitID), nil)
	if code != http.StatusOK {
		t.Fatalf("DELETE /api/habits/%d: code=%d body=%s", habitID, code, body)
	}

	code, body = f.httpDo(t, http.MethodGet,
		fmt.Sprintf("/api/habits?set_id=%d", setID), nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/habits (post-delete): code=%d body=%s", code, body)
	}
	_ = json.Unmarshal(body, &listed)
	if len(listed) != 0 {
		t.Errorf("post-delete habits list len = %d, want 0", len(listed))
	}
}

// -----------------------------------------------------------------------------
// Backup flow: create → list → restore → verify data persists.
// -----------------------------------------------------------------------------

func TestAPI_BackupFlow(t *testing.T) {
	f := newIntegrationServer(t)

	enableCode, enableBody := f.httpDo(t, http.MethodPost, "/api/backup/config", map[string]any{
		"enabled":              true,
		"target_type":          "local",
		"local_path":           f.backupDir,
		"auto_backup_interval": 86400,
	})
	if enableCode != http.StatusOK {
		t.Fatalf("POST /api/backup/config: code=%d body=%s", enableCode, enableBody)
	}

	code, body := f.httpDo(t, http.MethodPost, "/api/habit-sets", map[string]any{
		"name":  "Pre-Backup",
		"color": "#888888",
	})
	if code != http.StatusOK {
		t.Fatalf("POST /api/habit-sets (pre-backup): code=%d body=%s", code, body)
	}

	code, body = f.httpDo(t, http.MethodPost, "/api/backup/create", nil)
	if code != http.StatusOK {
		t.Fatalf("POST /api/backup/create: code=%d body=%s", code, body)
	}
	var createResp map[string]any
	if err := json.Unmarshal(body, &createResp); err != nil {
		t.Fatalf("create response not JSON: %v body=%s", err, body)
	}
	if ok, _ := createResp["success"].(bool); !ok {
		t.Fatalf("create success = false: %v", createResp)
	}
	backupPath, _ := createResp["backup_path"].(string)
	if backupPath == "" {
		t.Fatalf("backup_path missing: %v", createResp)
	}
	if !strings.HasPrefix(backupPath, "presets_backup_") {
		t.Errorf("backup_path = %q, want presets_backup_* prefix", backupPath)
	}

	if _, err := os.Stat(filepath.Join(f.backupDir, backupPath)); err != nil {
		t.Errorf("backup file missing on disk: %v", err)
	}

	code, body = f.httpDo(t, http.MethodGet, "/api/backup/list", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/backup/list: code=%d body=%s", code, body)
	}
	var listResp struct {
		Success bool              `json:"success"`
		Backups []json.RawMessage `json:"backups"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("list response not JSON: %v body=%s", err, body)
	}
	if !listResp.Success {
		t.Errorf("list success = false")
	}
	if len(listResp.Backups) < 1 {
		t.Errorf("list returned %d backups, want >= 1", len(listResp.Backups))
	}

	code, body = f.httpDo(t, http.MethodPost, "/api/backup/restore",
		map[string]any{"name": backupPath})
	if code != http.StatusOK {
		t.Fatalf("POST /api/backup/restore: code=%d body=%s", code, body)
	}
	var restoreResp map[string]any
	_ = json.Unmarshal(body, &restoreResp)
	if ok, _ := restoreResp["success"].(bool); !ok {
		t.Errorf("restore success = false: %v", restoreResp)
	}

	code, body = f.httpDo(t, http.MethodGet, "/api/habit-sets", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /api/habit-sets (post-restore): code=%d body=%s", code, body)
	}
	var sets []map[string]any
	if err := json.Unmarshal(body, &sets); err != nil {
		t.Fatalf("habit sets not JSON: %v body=%s", err, body)
	}
	found := false
	for _, s := range sets {
		if s["name"] == "Pre-Backup" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Pre-Backup habit set missing after restore: %v", sets)
	}
}