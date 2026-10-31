# Embedded UI staging

`-tags embed_ui` 的构建会把本目录通过 `//go:embed all:assets` 打进二进制。
构建前先执行 `scripts/stage-embedded-ui.sh`，它会把 `assets/dist` 的
`index.html`、`fonts/`、`i18n/` 同步到这里（`bindings/` 由 Wails 绑定
生成脚本维护）。本 README 只用于保证干净检出时该目录存在。
