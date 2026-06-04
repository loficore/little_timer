# TODO: 分层防御 — 彻底消除 FileNotFound 用户提示

- [ ] 1. 后端：修改 `src/core/http/std_server.zig` `handleBackupList` 的 catch 分支（第 955-962 行），将 `@errorName` 透传改为 `logger.err` + 返回空列表 `{"success":true,"backups":[]}`
- [ ] 2. 前端：修改 `assets/src/components/settings/BackupTab.tsx` `loadBackups` 函数，添加错误码→友好文案映射表，良性错误（FileNotFound/BackupFailed）不展示
- [ ] 3. 运行 `zig build test` 确保后端编译和测试通过
- [ ] 4. 运行 `cd assets && bun run lint` 确认前端代码规范
- [ ] 5. 运行 `cd assets && bun run test` 确认前端测试通过
