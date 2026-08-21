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
// 凭据加密：webdav_password / s3_access_key / s3_secret_key 用 AES-256-GCM
// 静态加密，密钥由固定应用 secret + SQLite 文件路径派生（不依赖 OS
// keychain —— 见 `deriveCredentialKey`）。这满足“磁盘上绝无明文凭据”的
// 约束，同时让加密包端到端受测。TODO: 等 dbus / Windows 凭据库落地后接入
// OS keychain。
package settings

import (
	"crypto/sha256"
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
	"little-timer/internal/storage"
)

// SettingsManager 持有基于 SQLite 的内存 settings + backup 配置。
type SettingsManager struct {
	sqlite *storage.SqliteManager
	dbPath string // 构造时捕获，用于凭据密钥派生

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

	// credentialUnlockPassword 跟踪凭据子系统的“主口令已解锁”状态。
	// 锁定时为 nil。
	credentialUnlockPassword []byte
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

	return newFromSqlite(mgr, resolved)
}

// NewFromSqliteManager 包装一个已打开的 SqliteManager。适用于测试框架
// 已经开好 DB 的场景。`dbPath` 会被捕获用于凭据密钥派生；若底层 DB 在
// 其他路径，请显式提供。
func NewFromSqliteManager(mgr *storage.SqliteManager, dbPath string) (*SettingsManager, error) {
	if mgr == nil || !mgr.IsOpen() {
		return nil, errors.New("settings: sqlite manager is nil or not open")
	}
	return newFromSqlite(mgr, dbPath)
}

func newFromSqlite(mgr *storage.SqliteManager, dbPath string) (*SettingsManager, error) {
	sm := &SettingsManager{
		sqlite:  mgr,
		dbPath:  dbPath,
		presets: NewPresetsManager(),
		viper:   viper.New(),
		// config / backupConfig 由 Load() 填充；不先 Load 的调用方
		// 会看到零值。
	}
	if err := sm.loadAll(); err != nil {
		return nil, err
	}
	return sm, nil
}

// Load / Save。

// Load 从 SQLite 重读全部 settings 行并重新填充 viper。
func (sm *SettingsManager) Load() error {
	return sm.loadAll()
}

// Save 把内存中的 config + backup 配置刷入 SQLite。
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
	if err := sm.Save(); err != nil {
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
// settings/crypto/backup 边界自包含。schema 列不变。

const backupConfigSQL = `INSERT OR REPLACE INTO backup_config (
    id, target_type, enabled, auto_backup, auto_backup_interval,
    local_path, webdav_url, webdav_username, webdav_password_encrypted,
    s3_endpoint, s3_bucket, s3_region,
    s3_access_key_encrypted, s3_secret_key_encrypted,
    s3_path_prefix,
    has_master_password, credentials_unlock_time,
    credential_unlock_attempts, credential_locked_until
) VALUES (
    1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);`

const loadBackupConfigSQL = `SELECT
    target_type, enabled, auto_backup, auto_backup_interval,
    COALESCE(local_path, ''), COALESCE(webdav_url, ''), COALESCE(webdav_username, ''),
    webdav_password_encrypted,
    COALESCE(s3_endpoint, ''), COALESCE(s3_bucket, ''), COALESCE(s3_region, ''),
    s3_access_key_encrypted, s3_secret_key_encrypted,
    COALESCE(s3_path_prefix, 'little_timer/'),
    has_master_password, credentials_unlock_time,
    credential_unlock_attempts, credential_locked_until
FROM backup_config WHERE id = 1;`

func (sm *SettingsManager) saveBackupConfigToDB() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	key := sm.deriveCredentialKey()

	webdavPwdBlob, err := sm.encryptOptional(key, []byte(sm.backupConfig.WebDAVPassword))
	if err != nil {
		return fmt.Errorf("settings: encrypt webdav_password: %w", err)
	}
	s3AccessBlob, err := sm.encryptOptional(key, []byte(sm.backupConfig.S3AccessKey))
	if err != nil {
		return fmt.Errorf("settings: encrypt s3_access_key: %w", err)
	}
	s3SecretBlob, err := sm.encryptOptional(key, []byte(sm.backupConfig.S3SecretKey))
	if err != nil {
		return fmt.Errorf("settings: encrypt s3_secret_key: %w", err)
	}

	_, err = db.Exec(backupConfigSQL,
		sm.backupConfig.TargetType.String(),
		domain.BoolToInt(sm.backupConfig.Enabled),
		domain.BoolToInt(sm.backupConfig.AutoBackup),
		int64(sm.backupConfig.AutoBackupSecs),
		nullable(sm.backupConfig.LocalPath),
		nullable(sm.backupConfig.WebDAVURL),
		nullable(sm.backupConfig.WebDAVUsername),
		webdavPwdBlob,
		nullable(sm.backupConfig.S3Endpoint),
		nullable(sm.backupConfig.S3Bucket),
		nullable(sm.backupConfig.S3Region),
		s3AccessBlob,
		s3SecretBlob,
		nullable(sm.backupConfig.S3PathPrefix),
		domain.BoolToInt(sm.backupConfig.HasMasterPassword),
		sm.backupConfig.CredentialsUnlockTime,
		int64(sm.backupConfig.CredentialUnlockAttempts),
		sm.backupConfig.CredentialLockedUntil,
	)
	if err != nil {
		return fmt.Errorf("settings: save backup_config: %w", err)
	}
	return nil
}

func (sm *SettingsManager) loadBackupConfigFromDB() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	key := sm.deriveCredentialKey()

	row := db.QueryRow(loadBackupConfigSQL)
	var (
		targetTypeStr         string
		enabledRaw            bool
		autoBackupRaw         bool
		autoBackupIntervalRaw int64
		localPath             string
		webdavURL             string
		webdavUsername        string
		webdavPwdBlob         []byte
		s3Endpoint            string
		s3Bucket              string
		s3Region              string
		s3AccessBlob          []byte
		s3SecretBlob          []byte
		s3PathPrefix          string
		hasMasterRaw          bool
		unlockTimeRaw         int64
		unlockAttemptsRaw     int64
		lockedUntilRaw        int64
	)
	if err := row.Scan(
		&targetTypeStr, &enabledRaw, &autoBackupRaw, &autoBackupIntervalRaw,
		&localPath, &webdavURL, &webdavUsername, &webdavPwdBlob,
		&s3Endpoint, &s3Bucket, &s3Region,
		&s3AccessBlob, &s3SecretBlob, &s3PathPrefix,
		&hasMasterRaw, &unlockTimeRaw, &unlockAttemptsRaw, &lockedUntilRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sm.backupConfig = domain.NewDefaultBackupConfig()
			return nil
		}
		return fmt.Errorf("settings: load backup_config: %w", err)
	}

	webdavPwd, err := sm.decryptOptional(key, webdavPwdBlob)
	if err != nil {
		webdavPwd = ""
	}
	s3Access, err := sm.decryptOptional(key, s3AccessBlob)
	if err != nil {
		s3Access = ""
	}
	s3Secret, err := sm.decryptOptional(key, s3SecretBlob)
	if err != nil {
		s3Secret = ""
	}

	sm.backupConfig = domain.BackupConfig{
		Enabled:                  enabledRaw,
		AutoBackup:               autoBackupRaw,
		AutoBackupSecs:           uint64(autoBackupIntervalRaw),
		TargetType:               parseTargetType(targetTypeStr),
		LocalPath:                localPath,
		WebDAVURL:                webdavURL,
		WebDAVUsername:           webdavUsername,
		WebDAVPassword:           webdavPwd,
		S3Endpoint:               s3Endpoint,
		S3Bucket:                 s3Bucket,
		S3Region:                 s3Region,
		S3AccessKey:              s3Access,
		S3SecretKey:              s3Secret,
		S3PathPrefix:             s3PathPrefix,
		HasMasterPassword:        hasMasterRaw,
		CredentialsUnlockTime:    unlockTimeRaw,
		CredentialUnlockAttempts: uint32(unlockAttemptsRaw),
		CredentialLockedUntil:    lockedUntilRaw,
	}
	return nil
}

// 凭据加密（AES-256-GCM + 确定性密钥 —— 设计理由与 OS-keychain TODO
// 见文件头）。

func (sm *SettingsManager) deriveCredentialKey() []byte {
	h := sha256.New()
	h.Write([]byte("little-timer-credential-key-v1"))
	h.Write([]byte(sm.dbPath))
	return h.Sum(nil)
}

// encryptOptional 对空明文返回 nil。
func (sm *SettingsManager) encryptOptional(key, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	nonce := crypto.GenerateNonce()
	return crypto.Encrypt(plaintext, key, nonce)
}

// decryptOptional 在 blob 缺失或短到不可能是合法密文时返回 ""。
func (sm *SettingsManager) decryptOptional(key, blob []byte) (string, error) {
	if len(blob) < crypto.AES256GCMNonceSize+crypto.AES256GCMTagSize {
		return "", nil
	}
	pt, err := crypto.Decrypt(blob, key)
	if err != nil {
		return "", err
	}
	return string(pt), nil
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
