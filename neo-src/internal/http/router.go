// Package http — Gin 路由。
//
// 所有 endpoint 统一挂在 `/api/<area>` 下。
//
// 中间件顺序: CORS（最外层）→ recovery（Gin 默认）→ auth → ConflictGuard。
// auth 中间件设置每请求 "app" 键，
// 处理器可通过 `c.MustGet("app")` 获取 App 束。
// ConflictGuard 强制所有 mutation 请求携带 `If-Match` header 或 body
// `version` 字段；读请求（GET/HEAD/OPTIONS）放行。具体乐观锁校验在
// storage 层 `updateWithVersion`（Task 4）。
package http

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"little-timer/internal/http/handlers"
	"little-timer/internal/http/middleware"
)

// NewRouter 创建并注册全部 HTTP 路由的 Gin 引擎。
// `corsOrigin` 控制 Access-Control-Allow-Origin 响应头；
// 开发环境传 "*"，生产环境传具体 origin。
func NewRouter(a *app.App, corsOrigin string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(corsOrigin))
	r.Use(middleware.Auth(a))

	// 子项目 2 起：所有 mutation 必须带 version。统一挂在 /api 路由组，
	// 注册时把该组传给各 register 函数，避免重复 Group 调用。
	api := r.Group("/api")
	api.Use(middleware.RequireVersionForMutation())

	registerRoot(r)
	registerTimer(api)
	registerHabits(api)
	registerSettings(api)
	registerLLMSettings(api)
	registerTasks(api)
	registerAIGtd(api)
	registerBackup(api)
	registerWallpapers(api)
	registerEvents(api)

	return r
}

// GET /（SPA 兜底页）。

func registerRoot(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<html><body><h1>Little Timer</h1><p>Build the frontend (cd assets && pnpm run build) or open the dev server.</p></body></html>")
	})
}

// 计时器路由。

func registerTimer(g *gin.RouterGroup) {
	g.GET("/state", handlers.TimerState)
	g.GET("/timer/state", handlers.TimerState)
	g.GET("/timer/progress", handlers.TimerProgress)
	g.GET("/timer/config", handlers.TimerConfig)
	g.POST("/start", handlers.TimerStart)
	g.POST("/pause", handlers.TimerPause)
	g.POST("/reset", handlers.TimerReset)
	g.POST("/finish", handlers.TimerFinish)
	g.POST("/mode", handlers.TimerMode)
	g.POST("/timer/finish", handlers.TimerFinish)
	g.POST("/timer/rest", handlers.TimerStartRest)
	g.POST("/timer/config", handlers.TimerUpdateConfig)
}

// 习惯路由。

func registerHabits(g *gin.RouterGroup) {
	g.GET("/habit-sets", handlers.HabitSetList)
	g.POST("/habit-sets", handlers.HabitSetCreate)
	g.PUT("/habit-sets/:id", handlers.HabitSetUpdate)
	g.DELETE("/habit-sets/:id", handlers.HabitSetDelete)

	g.GET("/habits", handlers.HabitList)
	g.POST("/habits", handlers.HabitCreate)
	g.PUT("/habits/:id", handlers.HabitUpdate)
	g.DELETE("/habits/:id", handlers.HabitDelete)
	g.GET("/habits/:id/detail", handlers.HabitDetail)
	g.GET("/habits/:id/stats", handlers.HabitStats)

	g.GET("/sessions", handlers.SessionList)
	g.POST("/sessions", handlers.SessionCreate)
	g.DELETE("/sessions/:id", handlers.SessionDelete)

	g.GET("/timer-sessions", handlers.TimerSessionList)
	g.POST("/timer-sessions", handlers.TimerSessionCreate)
	g.PUT("/timer-sessions/:id", handlers.TimerSessionUpdate)
	g.DELETE("/timer-sessions/:id", handlers.TimerSessionDelete)
}

// 设置路由。

func registerSettings(g *gin.RouterGroup) {
	g.GET("/settings", handlers.SettingsGet)
	g.POST("/settings", handlers.SettingsUpdate)
}

// LLM 设置路由。

func registerLLMSettings(g *gin.RouterGroup) {
	sg := g.Group("/settings/llm")
	sg.GET("", handlers.LLMSettingsGet)
	sg.PUT("", handlers.LLMSettingsUpdate)
}

// 任务路由。

func registerTasks(g *gin.RouterGroup) {
	tg := g.Group("/tasks")
	tg.GET("", handlers.TaskList)
	tg.POST("", handlers.TaskCreate)
	tg.GET("/:id", handlers.TaskGet)
	tg.PATCH("/:id", handlers.TaskUpdate)
	tg.DELETE("/:id", handlers.TaskDelete)
	tg.POST("/:id/subtasks", handlers.SubtaskCreate)
	tg.PATCH("/:id/subtasks/:sub_id", handlers.SubtaskUpdate)
	tg.DELETE("/:id/subtasks/:sub_id", handlers.SubtaskDelete)
}

// AI GTD 路由。

func registerAIGtd(g *gin.RouterGroup) {
	ag := g.Group("/aigtd")
	ag.POST("/capture", handlers.AIGtdCapture)
	ag.POST("/reparse/:task_id", handlers.AIGtdReparse)
	ag.GET("/jobs", handlers.AIGtdJobs)
	// 测试端点（注入固定 LLM 响应/模拟 5xx）只在显式开启时注册。
	// e2e:Playwright webServer 启动时设置 LITTLE_TIMER_TEST_ENDPOINTS=1。
	if os.Getenv("LITTLE_TIMER_TEST_ENDPOINTS") == "1" {
		ag.POST("/_test/prompt", handlers.AIGtdTestPrompt)
	}
}

// 备份路由。

func registerBackup(g *gin.RouterGroup) {
	g.GET("/backup/config", handlers.BackupConfigGet)
	g.POST("/backup/config", handlers.BackupConfigUpdate)
	g.POST("/backup/create", handlers.BackupCreate)
	g.POST("/backup/restore", handlers.BackupRestore)
	g.POST("/backup/restore/:name", handlers.BackupRestoreByName)
	g.GET("/backup/list", handlers.BackupList)
	g.GET("/backup/info", handlers.BackupInfo)
	g.DELETE("/backup/delete/:name", handlers.BackupDeleteByName)
	g.DELETE("/backup/:id", handlers.BackupDelete)
	g.POST("/backup/verify", handlers.BackupVerify)
	g.POST("/backup/unlock", handlers.BackupUnlock)
	g.POST("/backup/lock", handlers.BackupLock)
	g.GET("/backup/master-password", handlers.MasterPasswordGet)
	g.POST("/backup/master-password", handlers.MasterPasswordSet)

	g.GET("/auth/status", handlers.AuthStatus)
	g.POST("/auth/enable", handlers.AuthEnable)
	g.POST("/auth/disable", handlers.AuthDisable)
}

// 壁纸路由。

func registerWallpapers(g *gin.RouterGroup) {
	wg := g.Group("/wallpapers")
	wg.POST("/from-url", handlers.WallpaperFromURL)
	wg.POST("", handlers.WallpaperUpload)
	wg.GET("", handlers.WallpaperList)
	wg.GET("/:id", handlers.WallpaperServe)
	wg.DELETE("/:id", handlers.WallpaperDelete)
}

// SSE + 前端日志路由。

func registerEvents(g *gin.RouterGroup) {
	g.GET("/events", handlers.Events)
	g.POST("/log", handlers.FrontendLog)
}
