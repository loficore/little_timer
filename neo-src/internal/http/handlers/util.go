package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/http/app"
	"little-timer/internal/storage"
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

// aiConfigFromApp 从 settings 表读当前 LLM provider/model,作为 ai_jobs 入队参数。
// baseURL/apiKey/timeout 等由 worker 在 settings.LLMSettings() 时读取,这里只
// 关心 jobs 表里要记录的 provider+model 标识。
func aiConfigFromApp(a *app.App) aigtd.LLMConfig {
	cfg := aigtd.LLMConfig{
		Provider: "openai_compat",
		Model:    "deepseek-chat",
	}
	if a == nil || a.SQLite == nil {
		return cfg
	}
	row := a.SQLite.DB().QueryRow(`SELECT llm_provider, llm_model FROM settings WHERE id = 1;`)
	var p, m string
	if err := row.Scan(&p, &m); err == nil {
		if p != "" {
			cfg.Provider = p
		}
		if m != "" {
			cfg.Model = m
		}
	}
	return cfg
}

// taskRawUpdate 返回把 task.raw_text 改成给定字符串的 UpdateTask 字段。
func taskRawUpdate(raw string) storage.TaskUpdateFields {
	return storage.TaskUpdateFields{RawText: &raw}
}

// parseInt64Query 从 query string 取整型参数。
func parseInt64Query(c *gin.Context, name string) (int64, error) {
	v := c.Query(name)
	if v == "" {
		return 0, &handlerError{message: "missing " + name}
	}
	return strconv.ParseInt(v, 10, 64)
}
