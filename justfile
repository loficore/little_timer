set shell := ["bash", "-eu", "-c"]

frontend-dev:
	@./scripts/dev.sh

dev-webview:
	@./scripts/dev.sh --webview

build-check:
	@zig build test && cd assets && bun run lint && bun run build:checkc
	

frontend-build:
	@cd assets && bun install && bun run build

backend-dev:
	@zig build -Dembed_ui=false -Doptimize=Debug run -- --webview
