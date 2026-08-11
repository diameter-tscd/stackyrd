@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "DIST_DIR=%SCRIPT_DIR%dist"

for /f "tokens=2 delims==" %%I in ('wmic os get OSArchitecture /value 2^>nul ^| find "="') do set "ARCH=%%I"
if "%ARCH%"=="" (
  set "ARCH=%PROCESSOR_ARCHITECTURE%"
)
if /i "%ARCH%"=="AMD64" set "ARCH=amd64"
if /i "%ARCH%"=="ARM64" set "ARCH=arm64"
if /i "%ARCH%"=="x86" set "ARCH=386"

set "BIN=%DIST_DIR%\yrd_windows_%ARCH%.exe"

if not exist "%BIN%" (
  echo Missing binary: %BIN%>&2
  echo Run "%SCRIPT_DIR%build-all.sh" to build.>&2
  exit /b 1
)

"%BIN%" %*
