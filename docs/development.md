# 开发流程（main-only / GitHub Flow）

## 分支模型
- `main` 是唯一长期分支，也是唯一发布线；任何时刻都应处于可发布状态。
- 所有改动从 `main` 拉短命分支（`feat/*`、`fix/*`、`chore/*`），PR 合入后自动删除。
- 合并方式固定为 **squash**；`main` 保持线性历史。

## 提交约定
- 遵循 Conventional Commits：`type(scope): 描述`（如 `fix(aigtd): ...`、`ci: ...`、`chore(vrt): ...`）。
- 描述用中文；scope 可选，按目录/模块（`ci`、`vrt`、`aigtd`、`docs` 等）。
- 历史提交（`git log --oneline`）均已遵守此约定。

## 对外文案与缺口跟踪

- 用户可见字符串与错误文案禁止出现内部路线词(Plan N / v3 / not implemented / 内部文档路径)。
- 功能缺口一律走 GitHub issue 跟踪;代码注释引用 issue 号,不引用计划文档名。
- 平台能力差异用能力探测隐藏入口,不展示「暂不可用」类提示页。

## 必需检查
- 合并到 `main` 前必须通过：`frontend`、`backend`、`e2e`。
- `vrt`（视觉回归）目前为 advisory（`continue-on-error`），不阻断合并。
- 平台构建并入 `release.yml` 的三个构建 job，只在 release / 手动触发时运行，不阻断 PR。

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
- 在 `main` 上打 tag 并发 GitHub Release；`release.yml` 随即跑四个 job：
  - `linux-packages`（ubuntu-latest）：`just package` 出 tar.gz，`just package-linux` 出 rpm/deb（容器内构建，见 `docs/linux-packaging.md`）。
  - `windows-installer`（windows-latest）：`scripts/package-windows.sh` 出 NSIS 安装器（按用户安装，缺失 WebView2 运行时会被静默补装）与便携 zip；job 内再用 `scripts/smoke-windows-installer.sh` 跑「静默安装 → `LittleTimer.exe version` → 静默卸载 → 安装目录消失、`%AppData%\LittleTimer` 仍在」的冒烟断言。
  - `android-apk`（ubuntu-latest）：`scripts/build-android.sh` 出 debug 签名 APK。
  - `publish`（`needs` 上面三个）：git-cliff 生成 release notes，`ncipollo/release-action` 把六个产物挂上 Release。它是 Release 页产物的**唯一 writer**（`ci.yml` 不再构建任何平台产物）。
- 六个产物名（`scripts/check-artifact-names.sh` 在每个构建 job 打包后强制校验，不合规即 job 失败）：

  ```
  little-timer-<版本>-webkit40.el9.x86_64.rpm
  little-timer-<版本>-webkit41-x86_64.deb
  little-timer-<版本>-linux-x86_64.tar.gz
  little-timer-<版本>-windows-x86_64-setup.exe
  little-timer-<版本>-windows-x86_64-portable.zip
  little-timer-<版本>-android-universal-debug.apk
  ```

  形态固定为 `<项目名>-<版本>-<平台/引擎>-<架构>[-<种类>].<ext>`：平台/引擎槽只有 `webkit40` / `webkit41` / `linux` / `windows` / `android`，架构槽只有 `x86_64` / `universal`。
- 安装器与便携包的分工：`windows-x86_64-setup.exe` 会装 WebView2 运行时（机器上没有时静默补装）、建开始菜单与卸载项；`windows-x86_64-portable.zip` 只有一个 `LittleTimer.exe`，**不含运行时也不含安装步骤** —— 目标机器没有 WebView2 时解压即用是打不开的，这类机器请用安装器。两者都没有代码签名，首次运行会触发 SmartScreen 警告（见 spec §6）。
- 不再需要「同步 develop 到 main」的步骤。

### 版本号（唯一事实来源）

- 用户可见版本号 = **git tag**。构建时通过
  `-ldflags "-X little-timer/internal/app.Version=<tag>"` 注入；`just go-build` /
  `just go-dev` 取最近 tag（无 tag 用短 commit），release 流水线取 release tag。
- 后端 `GET /api/version` 返回 `{version, git_commit, build_time}`；前端 Sidebar
  只从这个接口取版本（`hooks/useAppVersion.ts`）。**禁止在组件/常量里写死版本**。
- 未注入的构建（CI、纯 `go run`）如实显示 `dev`，不再谎报某个历史版本。
- VRT 里版本元素已 `mask`，发版不会 churn 基线。

### Linux 包（本地）

`just package-linux [version]` 一次产出 rpm 与 deb；版本缺省取最近 git tag（去掉 `v`），无 tag 时用当天日期。也可分开跑 `just package-rpm` / `just package-deb`。

- rpm：AlmaLinux 9 容器构建，`-tags "webview,embed_ui"`（默认 webkit2gtk-4.0），链 `libwebkit2gtk-4.0.so.37`，产物 `dist/little-timer-<ver>-webkit40.el9.x86_64.rpm`（包内 `Release: webkit40%{?dist}`，不再是 `1webkit40`）。
- deb：Ubuntu 22.04 容器构建（glibc 2.35，覆盖 jammy/noble），`-tags "webview,embed_ui,webkit41"`，链 `libwebkit2gtk-4.1.so.0`，产物 `dist/little-timer-<ver>-webkit41-x86_64.deb`（包内 `Version: <ver>-webkit41`）。

宿主机已有 gtk3/webkit 开发包时脚本直接本机构建，否则自动进容器（依赖 `docker`，并挂载宿主机 `GOROOT`/`GOMODCACHE`）。两个包链接不同 WebKit SONAME，不能互换；包内容、依赖门禁与验收口径见 `docs/linux-packaging.md`。`just package` 的 tar.gz 仍走 `embed_ui`（无 webview 依赖），自带前端。

## 本地（jj-colocated）
- 用 jj bookmark 对应 GitHub 分支：`jj bookmark create <name> -r @`，推送用 `jj git push`。
- 定期 `jj git fetch` 同步远程；删除已合并的 bookmark 用 `jj bookmark delete <name>`。

## Windows 开发机运行指引

- **在 Windows 上从源码开发**：见 [docs/windows-msys2-dev.md](windows-msys2-dev.md) ——
  msys2 下的工具链、cgo 环境变量、HTTP-only 与原生 WebView 两种形态的跑法，以及
  webview 窗口「只在交互式桌面会话可见」这条硬限制。
- Windows 凭据后端为 DPAPI（`CryptProtectData`）：**无需任何额外服务或守护进程，登录即用**；
  主密钥由操作系统随用户档案托管，凭据 blob 绑定当前登录账户（跨账户/跨机不可解，属特性）。
- Windows 侧测试由 CI 的 `test-windows` job 执行（`go test -tags windows ./internal/keystore/`
  与 `go test -tags "windows integration_dpapi" ./internal/keystore/`），Linux 开发机不模拟。
- Linux 侧对 Windows 只做交叉验证（CI `backend` job 已内置同款门禁兜底）：

```bash
cd neo-src
CGO_ENABLED=0 GOOS=windows go build ./... && CGO_ENABLED=0 GOOS=windows go vet ./...
```

- 上面那道交叉门禁**不开 cgo、也不带 `webview` tag**，所以它证明不了「Windows 上能否
  从源码构建出带窗口的二进制」。那件事由 CI 的 `windows-msys2` job 负责：它在真实的
  msys2 + mingw-w64 环境里编译 HTTP-only 与 WebView 两个目标，且**不依赖 Windows SDK**
  （`EventToken.h` 兼容头由 `neo-src/third_party/webview_go/libs/mswebview2/include/`
  提供）。没有那个 job，「Windows 能不能开发」在 CI 里就永远是空白。

## Keystore 集成测试（Linux）

需要真实 Secret Service daemon（默认构建不跑，CI 的 `keystore-integration` job 会起一个）：

```bash
dbus-run-session -- \
  sh -c 'echo -n "" | gnome-keyring-daemon --unlock --daemonize --components=secrets && \
         go test -tags "linux integration_dbus" ./internal/keystore/'
```
