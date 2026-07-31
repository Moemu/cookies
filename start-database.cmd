@echo off
setlocal
cd /d "%~dp0"

where powershell >nul 2>&1
if errorlevel 1 (
    echo [database] powershell was not found in PATH.
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start-local-database.ps1"
exit /b %ERRORLEVEL%
