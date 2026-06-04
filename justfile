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
