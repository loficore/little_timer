@echo off
REM 运行脚本 - Windows

set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."

cd /d "%PROJECT_ROOT%"

set "EXE_PATH=%PROJECT_ROOT%\zig-out\bin\little_timer.exe"

if not exist "%EXE_PATH%" (
    echo 未找到可执行文件，正在构建...
    call "%SCRIPT_DIR%build.bat" release
)

echo === 启动 Little Timer ===
"%EXE_PATH%"
