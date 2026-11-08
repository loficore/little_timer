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
// 所有"客户端持有的 version 已过期"的分支都走这里,避免状态码、code 字符串
// 与响应形状在各 handler 之间漂移:
//   - message 为空时用默认文案;schedule 传入带 current=N 的原文以保留提示;
//   - includeCode 控制是否带 `code` 字段(task/scores/schedule 带,daily-sweep
//     adopt 的历史契约不带);
//   - extra 用于端点特有的额外字段(如 adopt 的 success / task_id)。
func respondVersionConflict(c *gin.Context, message string, includeCode bool, extra gin.H) {
	if message == "" {
		message = versionConflictMessage
	}
	body := gin.H{"error": message}
	if includeCode {
		body["code"] = "version_conflict"
	}
	for k, v := range extra {
		body[k] = v
	}
	c.JSON(http.StatusConflict, body)
}
