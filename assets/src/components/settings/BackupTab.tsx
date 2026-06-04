import type { FunctionalComponent } from "preact";
import { useState, useEffect } from "preact/hooks";
import { t } from "../../utils/i18n";
import { getAPIClient } from "../../utils/apiClientSingleton";
import type { BackupConfig } from "../../utils/apiClient";

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
  const [backupConfig, setBackupConfig] = useState<BackupConfig>({
    enabled: config?.backup_enabled ?? false,
    target_type: config?.backup_target_type ?? 'local',
    local_path: config?.backup_local_path ?? '',
    webdav_url: config?.backup_webdav_url ?? '',
    webdav_username: config?.backup_webdav_username ?? '',
    webdav_password: config?.backup_webdav_password ?? '',
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
      s3_endpoint: config?.backup_s3_endpoint ?? '',
      s3_bucket: config?.backup_s3_bucket ?? '',
      s3_region: config?.backup_s3_region ?? '',
      s3_access_key: config?.backup_s3_access_key ?? '',
      s3_secret_key: config?.backup_s3_secret_key ?? '',
      s3_path_prefix: config?.backup_s3_path_prefix ?? '',
      auto_interval_hours: config?.backup_auto_interval_hours ?? 24,
      max_backups: config?.backup_max_backups ?? 7,
    });
  }, [config]);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 4000);
  };

  const getBackupErrorText = (code: string | undefined): string | null => {
    if (!code) return null;
    const map: Record<string, string> = {
      'FileNotFound': '',
      'BackupFailed': '',
      'NetworkError': '网络连接失败，请检查网络后重试',
      'ConnectionFailed': '连接失败，请检查服务器地址',
      'AuthenticationFailed': '认证失败，请检查用户名和密码',
      'PermissionDenied': '权限不足，请检查访问权限',
    };
    if (code in map) {
      const text = map[code];
      return text || null;
    }
    return code;
  };

  const loadBackups = async () => {
    setIsLoading(true);
    try {
      const result = await apiClient.listBackups();
      if (result.success) {
        setBackups(result.backups);
      } else if (result.error) {
        const friendly = getBackupErrorText(result.error);
        if (friendly) {
          showMessage('error', friendly);
        }
      }
    } catch (err) {
      console.error('Failed to load backups:', err);
      showMessage('error', 'Failed to load backups');
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
        showMessage('success', `Backup created: ${result.backup_path}`);
        await loadBackups();
      } else {
        showMessage('error', result.error || 'Failed to create backup');
      }
    } catch (err) {
      console.error('Failed to create backup:', err);
      showMessage('error', 'Failed to create backup');
    } finally {
      setIsCreating(false);
    }
  };

  const handleRestoreBackup = async (name: string) => {
    if (!confirm(`Restore from backup "${name}"? This will replace current data.`)) {
      return;
    }
    setIsRestoring(true);
    setMessage(null);
    try {
      const result = await apiClient.restoreBackup(name);
      if (result.success) {
        showMessage('success', 'Backup restored successfully');
      } else {
        showMessage('error', result.error || 'Failed to restore backup');
      }
    } catch (err) {
      console.error('Failed to restore backup:', err);
      showMessage('error', 'Failed to restore backup');
    } finally {
      setIsRestoring(false);
    }
  };

  const handleDeleteBackup = async (name: string) => {
    if (!confirm(`Delete backup "${name}"? This cannot be undone.`)) {
      return;
    }
    try {
      const result = await apiClient.deleteBackup(name);
      if (result.success) {
        showMessage('success', 'Backup deleted');
        await loadBackups();
      } else {
        showMessage('error', result.error || 'Failed to delete backup');
      }
    } catch (err) {
      console.error('Failed to delete backup:', err);
      showMessage('error', 'Failed to delete backup');
    }
  };

  const handleVerifyBackup = async () => {
    setIsVerifying(true);
    setMessage(null);
    try {
      const result = await apiClient.verifyBackup();
      if (result.success) {
        showMessage('success', 'Backup location verified successfully');
      } else {
        showMessage('error', result.error || 'Backup verification failed');
      }
    } catch (err) {
      console.error('Failed to verify backup:', err);
      showMessage('error', 'Failed to verify backup');
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
          <span className="label-text">启用自动备份</span>
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
          <span className="label-text">备份目标</span>
        </label>
        <select
          className="select select-bordered w-full"
          value={backupConfig.target_type}
          onChange={(e) => handleConfigChange('target_type', e.currentTarget.value)}
        >
          <option value="local">本地存储</option>
          <option value="webdav">WebDAV</option>
          <option value="s3">S3 兼容存储</option>
        </select>
      </div>

      {backupConfig.target_type === 'local' && (
        <div className="form-control">
          <label className="label">
            <span className="label-text">本地路径</span>
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
              <span className="label-text">WebDAV URL</span>
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
              <span className="label-text">用户名</span>
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
              <span className="label-text">密码</span>
            </label>
            <input
              type="password"
              className="input input-bordered w-full"
              value={backupConfig.webdav_password || ''}
              onChange={(e) => handleConfigChange('webdav_password', e.currentTarget.value)}
            />
          </div>
        </>
      )}

      {backupConfig.target_type === 's3' && (
        <>
          <div className="form-control">
            <label className="label">
              <span className="label-text">S3 Endpoint</span>
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
              <span className="label-text">Bucket</span>
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
              <span className="label-text">Region</span>
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
              <span className="label-text">Access Key</span>
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
              <span className="label-text">Secret Key</span>
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
              <span className="label-text">Path Prefix</span>
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
            '立即备份'
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
            '验证连接'
          )}
        </button>
      </div>

      <div className="mt-6">
        <h3 className="text-md font-semibold mb-2">现有备份</h3>
        {isLoading ? (
          <div className="flex justify-center py-4">
            <span className="loading loading-spinner loading-sm" />
          </div>
        ) : backups.length === 0 ? (
          <p className="text-sm text-base-content/60">暂无备份</p>
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
                    恢复
                  </button>
                  <button
                    className="btn btn-sm btn-ghost text-error"
                    onClick={() => { void handleDeleteBackup(backup.name); }}
                  >
                    删除
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