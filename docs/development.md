# 开发流程（main-only / GitHub Flow）

## 分支模型
- `main` 是唯一长期分支，也是唯一发布线；任何时刻都应处于可发布状态。
- 所有改动从 `main` 拉短命分支（`feat/*`、`fix/*`、`chore/*`），PR 合入后自动删除。
- 合并方式固定为 **squash**；`main` 保持线性历史。

## 提交约定
- 遵循 Conventional Commits：`type(scope): 描述`（如 `fix(aigtd): ...`、`ci: ...`、`chore(vrt): ...`）。
- 描述用中文；scope 可选，按目录/模块（`ci`、`vrt`、`aigtd`、`docs` 等）。
- 历史提交（`git log --oneline`）均已遵守此约定。

## 必需检查
- 合并到 `main` 前必须通过：`frontend`、`backend`、`e2e`。
- `vrt`（视觉回归）目前为 advisory（`continue-on-error`），不阻断合并。
- 平台构建（`build` matrix）只在 release / 手动触发时运行，不阻断 PR。

## VRT 基线重生成
- 基线必须在与 CI runner 等价的字体环境生成：本机 CJK 回退字形/行高与
  runner（Ubuntu 24.04 + `fonts-noto-cjk=1:20230817+repack1-3`）不一致，
  直接本机 `test:vrt:update` 会导致 vrt.yml 确定性失败。
- 用容器重生成（与 `.github/workflows/vrt.yml` 同栈）：
  `mcr.microsoft.com/playwright:v<与 package.json 同版本>-noble` 挂载仓库，
  容器内 `apt-get install fonts-noto-cjk=1:20230817+repack1-3 gcc` →
  `scripts/generate-bindings.sh`（需 `GOPROXY=https://goproxy.cn,direct`）→
  `CGO_ENABLED=1 go build -o bin/server ./cmd/server`（neo-src）→
  删除 `assets/test_tmp` 后 `CI=true PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
  pnpm exec playwright test --update-snapshots --project=vrt-mobile-390
  --project=vrt-mobile-412 --project=vrt-desktop-1280 --workers=1`。

## 发布
- 在 `main` 上打 tag 并发 GitHub Release；`release.yml` 随即打包产物。
- 不再需要「同步 develop 到 main」的步骤。

## 本地（jj-colocated）
- 用 jj bookmark 对应 GitHub 分支：`jj bookmark create <name> -r @`，推送用 `jj git push`。
- 定期 `jj git fetch` 同步远程；删除已合并的 bookmark 用 `jj bookmark delete <name>`。
