@echo off
REM 构建脚本 - Windows
REM 用法: scripts\build.bat [release]

setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."

cd /d "%PROJECT_ROOT%"

set "EMBED_UI=false"
set "OPTIMIZE=Debug"

if "%~1"=="release" (
    set "OPTIMIZE=Release"
    set "EMBED_UI=true"
)

echo === 构建前端 ===
cd assets
call bun install
call bun run build
cd ..

echo === 构建后端 (Optimize=%OPTIMIZE%, EmbedUI=%EMBED_UI%) ===
if "%EMBED_UI%"=="true" (
    zig build -Doptimize=%OPTIMIZE% -Dembed_ui=true
) else (
    zig build -Doptimize=%OPTIMIZE%
)

echo === 构建完成 ===
echo 运行: zig-out\bin\little_timer.exe
