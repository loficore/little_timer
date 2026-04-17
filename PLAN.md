# Little Timer 开发计划

## Phase 20: 统计页面过滤栏按钮 Hover 样式修复

### 问题

统计页面过滤栏的按钮在鼠标悬浮时，已选中状态的强调色被 hover 样式覆盖，导致用户无法立即判断按钮是否被点击。

### 根因分析

`controls.css` 中：
- `.my-filter-btn-active` 定义选中状态样式
- `.my-filter-btn:hover:not(:disabled)` 定义 hover 状态样式
- CSS 优先级：class + pseudo-class (hover) > 单个 class (active)
- 导致 hover 时 active 样式被覆盖

### 修复方案

在 `.my-filter-btn-active` 后添加 `.my-filter-btn-active:hover` 规则，保持选中状态在 hover 时的视觉样式。

### 实现步骤

1. 修改 `assets/src/styles/components/controls.css`
2. 添加 `.my-filter-btn-active:hover` 样式规则
3. 构建并验证前端

---

## Phase 19: HTTP 库迁移 (httpx → std.http.Server) ⏳

### 背景

- 项目当前使用 httpx 作为 HTTP 服务器
- httpx 不支持长期 SSE 流式连接（只能一次性发送）
- 需要 SSE 功能实现实时计时器状态推送

### 迁移目标

使用 Zig 标准库 `std.http.Server` 替代 httpx：
- 统一处理所有 HTTP 请求
- 原生支持 SSE 流式响应（通过 `respondStreaming`）
- 无外部依赖

### 保留策略

**先保留 httpx 实现，确认 std.http.Server 方案无误后再删除**

### 实现步骤

1. **新增 std.http.Server 实现** (`src/core/http/std_server.zig`)
   - 实现所有现有 API 路由
   - 实现 SSE 流式响应
   - 与现有 httpx 版本并存

2. **配置切换机制**
   - 通过 build option 切换使用哪个服务器实现
   - 验证两种实现功能一致

3. **验收测试**
   - 对比两种实现的 API 响应
   - 测试 SSE 实时推送

4. **清理**
   - 确认 std.http.Server 版本稳定后
   - 删除 httpx 相关代码
   - 清理 build.zig 中的 httpx 依赖

---

## 参考项目

- 番茄TODO类应用的功能设计
- 习惯追踪的核心指标：累计时长、连续天数、完成率