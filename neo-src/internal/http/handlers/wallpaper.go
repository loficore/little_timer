// Package handlers — 壁纸上传 / 列表 / 服务 / 删除。
//
// 壁纸以普通文件形式存于 `<db_dir>/wallpapers/<uuid32>.<ext>`。
// 仅在推导 `db_dir`（SQLite 文件的父目录）时访问数据库。
// 文件列表 endpoint 直接扫描目录。
package handlers

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// wallpapersDir 返回壁纸目录的绝对路径，首次使用时自动创建。
func wallpapersDir(dbPath string) (string, error) {
	dbDir := filepath.Dir(dbPath)
	if dbDir == "" {
		dbDir = "."
	}
	dir := filepath.Join(dbDir, "wallpapers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sanitizeFilename 将 [a-zA-Z0-9._-] 以外的所有字符替换为下划线。
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// WallpaperUpload 接受带单个 "file" 字段的 `multipart/form-data` 请求。
// 采用临时文件 + 50MB 上限 + UUID 命名 + 图像压缩。
func WallpaperUpload(c *gin.Context) {
	a := appFromCtx(c)

	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "wallpapers dir not available"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "missing file"})
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}
	defer src.Close()

	// 扩展名取自原始文件名（已净化）。
	safe := sanitizeFilename(fileHeader.Filename)
	ext := filepath.Ext(filepath.Base(safe))

	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to create temp file"})
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	limitReader := io.LimitReader(src, 50*1024*1024+1)
	written, err := io.Copy(tmp, limitReader)
	if err != nil {
		tmp.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to receive file"})
		return
	}
	if err := tmp.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to flush temp file"})
		return
	}
	if written > 50*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "file too large"})
		return
	}

	tmpFile, err := os.Open(tmpName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to open temp file"})
		return
	}
	defer tmpFile.Close()

	processed, outExt, err := processWallpaperImage(tmpFile, ext)
	if err != nil {
		switch err {
		case ErrWallpaperDimensionsTooLarge:
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "image dimensions too large"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"err": "invalid image: " + err.Error()})
		}
		return
	}

	finalName := newUUIDHex() + outExt
	dstPath := filepath.Join(dir, finalName)
	if err := os.WriteFile(dstPath, processed, 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to save wallpaper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filename": finalName})
}

// WallpaperList 返回 {name, size, refs} 对象的 JSON 数组——
// 仅文件名（不暴露路径）、磁盘字节大小，以及 habits/habit_sets/settings
// 中通过 `local:<name>` 引用该壁纸的行数。
func WallpaperList(c *gin.Context) {
	a := appFromCtx(c)
	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	out := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, e.Name()))
		if err != nil {
			// 跳过不可读条目——与"仅可读文件"语义一致。
			continue
		}
		refs, err := a.SQLite.CountWallpaperRefs("local:" + e.Name())
		if err != nil {
			// 引用计数查询失败，按无引用处理。
			refs = 0
		}
		out = append(out, gin.H{"name": e.Name(), "size": info.Size(), "refs": refs})
	}
	c.JSON(http.StatusOK, out)
}

// WallpaperServe 根据文件扩展名设置 Content-Type。
func WallpaperServe(c *gin.Context) {
	a := appFromCtx(c)
	filename := c.Param("id")
	if filename == "" || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid filename"})
		return
	}
	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Wallpapers dir not found"})
		return
	}
	filePath := filepath.Join(dir, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"err": "File not found"})
		return
	}
	if info.Size() > 50*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "File too large"})
		return
	}
	c.Header("Content-Type", mimeByExt(filepath.Ext(filename)))
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(filePath)
}

// WallpaperDelete 先解绑再删除壁纸文件。
//
// 顺序对一致性至关重要:先在一个事务内解除所有 DB 对该壁纸的引用
// （habits / habit_sets / settings），再物理删除文件。解绑失败则返回 500
// 且文件保持原样——不留悬空的 `local:` 引用。文件删除失败（如文件缺失）
// 同样返回 500；此时 DB 已解绑，可安全重试。
func WallpaperDelete(c *gin.Context) {
	a := appFromCtx(c)
	filename := c.Param("id")
	if filename == "" || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid filename"})
		return
	}
	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Wallpapers dir not found"})
		return
	}
	unbound, err := a.SQLite.UnbindWallpaper("local:" + filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to unbind wallpaper"})
		return
	}
	if err := os.Remove(filepath.Join(dir, filename)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to delete file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "unbound": unbound})
}

// mimeByExt 将文件扩展名映射为 MIME 类型。
func mimeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

// contentTypeToExt 将 Content-Type 头值映射为文件扩展名，无法识别时返回 ""。
//
// 匹配不区分大小写，依据 RFC 7231 §3.1.1.1（媒体类型大小写不敏感）:
// "IMAGE/PNG" 必须和 "image/png" 一样映射到 ".png"。
func contentTypeToExt(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/bmp":
		return ".bmp"
	default:
		return ""
	}
}

// errURLHostNotAllowed 标记被 SSRF 策略拦截的 URL host。
var errURLHostNotAllowed = errors.New("URL host not allowed")

// netLookupIP 是 isBlockedHost 使用的主机名解析器；声明为包级变量，
// 便于测试时替换，不触碰真实 DNS。
var netLookupIP = net.LookupIP

// isBlockedIP 判断 IP 是否落在壁纸下载器绝不可访问的网段内。
// loopback（127.0.0.0/8、::1）是有意放行的:这是一款个人桌面应用，
// 攻击者本就能触达本机；放开 loopback 可让 dev/test 流程使用绑定在
// 127.0.0.1 上的 httptest 服务器。真正的 SSRF 目标是云 metadata
// （169.254.169.254）和内网网段，下面全部封禁。
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// IPv4-mapped IPv6 地址（如 ::ffff:127.0.0.1）内嵌 IPv4 地址；
	// 先归一化，私有/loopback 检查才能看到它。
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	// link-local IPv4（169.254.0.0/16）不在 net.IP.IsPrivate() 覆盖范围内，
	// 而云 metadata endpoint 169.254.169.254 就在其中。
	if ip.IsLinkLocalUnicast() {
		return true
	}
	// 显式放行 loopback，再封禁其余不可路由网段:RFC 1918 私有
	// （10/8、172.16/12、192.168/16）、IPv6 唯一本地（fc00::/7）、
	// link-local、组播与未指定地址。
	if ip.IsLoopback() {
		return false
	}
	return ip.IsPrivate() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalMulticast()
}

// isBlockedHost 判断解析 `host` 后是否出现任何被 SSRF 拦截的地址。
// 裸 IP 字面量直接解析；否则只要有任一解析出的地址被拦截，整个 host
// 就被拦截——否则控制 DNS 的攻击者可以混合公网与私有应答来命中内网。
// 解析失败一律保守视为拦截:无法证明 host 安全，就拒绝抓取。
func isBlockedHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return isBlockedIP(ip)
	}
	addrs, err := netLookupIP(host)
	if err != nil {
		return true
	}
	if len(addrs) == 0 {
		return true
	}
	for _, ip := range addrs {
		if isBlockedIP(ip) {
			return true
		}
	}
	return false
}

// WallpaperFromURL 从 URL 下载壁纸，走与 WallpaperUpload 相同的
// decode/scale/encode 管线处理，并以 UUID 文件名保存。
func WallpaperFromURL(c *gin.Context) {
	a := appFromCtx(c)

	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "wallpapers dir not available"})
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"err": "missing url field"})
		return
	}

	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"err": "invalid url scheme"})
		return
	}

	// SSRF 防护:对初始 host 拦截指向私有 / link-local / metadata 网段的抓取。
	// 重定向目标在下方 CheckRedirect 中再次检查（即使原始 URL 是公网地址，
	// 重定向也可能把抓取引向内网 host）。
	if isBlockedHost(u.Hostname()) {
		c.JSON(http.StatusBadRequest, gin.H{"err": errURLHostNotAllowed.Error()})
		return
	}

	redirects := 0
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirects++
			if redirects > 5 {
				return fmt.Errorf("too many redirects")
			}
			// 拒绝跟随重定向进入被封禁的网段。
			if isBlockedHost(req.URL.Hostname()) {
				return errURLHostNotAllowed
			}
			return nil
		},
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, req.URL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "invalid url"})
		return
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		if errors.Is(err, errURLHostNotAllowed) {
			c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "Timeout") {
			c.JSON(http.StatusGatewayTimeout, gin.H{"err": "upstream timeout"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"err": "upstream fetch failed"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"err": "upstream returned non-2xx"})
		return
	}

	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}

	ext := contentTypeToExt(ct)
	if ext == "" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"err": "unsupported content type"})
		return
	}

	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to create temp file"})
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	limitReader := io.LimitReader(resp.Body, 50*1024*1024+1)
	written, err := io.Copy(tmp, limitReader)
	if err != nil {
		tmp.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to download file"})
		return
	}
	if err := tmp.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to flush temp file"})
		return
	}
	if written > 50*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "file too large"})
		return
	}

	tmpFile, err := os.Open(tmpName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to open temp file"})
		return
	}
	defer tmpFile.Close()

	processed, outExt, err := processWallpaperImage(tmpFile, ext)
	if err != nil {
		switch err {
		case ErrWallpaperDimensionsTooLarge:
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "image dimensions too large"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"err": "invalid image: " + err.Error()})
		}
		return
	}

	finalName := newUUIDHex() + outExt
	dstPath := filepath.Join(dir, finalName)
	if err := os.WriteFile(dstPath, processed, 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to save wallpaper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filename": finalName})
}
