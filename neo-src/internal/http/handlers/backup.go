// Package handlers —— Backup + 主口令 + 鉴权 endpoint。
//
// 本文件是 transport adapter：bind HTTP → a.Vault.X → 把 *vault.Error 映射成
// HTTP 响应。业务规则（凭据闸门、主密码 setup-vs-unlock 决策）在
// internal/vault 里。
package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/vault"
)

// /api/backup/config

// BackupConfigGet 返回持久化的 BackupConfig，secret 已打码（`"******"`）。
func BackupConfigGet(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Vault.GetBackupConfig()

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

// BackupConfigUpdate。切换到云端 target 时，vault 会在持久化变更之前强制
// 主口令 + 解锁检查。
func BackupConfigUpdate(c *gin.Context) {
	a := appFromCtx(c)
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "read body"})
		return
	}
	if err := a.Vault.UpdateBackupConfig(c.Request.Context(), string(raw)); err != nil {
		var ve *vault.Error
		if errors.As(err, &ve) {
			c.JSON(http.StatusOK, masterPasswordError(ve.Code, ve.Message, ve.Mode))
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/create

// BackupCreate 触发一次备份。
func BackupCreate(c *gin.Context) {
	a := appFromCtx(c)
	name, err := a.Vault.CreateBackup()
	if err != nil {
		respondBackupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "backup_path": name})
}

// /api/backup/restore

// BackupRestore。Body：{name}。
func BackupRestore(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing name"})
		return
	}
	if err := a.Vault.RestoreBackup(req.Name); err != nil {
		respondBackupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BackupRestoreByName 从 URL path 末段取 backup name。
func BackupRestoreByName(c *gin.Context) {
	a := appFromCtx(c)
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/restore/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := a.Vault.RestoreBackup(name); err != nil {
		respondBackupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/list

// BackupList 列出所有备份；未配置时返回空列表。
func BackupList(c *gin.Context) {
	a := appFromCtx(c)
	items, err := a.Vault.ListBackups()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "backups": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "backups": items})
}

// /api/backup/info

// BackupInfo 返回备份统计；未配置时返回空统计。
func BackupInfo(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Vault.GetBackupConfig()
	if cfg.TargetType == domain.BackupTargetLocal && cfg.LocalPath == "" {
		// 未配置 target 时返回空统计（沿用原 handler 语义）。
	}
	summary, err := a.Vault.BackupSummary()
	if err != nil {
		var ve *vault.Error
		if errors.As(err, &ve) && ve.Code == vault.ErrBackupNotConfigured.Code {
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
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "info": summary})
}

// DELETE /api/backup/delete/:name  /  DELETE /api/backup/:id

// BackupDeleteByName 从 `/api/backup/delete/<name>` 删。
func BackupDeleteByName(c *gin.Context) {
	a := appFromCtx(c)
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/delete/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := a.Vault.DeleteBackup(name); err != nil {
		respondBackupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BackupDelete 处理通用 `DELETE /api/backup/:id` 形式（没有 `/delete/`
// 段时，:id 按备份名解释）。
func BackupDelete(c *gin.Context) {
	a := appFromCtx(c)
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := a.Vault.DeleteBackup(name); err != nil {
		respondBackupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/verify

// BackupVerify 对配置的 adapter 调一次连通性测试。
func BackupVerify(c *gin.Context) {
	a := appFromCtx(c)
	if err := a.Vault.VerifyBackup(); err != nil {
		respondBackupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/unlock  /  /api/backup/lock

func BackupUnlock(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing password"})
		return
	}
	res := a.Vault.UnlockCredentials(req.Password)
	c.JSON(http.StatusOK, gin.H{
		"success":      res.Success,
		"locked_until": res.LockedUntil,
	})
}

func BackupLock(c *gin.Context) {
	a := appFromCtx(c)
	a.Vault.LockCredentials()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// /api/backup/master-password

func MasterPasswordGet(c *gin.Context) {
	a := appFromCtx(c)
	c.JSON(http.StatusOK, a.Vault.GetMasterPasswordStatus())
}

// MasterPasswordSet 要求口令至少 4 个字符。
func MasterPasswordSet(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing password"})
		return
	}
	if err := a.Vault.SetMasterPassword(req.Password); err != nil {
		var ve *vault.Error
		if errors.As(err, &ve) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": ve.Code, "error": ve.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

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

// respondBackupError 把 vault.Operations 返回的错误映射成 HTTP 响应。
//
//   - ErrBackupNotConfigured      → 503 ServiceUnavailable
//   - 主密码 / 凭据错误（有 Mode）→ 200 + masterPasswordError JSON
//     （前端读 action 弹 modal）
//   - 其它 vault.Error（not_enabled 等）→ 200 + 简单 {success:false, error}
//   - 非 vault.Error                     → 500
func respondBackupError(c *gin.Context, err error) {
	var ve *vault.Error
	if !errors.As(err, &ve) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	switch {
	case ve.Code == vault.ErrBackupNotConfigured.Code:
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": ve.Message})
	case ve.Mode != "":
		c.JSON(http.StatusOK, masterPasswordError(ve.Code, ve.Message, ve.Mode))
	default:
		c.JSON(http.StatusOK, gin.H{"success": false, "code": ve.Code, "error": ve.Message})
	}
}

// mask 对非空 secret 返回 "******"，否则返回 ""。
func mask(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}

// masterPasswordError 构造 backup handler 使用的标准“需要主口令” JSON
// 响应。
func masterPasswordError(code, message, actionMode string) gin.H {
	action := gin.H{
		"type":   "show_modal",
		"target": "master_password",
	}
	if actionMode != "" {
		action["params"] = gin.H{"mode": actionMode}
	}
	return gin.H{
		"success": false,
		"error":   code,
		"message": message,
		"action":  action,
	}
}
