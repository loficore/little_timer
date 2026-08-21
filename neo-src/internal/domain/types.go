// Package domain 持有 little-timer 的核心类型与业务逻辑原语：
// 时钟枚举、事件类型、预设，以及设置/备份配置 struct。
package domain

import "time"

var now = time.Now

// TodayString 按用户相对 UTC 的时区偏移返回今天的日期。
func TodayString(offsetHours int8) string {
	return formatDateAtOffset(now(), offsetHours)
}

func formatDateAtOffset(base time.Time, offsetHours int8) string {
	return base.UTC().Add(time.Duration(offsetHours) * time.Hour).Format("2006-01-02")
}

// 时间单位常量（秒）。

const (
	Second = 1
	Minute = 60
	Hour   = 3600
	Day    = 86400
	Year   = 31536000
)

// 默认值。

const (
	DefaultWorkDurationSeconds        = 25 * Minute // 番茄钟工作时长
	DefaultRestDurationSeconds        = 5 * Minute
	DefaultMaxStopwatchSeconds        = 24 * Hour
	DefaultMaxDurationSeconds         = Day
	DefaultMaxYearSeconds             = 365 * Year
	DefaultTickIntervalMs             = 1000
	DefaultAutoSaveIntervalMs         = 5000
	MinTickIntervalMs                 = 100
	MaxTickIntervalMs                 = 5000
	DefaultMaxLogFileSize      uint64 = 10 * 1024 * 1024

	DefaultColor       = "#6366f1"
	DefaultGoalSeconds = 1500
)

// ModeEnum 是时钟当前运行的高层模式。
type ModeEnum int

const (
	CountdownMode ModeEnum = iota
	StopwatchMode
)

// String 为日志/诊断输出稳定标签。
func (m ModeEnum) String() string {
	switch m {
	case CountdownMode:
		return "countdown"
	case StopwatchMode:
		return "stopwatch"
	default:
		return "unknown"
	}
}

// DefaultMode 是 SettingsConfig 中持久化的“首选模式”，
// 通过 `String()` 序列化为字符串形式。
type DefaultMode int

const (
	DefaultModeCountdown DefaultMode = iota
	DefaultModeStopwatch
)

func (d DefaultMode) String() string {
	switch d {
	case DefaultModeCountdown:
		return "countdown"
	case DefaultModeStopwatch:
		return "stopwatch"
	default:
		return "unknown"
	}
}

// ParseDefaultMode 把字符串解析为 DefaultMode，默认取倒计时。
func ParseDefaultMode(s string) DefaultMode {
	if s == "stopwatch" {
		return DefaultModeStopwatch
	}
	return DefaultModeCountdown
}

// CountdownConfig 是倒计时（timer）模式的配置块。
type CountdownConfig struct {
	DurationSeconds     uint64 `json:"duration_seconds"`
	Loop                bool   `json:"loop"`
	LoopIntervalSeconds uint64 `json:"loop_interval_seconds"`
	LoopCount           uint32 `json:"loop_count"`
}

// NewDefaultCountdownConfig 返回番茄钟式默认值：25 分钟、不循环。
func NewDefaultCountdownConfig() CountdownConfig {
	return CountdownConfig{
		DurationSeconds:     DefaultWorkDurationSeconds,
		Loop:                false,
		LoopIntervalSeconds: 0,
		LoopCount:           0,
	}
}

// StopwatchConfig 是正计时模式的配置块。
type StopwatchConfig struct {
	MaxSeconds uint64 `json:"max_seconds"`
}

// NewDefaultStopwatchConfig 返回默认的 24 小时上限。
func NewDefaultStopwatchConfig() StopwatchConfig {
	return StopwatchConfig{
		MaxSeconds: DefaultMaxStopwatchSeconds,
	}
}

// ClockTaskConfig 是倒计时 + 正计时配置的合体，外加默认模式 ——
// 与预设和设置一起持久化。
type ClockTaskConfig struct {
	DefaultMode ModeEnum        `json:"default_mode"`
	Countdown   CountdownConfig `json:"countdown"`
	Stopwatch   StopwatchConfig `json:"stopwatch"`
}

// NewDefaultClockTaskConfig 返回预置默认值的配置：倒计时模式、
// 25 分钟工作、24 小时正计时代上限。
func NewDefaultClockTaskConfig() ClockTaskConfig {
	return ClockTaskConfig{
		DefaultMode: CountdownMode,
		Countdown:   NewDefaultCountdownConfig(),
		Stopwatch:   NewDefaultStopwatchConfig(),
	}
}

// ClockEvent 是时钟可接受的每种事件的和类型。标记方法 `isClockEvent()`
// 把 interface 密封为下面这些具体变体（不含其他）。新增变体意味着写一个
// 实现该标记方法的新 struct。
type ClockEvent interface {
	isClockEvent()
}

// TickEvent 让倒计时和正计时状态各自前进 DeltaMs。
type TickEvent struct {
	DeltaMs int64
}

func (TickEvent) isClockEvent() {}

// UserStartTimerEvent 启动（或恢复）时钟。
type UserStartTimerEvent struct{}

func (UserStartTimerEvent) isClockEvent() {}

// UserPauseTimerEvent 暂停时钟。
type UserPauseTimerEvent struct{}

func (UserPauseTimerEvent) isClockEvent() {}

// UserResetTimerEvent 恢复初始配置。
type UserResetTimerEvent struct{}

func (UserResetTimerEvent) isClockEvent() {}

// UserFinishTimerEvent 冻结时钟并标记为已完成（供统计使用）。
type UserFinishTimerEvent struct{}

func (UserFinishTimerEvent) isClockEvent() {}

// UserChangeModeEvent 用硬编码默认值切换模式
// （复位语义见 ClockManager.handleEvent）。
type UserChangeModeEvent struct {
	Mode ModeEnum
}

func (UserChangeModeEvent) isClockEvent() {}

// UserChangeConfigEvent 整体替换当前配置。
type UserChangeConfigEvent struct {
	Config ClockTaskConfig
}

func (UserChangeConfigEvent) isClockEvent() {}

// TimerPreset 是用户可随时取用的已命名保存配置。
type TimerPreset struct {
	Name   string          `json:"name"`
	Mode   ModeEnum        `json:"mode"`
	Config ClockTaskConfig `json:"config"`
}

// SettingsBasic 收集基础用户偏好。
type SettingsBasic struct {
	Timezone    int8        `json:"timezone"` // 相对 UTC 的小时数，默认 8（中国）
	Language    string      `json:"language"` // 例如 "ZH"
	DefaultMode DefaultMode `json:"default_mode"`
	ThemeMode   string      `json:"theme_mode"` // "dark" | "light" | ...
	Wallpaper   string      `json:"wallpaper"`  // 全局壁纸路径/URL
}

func NewDefaultSettingsBasic() SettingsBasic {
	return SettingsBasic{
		Timezone:    8,
		Language:    "ZH",
		DefaultMode: DefaultModeCountdown,
		ThemeMode:   "dark",
		Wallpaper:   "",
	}
}

// SettingsLogging 收集日志与性能调优开关。
type SettingsLogging struct {
	Level             string `json:"level"`
	EnableTimestamp   bool   `json:"enable_timestamp"`
	TickIntervalMs    int64  `json:"tick_interval_ms"`
	EnableFileLogging bool   `json:"enable_file_logging"`
	LogDir            string `json:"log_dir"`
	MaxFileSize       uint64 `json:"max_file_size"`
	MaxFileCount      uint8  `json:"max_file_count"`
}

func NewDefaultSettingsLogging() SettingsLogging {
	return SettingsLogging{
		Level:             "INFO",
		EnableTimestamp:   true,
		TickIntervalMs:    1000,
		EnableFileLogging: true,
		LogDir:            "",
		MaxFileSize:       DefaultMaxLogFileSize,
		MaxFileCount:      5,
	}
}

// SettingsAuth 收集 bearer-token 鉴权开关。
type SettingsAuth struct {
	AuthEnabled bool   `json:"auth_enabled"`
	AuthToken   string `json:"auth_token"`
}

func NewDefaultSettingsAuth() SettingsAuth {
	return SettingsAuth{
		AuthEnabled: false,
		AuthToken:   "",
	}
}

// SettingsConfig 是顶层持久化应用配置。
type SettingsConfig struct {
	Basic         SettingsBasic   `json:"basic"`
	ClockDefaults ClockTaskConfig `json:"clock_defaults"`
	Logging       SettingsLogging `json:"logging"`
	Auth          SettingsAuth    `json:"auth"`
}

// NewDefaultSettingsConfig 返回填满默认值的 SettingsConfig。
// clock_defaults 块用 `New…` 函数构造，嵌套默认值同样生效。
func NewDefaultSettingsConfig() SettingsConfig {
	return SettingsConfig{
		Basic:         NewDefaultSettingsBasic(),
		ClockDefaults: NewDefaultClockTaskConfig(),
		Logging:       NewDefaultSettingsLogging(),
		Auth:          NewDefaultSettingsAuth(),
	}
}

// SettingsEvent —— settings 层的次要 union。
// HTTP 层消费 JSON payload；这里只是保留这个形状。

// SettingsGetEvent 请求 settings 层把当前配置序列化到指定的 buffer
// 槽位（在 Go 中是 no-op —— 由 http 层处理）。
type SettingsGetEvent struct {
	// Slot 是逻辑上的“写到哪”提示 —— Go 里 http 层直接读 store，
	// 所以它保留给将来使用。
	Slot string
}

func (SettingsGetEvent) isSettingsEvent() {}

// SettingsChangeEvent 携带待应用的 JSON 编码配置。
type SettingsChangeEvent struct {
	JSON string
}

func (SettingsChangeEvent) isSettingsEvent() {}

// SettingsEvent 是 settings 层事件的判别 union。
type SettingsEvent interface {
	isSettingsEvent()
}

// EventType —— clock 与 settings 两条事件流的顶层派发器。

// EventType 是横切事件总线使用的判别 union。
type EventType interface {
	isEventType()
}

func (ClockEventWrapper) isEventType()    {}
func (SettingsEventWrapper) isEventType() {}

// ClockEventWrapper 让 ClockEvent 经由总线传递。
type ClockEventWrapper struct {
	Event ClockEvent
}

// SettingsEventWrapper 让 SettingsEvent 经由总线传递。
type SettingsEventWrapper struct {
	Event SettingsEvent
}

// 备份 target / info / config。

// BackupTargetType 选择备份使用的目标端。
type BackupTargetType int

const (
	BackupTargetLocal BackupTargetType = iota
	BackupTargetWebDAV
	BackupTargetS3
)

func (b BackupTargetType) String() string {
	switch b {
	case BackupTargetLocal:
		return "local"
	case BackupTargetWebDAV:
		return "webdav"
	case BackupTargetS3:
		return "s3"
	default:
		return "unknown"
	}
}

// UnlockResult 是凭据解锁尝试的返回值。
type UnlockResult struct {
	Success     bool  `json:"success"`
	LockedUntil int64 `json:"locked_until"`
}

// MasterPasswordStatus 概览凭据子系统状态。
type MasterPasswordStatus struct {
	HasPassword bool  `json:"has_password"`
	Unlocked    bool  `json:"unlocked"`
	LockedUntil int64 `json:"locked_until"`
	UnlockTime  int64 `json:"unlock_time"`
}

// ApiAction 是跨层的 UI 触发器。放在这里以便下游包共享该契约；
// modal 参数目前是自由形式的 map。
type ApiAction struct {
	ShowModal *ApiShowModal `json:"show_modal,omitempty"`
}

// ApiShowModal 是 ApiAction 的显示 modal 变体。
type ApiShowModal struct {
	Target string            `json:"target"`
	Params map[string]string `json:"params"`
}

// BackupConfig 是持久化的备份配置。所有 WebDAV / S3 字段都存在
// （即使当前 target 用不到），这样一个 struct 就能完整往返 JSON、
// 不丢用户输入。
type BackupConfig struct {
	Enabled        bool             `json:"enabled"`
	AutoBackup     bool             `json:"auto_backup"`
	AutoBackupSecs uint64           `json:"auto_backup_interval"` // 秒
	TargetType     BackupTargetType `json:"target_type"`

	// 仅 Local 使用。
	LocalPath string `json:"local_path"`

	// 仅 WebDAV 使用。
	WebDAVURL        string `json:"webdav_url"`
	WebDAVUsername   string `json:"webdav_username"`
	WebDAVPassword   string `json:"webdav_password"` // 静态加密
	WebDAVPathPrefix string `json:"webdav_path_prefix"`

	// 仅 S3 使用。
	S3Endpoint   string `json:"s3_endpoint"`
	S3Bucket     string `json:"s3_bucket"`
	S3Region     string `json:"s3_region"`
	S3AccessKey  string `json:"s3_access_key"`
	S3SecretKey  string `json:"s3_secret_key"` // 静态加密
	S3PathPrefix string `json:"s3_path_prefix"`

	// 凭据。
	HasMasterPassword        bool   `json:"has_master_password"`
	CredentialsUnlockTime    int64  `json:"credentials_unlock_time"`
	CredentialUnlockAttempts uint32 `json:"credential_unlock_attempts"`
	CredentialLockedUntil    int64  `json:"credential_locked_until"`
}

// NewDefaultBackupConfig 返回默认值：关闭、local target、路径为空。
func NewDefaultBackupConfig() BackupConfig {
	return BackupConfig{
		Enabled:                  false,
		AutoBackup:               false,
		AutoBackupSecs:           Day,
		TargetType:               BackupTargetLocal,
		LocalPath:                "",
		WebDAVURL:                "",
		WebDAVUsername:           "",
		WebDAVPassword:           "",
		WebDAVPathPrefix:         "little_timer/",
		S3Endpoint:               "",
		S3Bucket:                 "",
		S3Region:                 "",
		S3AccessKey:              "",
		S3SecretKey:              "",
		S3PathPrefix:             "little_timer/",
		HasMasterPassword:        false,
		CredentialsUnlockTime:    0,
		CredentialUnlockAttempts: 0,
		CredentialLockedUntil:    0,
	}
}

// BackupInfo 描述单个备份产物（文件列表、历史等）。
type BackupInfo struct {
	Name      string `json:"name"`
	Timestamp int64  `json:"timestamp"`
	SizeBytes uint64 `json:"size_bytes"`
}

// BoolToInt 把 bool 转成 SQLite BOOLEAN 存储的 0/1 表示。
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// 时间辅助函数。

// NowMs 返回自 Unix epoch 起的墙钟毫秒。
func NowMs() int64 { return time.Now().UnixMilli() }
