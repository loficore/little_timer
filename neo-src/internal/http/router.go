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

	// ConflictGuard 全局作用域(Review focus,Task 3 / Task 8 复用):
	//
	//  - 下面这一行 `api.Use(middleware.RequireVersionForMutation())` 是
	//    **所有** `/api/*` 路径(包括嵌套 group: /api/tasks、/api/aigtd、
	//    /api/schedule、/api/settings、...)共用的 mutation 闸门。任何
	//    新增的 POST/PATCH/PUT/DELETE 端点都必须挂在 api 之下才能享受
	//    该中间件;否则请求会绕过乐观锁校验。
	//
	//  - 前端契约:浏览器 mutation 必须带 `If-Match: "<version>"` header
	//    (Node 端 fetch 兼容 CORS preflight 同样使用),或 body 内显式
	//    `version` 字段(浏览器端 fetch 走这个,避免 CORS preflight 因
	//    Access-Control-Allow-Headers 未列 if-match 而被拒)。两者缺失
	//    即 400。
	//
	//  - 没有 version 语义的资源(创建新记录、scheduler run、LLM 设置
	//    等)用 `"0"` 作为合法的存在性 token——语义是"随便一个非空版本号
	//    通过闸门",真正的乐观锁语义(0 行 → 409)由 storage 层
	//    updateWithVersion / 新引入的 DeleteTask(version) /
	//    UpdateSubtask(version) 集中判定。
	//
	//  - 当前实现是 process-wide 单点风险:把所有 mutation 锁在一个闸门
	//    后,version 解析失败全 400。后续如需豁免单端点,通过白名单显式
	//    标记而非反注册中间件。
	api := r.Group("/api")
	api.Use(middleware.RequireVersionForMutation())

	registerRoot(r)
	registerTimer(api)
	registerHabits(api)
	registerSettings(api)
	registerLLMSettings(api)
	registerDailySweep(api)
	registerTasks(api)
	registerAIGtd(api)
	registerSchedule(api)
	registerBackup(api)
	registerWallpapers(api)
	registerEvents(api)

	return r
}

// registerDailySweep 注册 /api/settings/daily-sweep 与 /api/daily-sweep/*
// 路由(v12,GTD Daily Sweep)。adopt / reject 由 Task 6 增量加入。
func registerDailySweep(g *gin.RouterGroup) {
	g.GET("/settings/daily-sweep", handlers.DailySweepSettingsGet)
	g.PUT("/settings/daily-sweep", handlers.DailySweepSettingsUpdate)
	g.POST("/daily-sweep/start", handlers.DailySweepStart)
	g.GET("/daily-sweep/today", handlers.DailySweepToday)
	g.POST("/daily-sweep/adopt", handlers.DailySweepAdopt)
	g.POST("/daily-sweep/reject", handlers.DailySweepReject)
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
	tg.PATCH("/:id/scores", handlers.TaskScoresUpdate)
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

// Scheduler 路由(Task 6):挂在 /api 路由组,继承 ConflictGuard。
//
// /run 接受 preview|apply;preview 不落库,apply 在事务里写
// scheduled_start/end 并乐观锁校验。/apply 接受显式 placements 集合,
// 用于 UI 拖拽后批量落库。
func registerSchedule(g *gin.RouterGroup) {
	g.POST("/schedule/run", handlers.ScheduleRun)
	g.POST("/schedule/apply", handlers.ScheduleApply)
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
