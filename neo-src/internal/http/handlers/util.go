package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// pathID 从 Gin 路由中截取给定 prefix 之后的末尾整数 ID。
func pathID(c *gin.Context, prefix string) (int64, error) {
	full := c.Param("id")
	if full == "" {
		return 0, errInvalidID
	}
	return strconv.ParseInt(full, 10, 64)
}

// pathIDWithSuffix 提取 prefix 与 suffix 之间的整数 ID
// （如 "/api/habits/:id/detail"）。
func pathIDWithSuffix(c *gin.Context, prefix, suffix string) (int64, error) {
	tail := strings.TrimPrefix(c.Request.URL.Path, prefix)
	tail = strings.TrimSuffix(tail, suffix)
	if tail == "" {
		return 0, errInvalidID
	}
	return strconv.ParseInt(tail, 10, 64)
}

// nowUnix 返回当前 unix 时间戳（秒）。
func nowUnix() int64 { return time.Now().Unix() }

// errInvalidID 由 pathID* 在 URL 段无法解析为 int64 时返回。
var errInvalidID = &handlerError{message: "invalid id"}
