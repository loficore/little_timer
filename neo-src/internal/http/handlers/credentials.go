// Package handlers —— 凭据端点（取代 v1 的主口令端点，issue #13）。
//
// 主口令时代的端点在这里重新表达，但语义完全不同：
//
//	旧                                  新
//	GET  /api/backup/master-password    GET  /api/credentials/status
//	POST /api/backup/master-password    POST /api/credentials/reset
//	POST /api/backup/unlock             （无对应物——v1 迁移链已整体删除）
//	POST /api/backup/lock               （无对应物——没有"锁"这个概念）
//
// reset 是破坏性的：它删掉 keystore 里的 master key 并清空全部凭据，
// 用户之后要重新填一遍 LLM Key / 备份凭据。所以它不做隐式确认，
// 由前端负责弹确认框。
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
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

var _ = domain.CredentialStatus{}