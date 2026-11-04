@echo off
REM ---------------------------------------------------------------------------
REM  Drydock - backend (local, no Docker)
REM  Starts the Go control plane on http://127.0.0.1:8788
REM ---------------------------------------------------------------------------
setlocal enabledelayedexpansion
title Drydock - Backend
cd /d "%~dp0backend"

where go >nul 2>&1
if errorlevel 1 goto nogo

if not exist ".env" call :makeenv

echo.
echo  Drydock backend  -  http://127.0.0.1:8788
echo  Stop with Ctrl+C
echo.
go run .
echo.
echo [backend stopped]
pause
exit /b 0

:nogo
echo [ERROR] Go was not found on PATH. Install it from https://go.dev/dl
pause
exit /b 1

:makeenv
echo [INFO] backend\.env not found - generating one with random secrets...
for /f "usebackq delims=" %%i in (`powershell -NoProfile -Command "[guid]::NewGuid().ToString('N')+[guid]::NewGuid().ToString('N')"`) do set "MK=%%i"
for /f "usebackq delims=" %%i in (`powershell -NoProfile -Command "[guid]::NewGuid().ToString('N')+[guid]::NewGuid().ToString('N')"`) do set "JS=%%i"
(
  echo DRYDOCK_MASTER_KEY=!MK!
  echo DRYDOCK_JWT_SECRET=!JS!
  echo DRYDOCK_ADDR=127.0.0.1:8788
  echo DRYDOCK_DATA_DIR=./data
  echo DRYDOCK_DB=./data/drydock.db
  echo DRYDOCK_WEB_DIR=./web
  echo DRYDOCK_HEALTH_INTERVAL=60
) > ".env"
exit /b 0
