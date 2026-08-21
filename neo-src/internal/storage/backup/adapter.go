// Package backup —— adapter interface + Local / WebDAV / S3 实现。
//
// 命名：
//
//   - BackupAdapter —— interface。
//   - LocalAdapter —— 把 DB 文件拷贝到本地目录。
//   - WebDAVAdapter —— 通过 HTTP（PUT/GET/PROPFIND/DELETE）+ HTTP Basic
//     鉴权上传/下载。使用标准库 `net/http`；x/net/webdav 包是服务端框架
//     而非客户端，所以客户端就该用标准库（x/net/webdav 仍需保留在 go.mod
//     里满足 W4 依赖要求）。
//   - S3Adapter —— 包装 `aws-sdk-go-v2/service/s3`，对接 S3 兼容存储
//     （AWS、MinIO、Backblaze B2 等）。
//
// 文件名约定：每个备份命名为
// `presets_backup_<unix-seconds>.db`。`List()` 按前缀 + 后缀过滤，
// 目标目录里的杂散文件会被忽略。
package backup

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"little-timer/internal/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// BackupError 是类型化哨兵错误，调用方可用 errors.Is 匹配。
type BackupError string

const (
	ErrBackupFailed       BackupError = "backup failed"
	ErrRestoreFailed      BackupError = "restore failed"
	ErrInvalidBackupPath  BackupError = "invalid backup path"
	ErrConnectionFailed   BackupError = "connection failed"
	ErrAuthenticationFail BackupError = "authentication failed"
	ErrFileNotFound       BackupError = "file not found"
	ErrPermissionDenied   BackupError = "permission denied"
	ErrNetworkError       BackupError = "network error"
)

func (e BackupError) Error() string { return string(e) }

// BackupTarget 选择要实例化哪个 adapter。
type BackupTarget string

const (
	TargetLocal  BackupTarget = "local"
	TargetWebDAV BackupTarget = "webdav"
	TargetS3     BackupTarget = "s3"
)

type BackupInfo struct {
	Name      string `json:"name"`
	Timestamp int64  `json:"timestamp"`
	SizeBytes uint64 `json:"size_bytes"`
}

// filenamePrefix / filenameSuffix 是 List() 使用的过滤器：文件名以
// 前缀开头且以后缀结尾才算作备份。
const (
	filenamePrefix = "presets_backup_"
	filenameSuffix = ".db"
)

// BackupAdapter 是 BackupManager 派发依赖的 interface。
// TestConnection 是一项 UI 辅助能力，无需尝试备份就能暴露凭据问题。
type BackupAdapter interface {
	// Backup 把 srcPath 处的文件以 backupName 为名拷贝进 adapter。
	Backup(srcPath, backupName string) error
	// Restore 取回 backupName 并写入 destPath。
	Restore(backupName, destPath string) error
	// List 枚举远端存储的全部备份。
	List() ([]BackupInfo, error)
	// Delete 删除单个备份。
	Delete(backupName string) error
	// TestConnection 校验凭据 / 网络可达性。
	TestConnection() error
	// WriteManifest 把备份 manifest JSON 写入 adapter 的 base path。
	WriteManifest(data string) error
	// Target 返回判别值（local / webdav / s3）。
	Target() BackupTarget
}

// LocalAdapter 把备份写入本地目录。
type LocalAdapter struct {
	path string
}

// NewLocalAdapter 返回以 path 为根的 LocalAdapter。首次使用时若路径
// 不存在会创建。
func NewLocalAdapter(path string) *LocalAdapter {
	return &LocalAdapter{path: path}
}

func (l *LocalAdapter) Target() BackupTarget { return TargetLocal }

func (l *LocalAdapter) TestConnection() error {
	if err := os.MkdirAll(l.path, 0o700); err != nil {
		return fmt.Errorf("%w: mkdir %s: %v", ErrConnectionFailed, l.path, err)
	}
	probeName := fmt.Sprintf("lt_probe_%d.tmp", time.Now().UnixNano())
	probePath := filepath.Join(l.path, probeName)
	probeContent := []byte("lt-probe-ok")

	if err := os.WriteFile(probePath, probeContent, 0o600); err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("%w: write probe: %v", ErrConnectionFailed, err)
	}
	got, err := os.ReadFile(probePath)
	if err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("%w: read probe: %v", ErrConnectionFailed, err)
	}
	if !bytes.Equal(got, probeContent) {
		_ = os.Remove(probePath)
		return fmt.Errorf("%w: probe content mismatch", ErrConnectionFailed)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("%w: delete probe: %v", ErrConnectionFailed, err)
	}
	return nil
}

func (l *LocalAdapter) Backup(srcPath, backupName string) error {
	if err := l.TestConnection(); err != nil {
		return err
	}
	dst := filepath.Join(l.path, backupName)
	if err := copyFile(srcPath, dst); err != nil {
		return fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	return nil
}

func (l *LocalAdapter) Restore(backupName, destPath string) error {
	src := filepath.Join(l.path, backupName)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrFileNotFound, src)
		}
		return fmt.Errorf("%w: stat: %v", ErrRestoreFailed, err)
	}
	if err := copyFile(src, destPath); err != nil {
		return fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}
	return nil
}

func (l *LocalAdapter) Delete(backupName string) error {
	full := filepath.Join(l.path, backupName)
	if err := os.Remove(full); err != nil {
		if os.IsNotExist(err) {
			return nil // delete 时 not-found 视为 OK。
		}
		return fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	return nil
}

func (l *LocalAdapter) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: readdir: %v", ErrBackupFailed, err)
	}
	out := make([]BackupInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, filenamePrefix) || !strings.HasSuffix(name, filenameSuffix) {
			continue
		}
		ts, ok := timestampFromName(name)
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{
			Name:      name,
			Timestamp: ts,
			SizeBytes: uint64(info.Size()),
		})
	}
	return out, nil
}

func (l *LocalAdapter) WriteManifest(data string) error {
	return os.WriteFile(filepath.Join(l.path, "manifest.json"), []byte(data), 0o600)
}

// WebDAVConfig 持有 WebDAVAdapter 的连接参数。
type WebDAVConfig struct {
	URL      string // 例如 https://dav.example.com/remote.php/webdav
	Username string
	Password string
	BasePath string // 服务端相对路径前缀；默认 "/"
}

// NewWebDAVAdapter 返回配置好的 WebDAVAdapter。
func NewWebDAVAdapter(cfg WebDAVConfig) *WebDAVAdapter {
	if cfg.BasePath == "" {
		cfg.BasePath = "/"
	}
	return &WebDAVAdapter{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}

// WebDAVAdapter 用标准库 net/http 执行 PUT / GET / DELETE / PROPFIND。
// WebDAV 服务器说 HTTP，因此对 Nextcloud、Apache mod_dav、
// 带 dav-module 的 Nginx 等都能工作。
type WebDAVAdapter struct {
	cfg    WebDAVConfig
	client *http.Client
}

// WebDAVDiagnostics 是 adapter 解析后配置的快照。
type WebDAVDiagnostics struct {
	URL          string // 原始 w.cfg.URL
	BasePath     string // 规范化：basePathWithSlash() 的结果
	FullProbeURL string // 模板：joinURL(basePathWithSlash(), "lt_probe_DIAGNOSTIC.tmp")
	UsernameSet  bool   // w.cfg.Username != "" 时为 true
	PasswordLen  int    // !UsernameSet 时为 0，否则为 len(w.cfg.Password)
}

func (w *WebDAVAdapter) Target() BackupTarget { return TargetWebDAV }

// Diagnostics 返回 adapter 解析后配置的快照，供 pre-flight 检查。
// 不发起任何 HTTP 请求。
func (w *WebDAVAdapter) Diagnostics() WebDAVDiagnostics {
	bp := w.basePathWithSlash()
	return WebDAVDiagnostics{
		URL:          w.cfg.URL,
		BasePath:     bp,
		FullProbeURL: w.joinURL(bp, "lt_probe_DIAGNOSTIC.tmp"),
		UsernameSet:  w.cfg.Username != "",
		PasswordLen: func() int {
			if w.cfg.Username != "" {
				return len(w.cfg.Password)
			}
			return 0
		}(),
	}
}

// TestConnection 在 base path 上执行 PUT-probe → GET-verify →
// DELETE-cleanup 的写入循环，确认 WebDAV 服务器可写。
func (w *WebDAVAdapter) TestConnection() error {
	authStatus := "none"
	if w.cfg.Username != "" {
		authStatus = "basic"
	}

	probeName := fmt.Sprintf("lt_probe_%d.tmp", time.Now().UnixNano())
	probeURL := w.joinURL(w.basePathWithSlash(), probeName)
	probeContent := []byte("lt-probe-ok")

	log.Debug("webdav probe", "url", probeURL)

	putReq, err := http.NewRequest(http.MethodPut, probeURL, bytes.NewReader(probeContent))
	if err != nil {
		return fmt.Errorf("%w: build PUT probe (auth=%s, body=%q): %v", ErrConnectionFailed, authStatus, "", err)
	}
	putReq.Header.Set("Content-Type", "application/octet-stream")
	w.applyAuth(putReq)
	log.Debug("webdav put", "url", probeURL, "auth", authStatus)
	putResp, err := w.client.Do(putReq)
	if err != nil {
		return fmt.Errorf("%w: PUT %s -> (auth=%s, body=%q): %v", ErrConnectionFailed, probeURL, authStatus, "", err)
	}

	b, _ := io.ReadAll(io.LimitReader(putResp.Body, 500))
	putResp.Body.Close()
	body := string(b)

	log.Debug("webdav put response", "status", putResp.StatusCode)

	if putResp.StatusCode == http.StatusUnauthorized {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: PUT %s -> %d (auth=%s, body=%q)", ErrAuthenticationFail, probeURL, putResp.StatusCode, authStatus, body)
	}
	if putResp.StatusCode == http.StatusForbidden {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: PUT %s -> %d (auth=%s, body=%q)", ErrPermissionDenied, probeURL, putResp.StatusCode, authStatus, body)
	}
	if putResp.StatusCode >= 300 {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: PUT %s -> %d (auth=%s, body=%q)", ErrConnectionFailed, probeURL, putResp.StatusCode, authStatus, body)
	}

	getReq, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: build GET probe (auth=%s, body=%q): %v", ErrConnectionFailed, authStatus, "", err)
	}
	w.applyAuth(getReq)
	getResp, err := w.client.Do(getReq)
	if err != nil {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: GET %s -> (auth=%s, body=%q): %v", ErrConnectionFailed, probeURL, authStatus, "", err)
	}

	log.Debug("webdav get response", "status", getResp.StatusCode)

	if getResp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(getResp.Body, 500))
		getResp.Body.Close()
		body := string(b)
		_ = w.deleteProbe(probeURL)
		if getResp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("%w: GET %s -> %d (auth=%s, body=%q)", ErrAuthenticationFail, probeURL, getResp.StatusCode, authStatus, body)
		}
		if getResp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("%w: GET %s -> %d (auth=%s, body=%q)", ErrPermissionDenied, probeURL, getResp.StatusCode, authStatus, body)
		}
		return fmt.Errorf("%w: GET %s -> %d (auth=%s, body=%q)", ErrConnectionFailed, probeURL, getResp.StatusCode, authStatus, body)
	}
	got, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: read GET probe (auth=%s, body=%q): %v", ErrConnectionFailed, authStatus, "<read error>", err)
	}
	if !bytes.Equal(got, probeContent) {
		_ = w.deleteProbe(probeURL)
		return fmt.Errorf("%w: probe content mismatch (auth=%s, body=%q)", ErrConnectionFailed, authStatus, "")
	}

	log.Debug("webdav probe cleanup", "url", probeURL)

	if err := w.deleteProbe(probeURL); err != nil {
		return fmt.Errorf("%w: DELETE %s -> (auth=%s, body=%q): %v", ErrConnectionFailed, probeURL, authStatus, "", err)
	}
	return nil
}

// deleteProbe 向 probe URL 发送 DELETE 请求；错误被吞掉
// （供 TestConnection 的尽力清理使用）。
func (w *WebDAVAdapter) deleteProbe(probeURL string) error {
	req, err := http.NewRequest(http.MethodDelete, probeURL, nil)
	if err != nil {
		return err
	}
	w.applyAuth(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (w *WebDAVAdapter) Backup(srcPath, backupName string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("%w: open src: %v", ErrBackupFailed, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%w: stat src: %v", ErrBackupFailed, err)
	}

	url := w.joinURL(w.basePathWithSlash(), backupName)
	req, err := http.NewRequest(http.MethodPut, url, f)
	if err != nil {
		return fmt.Errorf("%w: build PUT: %v", ErrBackupFailed, err)
	}
	req.ContentLength = stat.Size()
	// GetBody 绝不能复用 request body 的那个 *os.File：transport 可能在
	// writeLoop 仍从共享文件读取时调用 GetBody（307/308 redirect / 重试），
	// 在同一个 fd 上让 Read 和 Seek 相互竞争。每次调用打开新句柄就没有
	// 竞争；transport 会关闭 GetBody 返回的内容。
	req.GetBody = func() (io.ReadCloser, error) {
		return os.Open(srcPath)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("%w: rewind src: %v", ErrBackupFailed, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	w.applyAuth(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: PUT: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w: PUT status %d", ErrBackupFailed, resp.StatusCode)
	}
	return nil
}

func (w *WebDAVAdapter) WriteManifest(data string) error {
	url := w.joinURL(w.basePathWithSlash(), "manifest.json")
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: build PUT: %v", ErrBackupFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")
	w.applyAuth(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: PUT manifest: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w: PUT manifest status %d", ErrBackupFailed, resp.StatusCode)
	}
	return nil
}

func (w *WebDAVAdapter) Restore(backupName, destPath string) error {
	url := w.joinURL(w.basePathWithSlash(), backupName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("%w: build GET: %v", ErrRestoreFailed, err)
	}
	w.applyAuth(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: GET: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrFileNotFound, backupName)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w: GET status %d", ErrRestoreFailed, resp.StatusCode)
	}
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("%w: create dest: %v", ErrRestoreFailed, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, resp.Body); err != nil {
		_ = os.Remove(destPath) // 不留下半截文件
		return fmt.Errorf("%w: copy body: %v", ErrRestoreFailed, err)
	}
	return nil
}

func (w *WebDAVAdapter) Delete(backupName string) error {
	url := w.joinURL(w.basePathWithSlash(), backupName)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("%w: build DELETE: %v", ErrBackupFailed, err)
	}
	w.applyAuth(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: DELETE: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%w: DELETE status %d", ErrBackupFailed, resp.StatusCode)
	}
	return nil
}

// List 发送 PROPFIND Depth: 1 并解析 multistatus 响应。
// XML schema 遵循 RFC 4918。
func (w *WebDAVAdapter) List() ([]BackupInfo, error) {
	body := strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>` +
		`<D:propfind xmlns:D="DAV:"><D:prop><D:getlastmodified/><D:getcontentlength/></D:prop></D:propfind>`)
	url := w.joinURL(w.basePathWithSlash(), "")
	req, err := http.NewRequest("PROPFIND", url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: build PROPFIND: %v", ErrBackupFailed, err)
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	w.applyAuth(req)
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: PROPFIND: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: PROPFIND status %d", ErrBackupFailed, resp.StatusCode)
	}
	return parsePropfindResponse(resp.Body)
}

// webdavResponse 映射我们要解析的 multistatus XML 子集。
type webdavResponse struct {
	XMLName   xml.Name `xml:"response"`
	Href      string   `xml:"href"`
	PropStats []struct {
		Prop struct {
			GetLastModified  string `xml:"getlastmodified"`
			GetContentLength int64  `xml:"getcontentlength"`
			ResourceType     struct {
				Collection *struct{} `xml:"collection"`
			} `xml:"resourcetype"`
		} `xml:"prop"`
		Status string `xml:"status"`
	} `xml:"propstat"`
}

type webdavMultistatus struct {
	XMLName   xml.Name         `xml:"multistatus"`
	Responses []webdavResponse `xml:"response"`
}

func parsePropfindResponse(r io.Reader) ([]BackupInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrBackupFailed, err)
	}
	var ms webdavMultistatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("%w: parse XML: %v", ErrBackupFailed, err)
	}
	out := make([]BackupInfo, 0, len(ms.Responses))
	for _, resp := range ms.Responses {
		name := pathBase(resp.Href)
		if !strings.HasPrefix(name, filenamePrefix) || !strings.HasSuffix(name, filenameSuffix) {
			continue
		}
		ts, ok := timestampFromName(name)
		if !ok {
			continue
		}
		var size uint64
		var modified int64
		for _, ps := range resp.PropStats {
			if ps.Status != "" && !strings.Contains(ps.Status, "200") {
				continue
			}
			if ps.Prop.GetContentLength > 0 {
				size = uint64(ps.Prop.GetContentLength)
			}
			if t, perr := http.ParseTime(ps.Prop.GetLastModified); perr == nil {
				modified = t.Unix()
			}
		}
		_ = modified // 当前未使用；时间戳取自文件名。
		out = append(out, BackupInfo{Name: name, Timestamp: ts, SizeBytes: size})
	}
	return out, nil
}

// basePathWithSlash 把 BasePath 规范化为服务端相对路径，前后都带 "/"
// （如 "little_timer/" → "/little_timer/"）。前导斜杠必不可少：
// joinURL 会把 basePath 拼到 URL 的最后一个 path 段上，缺了前导斜杠
// 就会把前缀黏到那段上（".../dav" + "little_timer/" →
// ".../davlittle_timer/"）。
func (w *WebDAVAdapter) basePathWithSlash() string {
	bp := w.cfg.BasePath
	if bp == "" {
		bp = "/"
	}
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	if !strings.HasSuffix(bp, "/") {
		bp += "/"
	}
	return bp
}

// joinURL 安全地拼出 `${URL}${basePath}${name}`。URL 应已带
// scheme/host（不要求末尾斜杠）。
func (w *WebDAVAdapter) joinURL(basePath, name string) string {
	u := strings.TrimRight(w.cfg.URL, "/")
	return u + basePath + name
}

func (w *WebDAVAdapter) applyAuth(req *http.Request) {
	if w.cfg.Username != "" {
		req.SetBasicAuth(w.cfg.Username, w.cfg.Password)
		log.Debug("webdav auth", "username", w.cfg.Username, "password_len", len(w.cfg.Password))
	} else {
		log.Debug("webdav auth skipped", "reason", "username empty")
	}
}

// S3Config 持有 S3Adapter 的连接参数。
type S3Config struct {
	Endpoint   string // 例如 https://s3.amazonaws.com 或 https://minio.local:9000
	Bucket     string
	Region     string
	AccessKey  string
	SecretKey  string
	PathPrefix string // 服务端相对前缀；默认 "little_timer/"
	// PathStyle 切换 path-style 寻址（MinIO 必需）。
	PathStyle bool
}

// S3APIClient 是 S3Adapter 用到的 *s3.Client 方法子集。
// 抽成 interface 是为了让测试能替换假的 S3 client。
type S3APIClient interface {
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// NewS3Adapter 用内联 AWS config（AccessKey + SecretKey）构造
// S3Adapter。Endpoint / region 取自 cfg。
func NewS3Adapter(ctx context.Context, cfg S3Config) (*S3Adapter, error) {
	if cfg.Bucket == "" || cfg.Region == "" {
		return nil, errors.New("s3: bucket and region are required")
	}
	if cfg.PathPrefix == "" {
		cfg.PathPrefix = "little_timer/"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.PathStyle
	})
	return &S3Adapter{cfg: cfg, client: client}, nil
}

// S3Adapter 通过 AWS SDK v2 存备份（go.mod 已钉版）。
// 可经自定义 BaseEndpoint 与 path-style 寻址对接 S3 兼容端点
// （MinIO、R2、B2）。
type S3Adapter struct {
	cfg    S3Config
	client S3APIClient
}

func (s *S3Adapter) Target() BackupTarget { return TargetS3 }

func (s *S3Adapter) keyFor(backupName string) string {
	return strings.TrimRight(s.cfg.PathPrefix, "/") + "/" + backupName
}

func (s *S3Adapter) TestConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.cfg.Bucket)})
	if err != nil {
		return fmt.Errorf("%w: HeadBucket: %v", classifyS3Error(err), err)
	}

	probeName := fmt.Sprintf("lt_probe_%d.tmp", time.Now().UnixNano())
	probeKey := s.keyFor(probeName)
	probeContent := strings.NewReader("lt-probe-ok")

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(probeKey),
		Body:   probeContent,
	})
	if err != nil {
		_ = s.deleteProbeKey(ctx, probeKey)
		return fmt.Errorf("%w: PutObject probe: %v", classifyS3Error(err), err)
	}

	getOut, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(probeKey),
	})
	if err != nil {
		_ = s.deleteProbeKey(ctx, probeKey)
		return fmt.Errorf("%w: GetObject probe: %v", classifyS3Error(err), err)
	}
	if getOut.Body == nil {
		_ = s.deleteProbeKey(ctx, probeKey)
		return fmt.Errorf("%w: GetObject returned nil body", ErrConnectionFailed)
	}
	got, err := io.ReadAll(getOut.Body)
	getOut.Body.Close()
	if err != nil || string(got) != "lt-probe-ok" {
		_ = s.deleteProbeKey(ctx, probeKey)
		return fmt.Errorf("%w: probe content mismatch", ErrConnectionFailed)
	}

	if err := s.deleteProbeKey(ctx, probeKey); err != nil {
		return fmt.Errorf("%w: DeleteObject probe: %v", classifyS3Error(err), err)
	}
	return nil
}

// deleteProbeKey 尽力发送 DeleteObject；错误被吞掉。
func (s *S3Adapter) deleteProbeKey(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Adapter) Backup(srcPath, backupName string) error {
	body, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("%w: open src: %v", ErrBackupFailed, err)
	}
	defer body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(s.keyFor(backupName)),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", classifyS3Error(err), err)
	}
	return nil
}

func (s *S3Adapter) Restore(backupName, destPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(s.keyFor(backupName)),
	})
	if err != nil {
		return fmt.Errorf("%w: %v", classifyS3Error(err), err)
	}
	defer out.Body.Close()
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("%w: create dest: %v", ErrRestoreFailed, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, out.Body); err != nil {
		return fmt.Errorf("%w: copy body: %v", ErrRestoreFailed, err)
	}
	return nil
}

func (s *S3Adapter) Delete(backupName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(s.keyFor(backupName)),
	})
	if err != nil {
		return fmt.Errorf("%w: %v", classifyS3Error(err), err)
	}
	return nil
}

func (s *S3Adapter) WriteManifest(data string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	manifestKey := s.keyFor("manifest.json")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.Bucket),
		Key:         aws.String(manifestKey),
		Body:        strings.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("%w: %v", classifyS3Error(err), err)
	}
	return nil
}

func (s *S3Adapter) List() ([]BackupInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.Bucket),
		Prefix: aws.String(strings.TrimRight(s.cfg.PathPrefix, "/") + "/"),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", classifyS3Error(err), err)
	}
	results := make([]BackupInfo, 0, len(out.Contents))
	for _, obj := range out.Contents {
		if obj.Key == nil || obj.LastModified == nil {
			continue
		}
		full := *obj.Key
		// 剥掉配置前缀，还原裸备份名。
		name := strings.TrimPrefix(full, strings.TrimRight(s.cfg.PathPrefix, "/")+"/")
		if !strings.HasPrefix(name, filenamePrefix) || !strings.HasSuffix(name, filenameSuffix) {
			continue
		}
		size := uint64(0)
		if obj.Size != nil {
			size = uint64(*obj.Size)
		}
		// 时间戳取自文件名（与 Local/WebDAV 一致）；LastModified 只是回退。
		ts := obj.LastModified.Unix()
		if parsed, ok := timestampFromName(name); ok {
			ts = parsed
		}
		results = append(results, BackupInfo{
			Name:      name,
			Timestamp: ts,
			SizeBytes: size,
		})
	}
	return results, nil
}

// classifyS3Error 把 S3 SDK 的错误类型映射为哨兵 BackupError 值。
func classifyS3Error(err error) error {
	var (
		nsb *types.NoSuchBucket
		nsk *types.NoSuchKey
		gae *smithy.GenericAPIError
		re  *smithyhttp.ResponseError
		ue  *url.Error
	)
	switch {
	case errors.As(err, &nsb):
		return ErrFileNotFound
	case errors.As(err, &nsk):
		return ErrFileNotFound
	case errors.As(err, &gae):
		switch gae.Code {
		case "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return ErrAuthenticationFail
		case "AccessDenied":
			return ErrPermissionDenied
		}
		return ErrConnectionFailed
	case errors.As(err, &re):
		switch re.HTTPStatusCode() {
		case 401:
			return ErrAuthenticationFail
		case 403:
			return ErrPermissionDenied
		case 404:
			return ErrFileNotFound
		default:
			return ErrNetworkError
		}
	case errors.As(err, &ue):
		return ErrNetworkError
	}
	return ErrConnectionFailed
}

// copyFile 流式拷贝 + chmod（0600）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// timestampFromName 从 `presets_backup_<ts>.db` 中提取 unix 秒字段。
// 名称不符合预期形状时返回 false。
func timestampFromName(name string) (int64, bool) {
	mid := strings.TrimSuffix(strings.TrimPrefix(name, filenamePrefix), filenameSuffix)
	if mid == "" || mid == name {
		return 0, false
	}
	ts, err := strconv.ParseInt(mid, 10, 64)
	if err != nil {
		return 0, false
	}
	return ts, true
}

// pathBase 返回 URL 解码后 href 的末尾 path 段。
func pathBase(href string) string {
	if i := strings.LastIndex(href, "/"); i >= 0 {
		return href[i+1:]
	}
	return href
}
