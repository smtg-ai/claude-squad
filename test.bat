@echo off
setlocal

cd /d "%~dp0"

:: Find Go on PATH or in standard install locations
where go >nul 2>&1
if %errorlevel% equ 0 (
    set "GO=go"
) else if exist "C:\Program Files\Go\bin\go.exe" (
    set "GO=C:\Program Files\Go\bin\go.exe"
) else (
    echo Error: Go is not installed or not in PATH.
    echo Install from https://go.dev/dl/
    exit /b 1
)

echo Running tests...
"%GO%" test ./... -timeout 120s %*
if %errorlevel% neq 0 (
    echo Tests failed.
    exit /b 1
)

echo All tests passed.
