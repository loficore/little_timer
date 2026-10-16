# 领域词表

本文件是 Little Timer 项目的领域词汇表。代码、注释、ADR、PR、issue 在指代这些概念时，使用本表里的术语，不要另造同义词或缩写。

## 受限上下文

- **AIGTD** — AI 版的 GTD（Getting Things Done）随手记：用户输入一句自然语言，后台 worker 调 LLM 拆解为 Task + Subtasks。Job 模式分四种：`append`（合并已有子任务）、`replace`（整段重写）、`review`（只产 proposals，不写 task）、`review_refine`（基于用户提示重写一条 review proposal）。底层表是 `tasks` 自引用树 + `ai_jobs` 队列。

- **Habit** — 可重复计时的主体，归属于一个 **HabitSet**。Habit 累积 **Session**（按日期汇总的时长），并带一个 `GoalSeconds`（每日目标）。统计与进度按配置时区的当天计算。

- **Vault** — 凭据 + 备份域。掌管主密码、内存加密的 `SecretStorage` blob、解锁状态、以及 `BackupManager` 的生命周期。备份的所有变更都过 `vault.Operations` 这一道闸门。Vault 关闭时返回一个带 `{Code, Message, Mode}` 的类型化 `*vault.Error`，适配器把它映射成前端"弹出主密码 modal"动作。

- **Backup** — SQLite 数据库的快照，写到配置的目标（`local` / `webdav` / `s3`）。云端目标的 backup 始终受 Vault 解锁状态约束。

- **Scheduler** — 纯函数式规则引擎，把 Task 按 Habit slot、四维 Score 和固定工作窗口摆到一天的时间槽里。无 DB、无 IO、可重现。契约：LLM 只打分，Scheduler 只摆位。

- **Settings** — 持久化配置（`auth`、主题、LLM、`daily-sweep`、备份目标、时区等）。掌管 JSON 序列化格式与磁盘读写。被 Clock、Vault、LLM 工厂订阅。

## 横向术语

- **Operation** — 深化模块的 transport-neutral 用例方法（如 `habits.Operations.CreateHabit`）。Adapter 只做 transport 绑定到 operation；operation 拥有不变量。

- **Adapter** — transport 特定的实现，把 transport（HTTP JSON / Wails 反射）和 operation 模块互相转换。Adapter 唯一的逻辑是绑定和错误映射；不允许包含业务规则。

- **Seam** — 模块接口靠注入依赖把实现藏住的地方。Seam 在两个或更多 adapter 需要它时是"真的"（值得为之设计）；单个 adapter 是"假想 seam"，可以先内联。

- **Optimistic lock** — 客户端发它最后见到的 `version`，server 写成功后 bump `version`。冲突（`version` 过期）→ HTTP 409 + `code: "version_conflict"`。由 `middleware.ConflictGuard` 和 storage 的 `updateWithVersion` 系列强制。

- **Idempotency token** — `(ai_job_id, change_id)` 对，唯一识别一次 review 采纳。由 `UNIQUE` 索引兜底；并发重试折叠成一条审计记录。

- **Compile-time clock** — 注入模块的 `time.Time` 函数（替代模块里直接调 `time.Now`），如 `cron.DailySweepCron.SetNow`。测试可钉死假时钟，不重写模块。
