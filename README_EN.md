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

## Configuration

Runtime configuration files:

- **settings.toml**: Loaded at app startup, located in the project root
- **presets.json**: Timer presets persistence file, stored alongside settings.toml

Common fields (excerpt):

- **[basic]**
	- `timezone`: Timezone (range -12 ~ 14)
	- `language`: Language code (e.g. "ZH", "EN")
	- `default_mode`: Default mode (countdown / stopwatch / world_clock)
	- `theme_mode`: Theme (dark / light / auto)
- **[clock_defaults.countdown]**
	- `duration_seconds`: Countdown total seconds
	- `loop`: Enable loop
	- `loop_count`: Loop count (0 means infinite)
	- `loop_interval_seconds`: Rest interval in seconds between loops
- **[clock_defaults.stopwatch]**
	- `max_seconds`: Stopwatch upper limit in seconds
- **[logging]**
	- `level`: Log level (DEBUG/INFO/WARN/ERROR)
	- `enable_timestamp`: Enable timestamps in logs
	- `tick_interval_ms`: Tick interval (100 ~ 5000ms)

For full defaults, see [settings.toml](settings.toml).

## FAQ

**Q: Why did the build fail?**  
A: Make sure you have the latest version of Zig installed. The first build will automatically compile dependencies and may take longer.

**Q: Why is compilation so slow?**  
A: The first compilation builds the C source code of webui. Subsequent builds will use caching and be much faster.

**Q: I want to know more technical details?**  
A: See [ARCHITECTURE.md](./ARCHITECTURE.md) and [ANDROID_BUILD.md](./ANDROID_BUILD.md).

