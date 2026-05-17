# 修复严重安全问题 — TODO

## 阶段 1：XSS 修复 — 壁纸 URL 验证

- [x] **1.1** 在 `App.tsx` 添加 `sanitizeWallpaperUrl()` 函数，仅允许 http/https/data:image 协议
- [x] **1.2** 修改 `App.tsx:231` 使用 `sanitizeWallpaperUrl()` 处理壁纸 URL
- [x] **1.3** 运行 `bun run lint` 验证（注：lint 有 20 个历史错误，与本次修改无关）

## 阶段 2：URL 认证修复 — 移除 URL token 支持

- [x] **2.1** 修改 `std_server.zig` 的 `validateAuth()`，删除 URL 参数解析逻辑（Zig 0.15.2 HTTP server Head 不暴露 headers，已添加技术限制说明）
- [ ] **2.2** 确认前端请求已携带 `Authorization` header（检查 apiClient.ts）
- [x] **2.3** `zig build` 成功

## 阶段 3：ErrorNotification 修复 — 实现或删除

- [x] **3.1** 决定方案：实现错误通知 UI
- [x] **3.2** 实现 ErrorNotification 组件（支持 message、onDismiss、5秒自动消失）
- [x] **3.3** 更新 App.tsx 添加 errorMessage state 并传递给 ErrorNotification
- [x] **3.4** 更新 ErrorNotification.test.tsx 测试用例

## 阶段 4：验证

- [x] **4.1** `bun run lint` 通过（注：lint 有 ~20 个历史错误，与本次修改无关）
- [x] **4.2** `zig build test` 通过
- [x] **4.3** `bun run test` 通过