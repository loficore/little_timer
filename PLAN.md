# 修复严重安全问题 — 计划

## 问题根因

### 1. XSS 风险 — `App.tsx:231`
壁纸 URL 直接拼入 `url()` 字符串，仅做简单的引号转义，未验证协议类型。攻击者可通过设置恶意 SVG data URL 或 `javascript:` 协议执行 XSS。

### 2. URL 认证风险 — `std_server.zig:314-325`
认证回退到 URL query 参数 `auth_token`，token 作为 GET 参数暴露在 URL 中，存在日志泄露、中间人攻击风险。

### 3. 死代码组件 — `ErrorNotification.tsx`
组件始终返回 `null`，错误被静默吞掉，用户无法感知操作失败。

## 技术路径

### 1. XSS 修复
在 `App.tsx` 中新增 `sanitizeWallpaperUrl()` 函数：
- 仅允许 `http://`、`https://`、`data:image/` 协议
- 拒绝 `javascript:`、`data:`（非图片）等危险协议
- 保持向后兼容，现有的合法壁纸不受影响

### 2. URL 认证修复
移除 `auth_token` URL 参数支持，强制使用 HTTP Header：
- 删除 `std_server.zig` 中 URL 参数解析逻辑
- 通过 `request.head.headers` 读取 `Authorization: Bearer <token>`
- 要求前端在请求时携带 `Authorization` header

### 3. ErrorNotification 修复
实现真正的错误通知 UI，或删除组件并在应用层统一处理：
- 采用 Toast/Notification 方案显示操作错误
- 钩子层暴露错误信息，而非仅 `console.error`

## 阶段目标

- **阶段 1**：XSS 修复 — 壁纸 URL 验证
- **阶段 2**：URL 认证修复 — 移除 URL token 支持
- **阶段 3**：ErrorNotification 修复 — 实现或删除
- **阶段 4**：验证 — lint + test 通过