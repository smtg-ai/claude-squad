@echo off
setlocal

set "INSTALL_DIR=%LOCALAPPDATA%\bin"
set "SOURCE=%~dp0cs.exe"

:: Build if cs.exe doesn't exist
if not exist "%SOURCE%" (
    echo cs.exe not found, building...
    call "%~dp0build.bat"
    if %errorlevel% neq 0 exit /b 1
)

:: Create install directory
if not exist "%INSTALL_DIR%" (
    mkdir "%INSTALL_DIR%"
    echo Created %INSTALL_DIR%
)

:: Copy binary
copy /y "%SOURCE%" "%INSTALL_DIR%\cs.exe" >nul
if %errorlevel% neq 0 (
    echo Failed to copy cs.exe to %INSTALL_DIR%
    exit /b 1
)
echo Installed to %INSTALL_DIR%\cs.exe

:: Add to user PATH if not already there
echo %PATH% | findstr /i /c:"%INSTALL_DIR%" >nul
if %errorlevel% neq 0 (
    for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v Path 2^>nul') do set "USER_PATH=%%B"
    setx PATH "%USER_PATH%;%INSTALL_DIR%" >nul
    echo Added %INSTALL_DIR% to PATH. Restart your terminal to use "cs" globally.
) else (
    echo %INSTALL_DIR% is already in PATH.
)

echo Done. Run "cs" to start claude-squad.
