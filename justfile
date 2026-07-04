set shell := ["bash", "-eu", "-c"]

frontend-dev:
	@./scripts/dev.sh

dev-webview:
	@./scripts/dev.sh --webview

build-check:
	@zig build test && cd assets && pnpm run lint && pnpm run build:checkc
	

frontend-build:
	@cd assets && pnpm install && pnpm run build

backend-dev:
	@zig build -Dembed_ui=false -Doptimize=Debug run -- --webview

# ── Go backend (neo-src) ──────────────────────────────────────────────

go_src := justfile_directory() / "neo-src"

go-build:
	@cd {{go_src}} && go build -o bin/server ./cmd/server

go-test:
	@cd {{go_src}} && go test ./...

go-test-race:
	@cd {{go_src}} && go test -race ./...

go-vet:
	@cd {{go_src}} && go vet ./...

go-tidy:
	@cd {{go_src}} && go mod tidy

go-lint: go-vet

go-build-check: go-tidy go-vet go-test

go-dev:
	#!/usr/bin/env bash
	set -e
	trap 'kill $VITE_PID $GO_PID 2>/dev/null; exit 0' INT TERM

	echo "=== 启动前端 Dev Server ==="
	cd assets && pnpm run dev &
	VITE_PID=$!
	cd ..
	echo "等待前端服务启动..."
	sleep 3

	echo "=== 启动 Go 后端 ==="
	cd {{go_src}} && go build -o bin/server ./cmd/server && bin/server serve --http-only &
	GO_PID=$!

	echo ""
	echo "=== 服务已启动 ==="
	echo "前端: http://localhost:5173"
	echo "Go API: http://localhost:8080"
	echo ""
	echo "按 Ctrl+C 停止所有服务"
	wait

go-dev-webview:
	#!/usr/bin/env bash
	set -e
	trap 'kill $VITE_PID $GO_PID 2>/dev/null; exit 0' INT TERM

	echo "=== 启动前端 Dev Server ==="
	cd assets && pnpm run dev &
	VITE_PID=$!
	cd ..
	echo "等待前端服务启动..."
	sleep 3

	echo "=== 启动 Go 后端 (webview) ==="
	cd {{go_src}} && go build -o bin/server ./cmd/server && bin/server serve --webview &
	GO_PID=$!

	echo ""
	echo "=== 服务已启动 ==="
	echo "前端: http://localhost:5173"
	echo "Go API + WebView: http://localhost:8080"
	echo ""
	echo "按 Ctrl+C 停止所有服务"
	wait

go-run: go-build
	@{{go_src}}/bin/server serve

go-clean:
	@rm -rf {{go_src}}/bin

go-build-embed:
	@cd {{go_src}} && go build -tags embed_ui -o bin/server ./cmd/server

# ── Android ────────────────────────────────────────────────────────────

# 编译 Android APK（一键：bindings 生成 + 前端 + Go .so + Gradle）
apk:
	@./scripts/build-android.sh

# 仅打包 APK（假设 .so 已编译好）
apk-package:
	@./scripts/build-android.sh --package-only

# 生成 Wails bindings（手动）
bindings:
	@./scripts/generate-bindings.sh

default: go-dev
