package backup

// WebDAVAdapter 的测试，用 httptest.NewServer 对所有 adapter 方法做
// 完整往返覆盖。

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// newWebDAVTestServer 创建一个接到 httptest server 的 WebDAVAdapter，
// server 把 PUT body 存进 map。调用方可用 store map 检查 adapter 写了
// 什么，或为 GET 请求预置数据。
//
// server 处理 PUT、GET、DELETE 和 PROPFIND（空 multistatus）。
// 需要自定义行为（鉴权失败、特定 PROPFIND 响应）的测试应自行内联建 server。
func newWebDAVTestServer(t *testing.T) (*WebDAVAdapter, *httptest.Server, map[string][]byte) {
	adapter, srv, store, _ := newWebDAVTestServerWithCL(t)
	return adapter, srv, store
}

// newWebDAVTestServerWithCL 在 newWebDAVTestServer 之上附带一个 map：
// server 所见 PUT Content-Length 值，按请求路径为 key。断言显式
// Content-Length（而非 chunked 传输编码）的测试用这个变体。
func newWebDAVTestServerWithCL(t *testing.T) (*WebDAVAdapter, *httptest.Server, map[string][]byte, map[string]int64) {
	t.Helper()
	store := make(map[string][]byte)
	putCL := make(map[string]int64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		switch r.Method {
		case http.MethodPut:
			putCL[key] = r.ContentLength
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			store[key] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(data)
		case http.MethodDelete:
			delete(store, key)
			w.WriteHeader(http.StatusNoContent)
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:"/>`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()
	return adapter, srv, store, putCL
}

func TestWebDAVBackupRestoreRoundTrip(t *testing.T) {
	adapter, _, store := newWebDAVTestServer(t)

	payload := []byte("SQLite-format-3\x00webdav-round-trip-payload")
	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, payload)

	name := backupName(time.Now().Unix())
	if err := adapter.Backup(src, name); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// server 必须收到了 PUT body。
	gotStored := store["/backups/"+name]
	if string(gotStored) != string(payload) {
		t.Errorf("stored payload mismatch:\n got: %q\nwant: %q", gotStored, payload)
	}

	// 恢复到全新目的地并确认逐字节一致的拷贝。
	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); string(got) != string(payload) {
		t.Errorf("restored content mismatch:\n got: %q\nwant: %q", got, payload)
	}
}

func TestWebDAVBackupRestoreRoundTrip_LargeFile(t *testing.T) {
	adapter, _, store, putCL := newWebDAVTestServerWithCL(t)

	// 2 MiB、确定性图案 —— 往返两侧的偏移/截断都能逐字节抓到。
	const size = 2 << 20
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(i * 31)
	}
	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, payload)

	name := backupName(time.Now().Unix())
	if err := adapter.Backup(src, name); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	key := "/backups/" + name
	if gotStored := store[key]; !bytes.Equal(gotStored, payload) {
		t.Fatalf("stored payload mismatch: got %d bytes, want %d", len(gotStored), size)
	}
	// 流式路径以 *os.File 作 request body；客户端必须显式设置
	// Content-Length（chunked 传输会被许多 WebDAV 服务器拒绝）。
	// 值为 -1 表示没发 Content-Length。
	if gotCL := putCL[key]; gotCL != int64(size) {
		t.Errorf("PUT Content-Length: got %d, want %d", gotCL, size)
	}

	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); !bytes.Equal(got, payload) {
		t.Errorf("restored content mismatch: got %d bytes, want %d", len(got), size)
	}
}

func TestWebDAVTestConnection_WriteProbe(t *testing.T) {
	var ops []string
	store := make(map[string][]byte)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ops = append(ops, r.Method)
		key := r.URL.Path
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			store[key] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(data)
		case http.MethodDelete:
			delete(store, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	if err := adapter.TestConnection(); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}

	if len(ops) < 3 {
		t.Fatalf("expected at least 3 operations (PUT, GET, DELETE), got %d: %v", len(ops), ops)
	}
	if ops[0] != http.MethodPut {
		t.Errorf("first operation: got %s, want PUT", ops[0])
	}
	if ops[1] != http.MethodGet {
		t.Errorf("second operation: got %s, want GET", ops[1])
	}
	if ops[2] != http.MethodDelete {
		t.Errorf("third operation: got %s, want DELETE", ops[2])
	}
}

func TestWebDAVTestConnection_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PUT 返回 401（以及尽力清理里后续的任意 DELETE）。
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Errorf("error: got %v, want errors.Is ErrAuthenticationFail", err)
	}
}

func TestWebDAVTestConnection_PermissionDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PUT 返回 403 → ErrPermissionDenied，与 classifyS3Error 对齐。
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("error: got %v, want errors.Is ErrPermissionDenied", err)
	}
}

func TestWebDAVTestConnection_GETProbeServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			// PUT 成功，让 probe 走到 GET 这步。
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			// GET probe 的 500 必须映射为 ErrConnectionFailed，
			// 而不是误导性的 "probe content mismatch"。
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for GET probe 500, got nil")
	}
	if !errors.Is(err, ErrConnectionFailed) {
		t.Errorf("error: got %v, want errors.Is ErrConnectionFailed", err)
	}
}

func TestWebDAVTestConnection_GETProbePermissionDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			// PUT 成功，让 probe 走到 GET 这步。
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for GET probe 403, got nil")
	}
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("error: got %v, want errors.Is ErrPermissionDenied", err)
	}
}

func TestWebDAVTestConnection_GETProbeAuthFailure(t *testing.T) {
	store := make(map[string][]byte)
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			store[r.URL.Path] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			w.WriteHeader(http.StatusUnauthorized)
		case http.MethodDelete:
			delete(store, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
		Username: "testuser",
		Password: "testpass",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Errorf("expected ErrAuthenticationFail, got %v", err)
	}
	// 验证错误信息包含 probe URL 与鉴权状态
	errStr := err.Error()
	if !strings.Contains(errStr, "GET") {
		t.Errorf("error message should contain GET, got: %s", errStr)
	}
	if !strings.Contains(errStr, "401") {
		t.Errorf("error message should contain 401, got: %s", errStr)
	}
	if !strings.Contains(errStr, "auth=basic") {
		t.Errorf("error message should contain auth=basic, got: %s", errStr)
	}
	if !strings.Contains(errStr, "lt_probe_") {
		t.Errorf("error message should contain probe URL, got: %s", errStr)
	}
}

func TestWebDAVList(t *testing.T) {
	ts1 := int64(1000)
	ts2 := int64(2000)

	multistatusXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/backups/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/backups/%s</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>42</D:getcontentlength>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/backups/%s</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>99</D:getcontentlength>
        <D:getlastmodified>Tue, 02 Jan 2024 00:00:00 GMT</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/backups/not_a_backup.txt</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>7</D:getcontentlength>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, backupName(ts1), backupName(ts2))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			w.Write([]byte(multistatusXML))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	got, err := adapter.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("List len: got %d, want 2 (entries: %+v)", len(got), got)
	}

	// 目录条目与错误前缀条目必须被过滤掉。
	byName := map[string]BackupInfo{}
	for _, info := range got {
		byName[info.Name] = info
	}

	info1, ok := byName[backupName(ts1)]
	if !ok {
		t.Errorf("missing backup %s in results", backupName(ts1))
	} else {
		if info1.Timestamp != ts1 {
			t.Errorf("timestamp for %s: got %d, want %d", backupName(ts1), info1.Timestamp, ts1)
		}
		if info1.SizeBytes != 42 {
			t.Errorf("size for %s: got %d, want 42", backupName(ts1), info1.SizeBytes)
		}
	}

	info2, ok := byName[backupName(ts2)]
	if !ok {
		t.Errorf("missing backup %s in results", backupName(ts2))
	} else {
		if info2.Timestamp != ts2 {
			t.Errorf("timestamp for %s: got %d, want %d", backupName(ts2), info2.Timestamp, ts2)
		}
		if info2.SizeBytes != 99 {
			t.Errorf("size for %s: got %d, want 99", backupName(ts2), info2.SizeBytes)
		}
	}
}

func TestWebDAVDelete(t *testing.T) {
	var deletedPath string
	store := make(map[string][]byte)

	name := backupName(time.Now().Unix())
	store["/backups/"+name] = []byte("doomed")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			deletedPath = r.URL.Path
			delete(store, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	if err := adapter.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	expectedPath := "/backups/" + name
	if deletedPath != expectedPath {
		t.Errorf("DELETE path: got %q, want %q", deletedPath, expectedPath)
	}
	if _, exists := store[expectedPath]; exists {
		t.Errorf("file %q still exists in store after Delete", expectedPath)
	}
}

func TestWebDAVWriteManifest(t *testing.T) {
	adapter, _, store := newWebDAVTestServer(t)

	manifestData := `{"version":1,"backups":[]}`
	if err := adapter.WriteManifest(manifestData); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	manifestKey := "/backups/manifest.json"
	got, ok := store[manifestKey]
	if !ok {
		t.Fatalf("manifest not found at key %q; store keys: %v", manifestKey, store)
	}
	if string(got) != manifestData {
		t.Errorf("manifest data mismatch:\n got: %q\nwant: %q", got, manifestData)
	}
}

func TestWebDAVDeleteNonexistentIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	// 删除不存在的文件应成功（幂等）。
	if err := adapter.Delete(backupName(9999)); err != nil {
		t.Errorf("Delete on missing file: got %v, want nil", err)
	}
}

func TestWebDAVTarget(t *testing.T) {
	adapter, _, _ := newWebDAVTestServer(t)
	if got := adapter.Target(); got != TargetWebDAV {
		t.Errorf("Target: got %q, want %q", got, TargetWebDAV)
	}
}

func TestWebDAVRestoreNotFound(t *testing.T) {
	adapter, _, _ := newWebDAVTestServer(t)

	dst := filepath.Join(t.TempDir(), "dst.db")
	err := adapter.Restore("presets_backup_does_not_exist.db", dst)
	if err == nil {
		t.Fatalf("Restore of missing backup should fail")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("Restore error: got %v, want errors.Is ErrFileNotFound", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("destination file should not exist after failed Restore (stat err: %v)", statErr)
	}
}

// TestWebDAVBasePathWithSlash 锁定 basePathWithSlash + joinURL 的 URL
// 规范化矩阵。设置默认 webdav_path_prefix 是 "little_timer/"（无前导
// 斜杠）；不规范化就会被黏到 URL 的最后一个 path 段上（".../webdav" +
// "little_timer/" → ".../webdavlittle_timer/"），让所有 WebDAV 操作失效。
func TestWebDAVBasePathWithSlash(t *testing.T) {
	const base = "https://dav.example.test/remote.php/webdav"
	tests := []struct {
		name     string
		basePath string
		want     string
	}{
		{"empty basePath", "", base + "/backup-1.db"},
		{"settings default (no leading slash)", "little_timer/", base + "/little_timer/backup-1.db"},
		{"leading slash only", "/backups", base + "/backups/backup-1.db"},
		{"both slashes", "/backups/", base + "/backups/backup-1.db"},
		{"multi-segment without leading slash", "dav/lt/", base + "/dav/lt/backup-1.db"},
		{"neither slash", "lt", base + "/lt/backup-1.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewWebDAVAdapter(WebDAVConfig{URL: base, BasePath: tt.basePath})
			got := adapter.joinURL(adapter.basePathWithSlash(), "backup-1.db")
			if got != tt.want {
				t.Errorf("joinURL(basePathWithSlash()): got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWebDAVDiagnostics(t *testing.T) {
	t.Run("full config", func(t *testing.T) {
		adapter := NewWebDAVAdapter(WebDAVConfig{
			URL:      "https://dav.example.com/dav",
			BasePath: "lt",
			Username: "u",
			Password: "pwd",
		})
		d := adapter.Diagnostics()
		if d.URL != "https://dav.example.com/dav" {
			t.Errorf("URL = %q, want %q", d.URL, "https://dav.example.com/dav")
		}
		if d.BasePath != "/lt/" {
			t.Errorf("BasePath = %q, want %q", d.BasePath, "/lt/")
		}
		if d.FullProbeURL != "https://dav.example.com/dav/lt/lt_probe_DIAGNOSTIC.tmp" {
			t.Errorf("FullProbeURL = %q, want %q", d.FullProbeURL, "https://dav.example.com/dav/lt/lt_probe_DIAGNOSTIC.tmp")
		}
		if !d.UsernameSet {
			t.Errorf("UsernameSet = false, want true")
		}
		if d.PasswordLen != 3 {
			t.Errorf("PasswordLen = %d, want 3", d.PasswordLen)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		adapter := NewWebDAVAdapter(WebDAVConfig{
			URL:      "https://dav.example.com",
			BasePath: "",
			Username: "",
			Password: "",
		})
		d := adapter.Diagnostics()
		if d.URL != "https://dav.example.com" {
			t.Errorf("URL = %q, want %q", d.URL, "https://dav.example.com")
		}
		if d.BasePath != "/" {
			t.Errorf("BasePath = %q, want %q", d.BasePath, "/")
		}
		if d.UsernameSet {
			t.Errorf("UsernameSet = true, want false")
		}
		if d.PasswordLen != 0 {
			t.Errorf("PasswordLen = %d, want 0", d.PasswordLen)
		}
	})
}

// TestWebDAVBackupRoundTrip_DefaultStylePrefix 是该 P0 bug 的端到端
// 防护：BasePath 为 "little_timer/"（设置默认值）时，PUT 必须落在服务器
// 的 /little_timer/<name>，而不是黏在 host 上。
func TestWebDAVBackupRoundTrip_DefaultStylePrefix(t *testing.T) {
	store := make(map[string][]byte)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			store[key] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(data)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "little_timer/", // 设置默认 webdav_path_prefix
	})
	adapter.client = srv.Client()

	payload := []byte("SQLite-format-3\x00default-prefix-round-trip")
	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, payload)

	name := backupName(time.Now().Unix())
	if err := adapter.Backup(src, name); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// server 必须在规范化路径上收到 PUT，且 map store 的 key 是
	// r.URL.Path —— 黏成 "davlittle_timer/<name>" 的路径
	// （或 invalid-port 解析失败）都会让本断言失败。
	key := "/little_timer/" + name
	if got := store[key]; !bytes.Equal(got, payload) {
		t.Errorf("stored payload at %q: got %q, want %q", key, got, payload)
	}

	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); !bytes.Equal(got, payload) {
		t.Errorf("restored content mismatch:\n got: %q\nwant: %q", got, payload)
	}
}
