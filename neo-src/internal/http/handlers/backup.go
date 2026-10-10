// Package handlers —— Backup + 鉴权 endpoint(主口令端点已随去锁化移除)。
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"little-timer/internal/log"
)

// /api/backup/config

// handleBackupConfigGet 返回持久化的 BackupConfig，secret 已打码
// （`"******"`）。
func BackupConfigGet(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.BackupConfig()

	c.JSON(http.StatusOK, gin.H{
		"enabled":              cfg.Enabled,
		"auto_backup":          cfg.AutoBackup,
		"auto_backup_interval": cfg.AutoBackupSecs,
		"target_type":          cfg.TargetType.String(),
		"local_path":           cfg.LocalPath,
		"webdav_url":           cfg.WebDAVURL,
		"webdav_username":      cfg.WebDAVUsername,
		"webdav_password":      mask(cfg.WebDAVPassword),
		"webdav_path_prefix":   cfg.WebDAVPathPrefix,
		"s3_endpoint":          cfg.S3Endpoint,
		"s3_bucket":            cfg.S3Bucket,
		"s3_region":            cfg.S3Region,
		"s3_access_key":        mask(cfg.S3AccessKey),
		"s3_secret_key":        mask(cfg.S3SecretKey),
		"s3_path_prefix":       cfg.S3PathPrefix,
	})
}

// handleBackupConfigUpdate。去锁化(Task 5):云端 target 的主口令闸门已移除
// —— 凭据改由 keystore 托管,缺凭据时 RebuildBackup 把本进程备份置 nil(下游
// 503),不再要求先设/解锁主口令。凭据端点由后续 Task 9 以 /api/credentials/*
// 重建。
func BackupConfigUpdate(c *gin.Context) {
	a := appFromCtx(c)
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "read body"})
		return
	}

	// 确保 secret store 已建并接线 SettingsManager:UpdateBackupConfigFromJSON
	// 需要已接线的凭据存储才能把 WebDAV/S3 凭据分流到 secrets(Task 4)。
	_ = a.Secrets()

	if err := a.Settings.UpdateBackupConfigFromJSON(string(raw)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	// 配置已持久化；据此重建 manager。重建失败绝不能让请求失败 ——
	// 配置已存下，manager 会在下次配置变更或启动时重建。
	if err := a.RebuildBackup(c.Request.Context()); err != nil {
		log.Error("BackupConfigUpdate: rebuild failed", "error", err.Error())
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/create

// handleBackupCreate。App 里接了 BackupManager 就委托给它；否则返回
// 类 503 错误。
func BackupCreate(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	if !a.Settings.BackupConfig().Enabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "backup not enabled"})
		return
	}
	// 去锁化(Task 5):云端凭据闸门移除。缺凭据时 RebuildBackup 已把 a.Backup
	// 置 nil,上面的 "backup not configured" 503 分支会拦住;凭据校验归 Task 9。

	name, err := bm.CreateBackup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "backup_path": name})
}

// /api/backup/restore

// handleBackupRestore。Body：{name}。
func BackupRestore(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing name"})
		return
	}
	// 去锁化(Task 5):云端凭据闸门移除,缺凭据时 RebuildBackup 已把 bm 置 nil,
	// 上面的 503 分支拦住;凭据校验归 Task 9。
	if err := bm.RestoreFromBackup(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func BackupRestoreByName(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/restore/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := bm.RestoreFromBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/list

func BackupList(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "backups": []any{}})
		return
	}
	items, err := bm.ListBackups()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "backups": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "backups": items})
}

// /api/backup/info

func BackupInfo(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"info": gin.H{
				"total_backups":    0,
				"total_size_bytes": 0,
				"oldest_backup":    nil,
				"newest_backup":    nil,
			},
		})
		return
	}
	summary, err := bm.Summary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"info":    summary,
	})
}

// DELETE /api/backup/delete/:name  /  DELETE /api/backup/:id

func BackupDeleteByName(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/delete/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := bm.DeleteBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BackupDelete 处理通用的 `DELETE /api/backup/:id` 形式（没有 `/delete/`
// 段时，:id 按备份名解释）。
func BackupDelete(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := bm.DeleteBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/verify

// BackupVerify 对配置的 adapter 调用 TestConnection；local adapter 只检查
// 目标目录可达。
func BackupVerify(c *gin.Context) {
	a := appFromCtx(c)
	bm := a.BackupManager()
	if bm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	if !a.Settings.BackupConfig().Enabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "backup not enabled"})
		return
	}
	if err := bm.TestConnection(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// 去锁化(Task 5):/api/backup/unlock、/api/backup/lock、
// /api/backup/master-password 端点随主口令概念一并移除,凭据端点由后续
// Task 9 以 /api/credentials/* 重建。

// /api/auth/*

// 公开路由 —— auth 中间件无 token 也放行。
func AuthStatus(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.Config().Auth
	c.JSON(http.StatusOK, gin.H{
		"auth_enabled": cfg.AuthEnabled,
		"has_token":    cfg.AuthToken != "",
	})
}

// AuthEnable 生成新 token，经 SettingsManager.UpdateAuth 持久化，并返回
// 给客户端保存。
func AuthEnable(c *gin.Context) {
	a := appFromCtx(c)
	token := app.GenerateToken()
	newAuth := a.Settings.Config().Auth
	newAuth.AuthEnabled = true
	newAuth.AuthToken = token
	if err := a.Settings.UpdateAuth(newAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "token": token})
}

func AuthDisable(c *gin.Context) {
	a := appFromCtx(c)
	newAuth := a.Settings.Config().Auth
	newAuth.AuthEnabled = false
	if err := a.Settings.UpdateAuth(newAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// 内部实现。

// mask 对非空 secret 返回 "******"，否则返回 ""。
func mask(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}
