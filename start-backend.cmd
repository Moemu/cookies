@echo off
setlocal
cd /d "%~dp0"

where powershell >nul 2>&1
if errorlevel 1 (
    echo [backend] powershell was not found in PATH.
    exit /b 1
)

echo [backend] Building and starting the Go API...
echo [backend] The API will listen on http://127.0.0.1:8080
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start-local-adapter-provider.ps1"
exit /b %ERRORLEVEL%
