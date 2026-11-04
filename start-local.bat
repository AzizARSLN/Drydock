@echo off
REM ---------------------------------------------------------------------------
REM  Drydock - local launcher (no Docker)
REM  Opens the backend and the frontend in two independent windows.
REM ---------------------------------------------------------------------------
title Drydock - Local
cd /d "%~dp0"

start "Drydock Backend" cmd /k "%~dp0start-backend.bat"
timeout /t 2 /nobreak >nul
start "Drydock Frontend" cmd /k "%~dp0start-frontend.bat"

echo.
echo  Backend  : http://127.0.0.1:8788
echo  Frontend : http://127.0.0.1:5173   ^(open this one^)
echo.
echo  Both services were started in separate windows.
echo.
pause
