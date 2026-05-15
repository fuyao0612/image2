@echo off
setlocal

cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\restart-local.ps1"

echo.
echo If the page does not open automatically, visit: http://127.0.0.1:5173/
pause
