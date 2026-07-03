// Package handlers — Wallpaper upload / list / serve / delete.
//
// File `wallpaper.go` ports the wallpaper handlers from std_server.zig.
// Routes:
//
//   POST   /api/wallpapers        → handleWallpaperUpload
//   GET    /api/wallpapers        → handleWallpaperList
//   GET    /api/wallpapers/:id    → handleWallpaperServe
//   DELETE /api/wallpapers/:id    → handleWallpaperDelete
//
// Wallpapers are stored as plain files under
// `<db_dir>/wallpapers/<timestamp>_<safe_name>`.  The DB is consulted
// only to derive `db_dir` (the parent of the SQLite file).  The file
// list endpoint scans the directory directly.
package handlers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// wallpapersDir returns the absolute path to the wallpapers directory,
// creating it on first use.  Mirrors `getWallpapersDir` in std_server.zig.
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

// sanitizeFilename replaces every character outside [a-zA-Z0-9._-] with
// an underscore.  Mirrors Zig `sanitizeFilename`.
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

// handleWallpaperUpload mirrors `handleUploadWallpaper`.  Accepts a
// `multipart/form-data` request with a single "file" field.
func handleWallpaperUpload(c *gin.Context) {
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

	safe := sanitizeFilename(fileHeader.Filename)
	base := filepath.Base(safe)
	ext := filepath.Ext(base)
	stem := base
	if ext != "" && len(ext) < len(base) {
		stem = base[:len(base)-len(ext)]
	}
	unique := fmt.Sprintf("%d_%s%s", time.Now().Unix(), stem, ext)
	dstPath := filepath.Join(dir, unique)
	if err := saveMultipart(src, dstPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"filename": unique})
}

// saveMultipart streams a multipart file part to disk.  We avoid
// loading the whole file in memory because wallpapers can be up to 50MB.
func saveMultipart(src multipart.File, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

// handleWallpaperList mirrors `handleListWallpapers`.  Returns a JSON
// array of {name} objects (filename only — no path exposure).
func handleWallpaperList(c *gin.Context) {
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
		out = append(out, gin.H{"name": e.Name()})
	}
	c.JSON(http.StatusOK, out)
}

// handleWallpaperServe mirrors `handleServeWallpaper`.  Sets the
// Content-Type based on the file extension.
func handleWallpaperServe(c *gin.Context) {
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

// handleWallpaperDelete mirrors `handleDeleteWallpaper`.
func handleWallpaperDelete(c *gin.Context) {
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
	if err := os.Remove(filepath.Join(dir, filename)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to delete file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// mimeByExt maps a file extension to a MIME type.  Mirrors the Zig
// extension switch.
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