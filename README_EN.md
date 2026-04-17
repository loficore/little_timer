# Little Timer

Little Timer is a cross-platform timer application built with Zig and WebUI, supporting countdown, stopwatch, and world clock features.

## Features

- 🎯 **Cross-Platform**: Supports Linux and Windows (Android support planned)
- ⚡ **High Performance**: Efficient backend written in Zig
- 🎨 **Modern UI**: Responsive interface based on Preact + Tailwind CSS
- 🔄 **Modular Architecture**: Clear separation between frontend and backend
- 📱 **Mobile First**: Touch-friendly with mobile device optimization

## License

This project is licensed under the [Apache License 2.0](./LICENSE). Please use it in compliance with the license.

## Quick Start

### Desktop (Linux / WSL / Windows)

For Windows, it's recommended to use MinGW64 to avoid potential dependency issues.

```bash
zig build run
```

Zig will automatically download dependencies, compile the C library, and run the application.

### Android

⚠️ **Current Status**: Android support is currently under development and not yet available. We plan to provide complete Android support in a future release.

## Dependencies & Environment

- **Zig**: Recommended version **0.15.2** (for building the backend and dependencies)
- **Node.js + pnpm**: For frontend development and builds (frontend is under assets/), use **v22.21.1**
- **System Libraries**: WebUI runtime dependencies (on Linux/Windows, ensure WebUI-related shared libraries can be loaded correctly)

> If you only run the backend with `zig build run`, the prebuilt frontend assets can be used directly. If you modify the UI, follow the “Frontend Development & Build” section below.

## Frontend Development & Build

Install dependencies in the frontend directory:

```bash
cd assets
pnpm install
```

Local development (HMR):

```bash
pnpm dev
```

Production build (outputs to assets/dist):

```bash
pnpm build
```

Lint:

```bash
pnpm lint
```

## Scripted Build & Packaging

Build on Linux / macOS:

```bash
./scripts/build.sh --release --embed-html
./scripts/build.sh --debug --embed-html
./scripts/build.sh --debug --no-embed-html
```

Build on Windows (PowerShell recommended):

```powershell
./scripts/build.ps1 --release --embed-html
./scripts/build.ps1 --debug --embed-html
./scripts/build.ps1 --debug --no-embed-html
```

`scripts/build.bat` is still available, but it is now a compatibility wrapper that forwards to `build.ps1`.

Packaging scripts:

```bash
./scripts/package_linux.sh --release --embed-html
./scripts/package_linux.sh --debug --no-embed-html

./scripts/package_mingw64.sh --release --embed-html
./scripts/package_mingw64.sh --debug --no-embed-html
```

## Configuration

Runtime configuration is now persisted in SQLite and no longer loaded from `settings.toml`.

- Main database: `little_timer.db` (default in the app working directory)
- Settings: stored in SQLite settings-related tables
- Presets and habits: stored in SQLite as well

You can read/update settings via API:

- `GET /api/settings`
- `POST /api/settings`

## FAQ

**Q: Why did the build fail?**  
A: Make sure you have the latest version of Zig installed. The first build will automatically compile dependencies and may take longer.

**Q: Why is compilation so slow?**  
A: The first compilation builds the C source code of webui. Subsequent builds will use caching and be much faster.

**Q: I want to know more technical details?**  
A: See [ARCHITECTURE.md](./ARCHITECTURE.md) and [ANDROID_BUILD.md](./ANDROID_BUILD.md).
