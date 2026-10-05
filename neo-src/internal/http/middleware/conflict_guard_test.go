// Package middleware —— ConflictGuard 中间件测试。
//
// 验证 RequireVersionForMutation 在三种关键路径上的行为:
//   - 缺 If-Match 且 body 无 "version" key → 400
//   - 带合法 If-Match header → 放行
//   - body 显式带 "version" key → 放行
//
// 这是乐观锁强制在 HTTP 边界的"请求侧存在性"门。具体的 WHERE version = ?
// 由 storage 层 (Task 4) 处理;这里只保证请求携带了版本号。
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequireVersion_MissingHeader_400 验证: 无 If-Match 且 body 为空
// (或不含 "version" key) 时,中间件必须返回 400,handler 不被调用。
func TestRequireVersion_MissingHeader_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	nextCalled := false
	r.Use(RequireVersionForMutation())
	r.PATCH("/x", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if nextCalled {
		t.Error("next handler 不应在 400 时被调用")
	}
	// 响应 body 应含可读错误信息
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["error"]; !ok {
		t.Errorf("响应缺 error 字段: %s", w.Body.String())
	}
}

// TestRequireVersion_Header_Passes 验证: 带合法 If-Match header 的 PATCH
// 请求能直接放行,handler 收到 context 中的 version。
func TestRequireVersion_Header_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotVersion int64
	r.Use(RequireVersionForMutation())
	r.PATCH("/x", func(c *gin.Context) {
		gotVersion = GetVersion(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/x", nil)
	req.Header.Set("If-Match", "5")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if gotVersion != 5 {
		t.Errorf("GetVersion 返回 %d, want 5", gotVersion)
	}
}

// TestRequireVersion_HeaderWithQuotes_StripsAndParses 验证: If-Match 形如
// `"5"`(带 ETag 风格的引号)也能被正确解析。中间件必须先剥引号再 parse。
func TestRequireVersion_HeaderWithQuotes_StripsAndParses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotVersion int64
	r.Use(RequireVersionForMutation())
	r.PATCH("/x", func(c *gin.Context) {
		gotVersion = GetVersion(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/x", nil)
	req.Header.Set("If-Match", `"7"`)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if gotVersion != 7 {
		t.Errorf("GetVersion 返回 %d, want 7", gotVersion)
	}
}

// TestRequireVersion_BodyVersion_Passes 验证: body 带 "version":3 时,
// 无 If-Match 也放行。同时验证 body 被正确还原(handler 仍能读 body)。
func TestRequireVersion_BodyVersion_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotVersion int64
	var bodyRead string
	r.Use(RequireVersionForMutation())
	r.PATCH("/x", func(c *gin.Context) {
		gotVersion = GetVersion(c)
		// handler 必须能读 body(body-peek 后要恢复)
		buf := make([]byte, 256)
		n, _ := c.Request.Body.Read(buf)
		bodyRead = string(buf[:n])
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/x",
		strings.NewReader(`{"version":3,"name":"foo"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if gotVersion != 3 {
		t.Errorf("GetVersion 返回 %d, want 3", gotVersion)
	}
	if bodyRead != `{"version":3,"name":"foo"}` {
		t.Errorf("body 被破坏: %q", bodyRead)
	}
}

// TestRequireVersion_GET_Skips 验证: GET 请求不要求 version,直接放行。
func TestRequireVersion_GET_Skips(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	nextCalled := false
	r.Use(RequireVersionForMutation())
	r.GET("/x", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET 被错误拦截: code = %d, body = %s", w.Code, w.Body.String())
	}
	if !nextCalled {
		t.Error("GET handler 不应被跳过")
	}
}

// TestGetVersion_PanicsOnMissing 验证: 没装中间件的路由上调用 GetVersion
// 必须 panic(说明路由缺少 ConflictGuard,而不是静默返 0)。
func TestGetVersion_PanicsOnMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("GetVersion 在缺 context 时应 panic,实际没 panic")
		}
	}()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	GetVersion(c)
}
