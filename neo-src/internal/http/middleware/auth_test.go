package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/settings"
)

// newTestApp creates a minimal App for middleware testing.  We hand-build
// the App struct directly via NewApp rather than constructing a real App
// the long way — the middleware only touches a.Settings, so everything
// else stays nil.
func newTestApp(t *testing.T, sm *settings.SettingsManager) *app.App {
	t.Helper()
	return app.NewApp(nil, sm, nil, nil, "")
}

// runAuth is the test harness for the Auth middleware.  It builds a tiny
// gin.Engine that runs Auth(a) followed by a single next-handler that
// toggles `nextCalled` on entry.  Returns the recorder + the nextCalled
// flag pointer so callers can assert both the response code and whether
// Auth decided to let the request through.
func runAuth(t *testing.T, a *app.App, req *http.Request) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	nextCalled := false

	r := gin.New()
	r.Use(Auth(a))
	r.Handle(req.Method, req.URL.Path, func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, &nextCalled
}

// setupAuthSettings creates a SettingsManager with auth configured.
func setupAuthSettings(t *testing.T, authEnabled bool, authToken string) *settings.SettingsManager {
	t.Helper()
	cfg := domain.NewDefaultSettingsConfig()
	cfg.Auth.AuthEnabled = authEnabled
	cfg.Auth.AuthToken = authToken

	// ponytail: each test gets its own DB so settings don't bleed across cases.
	tmpDB := t.TempDir() + "/test.db"
	sm, err := settings.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create SettingsManager: %v", err)
	}
	_ = sm.UpdateAuth(cfg.Auth)
	return sm
}

// =============================================================================
// isPublic tests (6 cases)
// =============================================================================

func TestIsPublic_ExactMatch_Events(t *testing.T) {
	if !isPublic("/api/events") {
		t.Error("/api/events should be public")
	}
}

func TestIsPublic_ExactMatch_AuthStatus(t *testing.T) {
	if !isPublic("/api/auth/status") {
		t.Error("/api/auth/status should be public")
	}
}

func TestIsPublic_RootPath_NotPublic(t *testing.T) {
	if isPublic("/") {
		t.Error("/ should not be public")
	}
}

func TestIsPublic_ApiLog_NotPublic(t *testing.T) {
	if isPublic("/api/log") {
		t.Error("/api/log should not be public")
	}
}

func TestIsPublic_Anything_NotPublic(t *testing.T) {
	if isPublic("/api/anything") {
		t.Error("/api/anything should not be public")
	}
}

func TestIsPublic_SubPath_NotPublic(t *testing.T) {
	if isPublic("/api/events/sub") {
		t.Error("/api/events/sub should not be public (exact match only)")
	}
}

// =============================================================================
// extractBearer tests (7 cases)
// =============================================================================

func TestExtractBearer_Empty(t *testing.T) {
	if got := extractBearer(""); got != "" {
		t.Errorf("extractBearer(\"\") = %q, want \"\"", got)
	}
}

func TestExtractBearer_BearerOnly(t *testing.T) {
	if got := extractBearer("Bearer "); got != "" {
		t.Errorf("extractBearer(\"Bearer \") = %q, want \"\"", got)
	}
}

func TestExtractBearer_NoSpace(t *testing.T) {
	if got := extractBearer("Bearer"); got != "" {
		t.Errorf("extractBearer(\"Bearer\") = %q, want \"\"", got)
	}
}

func TestExtractBearer_Lowercase(t *testing.T) {
	if got := extractBearer("bearer mytok"); got != "" {
		t.Errorf("extractBearer(\"bearer mytok\") = %q, want \"\" (case sensitive)", got)
	}
}

func TestExtractBearer_Valid(t *testing.T) {
	if got := extractBearer("Bearer mytok"); got != "mytok" {
		t.Errorf("extractBearer(\"Bearer mytok\") = %q, want \"mytok\"", got)
	}
}

func TestExtractBearer_TwoSpaces(t *testing.T) {
	if got := extractBearer("Bearer  mytok"); got != " mytok" {
		t.Errorf("extractBearer(\"Bearer  mytok\") = %q, want \" mytok\"", got)
	}
}

func TestExtractBearer_FullHeader(t *testing.T) {
	if got := extractBearer("Authorization: Bearer mytok"); got != "" {
		t.Errorf("extractBearer(\"Authorization: Bearer mytok\") = %q, want \"\"", got)
	}
}

// =============================================================================
// Auth middleware tests (14 branches)
// =============================================================================

func TestAuth_PublicPath_Events(t *testing.T) {
	a := newTestApp(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/events: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called for public path")
	}
}

func TestAuth_PublicPath_AuthStatus(t *testing.T) {
	a := newTestApp(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/auth/status: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called for public path")
	}
}

func TestAuth_NilSettings_AllowsRequest(t *testing.T) {
	a := newTestApp(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("nil settings: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called when settings is nil")
	}
}

func TestAuth_AuthDisabled_AllowsRequest(t *testing.T) {
	sm := setupAuthSettings(t, false, "test-token")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("auth disabled: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called when auth is disabled")
	}
}

func TestAuth_EmptyToken_AllowsRequest(t *testing.T) {
	sm := setupAuthSettings(t, true, "")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty token: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called when token is empty")
	}
}

func TestAuth_ValidBearerToken_AllowsRequest(t *testing.T) {
	sm := setupAuthSettings(t, true, "valid-token")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid bearer: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called with valid token")
	}
}

func TestAuth_InvalidBearerToken_Rejects(t *testing.T) {
	sm := setupAuthSettings(t, true, "valid-token")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid bearer: code = %d, want 401", w.Code)
	}
	if *next {
		t.Error("c.Next() should NOT be called with invalid token")
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp["err"] != "Unauthorized: Invalid or missing token" {
		t.Errorf("error message = %q, want \"Unauthorized: Invalid or missing token\"", resp["err"])
	}
}

func TestAuth_ValidQueryToken_AllowsRequest(t *testing.T) {
	sm := setupAuthSettings(t, true, "valid-token")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state?auth_token=valid-token", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid query token: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called with valid query token")
	}
}

func TestAuth_InvalidQueryToken_Rejects(t *testing.T) {
	sm := setupAuthSettings(t, true, "valid-token")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state?auth_token=invalid-token", nil)
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid query token: code = %d, want 401", w.Code)
	}
	if *next {
		t.Error("c.Next() should NOT be called with invalid query token")
	}
}

func TestAuth_HeaderWinsOverQuery(t *testing.T) {
	sm := setupAuthSettings(t, true, "header-token")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state?auth_token=query-token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("header wins: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called when header token is valid")
	}
}

func TestAuth_LowercaseBearer_Rejects(t *testing.T) {
	sm := setupAuthSettings(t, true, "mytok")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	req.Header.Set("Authorization", "bearer mytok")
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("lowercase bearer: code = %d, want 401", w.Code)
	}
	if *next {
		t.Error("c.Next() should NOT be called with lowercase bearer")
	}
}

func TestAuth_NoSpaceAfterBearer_Rejects(t *testing.T) {
	sm := setupAuthSettings(t, true, "mytok")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	req.Header.Set("Authorization", "Bearer:mytok")
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("no space after bearer: code = %d, want 401", w.Code)
	}
	if *next {
		t.Error("c.Next() should NOT be called with malformed bearer")
	}
}

func TestAuth_TwoSpacesAfterBearer_Allows(t *testing.T) {
	// ponytail: the configured token is " mytok" (leading space) — extractBearer
	// returns the literal remainder after "Bearer ", so "Bearer  mytok" yields
	// " mytok" and matches.
	sm := setupAuthSettings(t, true, " mytok")
	a := newTestApp(t, sm)
	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	req.Header.Set("Authorization", "Bearer  mytok")
	w, next := runAuth(t, a, req)

	if w.Code != http.StatusOK {
		t.Errorf("two spaces after bearer: code = %d, want 200", w.Code)
	}
	if !*next {
		t.Error("c.Next() should be called with two-space bearer")
	}
}

func TestAuth_ContextKeySet_OnSuccess(t *testing.T) {
	sm := setupAuthSettings(t, true, "valid-token")
	a := newTestApp(t, sm)

	gin.SetMode(gin.TestMode)
	var captured any
	r := gin.New()
	r.Use(Auth(a))
	r.GET("/api/timer/state", func(c *gin.Context) {
		captured = c.MustGet("app")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/timer/state", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	if captured == nil {
		t.Error("context key \"app\" should be set on successful auth")
	}
	if captured != a {
		t.Error("context key \"app\" should be the App instance")
	}
}

// =============================================================================
// CORS middleware tests (7 cases)
// =============================================================================

func TestCORS_EmptyOrigin_DefaultsToStar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(""))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("empty origin: Access-Control-Allow-Origin = %q, want \"*\"", got)
	}
}

func TestCORS_SpecificOrigin_Set(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS("https://example.com"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("specific origin: Access-Control-Allow-Origin = %q, want \"https://example.com\"", got)
	}
}

func TestCORS_OptionsRequest_NoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS("*"))
	r.OPTIONS("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: code = %d, want 204", w.Code)
	}
}

func TestCORS_GetWithEmptyOrigin_Star(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(""))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("GET with empty origin: Access-Control-Allow-Origin = %q, want \"*\"", got)
	}
}

func TestCORS_AllHeaders_Set(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS("https://example.com"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	header := w.Header()

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"Access-Control-Allow-Origin", "Access-Control-Allow-Origin", "https://example.com"},
		{"Access-Control-Allow-Methods", "Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS"},
		{"Access-Control-Allow-Headers", "Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Requested-With"},
		{"Access-Control-Max-Age", "Access-Control-Max-Age", "86400"},
		{"Access-Control-Allow-Credentials", "Access-Control-Allow-Credentials", "true"},
		{"Access-Control-Expose-Headers", "Access-Control-Expose-Headers", "Content-Length, Content-Type"},
	}

	for _, tt := range tests {
		if got := header.Get(tt.key); got != tt.expected {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.expected)
		}
	}
}