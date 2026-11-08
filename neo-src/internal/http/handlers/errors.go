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

// respondVersionConflict 是乐观锁 409 的唯一映射点(issue #66)。
//
// 所有"客户端持有的 version 已过期"的分支都走这里,避免 code 字符串与
// body 形状在各 handler 之间漂移。
func respondVersionConflict(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{
		"error": "version conflict",
		"code":  "version_conflict",
	})
}
