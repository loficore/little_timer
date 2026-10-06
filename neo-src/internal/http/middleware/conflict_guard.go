// Package middleware —— ConflictGuard:乐观锁强制的请求侧门。
//
// 设计目标 (spec §4): 所有 mutation (POST/PATCH/PUT/DELETE) 必须携带
// `If-Match: <version>` header,或 body 中的 `version` 字段。缺则 400。
//
// 本中间件**不做** version 正确性校验(即不比对当前 server version)——
// 它只保证请求带了版本号,并把解析出的 int64 存进 context。真正的
// `WHERE id = ? AND version = ?` + 影响行数 0 → 409 由 storage 层
// (Task 4 的 updateWithVersion)集中实现。
//
// 读请求 (GET/HEAD/OPTIONS) 不需要版本号,直接放行。
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ctxKeyVersion 是 middleware 写入、handler 通过 GetVersion 读取的
// context key。用小写私有常量,避免与其它中间件的 key 冲突。
const ctxKeyVersion = "mutation_version"

// RequireVersionForMutation 强制 POST/PATCH/PUT/DELETE 请求带
// `If-Match: <version>` header 或 body 的 `version` 字段。
//
// 语义:
//   - GET/HEAD/OPTIONS 直接放行(读不需要乐观锁)。
//   - 优先 `If-Match` header;剥掉可选的双引号后 parse int64,
//     解析失败 → 400 `{"error":"invalid If-Match"}`。
//   - Header 缺失时回退到 body:peek JSON 找 `version` key。
//     即使值为 0 也算存在(`"version":0` 合法)。缺失 → 400
//     `{"error":"missing If-Match or version"}`。
//   - 通过后把 version 以 int64 存入 context key `mutation_version`。
//
// body-peek 会用 `io.NopCloser(bytes.NewReader(body))` 还原 body,
// 保证下游 handler 仍能正常 bind JSON。
func RequireVersionForMutation() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 读请求不需要版本号。
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		// 优先 If-Match header(ETag 风格,值可能带双引号)。
		if etag := c.GetHeader("If-Match"); etag != "" {
			n, err := strconv.ParseInt(strings.Trim(etag, `"`), 10, 64)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "invalid If-Match",
				})
				return
			}
			c.Set(ctxKeyVersion, n)
			c.Next()
			return
		}

		// 回退:读 body 找 version key。先整体读出以便还原。
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "missing If-Match or version",
			})
			return
		}
		// 还原 body 供 handler 读取(peek 不消费)。
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		var probe struct {
			Version int64 `json:"version"`
		}
		_ = json.Unmarshal(body, &probe)

		// 注意:用 key 的文本存在性判断,而不是 probe.Version != 0,
		// 因为 `"version":0` 是合法版本(新建记录的初始版本)。
		if !bytes.Contains(body, []byte(`"version"`)) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "missing If-Match or version",
			})
			return
		}

		c.Set(ctxKeyVersion, probe.Version)
		c.Next()
	}
}

// GetVersion 供 handler 读取当前请求的 mutation version。
//
// 若 context 中不存在该 key,说明这条路由没有挂 ConflictGuard 中间件,
// 这是配置错误 —— 直接 panic(而不是静默返 0),让漏挂在测试/启动时
// 立刻暴露,避免把 0 当成"合法版本"引发错误的乐观锁行为。
func GetVersion(c *gin.Context) int64 {
	v, ok := c.Get(ctxKeyVersion)
	if !ok {
		panic("ConflictGuard: missing version in context (middleware 未挂到该路由?)")
	}
	n, ok := v.(int64)
	if !ok {
		panic("ConflictGuard: version in context 类型不是 int64")
	}
	return n
}
