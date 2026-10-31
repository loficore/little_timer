package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func newRootRouter(ui fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerRoot(r, ui)
	return r
}

func TestRegisterRootServesEmbeddedUI(t *testing.T) {
	r := newRootRouter(fstest.MapFS{
		"index.html":   {Data: []byte("<html><body>little timer ui</body></html>")},
		"i18n/en.toml": {Data: []byte(`hello = "world"`)},
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
		wantType   string
		wantNoBody string
	}{
		{name: "root", path: "/", wantStatus: http.StatusOK, wantBody: "little timer ui", wantType: "text/html"},
		{name: "spa fallback", path: "/habits", wantStatus: http.StatusOK, wantBody: "little timer ui", wantType: "text/html"},
		{name: "static toml", path: "/i18n/en.toml", wantStatus: http.StatusOK, wantBody: `hello = "world"`},
		{name: "wails runtime stub", path: "/wails/runtime.js", wantStatus: http.StatusOK, wantBody: "export const Call"},
		{name: "unknown api", path: "/api/nope", wantStatus: http.StatusNotFound, wantNoBody: "little timer ui"},
		{name: "unknown wails asset", path: "/wails/nope", wantStatus: http.StatusNotFound, wantNoBody: "little timer ui"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", w.Body.String(), tt.wantBody)
			}
			if tt.wantNoBody != "" && strings.Contains(w.Body.String(), tt.wantNoBody) {
				t.Fatalf("body = %q, must not contain %q", w.Body.String(), tt.wantNoBody)
			}
			if tt.wantType != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), tt.wantType) {
				t.Fatalf("Content-Type = %q, want prefix %q", w.Header().Get("Content-Type"), tt.wantType)
			}
		})
	}
}

func TestRegisterRootRejectsNonReadMethods(t *testing.T) {
	r := newRootRouter(fstest.MapFS{
		"index.html": {Data: []byte("ui")},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/habits", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST /habits status = %d, want 404", w.Code)
	}
}

func TestRegisterRootWithoutUIKeepsPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerRoot(r, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Build the frontend") {
		t.Fatalf("placeholder response = %d %q", w.Code, w.Body.String())
	}
}

func TestRegisterRootReportsMissingIndex(t *testing.T) {
	r := newRootRouter(fstest.MapFS{
		"i18n/en.toml": {Data: []byte(`hello = "world"`)},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "stage-embedded-ui") {
		t.Fatalf("missing-index response = %d %q", w.Code, w.Body.String())
	}
}
