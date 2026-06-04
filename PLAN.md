# PLAN: 修复备份目标选择状态持久化

## 问题描述

用户在备份设置页面选择 WebDAV 或 S3 作为备份目标后，切换页面再回到设置页，备份目标自动回退为"本地存储"。配置虽然已正确保存到后端，但前端未能正确恢复用户的选择。

## 根因分析

两个独立 bug 叠加导致：

1. **Key 名称不匹配**：`Settings.tsx` 将备份目标类型存储为 `backup_target_type`，但 `BackupTab.tsx` 读取时用的 key 是 `backup_target`，导致始终读取到 `undefined`，回退为默认值 `'local'`。

2. **Props → 本地状态单向同步缺失**：`BackupTab` 用 `useState` 维护本地 `backupConfig` 状态，仅在首次挂载时从 props 初始化。`SettingsPage` 首次渲染时尚未拉取到后端配置，随后 `loadSettings()` 异步完成后更新了 props，但 `BackupTab` 的本地状态不会自动同步。即使修复 bug 1，若首次渲染早于异步加载完成，仍会显示错误默认值。

## 技术方案

在 `BackupTab.tsx` 中做两处修改：

1. **修复 key 名称**：`config?.backup_target` → `config?.backup_target_type`
2. **添加 useEffect 同步**：监听 `config` prop 变化，将变更同步到本地 `backupConfig` 状态。由于 `handleConfigChange` 同时更新本地状态和父组件状态，同步后的值一致，不会触发额外重渲染或无限循环。

## 影响范围

- **修改文件**：`assets/src/components/settings/BackupTab.tsx`（仅此一个文件）
- **不涉及后端**：后端 API 和存储层均正确，无需修改

---

# PLAN: 修复备份列表 FileNotFound 错误

## 问题描述

每次启动后首次进入备份设置页面，前端提示 `FileNotFound` 错误。这是因为尚未创建任何备份时，备份目录不存在，后端 `listBackups` 将目录不存在的系统错误直接返回给了前端。

## 根因分析

两个代码路径都会因备份目录不存在而报错：

1. **无适配器路径** (`storage_backup.zig:374`)：`try std.fs.cwd().openDir(self.backup_dir, ...)` — 目录不存在时 `try` 向上传播 Zig 标准错误 `error.FileNotFound`

2. **Local 适配器路径** (`BackupAdapter.zig:107`)：`openDir(...) catch return BackupError.BackupFailed` — 不区分"目录不存在"与真正的 I/O 错误

两种错误最终被 `handleBackupList` (std_server.zig:955) catch，用 `@errorName` 转为 `{"success":false,"error":"FileNotFound"}` 返回前端。

## 策略

备份目录不存在是**正常状态**（尚未创建任何备份），不应报错，应返回空备份列表。只有真正的 I/O 错误（权限、磁盘故障等）才应返回错误。

## 技术方案

| 文件 | 行号 | 修改 |
|------|------|------|
| `src/storage/storage_backup.zig` | 374 | `try openDir` → `openDir catch`，区分 `FileNotFound`（返回空列表）与其他错误（向上传播） |
| `src/storage/backup/BackupAdapter.zig` | 107 | `catch return BackupFailed` → 区分 `FileNotFound`（返回空列表）与其他错误（返回 `BackupFailed`） |

## 影响范围

- **修改文件**：
  - `src/storage/storage_backup.zig`
  - `src/storage/backup/BackupAdapter.zig`
- **不影响**前端代码

---

# PLAN: 修复 backup_dir 为空导致的 FileNotFound（第二波）

## 根因

`settings_manager.zig:92` 初始化 `SqliteManager` 时传入空字符串 `""` 作为 `backup_dir`。且 `updateBackupConfig` 从不重新初始化 `backup_manager` 的适配器，导致 `has_adapter` 始终为 `false`，所有备份列表/删除/恢复操作都走无适配器的本地文件系统路径，用空字符串去 `openDir("")`，在 Linux 上返回 `error.FileNotFound`。

此外 `storage_backup.zig` 中还有两处未受保护的 `try openDir`（`cleanupOldBackups:217` 和 `getBackupInfo:283`），在 backup_dir 为空时同样会传播 `FileNotFound`。

## 技术方案

1. **修复 `cleanupOldBackups`** (storage_backup.zig:217)：`try openDir` → 捕获 `FileNotFound` 时直接返回
2. **修复 `getBackupInfo`** (storage_backup.zig:283)：`try openDir` → 捕获 `FileNotFound` 时返回零值
3. **修复 `backup_dir` 默认值** (settings_manager.zig:92)：从 `""` 改为 DB 文件所在目录 `std.fs.path.dirname(db_path_full)`

## 修改汇总

| 文件 | 行号 | 修改 |
|------|------|------|
| `src/storage/storage_backup.zig` | 217 | `cleanupOldBackups` 中 `openDir` 捕获 `FileNotFound` → 直接返回 |
| `src/storage/storage_backup.zig` | 283 | `getBackupInfo` 中 `openDir` 捕获 `FileNotFound` → 返回零值 |
| `src/storage/storage_backup.zig` | 374 | (已有) `listBackups` 中 `openDir` 捕获 `FileNotFound` → 返回空列表 |
| `src/storage/backup/BackupAdapter.zig` | 107 | (已有) Local适配器 `listImpl` 捕获 `FileNotFound` → 返回空列表 |
| `src/settings/settings_manager.zig` | 92 | `backup_dir` 从 `""` 改为 `dirname(db_path_full)` |

## 未解决的架构问题

`updateBackupConfig` 不会重新初始化 `backup_manager` 适配器。`handleBackupList/Restore/Delete` 始终走无适配器路径（本地文件系统），未使用用户配置的 WebDAV/S3 适配器。相比之下 `handleBackupCreate/Verify` 会根据 `getBackupConfig()` 动态创建正确的适配器。后续应统一：要么让 `handleBackupList` 也动态创建适配器，要么让 `updateBackupConfig` 重新初始化 `backup_manager`。

---

# PLAN: 分层防御 — 彻底消除 FileNotFound 用户提示

## 问题本质

之前的修复逐行处理 `openDir` 的 `FileNotFound`，但只要 `handleBackupList` 的 catch 分支用 `@errorName(err)` 把原始错误码直接透传给前端，任何未覆盖到的代码路径泄漏的 `FileNotFound` 都会成为用户看到的无意义错误字符串。

## 技术方案：两层防护

### 第 1 层（后端兜底）：`handleBackupList` 不再向前端泄露原始错误码

| 文件 | 行号 | 修改 |
|------|------|------|
| `src/core/http/std_server.zig` | 955-962 | `listBackups()` catch 分支改为：服务端 `logger.err` + 返回 `{"success":true,"backups":[]}`。list 操作的核心语义是「列出备份」，找不到备份目录/文件是正常状态，不应报错。 |

**为什么这么做**：`listBackups` 的本质是只读查询操作，不存在备份目录或文件属于正常状态（无备份），永不应返回 error。只有基础设施错误（数据库未打开）才需要返回错误。

### 第 2 层（前端兜底）：错误文案映射

| 文件 | 行号 | 修改 |
|------|------|------|
| `assets/src/components/settings/BackupTab.tsx` | 70-74 | `loadBackups` 中：收到 `result.error` 时先查映射表，`FileNotFound`/`BackupFailed` 等良性错误不展示，仅网络/认证类错误展示语义化中文提示 |

## 修改汇总

一共 **2 个文件，2 处修改**，不再逐行修补而是从两端截断错误传播链路。
