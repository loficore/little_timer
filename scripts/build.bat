@echo off
REM 构建脚本 - Windows（兼容入口，转发到 PowerShell 脚本）
REM 用法: scripts\build.bat [--debug|--release] [--embed-html|--no-embed-html]

setlocal

set "SCRIPT_DIR=%~dp0"
set "PS_SCRIPT=%SCRIPT_DIR%build.ps1"

if not exist "%PS_SCRIPT%" (
    echo 错误: 未找到脚本 "%PS_SCRIPT%"
    exit /b 1
)

where pwsh >nul 2>nul
if %ERRORLEVEL%==0 (
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%PS_SCRIPT%" %*
) else (
    powershell -NoProfile -ExecutionPolicy Bypass -File "%PS_SCRIPT%" %*
)

exit /b %ERRORLEVEL%
