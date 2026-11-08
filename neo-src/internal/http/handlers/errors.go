package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handlerError 是 pathID 辅助函数使用的小型错误类型。
type handlerError struct {
	message string
}

func (e *handlerError) Error() string { return e.message }

// versionConflictMessage 是乐观锁 409 的默认 error 文案。
const versionConflictMessage = "version conflict"

// respondVersionConflict 是乐观锁 409 的唯一映射点(issue #66)。
//
// 所有"客户端持有的 version 已过期"的分支都走这里,避免 code 字符串与
// body 形状在各 handler 之间漂移。message 传空时用默认文案(schedule 走
// verbatim 透传以保留它原有的 current=N 提示)。
func respondVersionConflict(c *gin.Context, message string) {
	if message == "" {
		message = versionConflictMessage
	}
	c.JSON(http.StatusConflict, gin.H{
		"error": message,
		"code":  "version_conflict",
	})
}
