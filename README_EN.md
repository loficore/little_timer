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

## FAQ

**Q: Why did the build fail?**  
A: Make sure you have the latest version of Zig installed. The first build will automatically compile dependencies and may take longer.

**Q: Why is compilation so slow?**  
A: The first compilation builds the C source code of webui. Subsequent builds will use caching and be much faster.

**Q: I want to know more technical details?**  
A: See [ARCHITECTURE.md](./ARCHITECTURE.md) and [ANDROID_BUILD.md](./ANDROID_BUILD.md).

