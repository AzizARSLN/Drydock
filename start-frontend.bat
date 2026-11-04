@echo off
REM ---------------------------------------------------------------------------
REM  Drydock - frontend (local, no Docker)
REM  Starts the Vite dev server on http://127.0.0.1:5173 (proxies /api to 8788)
REM ---------------------------------------------------------------------------
setlocal
title Drydock - Frontend
cd /d "%~dp0frontend"

where node >nul 2>&1
if errorlevel 1 goto nonode

if not exist "node_modules" (
  echo [INFO] Installing dependencies with npm install...
  call npm install
  if errorlevel 1 goto npmfail
)

echo.
echo  Drydock frontend  -  http://127.0.0.1:5173
echo  Stop with Ctrl+C
echo.
call npm run dev
echo.
echo [frontend stopped]
pause
exit /b 0

:nonode
echo [ERROR] Node.js was not found on PATH. Install it from https://nodejs.org
pause
exit /b 1

:npmfail
echo [ERROR] npm install failed.
pause
exit /b 1
