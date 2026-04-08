#!/usr/bin/env pwsh
# 构建脚本 - Windows PowerShell
# 用法: ./scripts/build.ps1 [--debug|--release] [--embed-html|--no-embed-html]

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
Set-Location $ProjectRoot

$embedUi = $false
$optimize = 'Release'

function Show-Help {
    Write-Host "用法: ./scripts/build.ps1 [选项]"
    Write-Host "选项:"
    Write-Host "  --release         发布构建（仅设置优化级别）"
    Write-Host "  --debug           调试构建（仅设置优化级别）"
    Write-Host "  --embed-html      内嵌前端 HTML 到后端二进制"
    Write-Host "  --no-embed-html   不内嵌前端 HTML（默认）"
    Write-Host "  --help, -h        显示此帮助"
    Write-Host ""
    Write-Host "示例:"
    Write-Host "  ./scripts/build.ps1 --release --embed-html"
    Write-Host "  ./scripts/build.ps1 --debug --embed-html"
    Write-Host "  ./scripts/build.ps1 --debug --no-embed-html"
}

foreach ($arg in $args) {
    switch ($arg) {
        '--release' { $optimize = 'Release' }
        '--debug' { $optimize = 'Debug' }
        '--embed-html' { $embedUi = $true }
        '--embed-ui' { $embedUi = $true }
        '--no-embed-html' { $embedUi = $false }
        '--no-embed-ui' { $embedUi = $false }
        '--help' {
            Show-Help
            exit 0
        }
        '-h' {
            Show-Help
            exit 0
        }
        default {
            Write-Host "错误: 未知参数 '$arg'"
            Write-Host "使用 --help 查看可用选项"
            exit 1
        }
    }
}

Write-Host "=== 构建前端 ==="
if (-not (Get-Command bun -ErrorAction SilentlyContinue)) {
    Write-Host "错误: 未找到 bun"
    exit 1
}

Push-Location assets
& bun install
& bun run build
Pop-Location

Write-Host "=== 构建后端 (Optimize=$optimize, EmbedUI=$embedUi) ==="
if (-not (Get-Command zig -ErrorAction SilentlyContinue)) {
    Write-Host "错误: 未找到 zig"
    exit 1
}

if ($embedUi) {
    & zig build "-Doptimize=$optimize" "-Dembed_ui=true"
} else {
    & zig build "-Doptimize=$optimize"
}

Write-Host "=== 构建完成 ==="
Write-Host "运行: .\zig-out\bin\little_timer.exe"
