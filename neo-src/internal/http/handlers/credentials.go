// Package handlers —— 凭据端点（取代 v1 的主口令端点，issue #13）。
//
// 主口令时代的三件事在这里重新表达，但语义完全不同：
//
//	旧                                  新
//	GET  /api/backup/master-password    GET  /api/credentials/status
//	POST /api/backup/master-password    POST /api/credentials/reset
//	POST /api/backup/unlock             POST /api/credentials/migrate
//	POST /api/backup/lock               （无对应物——没有"锁"这个概念）
//
// reset 是破坏性的：它删掉 keystore 里的 master key 并清空全部凭据，
// 用户之后要重新填一遍 LLM Key / 备份凭据。所以它不做隐式确认，
// 由前端负责弹确认框。
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/keystore"
	"little-timer/internal/log"
)

// CredentialsStatus 返回凭据子系统健康度。
//
// 200 + domain.CredentialStatus。载入失败态（磁盘有 secret.db 但 master key
// 不在托管中 / 解封失败）也返 200，且 backend="unavailable" + detail 非空——
// 不把"读不出凭据"谎报成"没有凭据"，否则前端会引导用户去重新填写，掩盖掉
// 真正的环境问题（keyring 被重置等）。
func CredentialsStatus(c *gin.Context) {
	a := appFromCtx(c)
	c.JSON(http.StatusOK, a.CredentialStatus())
}

// CredentialsReset 清空全部凭据并重建 master key。
//
// 副作用（顺序固定）：
//  1. DeleteMasterKey + EnsureMasterKey —— 换一把全新的 master key；
//     旧 key 加密的一切都再也解不开，这是"重置"的本质。
//  2. Secrets().Clear() —— 落一份空的 v2 envelope，确认新 key 的读写链路通。
//  3. 清 SQLite 侧的 has_* 标记与旧 * _encrypted / 主口令标记列 ——
//     否则重置后 settings 仍声称"有凭据"，下次保存会把凭据写进一个
//     与 master key 无关的 store。
//
// 204 无内容。失败一律 5xx（调用方是显式的用户操作，不是静默重试）。
func CredentialsReset(c *gin.Context) {
	a := appFromCtx(c)
	ks := a.ActiveKeystore()

	if err := ks.DeleteMasterKey(); err != nil {
		log.Error("credentials.reset: 删除 master key 失败",
			"method", "credentials.reset", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "error": "重置失败：无法删除系统密钥环中的主密钥",
		})
		return
	}
	if err := ks.EnsureMasterKey(); err != nil {
		log.Error("credentials.reset: 重建 master key 失败",
			"method", "credentials.reset", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "error": "重置失败：无法在系统密钥环中创建新主密钥",
		})
		return
	}

	// Secrets() 会惰性建 store 并接线 SettingsManager；此处必须取到那一个
	// 实例再 Clear，否则清的是另一个 cache，磁盘上仍是旧内容。
	store := a.Secrets()
	if lf := store.LoadFailure(); lf != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "error": "重置失败：凭据存储不可用",
		})
		return
	}
	if err := store.Clear(); err != nil {
		log.Error("credentials.reset: 清空 secret store 失败",
			"method", "credentials.reset", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "error": "重置失败：无法清空凭据存储",
		})
		return
	}

	if a.Settings != nil {
		if err := a.Settings.ClearCredentialState(); err != nil {
			log.Error("credentials.reset: 清 settings 标记失败",
				"method", "credentials.reset", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false, "error": "重置失败：无法清理配置标记",
			})
			return
		}
	}

	log.Info("credentials.reset ok", "method", "credentials.reset")
	c.Status(http.StatusNoContent)
}

// CredentialsMigrate 接收用户补输的 v1 主口令，完成 secret.db 的迁移。
//
// 用于"keystore 可用但磁盘上仍是 v1 secret.db"的启动等待态：应用先起、
// 凭据端点可达，用户输一次口令，迁移完成后凭据即刻可用。
//
// 状态码刻意区分三类失败，让前端能给出不同指引：
//
//	204 —— 迁移完成（含"本来就是 v2 / 已迁移过"）。
//	403 —— 主口令错误或 v1 blob 损坏。磁盘未改动，可安全重试。
//	409 —— keystore 不可用。换个口令没用，得先修环境。
//	400 —— 请求体缺 password。
func CredentialsMigrate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false, "error": "missing password",
		})
		return
	}

	ks := a.ActiveKeystore()
	if err := ks.Available(); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false, "error": "系统密钥服务不可用，无法完成迁移",
		})
		return
	}

	err := a.MigrateLegacySecrets(req.Password)
	switch {
	case err == nil:
		log.Info("credentials.migrate ok", "method", "credentials.migrate")
		c.Status(http.StatusNoContent)

	case errors.Is(err, keystore.ErrUnseal):
		// 口令错 / blob 损坏。不区分两者（GCM tag 校验失败即两者皆可能），
		// 对用户而言都是"再输一次"，但明确说清磁盘没被动过。
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"code":    "invalid_legacy_password",
			"error":   "主口令错误或凭据文件已损坏，未做任何改动",
		})

	case errors.Is(err, keystore.ErrUnavailable), errors.Is(err, crypto.ErrKeystoreUnavailable):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"code":    "keystore_unavailable",
			"error":   "系统密钥服务不可用，无法完成迁移",
		})

	default:
		log.Error("credentials.migrate failed",
			"method", "credentials.migrate", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false, "error": "迁移失败",
		})
	}
}

var _ = domain.CredentialStatus{}