import type { FunctionalComponent } from "preact";
import { useState, useEffect } from "preact/hooks";
import { t } from "../../utils/i18n";
import { getAPIClient } from "../../utils/apiClientSingleton";
import { showToast } from "../common/Toast";
import type { BackupConfig, CredentialStatus } from "../../types/api";

interface BackupTabProps {
  config: any;
  onChange: (config: any) => void;
}

interface BackupInfo {
  name: string;
  timestamp: number;
  size_bytes: number;
}

export const BackupTab: FunctionalComponent<BackupTabProps> = ({ config, onChange }) => {
  const apiClient = getAPIClient();
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isRestoring, setIsRestoring] = useState(false);
  const [isVerifying, setIsVerifying] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  // 凭据由系统密钥环托管（issue #13），不再有主口令/锁状态。
  // backend === "unavailable" 表示磁盘有凭据但密钥环解不开 —— 那是环境问题，
  // 要提示用户去修密钥服务，而不是重新填凭据。
  const [credentialStatus, setCredentialStatus] = useState<CredentialStatus>({
    backend: "",
    detail: "",
    has_secret: false,
  });
  const [backupConfig, setBackupConfig] = useState<BackupConfig>({
    enabled: config?.backup_enabled ?? false,
    target_type: config?.backup_target_type ?? 'local',
    local_path: config?.backup_local_path ?? '',
    webdav_url: config?.backup_webdav_url ?? '',
    webdav_username: config?.backup_webdav_username ?? '',
    webdav_password: config?.backup_webdav_password ?? '',
    webdav_path_prefix: config?.backup_webdav_path_prefix ?? 'little_timer/',
    s3_endpoint: config?.backup_s3_endpoint ?? '',
    s3_bucket: config?.backup_s3_bucket ?? '',
    s3_region: config?.backup_s3_region ?? '',
    s3_access_key: config?.backup_s3_access_key ?? '',
    s3_secret_key: config?.backup_s3_secret_key ?? '',
    s3_path_prefix: config?.backup_s3_path_prefix ?? '',
    auto_interval_hours: config?.backup_auto_interval_hours ?? 24,
    max_backups: config?.backup_max_backups ?? 7,
  });

  useEffect(() => {
    setBackupConfig({
      enabled: config?.backup_enabled ?? false,
      target_type: config?.backup_target_type ?? 'local',
      local_path: config?.backup_local_path ?? '',
      webdav_url: config?.backup_webdav_url ?? '',
      webdav_username: config?.backup_webdav_username ?? '',
      webdav_password: config?.backup_webdav_password ?? '',
      webdav_path_prefix: config?.backup_webdav_path_prefix ?? 'little_timer/',
      s3_endpoint: config?.backup_s3_endpoint ?? '',
      s3_bucket: config?.backup_s3_bucket ?? '',
      s3_region: config?.backup_s3_region ?? '',
      s3_access_key: config?.backup_s3_access_key ?? '',
      s3_secret_key: config?.backup_s3_secret_key ?? '',
      s3_path_prefix: config?.backup_s3_path_prefix ?? '',
      auto_interval_hours: config?.backup_auto_interval_hours ?? 24,
      max_backups: config?.backup_max_backups ?? 7,
    });
    void loadCredentialStatus();
  }, [config]);

  // 把后端标识映射到 i18n 键。未知后端统一归到 "other"——
  // 翻译缺失时宁可显示兜底文案，也不要在界面上裸奔原始标识。
  const credentialBackendKey = (backend: string): string => {
    if (backend === "secret-service") return "secretService";
    if (backend === "android-keystore") return "androidKeystore";
    return "other";
  };

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 4000);
  };

  const loadCredentialStatus = async () => {
    try {
      setCredentialStatus(await apiClient.getCredentialStatus());
    } catch (err) {
      console.error('Failed to load credential status:', err);
    }
  };


  const getBackupErrorText = (error: string | undefined): string | null => {
    if (!error) return null;
    const lower = error.toLowerCase();
    if (lower.includes("authentication failed")) {
      return t("backup.authentication_failed");
    }
    if (lower.includes("permission denied")) {
      return t("backup.permission_denied");
    }
    if (lower.includes("network error") || lower.includes("network")) {
      return t("backup.network_error");
    }
    if (lower.includes("connection failed") || lower.includes("connection")) {
      return t("backup.connection_failed");
    }
    return error;
  };

  const loadBackups = async () => {
    setIsLoading(true);
    try {
      const result = await apiClient.listBackups();
      if (result.success) {
        setBackups(result.backups ?? []);
      } else if (result.error) {
        const friendly = getBackupErrorText(result.error);
        if (friendly) {
          showMessage('error', friendly);
        }
      }
    } catch (err) {
      console.error('Failed to load backups:', err);
      showMessage('error', t("backup.load_failed"));
    } finally {
      setIsLoading(false);
    }
  };

  const handleCreateBackup = async () => {
    setIsCreating(true);
    setMessage(null);
    try {
      const result = await apiClient.createBackup();
      if (result.success) {
        showToast(t("backup.created_at", { path: result.backup_path }), 'success');
        await loadBackups();
      } else {
        showMessage('error', result.error || t("backup.create_failed"));
      }
    } catch (err) {
      console.error('Failed to create backup:', err);
      showMessage('error', t("backup.create_failed"));
    } finally {
      setIsCreating(false);
    }
  };

  const handleRestoreBackup = async (name: string) => {
    if (!confirm(t("backup.restore_confirm", { name }))) {
      return;
    }
    setIsRestoring(true);
    setMessage(null);
    try {
      const result = await apiClient.restoreBackup(name);
      if (result.success) {
        showToast(t("backup.restore_success"), 'success');
      } else {
        showMessage('error', result.error || t("backup.restore_failed"));
      }
    } catch (err) {
      console.error('Failed to restore backup:', err);
      showMessage('error', t("backup.restore_failed"));
    } finally {
      setIsRestoring(false);
    }
  };

  const handleDeleteBackup = async (name: string) => {
    if (!confirm(t("backup.delete_confirm", { name }))) {
      return;
    }
    try {
      const result = await apiClient.deleteBackup(name);
      if (result.success) {
        showToast(t("backup.delete_success"), 'success');
        await loadBackups();
      } else {
        showMessage('error', result.error || t("backup.delete_failed"));
      }
    } catch (err) {
      console.error('Failed to delete backup:', err);
      showMessage('error', t("backup.delete_failed"));
    }
  };

  const handleVerifyBackup = async () => {
    setIsVerifying(true);
    setMessage(null);
    try {
      const result = await apiClient.verifyBackup();
      if (result.success) {
        showToast(t("backup.verify_success"), 'success');
      } else {
        showMessage('error', result.error || t("backup.verify_failed"));
      }
    } catch (err) {
      console.error('Failed to verify backup:', err);
      showMessage('error', t("backup.verify_failed"));
    } finally {
      setIsVerifying(false);
    }
  };

  const handleConfigChange = (key: keyof BackupConfig, value: any) => {
    const newConfig = { ...backupConfig, [key]: value };
    setBackupConfig(newConfig);
    const newSettingsConfig = {
      ...config,
      [`backup_${key}`]: value,
    };
    onChange(newSettingsConfig);
  };

  useEffect(() => {
    void loadBackups();
  }, []);

  const formatSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const formatDate = (timestamp: number): string => {
    return new Date(timestamp * 1000).toLocaleString();
  };

  return (
    <div className="space-y-4 p-4">
      <h2 className="text-lg font-bold">{t("settings.tabs.backup")}</h2>

      {message && (
        <div className={`alert ${message.type === 'success' ? 'alert-success' : 'alert-error'}`}>
          <span>{message.text}</span>
        </div>
      )}

      <div className="form-control">
        <label className="label cursor-pointer">
          <span className="label-text">{t("backup.auto_backup")}</span>
          <input
            type="checkbox"
            className="toggle toggle-primary"
            checked={backupConfig.enabled}
            onChange={(e) => handleConfigChange('enabled', e.currentTarget.checked)}
          />
        </label>
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("backup.target_type")}</span>
        </label>
        <select
          className="select select-bordered w-full"
          value={backupConfig.target_type}
          onChange={(e) => handleConfigChange('target_type', e.currentTarget.value)}
        >
          <option value="local">{t("backup.target_local")}</option>
          <option value="webdav">{t("backup.target_webdav")}</option>
          <option value="s3">{t("backup.target_s3")}</option>
        </select>
      </div>

      {(backupConfig.target_type === 'webdav' || backupConfig.target_type === 's3') && (
        <div className="border border-base-300 rounded-lg p-4 space-y-2">
          <div className="flex items-center gap-2 text-sm">
            <span>🔐 {t("credentials.status")}:</span>
            {credentialStatus.backend === "unavailable" ? (
              <span className="text-error">{t("credentials.unavailable")}</span>
            ) : (
              <span className="text-success">
                {t(`credentials.backend.${credentialBackendKey(credentialStatus.backend)}`)}
              </span>
            )}
          </div>

          {credentialStatus.backend === "unavailable" && (
            <div className="alert alert-warning text-sm">
              <span>
                {t("credentials.unavailable_detail")}
                {credentialStatus.detail ? ` ${credentialStatus.detail}` : ""}
              </span>
            </div>
          )}

          <div className="flex gap-2">
            <button
              className="btn btn-sm btn-ghost"
              onClick={() => {
                if (!confirm(t("credentials.reset_confirm"))) return;
                void (async () => {
                  try {
                    await apiClient.resetCredentials();
                    showMessage("success", t("credentials.reset_done"));
                    await loadCredentialStatus();
                  } catch (err) {
                    showMessage("error", t("credentials.reset_failed"));
                    void err;
                  }
                })();
              }}
            >
              {t("credentials.reset")}
            </button>
          </div>
        </div>
      )}

      {backupConfig.target_type === 'local' && (
        <div className="form-control">
          <label className="label">
            <span className="label-text">{t("backup.local_path")}</span>
          </label>
          <input
            type="text"
            className="input input-bordered w-full"
            value={backupConfig.local_path || ''}
            onChange={(e) => handleConfigChange('local_path', e.currentTarget.value)}
            placeholder="/path/to/backups"
          />
        </div>
      )}

      {backupConfig.target_type === 'webdav' && (
        <>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.webdav_url")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.webdav_url || ''}
              onChange={(e) => handleConfigChange('webdav_url', e.currentTarget.value)}
              placeholder="https://dav.example.com/backup"
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.username")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.webdav_username || ''}
              onChange={(e) => handleConfigChange('webdav_username', e.currentTarget.value)}
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.password")}</span>
            </label>
            <input
              type="password"
              className="input input-bordered w-full"
              value={backupConfig.webdav_password || ''}
              onChange={(e) => handleConfigChange('webdav_password', e.currentTarget.value)}
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.path_prefix")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.webdav_path_prefix ?? 'little_timer/'}
              onChange={(e) => handleConfigChange('webdav_path_prefix', e.currentTarget.value)}
              placeholder="little_timer/"
            />
          </div>
        </>
      )}

      {backupConfig.target_type === 's3' && (
        <>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.s3_endpoint")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.s3_endpoint || ''}
              onChange={(e) => handleConfigChange('s3_endpoint', e.currentTarget.value)}
              placeholder="https://s3.amazonaws.com"
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.s3_bucket")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.s3_bucket || ''}
              onChange={(e) => handleConfigChange('s3_bucket', e.currentTarget.value)}
              placeholder="my-backups"
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.s3_region")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.s3_region || ''}
              onChange={(e) => handleConfigChange('s3_region', e.currentTarget.value)}
              placeholder="us-east-1"
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.s3_access_key")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.s3_access_key || ''}
              onChange={(e) => handleConfigChange('s3_access_key', e.currentTarget.value)}
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.s3_secret_key")}</span>
            </label>
            <input
              type="password"
              className="input input-bordered w-full"
              value={backupConfig.s3_secret_key || ''}
              onChange={(e) => handleConfigChange('s3_secret_key', e.currentTarget.value)}
            />
          </div>
          <div className="form-control">
            <label className="label">
              <span className="label-text">{t("backup.path_prefix")}</span>
            </label>
            <input
              type="text"
              className="input input-bordered w-full"
              value={backupConfig.s3_path_prefix || ''}
              onChange={(e) => handleConfigChange('s3_path_prefix', e.currentTarget.value)}
              placeholder="little_timer/"
            />
          </div>
        </>
      )}

      <div className="flex gap-2 mt-6">
        <button
          className="btn btn-primary"
          onClick={() => { void handleCreateBackup(); }}
          disabled={isCreating}
        >
          {isCreating ? (
            <span className="loading loading-spinner loading-sm" />
          ) : (
            t("backup.create_now")
          )}
        </button>
        <button
          className="btn btn-secondary"
          onClick={() => { void handleVerifyBackup(); }}
          disabled={isVerifying}
        >
          {isVerifying ? (
            <span className="loading loading-spinner loading-sm" />
          ) : (
            t("backup.verify_connection")
          )}
        </button>
      </div>

      <div className="mt-6">
        <h3 className="text-md font-semibold mb-2">{t("backup.existing_backups")}</h3>
        {isLoading ? (
          <div className="flex justify-center py-4">
            <span className="loading loading-spinner loading-sm" />
          </div>
        ) : backups.length === 0 ? (
          <p className="text-sm text-base-content/60">{t("backup.no_backups")}</p>
        ) : (
          <div className="space-y-2">
            {backups.map((backup) => (
              <div key={backup.name} className="flex items-center justify-between p-3 bg-base-200 rounded-lg">
                <div>
                  <p className="font-medium text-sm">{backup.name}</p>
                  <p className="text-xs text-base-content/60">
                    {formatDate(backup.timestamp)} · {formatSize(backup.size_bytes)}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    className="btn btn-sm btn-ghost"
                    onClick={() => { void handleRestoreBackup(backup.name); }}
                    disabled={isRestoring}
                  >
                    {t("backup.restore")}
                  </button>
                  <button
                    className="btn btn-sm btn-ghost text-error"
                    onClick={() => { void handleDeleteBackup(backup.name); }}
                  >
                    {t("backup.delete")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

