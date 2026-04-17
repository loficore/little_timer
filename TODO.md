# Little Timer - 待办清单

---

## Phase 20: 统计页面过滤栏按钮 Hover 样式修复

### 任务清单

#### 1. 修复 CSS 样式

- [x] 修改 `assets/src/styles/components/controls.css`
- [x] 添加 `.my-filter-btn-active:hover` 样式规则
- [x] 保持选中状态在 hover 时的视觉样式

#### 2. 验证

- [x] `cd assets && bun run build` 通过
- [x] `cd assets && bun run lint` 通过

---

## Phase 19: HTTP 库迁移 (httpx → std.http.Server) ⏳

### 任务清单

#### 1. 新增 std.http.Server 实现 ✅

- [x] 创建 `src/core/http/std_server.zig` - 使用 std.http.Server
- [x] 实现所有现有 API 路由（与 httpx 版本一致）
- [x] 实现 SSE 流式响应 (`/api/events`)
- [x] 配置 build option 切换服务器实现 (`-Duse_std_http=true`)

#### 2. 验收测试 ✅

- [x] `zig build test` 通过（std 版本）
- [x] `zig build test` 通过（httpx 版本）

#### 3. 清理（旧版 httpx）

- [ ] 确认 std.http.Server 版本稳定
- [ ] 删除 httpx 版本代码 (`http_server.zig`)
- [ ] 清理 build.zig 中的 httpx 依赖

---

## Phase 18: HTTP 库迁移 (httpz → httpx) ✅

### 任务清单

#### 1. 后端迁移 ✅

- [x] 修改 `build.zig` - 移除 httpz 依赖
- [x] 重写 `src/core/http/http_server.zig` - 全部 handler 改为 httpx 风格
- [x] Windows 特殊处理适配（httpx 无需特殊处理）
- [x] `zig build test` 通过

#### 2. 前端 SSE 变更 ⏳ (待处理)

- [ ] 移除 SSE 连接逻辑 (`/api/events`)
- [ ] 添加轮询机制 (useEffect + setInterval)
- [ ] 验证计时器状态实时更新
- [ ] `bun run build` 通过
- [ ] `bun run lint` 通过

### 备注

- httpx 不支持长期 SSE 流（`startEventStreamSync()`），因此 SSE 功能改为前端轮询
- 路由 `/api/events` 已移除，后端不再提供
- 前端需要实现定期轮询 `/api/state` 获取计时器状态

#### 3. 验收测试

- [ ] 后端 API 全部正常响应
- [ ] 计时器开始/暂停/重置功能正常
- [ ] 习惯 CRUD 功能正常
- [ ] 前端轮询状态更新正常

---

## Phase 17: Windows 平台 SQLite 数据库初始化问题 ✅

### 已添加增强日志

- [x] `settings_manager.zig` - 初始化时打印数据库路径
- [x] `storage_sqlite.zig` - 打印迁移和健康检查各阶段日志
- [x] `habit_crud.zig` - 添加 logger 导入和详细错误日志
- [x] `storage_crud.zig` - 添加 db null 检查和日志
- [x] 打包成功

### 部署后需查看的日志

部署后应看到：
```
初始化 SQLite: C:\Users\...\little_timer.db
开始数据库迁移检查...
✓ SQLite 数据库已打开: ...
✓ 数据库模式检查完成，版本: 5
⚠️ 表 habit_sets 不存在  <- 如果有问题
重建表: habit_sets
✓ 所有关键表验证通过
✓ 数据库初始化完成
```
或如果 db 为 null：
```
❌ getAllHabitSets: db is null
```

---

## 待处理（需手动测试验证）

### 1. Windows 测试
- [ ] 在 Windows 平台上运行应用，验证数据库路径正确
- [ ] 查看日志中是否有表验证信息

### 2. UI 验证
- [ ] 设置页面遮罩视觉效果
- [ ] 深色模式文字颜色
- [ ] 弹窗焦点区域背景透明度

### 3. WebView 退出测试
- [ ] 启动 WebView → 点击退出 → 验证进程退出
- [ ] 验证端口 8080 被释放

---

## 测试扩展（低优先级）

| 类型 | 模块 |
|------|------|
| 组件 | TimerConfig, TimerProgress, DropdownSelect, SevenSegmentDisplay |
| 工具 | audio.ts, share.ts |