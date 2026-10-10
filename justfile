set shell := ["bash", "-eu", "-c"]
set dotenv-load

import? "justfile.local"

SSH_TARGET := env_var_or_default("MY_ACT_SSH_TARGET", "")
REMOTE_PATH := env_var_or_default("MY_ACT_REMOTE_PATH", "")
ROOT := justfile_directory()

ci-run: go-build-check frontend-check

act:
        @if [ -n "{{SSH_TARGET}}" ]; then \
            echo "=== [远程调试] 通过 SSH 在 {{SSH_TARGET}} 运行 act ==="; \
            just r-act-start; \
        else \
            echo "=== [本地调试] 在当前机器运行 act ==="; \
            act --secret-file .act.env -P ubuntu-latest=catthehacker/ubuntu:act-latest; \
        fi

frontend-check:
        @cd assets && pnpm run lint && pnpm run build:check

frontend-build:
        @cd assets && pnpm install && pnpm run build

go_src := justfile_directory() / "neo-src"

# go-sqlite3 是 cgo-only，构建需要 CGO_ENABLED=1 + 一个 C 编译器。
# 优先 zig cc（自带 libc，CI 无 gcc 时兜底），其次 gcc / clang / cc。
export CC := `command -v zig >/dev/null 2>&1 && echo "zig cc" || (command -v gcc >/dev/null 2>&1 && echo gcc || (command -v clang >/dev/null 2>&1 && echo clang || echo cc))`
export CGO_ENABLED := "1"

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

go-dev host="":
        #!/usr/bin/env bash
        set -e
        trap 'kill $VITE_PID $GO_PID 2>/dev/null; exit 0' INT TERM

        find_available_port() {
            local port=$1
            while ss -tlnp | grep -q ":${port} "; do
                port=$((port + 1))
            done
            echo "$port"
        }

        GO_PORT=$(find_available_port 8013)
        export BACKEND_PORT=$GO_PORT

        HOST_FLAG=""
        if [ -n "{{host}}" ]; then
            HOST_FLAG="--host"
        fi

        echo "=== 启动前端 Dev Server ==="
        cd assets && pnpm run dev $HOST_FLAG &
        VITE_PID=$!
        cd ..
        echo "等待前端服务启动..."
        sleep 3

        echo "=== 启动 Go 后端 ==="
        cd {{go_src}} && go build -o bin/server ./cmd/server && bin/server serve --http-only --port $GO_PORT 2>&1 &
        GO_PID=$!

        FRONTEND_URL="http://localhost:5173"
        if [ -n "{{host}}" ]; then
            FRONTEND_URL="http://0.0.0.0:5173"
        fi

        echo ""
        echo "=== 服务已启动 ==="
        echo "前端: $FRONTEND_URL"
        echo "Go API: http://localhost:$GO_PORT"
        echo ""
        echo "按 Ctrl+C 停止所有服务"
        wait

go-dev-webview:
        #!/usr/bin/env bash
        set -e
        trap 'kill $VITE_PID $GO_PID 2>/dev/null; exit 0' INT TERM

        find_available_port() {
            local port=$1
            while ss -tlnp | grep -q ":${port} "; do
                port=$((port + 1))
            done
            echo "$port"
        }

        GO_PORT=$(find_available_port 8013)
        export BACKEND_PORT=$GO_PORT

        echo "=== 启动前端 Dev Server ==="
        cd assets && pnpm run dev &
        VITE_PID=$!
        cd ..
        echo "等待前端服务启动..."
        sleep 3

        echo "=== 启动 Go 后端 (webview) ==="
        cd {{go_src}} && go build -o bin/server ./cmd/server && bin/server serve --webview --port $GO_PORT &
        GO_PID=$!

        echo ""
        echo "=== 服务已启动 ==="
        echo "前端: http://localhost:5173"
        echo "Go API + WebView: http://localhost:$GO_PORT"
        echo ""
        echo "按 Ctrl+C 停止所有服务"
        wait

go-run: go-build
        @{{go_src}}/bin/server serve

go-clean:
        @rm -rf {{go_src}}/bin

go-build-embed:
        @cd {{go_src}} && go build -tags embed_ui -o bin/server ./cmd/server

apk:
        @./scripts/build-android.sh

apk-package:
        @./scripts/build-android.sh --package-only

bindings:
        @./scripts/generate-bindings.sh

webview-image:
        @podman build --network=host -f Containerfile -t little-timer-webview:latest \
                --build-arg HTTP_PROXY=${HTTP_PROXY:-} \
                --build-arg HTTPS_PROXY=${HTTPS_PROXY:-} \
                --build-arg http_proxy=${http_proxy:-${HTTP_PROXY:-}} \
                --build-arg https_proxy=${https_proxy:-${HTTPS_PROXY:-}} \
                .

webview-build:
        @podman run --rm --network=host -v "{{ROOT}}:/workspace:Z" -v little-timer-gomod:/root/go/pkg/mod -v little-timer-gocache:/root/.cache/go-build -w /workspace little-timer-webview:latest bash -c 'mkdir -p neo-src/bin && cd neo-src && go build -tags "webview,embed_ui" -o bin/server ./cmd/server && cd .. && bash scripts/generate-bindings.sh'

# 打包发布产物：前端 build + go build (embed_ui) + tar.gz 到 dist/
# 用法: just package [version] [embed_ui]
#   version   默认值 = 今天日期 (YYYYMMDD)
#   embed_ui  默认值 = true；传 false 关闭
package version="" embed_ui="true":
        #!/usr/bin/env bash
        set -euo pipefail
        ver="{{version}}"
        if [ -z "$ver" ]; then ver="$(date +%Y%m%d)"; fi
        if [ "{{embed_ui}}" = "true" ]; then tag="embed_ui"; else tag=""; fi
        cd {{ROOT}}/assets
        [ -d node_modules ] || pnpm install
        pnpm run build
        mkdir -p dist/i18n
        cp -f {{ROOT}}/assets/i18n/*.toml dist/i18n/
        cd {{ROOT}}/neo-src
        go build -tags "$tag" -ldflags="-s -w -X little-timer/internal/app.Version=$ver" -o bin/server ./cmd/server
        stage={{ROOT}}/dist/stage
        rm -rf "$stage"
        mkdir -p "$stage"
        cp bin/server "$stage/"
        sanitized="$(echo "$ver" | sed -E 's/[^A-Za-z0-9._-]/_/g')"
        mkdir -p {{ROOT}}/dist
        tar -czf {{ROOT}}/dist/little_timer-${sanitized}-linux-x64.tar.gz -C "$stage" .
        echo "✅ {{ROOT}}/dist/little_timer-${sanitized}-linux-x64.tar.gz"

default: go-dev