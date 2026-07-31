@echo off
setlocal
cd /d "%~dp0"

where powershell >nul 2>&1
if errorlevel 1 (
    echo [backend] powershell was not found in PATH.
    exit /b 1
)

echo [backend] Preparing MySQL, Apache Tika, real Seed routing, and the Go API...
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start-local-adapter-provider.ps1" -Foreground
exit /b %ERRORLEVEL%
