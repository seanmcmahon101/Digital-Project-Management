@echo off
setlocal
rem Build a local Windows executable. The console remains open so the app can
rem be stopped safely with Ctrl+C and startup errors remain visible.
cd /d "%~dp0"

echo Running tests...
go test ./...
if errorlevel 1 (
    echo Tests failed - build aborted.
    exit /b 1
)

echo Building digital-project-management.exe...
go build -trimpath -o digital-project-management.exe .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

echo Done: digital-project-management.exe
endlocal
