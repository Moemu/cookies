@echo off
setlocal
cd /d "%~dp0"

where npm >nul 2>&1
if errorlevel 1 (
    echo [frontend] npm was not found in PATH.
    exit /b 1
)

if not exist "node_modules" (
    echo [frontend] Installing Kanon dependencies for the first run...
    call npm ci
    if errorlevel 1 exit /b 1
)

echo [frontend] Starting Kanon frontend...
echo [frontend] Press Ctrl+C to stop.
call npm run dev
exit /b %ERRORLEVEL%
