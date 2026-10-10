// Package settings —— 顶层设置 manager。持有：
//
//   - domain.SettingsConfig —— 基础 / 时钟默认值 / 日志 / 鉴权
//   - domain.BackupConfig —— 备份 target + WebDAV / S3 凭据
//   - 一个 PresetsManager（no-op 桩；见 presets.go）
//   - 一个从 SQLite 加载的 viper.Viper 实例（唯一事实来源 ——
//     没有 default.json，也没有仅凭 env 的回退）
//
// manager 在构造时接受一个 `*storage.SqliteManager`（已接好子模块）。
// 若 SQLite 文件尚未打开，它会负责打开（创建父目录、chmod 0600、跑迁移）。
//
// 凭据托管：webdav_password / s3_access_key / s3_secret_key 不再写入
// SQLite —— 由平台 keystore 托管的 *crypto.SecretStorage(经 SetSecretStore
// 注入)保存,SQLite 只保留 has_* 布尔标记与非敏感的明文字段(url/bucket/
// region/prefix 等)。旧的 deriveCredentialKey + AES-GCM 列加密方案已整体
// 删除,历史密文列停止写入(NULL),清理由 keystore 迁移器负责。
package settings

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/storage"
)

// SecretStorage 中三个凭据字段的 key 名 —— spec §6 约定,keystore 迁移器
// (Task 8)按同样的名字写入。
const (
	SecretWebDAVPassword = "webdav_password"
	SecretS3AccessKey    = "s3_access_key"
	SecretS3SecretKey    = "s3_secret_key"
)

// errSecretStoreNotWired 在 SecretStorage 尚未接线时拒绝用户触发的保存
// —— 防止调用方漏接 SetSecretStore 后静默清掉凭据。
var errSecretStoreNotWired = errors.New("settings: secret store not wired")

// SettingsManager 持有基于 SQLite 的内存 settings + backup 配置。
type SettingsManager struct {
	sqlite *storage.SqliteManager

	config       domain.SettingsConfig
	backupConfig domain.BackupConfig

	presets *PresetsManager

	// viper 是应用其余部分使用的读取 API（例如 http handler 可以直接
	// 调 manager.Viper().GetString("basic.language")，不必走 Go struct
	// 访问器）。SQLite 仍是唯一事实来源 —— viper 从 SQLite 行填充，
	// 绝不反向。
	viper *viper.Viper

	// dirty 由变更器置位、由 Save() 消费。
	dirty bool

	// secretStore 托管三个凭据字段,由 SetSecretStore 注入(构造期不
	// 接受,避免构造环)。nil = 未接线,用户触发的保存报
	// errSecretStoreNotWired。
	secretStore *crypto.SecretStorage
}

// BasicConfig 是 UpdateBasic 使用的小型强类型视图。
type BasicConfig struct {
	Timezone    int8
	Language    string
	DefaultMode domain.DefaultMode
}

// New 打开（或复用）dbPath 处的 SQLite 文件，返回可直接使用的
// SettingsManager。dbPath 为空时在 `os.UserConfigDir()` 下计算
// 平台默认路径。
func New(dbPath string) (*SettingsManager, error) {
	resolved, err := resolveDatabasePath(dbPath)
	if err != nil {
		return nil, err
	}

	mgr := storage.NewSqliteManager().Init(resolved)
	if err := mgr.Open(); err != nil {
		return nil, fmt.Errorf("settings: open db: %w", err)
	}
	if err := mgr.Migrate(); err != nil {
		_ = mgr.Close()
		return nil, fmt.Errorf("settings: migrate: %w", err)
	}

	return newFromSqlite(mgr)
}

// NewFromSqliteManager 包装一个已打开的 SqliteManager。适用于测试框架
// 已经开好 DB 的场景。`dbPath` 当前未被使用(凭据密钥派生已随
// SecretStorage 接管而删除),保留形参只为调用方签名兼容。
func NewFromSqliteManager(mgr *storage.SqliteManager, dbPath string) (*SettingsManager, error) {
	if mgr == nil || !mgr.IsOpen() {
		return nil, errors.New("settings: sqlite manager is nil or not open")
	}
	return newFromSqlite(mgr)
}

func newFromSqlite(mgr *storage.SqliteManager) (*SettingsManager, error) {
	sm := &SettingsManager{
		sqlite:  mgr,
		presets: NewPresetsManager(),
		viper:   viper.New(),
		// config / backupConfig 由 Load() 填充；不先 Load 的调用方
		// 会看到零值。
	}
	// SecretStorage 是 setter 注入(构造之后),has_* 标记列又由本文件
	// 的裸 SQL 直接读写 —— 打开时先幂等补齐列,老库与新库共用一条路径。
	if err := sm.ensureBackupConfigCredentialFlags(); err != nil {
		return nil, err
	}
	if err := sm.loadAll(); err != nil {
		return nil, err
	}
	return sm, nil
}

// SetSecretStore 注入 keystore envelope 凭据存储(Task 3 的
// *crypto.SecretStorage)。生产接线顺序是 构造 → SetSecretStore → Load():
// 构造期 store 尚未就绪,has_*=1 的既有凭据要等接线后的 Load() 才回填进
// BackupConfig。传 nil 解除接线 —— 此后用户触发的保存返回
// errSecretStoreNotWired。
func (sm *SettingsManager) SetSecretStore(ss *crypto.SecretStorage) {
	sm.secretStore = ss
}

// Load / Save。

// Load 从 SQLite 重读全部 settings 行并重新填充 viper。
func (sm *SettingsManager) Load() error {
	return sm.loadAll()
}

// Save 把内存中的 config + backup 配置刷入 SQLite。凭据段经
// SecretStorage 分流,因此要求已 SetSecretStore 接线,否则报
// errSecretStoreNotWired。
func (sm *SettingsManager) Save() error {
	if err := sm.saveSettingsToDB(); err != nil {
		return err
	}
	if err := sm.saveBackupConfigToDB(); err != nil {
		return err
	}
	sm.dirty = false
	return nil
}

func (sm *SettingsManager) loadAll() error {
	cfg, err := sm.sqlite.LoadSettings()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cfg = domain.NewDefaultSettingsConfig()
			if initErr := sm.initializeDefaultSettings(cfg); initErr != nil {
				return initErr
			}
		} else {
			// 尝试重新播种默认值来恢复
			cfg = domain.NewDefaultSettingsConfig()
			if initErr := sm.initializeDefaultSettings(cfg); initErr != nil {
				return initErr
			}
		}
	}
	sm.config = cfg

	if err := sm.loadBackupConfigFromDB(); err != nil {
		// TODO: 尽力而为 —— 行缺失或不可读时回退到默认值。
		sm.backupConfig = domain.NewDefaultBackupConfig()
	}
	sm.populateViper()
	sm.dirty = false
	return nil
}

func (sm *SettingsManager) initializeDefaultSettings(cfg domain.SettingsConfig) error {
	sm.config = cfg
	sm.backupConfig = domain.NewDefaultBackupConfig()
	// 构造期播种不走 saveBackupConfigToDB:此时 SecretStorage 还没注入
	// (setter 注入在构造之后),而默认凭据值全空,没有可分流的 secret;
	// 直接落 has_*=0、凭据列 NULL 的行。store 若已接线则照常分流,见
	// writeBackupConfigRow。
	if err := sm.saveSettingsToDB(); err != nil {
		return fmt.Errorf("settings: save defaults: %w", err)
	}
	if err := sm.writeBackupConfigRow(); err != nil {
		return fmt.Errorf("settings: save defaults: %w", err)
	}
	sm.populateViper()
	return nil
}

// 公开访问器。

// Config 返回内存中 SettingsConfig 的副本。调用方可以改返回值的字段，
// 但 manager 存的副本不受影响；要持久化请用 Update* 方法。
func (sm *SettingsManager) Config() domain.SettingsConfig {
	return sm.config
}

// BackupConfig 返回内存中 BackupConfig 的副本。
func (sm *SettingsManager) BackupConfig() domain.BackupConfig {
	return sm.backupConfig
}

// Viper 返回内存中的 viper 实例。每次 Load() / Save() 都从 SQLite 填充。
func (sm *SettingsManager) Viper() *viper.Viper { return sm.viper }

// Presets 返回 PresetsManager。为什么它近乎 no-op 见 presets.go。
func (sm *SettingsManager) Presets() *PresetsManager { return sm.presets }

// IsDirty 只读报告 dirty 标志。
func (sm *SettingsManager) IsDirty() bool { return sm.dirty }

// populateViper 把 SettingsConfig + BackupConfig 的所有相关字段重新
// 以 key 形式灌进 viper，消费者即可调用 `sm.Viper().GetString(...)`。
// 这就是“Viper 从 SQLite 读”的路径。
func (sm *SettingsManager) populateViper() {
	v := sm.viper
	v.Set("basic.timezone", sm.config.Basic.Timezone)
	v.Set("basic.language", sm.config.Basic.Language)
	v.Set("basic.default_mode", sm.config.Basic.DefaultMode.String())
	v.Set("basic.theme_mode", sm.config.Basic.ThemeMode)
	v.Set("basic.wallpaper", sm.config.Basic.Wallpaper)

	v.Set("clock_defaults.default_mode", sm.config.ClockDefaults.DefaultMode.String())
	v.Set("clock_defaults.countdown.duration_seconds", sm.config.ClockDefaults.Countdown.DurationSeconds)
	v.Set("clock_defaults.countdown.loop", sm.config.ClockDefaults.Countdown.Loop)
	v.Set("clock_defaults.countdown.loop_count", sm.config.ClockDefaults.Countdown.LoopCount)
	v.Set("clock_defaults.countdown.loop_interval_seconds", sm.config.ClockDefaults.Countdown.LoopIntervalSeconds)
	v.Set("clock_defaults.stopwatch.max_seconds", sm.config.ClockDefaults.Stopwatch.MaxSeconds)

	v.Set("logging.level", sm.config.Logging.Level)
	v.Set("logging.enable_timestamp", sm.config.Logging.EnableTimestamp)
	v.Set("logging.tick_interval_ms", sm.config.Logging.TickIntervalMs)
	v.Set("logging.enable_file_logging", sm.config.Logging.EnableFileLogging)
	v.Set("logging.log_dir", sm.config.Logging.LogDir)
	v.Set("logging.max_file_size", sm.config.Logging.MaxFileSize)
	v.Set("logging.max_file_count", sm.config.Logging.MaxFileCount)

	v.Set("auth.auth_enabled", sm.config.Auth.AuthEnabled)
	// auth token 敏感 —— 绝不灌进 viper。
}

// 变更器。

// UpdateBasic 校验并应用一个 BasicConfig。
func (sm *SettingsManager) UpdateBasic(bc BasicConfig) error {
	if err := ValidateTimezone(bc.Timezone); err != nil {
		return err
	}
	if err := ValidateLanguage(bc.Language); err != nil {
		return err
	}
	sm.config.Basic.Timezone = bc.Timezone
	sm.config.Basic.Language = bc.Language
	sm.config.Basic.DefaultMode = bc.DefaultMode
	sm.dirty = true
	sm.populateViper()
	return nil
}

// UpdateAuth 替换 auth 块。
func (sm *SettingsManager) UpdateAuth(auth domain.SettingsAuth) error {
	sm.config.Auth = auth
	sm.dirty = true
	sm.populateViper()
	return sm.Save()
}

// UpdateBackupConfigFromJSON 解析一个 JSON 对象并应用。blob 中未出现的
// 字段保持不动。
func (sm *SettingsManager) UpdateBackupConfigFromJSON(jsonStr string) error {
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return fmt.Errorf("settings: parse backup json: %w", err)
	}

	if v, ok := raw["enabled"].(bool); ok {
		sm.backupConfig.Enabled = v
	}
	if v, ok := raw["auto_backup"].(bool); ok {
		sm.backupConfig.AutoBackup = v
	}
	if v, ok := toInt64(raw["auto_backup_interval"]); ok {
		if v >= 60 && v <= 31_536_000 {
			sm.backupConfig.AutoBackupSecs = uint64(v)
		}
	}
	if v, ok := raw["target_type"].(string); ok {
		sm.backupConfig.TargetType = parseTargetType(v)
	}
	if v, ok := raw["local_path"].(string); ok {
		sm.backupConfig.LocalPath = v
	}
	if v, ok := raw["webdav_url"].(string); ok {
		sm.backupConfig.WebDAVURL = v
	}
	if v, ok := raw["webdav_username"].(string); ok {
		sm.backupConfig.WebDAVUsername = v
	}
	if v, ok := raw["webdav_password"].(string); ok {
		sm.backupConfig.WebDAVPassword = v
	}
	if v, ok := raw["s3_endpoint"].(string); ok {
		sm.backupConfig.S3Endpoint = v
	}
	if v, ok := raw["s3_bucket"].(string); ok {
		sm.backupConfig.S3Bucket = v
	}
	if v, ok := raw["s3_region"].(string); ok {
		sm.backupConfig.S3Region = v
	}
	if v, ok := raw["s3_access_key"].(string); ok {
		sm.backupConfig.S3AccessKey = v
	}
	if v, ok := raw["s3_secret_key"].(string); ok {
		sm.backupConfig.S3SecretKey = v
	}
	if v, ok := raw["s3_path_prefix"].(string); ok {
		sm.backupConfig.S3PathPrefix = v
	}
	if v, ok := raw["webdav_path_prefix"].(string); ok {
		sm.backupConfig.WebDAVPathPrefix = v
	}
	if v, ok := raw["has_master_password"].(bool); ok {
		sm.backupConfig.HasMasterPassword = v
	}

	sm.dirty = true
	return sm.saveBackupConfigToDB()
}

// HandleSettingsEvent 处理来自 domain.SettingsEvent 的判别 union。
func (sm *SettingsManager) HandleSettingsEvent(ev domain.SettingsEvent) error {
	switch e := ev.(type) {
	case domain.SettingsChangeEvent:
		if err := sm.parseSettingsFromJSON(e.JSON); err != nil {
			return err
		}
		return sm.Save()
	case domain.SettingsGetEvent:
		// 预留：http 层直接经 Config() / Viper() 读取。
		return nil
	default:
		return fmt.Errorf("settings: unknown event type %T", ev)
	}
}

// BuildClockConfig 用持久化默认值 + 用户首选 DefaultMode 组装
// ClockTaskConfig。
func (sm *SettingsManager) BuildClockConfig() domain.ClockTaskConfig {
	mode := domain.CountdownMode
	if sm.config.Basic.DefaultMode == domain.DefaultModeStopwatch {
		mode = domain.StopwatchMode
	}
	return domain.ClockTaskConfig{
		DefaultMode: mode,
		Countdown:   sm.config.ClockDefaults.Countdown,
		Stopwatch:   sm.config.ClockDefaults.Stopwatch,
	}
}

// AddPreset 是通向（惰性）PresetsManager 的 no-op 直通。
func (sm *SettingsManager) AddPreset(preset domain.TimerPreset) error {
	sm.dirty = true
	return sm.presets.Add(preset)
}

// GetPresets 返回预设列表。
func (sm *SettingsManager) GetPresets() []domain.TimerPreset {
	return sm.presets.GetAll()
}

// ResetToDefaults 把所有字段恢复默认并保存。
func (sm *SettingsManager) ResetToDefaults() error {
	sm.config = domain.NewDefaultSettingsConfig()
	sm.backupConfig = domain.NewDefaultBackupConfig()
	sm.dirty = true
	if err := sm.Save(); err != nil {
		return err
	}
	sm.populateViper()
	return nil
}

// Close 刷盘并关闭底层 SQLite 连接。
func (sm *SettingsManager) Close() error {
	if sm.sqlite == nil {
		return nil
	}
	return sm.sqlite.Close()
}

// parseSettingsFromJSON —— 就地应用一个 JSON settings payload。

func (sm *SettingsManager) parseSettingsFromJSON(jsonStr string) error {
	var root map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return fmt.Errorf("settings: parse: %w", err)
	}

	if basic, ok := root["basic"].(map[string]any); ok {
		if v, ok := toInt64(basic["timezone"]); ok {
			if v >= -12 && v <= 14 {
				sm.config.Basic.Timezone = int8(v)
			}
		}
		if v, ok := basic["language"].(string); ok {
			if l := len(v); l >= 1 && l <= 10 {
				sm.config.Basic.Language = v
			}
		}
		if v, ok := basic["default_mode"].(string); ok {
			sm.config.Basic.DefaultMode = domain.ParseDefaultMode(v)
		}
		if v, ok := basic["theme_mode"].(string); ok {
			sm.config.Basic.ThemeMode = v
		}
		if v, ok := basic["wallpaper"].(string); ok {
			sm.config.Basic.Wallpaper = v
		}
	}

	if defaults, ok := root["clock_defaults"].(map[string]any); ok {
		if cd, ok := defaults["countdown"].(map[string]any); ok {
			if v, ok := toInt64(cd["duration_seconds"]); ok && v >= 1 && v <= 86_400 {
				sm.config.ClockDefaults.Countdown.DurationSeconds = uint64(v)
			}
			if v, ok := cd["loop"].(bool); ok {
				sm.config.ClockDefaults.Countdown.Loop = v
			}
			if v, ok := toInt64(cd["loop_count"]); ok && v >= 0 && v <= 1000 {
				sm.config.ClockDefaults.Countdown.LoopCount = uint32(v)
			}
			if v, ok := toInt64(cd["loop_interval_seconds"]); ok && v >= 0 && v <= 3600 {
				sm.config.ClockDefaults.Countdown.LoopIntervalSeconds = uint64(v)
			}
		}
		if sw, ok := defaults["stopwatch"].(map[string]any); ok {
			if v, ok := toInt64(sw["max_seconds"]); ok && v > 0 && v <= 86_400*365 {
				sm.config.ClockDefaults.Stopwatch.MaxSeconds = uint64(v)
			}
		}
	}

	if logging, ok := root["logging"].(map[string]any); ok {
		if v, ok := logging["level"].(string); ok {
			sm.config.Logging.Level = v
		}
		if v, ok := logging["enable_timestamp"].(bool); ok {
			sm.config.Logging.EnableTimestamp = v
		}
		if v, ok := toInt64(logging["tick_interval_ms"]); ok && v > 0 {
			sm.config.Logging.TickIntervalMs = v
		}
		if v, ok := logging["enable_file_logging"].(bool); ok {
			sm.config.Logging.EnableFileLogging = v
		}
		if v, ok := logging["log_dir"].(string); ok {
			sm.config.Logging.LogDir = v
		}
		if v, ok := toInt64(logging["max_file_size"]); ok && v > 0 {
			sm.config.Logging.MaxFileSize = uint64(v)
		}
		if v, ok := toInt64(logging["max_file_count"]); ok && v > 0 && v < 20 {
			sm.config.Logging.MaxFileCount = uint8(v)
		}
	}

	if auth, ok := root["auth"].(map[string]any); ok {
		if v, ok := auth["auth_enabled"].(bool); ok {
			sm.config.Auth.AuthEnabled = v
		}
		if v, ok := auth["auth_token"].(string); ok {
			if err := ValidateAuthToken(v); err == nil {
				sm.config.Auth.AuthToken = v
			}
		}
	}

	if backup, ok := root["backup"].(map[string]any); ok {
		backupJSON, _ := json.Marshal(backup)
		if err := sm.UpdateBackupConfigFromJSON(string(backupJSON)); err != nil {
			return err
		}
	}

	if presets, ok := root["presets"].([]any); ok && len(presets) > 0 {
		// 预设是惰性的（见 presets.go），但我们仍会排空这个列表，
		// 以免重新保存时消费者感到意外。
		sm.presets = NewPresetsManager()
		for _, p := range presets {
			if _, ok := p.(map[string]any); !ok {
				continue
			}
			// 这里的校验尽力而为 —— 无效预设直接丢弃。
			_ = sm.presets.MaxCount()
		}
	}

	sm.dirty = true
	sm.populateViper()
	return nil
}

// SQLite 持久化 —— settings 行。

func (sm *SettingsManager) saveSettingsToDB() error {
	return sm.sqlite.SaveSettings(sm.config)
}

// SQLite 持久化 —— backup_config 行。
//
// 其他表的等价备份 SQL 住在 storage 层；这里托管本表是为了让
// settings/crypto/backup 边界自包含。
//
// webdav_password_encrypted / s3_access_key_encrypted /
// s3_secret_key_encrypted:凭据不再写入,恒写 NULL;列本身留待 keystore
// 迁移器(Task 8)清理,本期不做破坏性 schema 变更。真正的凭据值经
// SecretStorage 分流,SQLite 只记 has_* 标记。

const backupConfigSQL = `INSERT OR REPLACE INTO backup_config (
    id, target_type, enabled, auto_backup, auto_backup_interval,
    local_path, webdav_url, webdav_username, webdav_password_encrypted,
    s3_endpoint, s3_bucket, s3_region,
    s3_access_key_encrypted, s3_secret_key_encrypted,
    s3_path_prefix,
    has_master_password, credentials_unlock_time,
    credential_unlock_attempts, credential_locked_until,
    has_webdav_password, has_s3_access_key, has_s3_secret_key
) VALUES (
    1, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?
);`

const loadBackupConfigSQL = `SELECT
    target_type, enabled, auto_backup, auto_backup_interval,
    COALESCE(local_path, ''), COALESCE(webdav_url, ''), COALESCE(webdav_username, ''),
    COALESCE(s3_endpoint, ''), COALESCE(s3_bucket, ''), COALESCE(s3_region, ''),
    COALESCE(s3_path_prefix, 'little_timer/'),
    has_master_password, credentials_unlock_time,
    credential_unlock_attempts, credential_locked_until,
    has_webdav_password, has_s3_access_key, has_s3_secret_key
FROM backup_config WHERE id = 1;`

func (sm *SettingsManager) saveBackupConfigToDB() error {
	// 用户触发的保存一律要求已接线:即便当前凭据全空,漏接也必须显式
	// 失败,否则会把 has_* 静默清零、Secrets 里的旧值变孤儿。
	if sm.secretStore == nil {
		return errSecretStoreNotWired
	}
	return sm.writeBackupConfigRow()
}

// credentialFlag 绑定 has_* 列名与对应的 secret key —— 写分流与
// 打开时的列补齐共用这张表(顺序即 backupConfigSQL 末尾三列的顺序)。
type credentialFlag struct {
	column    string
	secretKey string
}

var credentialFlags = []credentialFlag{
	{column: "has_webdav_password", secretKey: SecretWebDAVPassword},
	{column: "has_s3_access_key", secretKey: SecretS3AccessKey},
	{column: "has_s3_secret_key", secretKey: SecretS3SecretKey},
}

// writeBackupConfigRow 透明分流(spec §6):凭据字段值非空 →
// SecretStorage.Store + has_*=1;值空 → Delete + has_*=0(SecretStorage
// 文档保证删不存在的 key 是 no-op)。secretStore 为 nil 时只允许全空的
// 凭据值 —— 那是构造期播种;任何非空值直接报 errSecretStoreNotWired。
// 日志只出现 has_* 布尔,绝不出现凭据值。
func (sm *SettingsManager) writeBackupConfigRow() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}

	values := []string{
		sm.backupConfig.WebDAVPassword,
		sm.backupConfig.S3AccessKey,
		sm.backupConfig.S3SecretKey,
	}
	has := make([]int64, len(credentialFlags))
	for i, c := range credentialFlags {
		v := values[i]
		if v == "" {
			if sm.secretStore != nil {
				if err := sm.secretStore.Delete([]byte(c.secretKey)); err != nil {
					return fmt.Errorf("settings: delete secret %s: %w", c.secretKey, err)
				}
			}
			continue
		}
		if sm.secretStore == nil {
			return errSecretStoreNotWired
		}
		if err := sm.secretStore.Store([]byte(c.secretKey), []byte(v)); err != nil {
			return fmt.Errorf("settings: store secret %s: %w", c.secretKey, err)
		}
		has[i] = 1
	}

	_, err := db.Exec(backupConfigSQL,
		sm.backupConfig.TargetType.String(),
		domain.BoolToInt(sm.backupConfig.Enabled),
		domain.BoolToInt(sm.backupConfig.AutoBackup),
		int64(sm.backupConfig.AutoBackupSecs),
		nullable(sm.backupConfig.LocalPath),
		nullable(sm.backupConfig.WebDAVURL),
		nullable(sm.backupConfig.WebDAVUsername),
		nullable(sm.backupConfig.S3Endpoint),
		nullable(sm.backupConfig.S3Bucket),
		nullable(sm.backupConfig.S3Region),
		nullable(sm.backupConfig.S3PathPrefix),
		domain.BoolToInt(sm.backupConfig.HasMasterPassword),
		sm.backupConfig.CredentialsUnlockTime,
		int64(sm.backupConfig.CredentialUnlockAttempts),
		sm.backupConfig.CredentialLockedUntil,
		has[0], has[1], has[2],
	)
	if err != nil {
		return fmt.Errorf("settings: save backup_config: %w", err)
	}
	log.Debug("settings: backup_config saved",
		"has_webdav_password", has[0] == 1,
		"has_s3_access_key", has[1] == 1,
		"has_s3_secret_key", has[2] == 1,
	)
	return nil
}

func (sm *SettingsManager) loadBackupConfigFromDB() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}

	row := db.QueryRow(loadBackupConfigSQL)
	var (
		targetTypeStr         string
		enabledRaw            bool
		autoBackupRaw         bool
		autoBackupIntervalRaw int64
		localPath             string
		webdavURL             string
		webdavUsername        string
		s3Endpoint            string
		s3Bucket              string
		s3Region              string
		s3PathPrefix          string
		hasMasterRaw          bool
		unlockTimeRaw         int64
		unlockAttemptsRaw     int64
		lockedUntilRaw        int64
		hasWebdav             int64
		hasS3Access           int64
		hasS3Secret           int64
	)
	if err := row.Scan(
		&targetTypeStr, &enabledRaw, &autoBackupRaw, &autoBackupIntervalRaw,
		&localPath, &webdavURL, &webdavUsername,
		&s3Endpoint, &s3Bucket, &s3Region,
		&s3PathPrefix,
		&hasMasterRaw, &unlockTimeRaw, &unlockAttemptsRaw, &lockedUntilRaw,
		&hasWebdav, &hasS3Access, &hasS3Secret,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sm.backupConfig = domain.NewDefaultBackupConfig()
			return nil
		}
		return fmt.Errorf("settings: load backup_config: %w", err)
	}

	sm.backupConfig = domain.BackupConfig{
		Enabled:                  enabledRaw,
		AutoBackup:               autoBackupRaw,
		AutoBackupSecs:           uint64(autoBackupIntervalRaw),
		TargetType:               parseTargetType(targetTypeStr),
		LocalPath:                localPath,
		WebDAVURL:                webdavURL,
		WebDAVUsername:           webdavUsername,
		WebDAVPassword:           sm.credentialFromStore(SecretWebDAVPassword, hasWebdav != 0),
		S3Endpoint:               s3Endpoint,
		S3Bucket:                 s3Bucket,
		S3Region:                 s3Region,
		S3AccessKey:              sm.credentialFromStore(SecretS3AccessKey, hasS3Access != 0),
		S3SecretKey:              sm.credentialFromStore(SecretS3SecretKey, hasS3Secret != 0),
		S3PathPrefix:             s3PathPrefix,
		HasMasterPassword:        hasMasterRaw,
		CredentialsUnlockTime:    unlockTimeRaw,
		CredentialUnlockAttempts: uint32(unlockAttemptsRaw),
		CredentialLockedUntil:    lockedUntilRaw,
	}
	return nil
}

// credentialFromStore 按 has_* 标记把凭据从 SecretStorage 拉回明文:
// 标记 0 → "";标记 1 但 store 未接线(构造窗口期)或 key 缺失 → "" 且
// 不报错。仅 keystore 不可用类错误记 warn —— 日志只带 key 名与错误文本,
// 绝不带凭据值(读取失败的"不可用"语义由 App 层 CredentialStatus 透出,
// spec §10)。
func (sm *SettingsManager) credentialFromStore(secretKey string, hasFlag bool) string {
	if !hasFlag {
		return ""
	}
	if sm.secretStore == nil {
		return ""
	}
	v, err := sm.secretStore.Retrieve([]byte(secretKey))
	if err != nil {
		if !errors.Is(err, crypto.ErrSecretNotFound) {
			log.Warn("settings: credential unreadable from secret store, degraded to empty",
				"key", secretKey, "error", err.Error())
		}
		return ""
	}
	return string(v)
}

// 凭据列补齐 —— backup_config 的 CREATE 在 storage/migration.go(本期不动
// storage 层),三个 has_* 标记列由这里在每次打开时幂等 ALTER 补上,
// 覆盖老库与 storage 旧 CREATE 产出的新库。

func (sm *SettingsManager) ensureBackupConfigCredentialFlags() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	for _, c := range credentialFlags {
		if err := addColumnIfMissing(db, "backup_config", c.column, "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("settings: ensure column %s: %w", c.column, err)
		}
	}
	return nil
}

// addColumnIfMissing 列已存在则 no-op;否则 ALTER TABLE ADD COLUMN。
// SQLite 的 ADD COLUMN 没有 IF NOT EXISTS,须先查 PRAGMA table_info
// (与 storage 层迁移助手的做法一致;table 名为固定字面量,无注入面)。
func addColumnIfMissing(db *sql.DB, table, column, def string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s;`, table, column, def))
	return err
}

// 辅助函数。

// resolveDatabasePath —— 调用方传空路径时，我们在
// `os.UserConfigDir()` 下挑平台默认值。Linux 用 `little_timer`，
// macOS/Windows 用 `LittleTimer`。
func resolveDatabasePath(input string) (string, error) {
	if input != "" {
		return input, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: UserConfigDir: %w", err)
	}
	appDir := "little_timer"
	// 基于主机名的切换（Windows/macOS 默认值 vs 其他）。
	if isWindowsLike() {
		appDir = "LittleTimer"
	} else if isMacLike() {
		appDir = "LittleTimer"
	}
	full := filepath.Join(dir, appDir, "little_timer.db")
	return full, nil
}

func isWindowsLike() bool {
	return os.PathSeparator == '\\' || strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")
}

func isMacLike() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OSTYPE")), "darwin") ||
		strings.Contains(strings.ToLower(os.Getenv("GOOS")), "darwin")
}

func parseTargetType(s string) domain.BackupTargetType {
	switch s {
	case "webdav":
		return domain.BackupTargetWebDAV
	case "s3":
		return domain.BackupTargetS3
	default:
		return domain.BackupTargetLocal
	}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, true
		}
		if f, err := x.Float64(); err == nil {
			return int64(f), true
		}
	case string:
		if i, err := strconv.ParseInt(x, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}
