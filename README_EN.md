# Little Timer

Little Timer is a cross-platform timer application built with Go, Gin, SQLite, and WebView. It supports countdown, stopwatch, and world clock features. The frontend uses Preact, TypeScript, Vite, and Tailwind CSS.

## Features

- 🎯 **Cross-Platform**: Supports Linux and Windows, with an experimental Wails build path for Android
- ⚡ **Reliable Backend**: Built with Go, Gin, and SQLite
- 🖥️ **Desktop Runtime**: Optional WebView window or HTTP-only mode
- 🎨 **Modern UI**: Responsive interface based on Preact + Tailwind CSS
- 🔄 **Modular Architecture**: Clear separation between frontend and backend
- 📱 **Mobile First**: Touch-friendly with mobile device optimization
- 🤖 **AI GTD (experimental)**: capture → LLM decomposes into actionable subtasks; task tree, habits, and focus timer share the same local SQLite

## Scheduler + Smart Scheduling

- After setting an LLM API key under Settings, tasks captured in `/aigtd` automatically receive 5-dimensional scores (importance / urgency / energy / context / dependency)
- The `/schedule` page runs the Scheduler: deterministic placement driven by a rule engine plus LLM scores
- Every mutation carries a `version` optimistic lock; cross-device edit conflicts return a strict 409 and prompt a refetch
- Scoring is grounded in David Allen's GTD, the Eisenhower matrix, Loehr & Schwartz's energy management, and context-switching-cost research

## Daily Sweep / GTD Review

The `/daily-sweep` page offers a lightweight, daily-flavored version of GTD's classic weekly review:

- **Automatic trigger** — Settings → **Daily Review** tab: configure `enabled` + `HH:MM` (default 08:00). Cron runs at that time, once per day (manual trigger bypasses the once-per-day gate).
- **Manual trigger** — Sidebar entry **Daily Review** → "Run Review Now" button at the top.
- **Four proposal actions** — `reschedule` (re-schedule) / `drop` (abandon stale, >30 days) / `modify_subtasks` (change / delete / add) / `no_op`. Subtask ops: `delete` / `modify_title` / `add` / **`mark_done`** (active subtasks only — done/archived data is never touched by AI).
- **Adopt / reject, one by one** — adoption is recorded in the `adoptions` audit table. **Idempotent** — the same `(ai_job_id, change_id)` adopted twice applies only once; a UNIQUE index in the DB is the last line of defense against concurrent races. Task `version` is incremented by exactly 1 per adopt; cross-device conflicts return a strict 409 and prompt a refetch.
- **Prospective view** — shows today's (last 24h) successful review proposals; the `useDailySweep` hook handles 409 retries. Adopted proposals drop off the list and live in the audit table. A retrospective (yesterday's unfinished) view is left for a future release.

## License

This project is licensed under the [Apache License 2.0](./LICENSE). Please use it in compliance with the license.

## Quick Start

### Desktop (Linux / WSL / Windows)

The default development workflow starts the frontend development server and Go HTTP backend together:

```bash
just go-dev
```

To run only the HTTP backend on Linux:

```bash
cd neo-src
go run ./cmd/server serve --http-only
```

Use `just go-dev-webview` when you need a desktop WebView window. Linux WebView mode requires the `webkit2gtk-4.1` or `webkitgtk-6.0` system libraries.

### Android

⚠️ **Current Status**: An experimental Wails-based Android build script is available, but it requires a local Android SDK, NDK, and Gradle setup. Android release support is not complete. Use `just apk` to build a debug APK, or `just apk-package` to run only the Gradle packaging step.

## Dependencies & Environment

- **Go**: Use Go **1.25.0**, as declared in `neo-src/go.mod`
- **Node.js + pnpm**: For frontend development and builds (frontend is under assets/)
- **System Libraries**: Desktop WebView mode on Linux requires `webkit2gtk-4.1` or `webkitgtk-6.0`

> If you only run the HTTP backend, existing frontend assets can be used directly. If you modify the UI, follow the “Frontend Development & Build” section below.

## Frontend Development & Build

Install dependencies in the frontend directory:

```bash
cd assets
pnpm install
```

Local development (HMR):

```bash
pnpm run dev
```

Production build (outputs to assets/dist):

```bash
pnpm run build
```

Lint:

```bash
pnpm run lint
```

## Scripted Build & Packaging

Build the Go backend on Linux / macOS:

```bash
./scripts/build.sh --go --release
./scripts/build.sh --go --debug
```

The Windows PowerShell script still targets the older desktop build flow. Build the Go backend directly instead:

```powershell
cd neo-src
go build -o bin/server ./cmd/server/
```

Package the Go backend on Linux:

```bash
./scripts/package_go.sh --version 1.0.0
```

## Configuration

Runtime configuration is now persisted in SQLite and no longer loaded from `settings.toml`.

- Main database: `little_timer.db` (default in the app working directory)
- Settings: stored in SQLite settings-related tables
- Presets and habits: stored in SQLite as well

You can read/update settings via API:

- `GET /api/settings`
- `POST /api/settings`

## Tooling

```bash
# Build and vet the Go backend
just go-build
just go-vet

# List available tasks
just --list
```

## FAQ

**Q: Why did the build fail?**  
A: Make sure Go 1.25.0, Node.js, and pnpm are installed. When embedding the frontend, run `pnpm run build` in `assets/` first.

**Q: Why is compilation so slow?**  
A: The first build downloads Go and frontend dependencies. Later builds use the local caches.

**Q: I want to know more technical details?**  
A: See the [neo-src/](./neo-src/) and [android/](./android/) directories.
