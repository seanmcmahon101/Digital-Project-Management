@echo off
rem Build DigitalisationPM.exe (windowed — no console; logs go to data\app.log)
cd /d "%~dp0"

echo Running tests...
go test ./...
if errorlevel 1 (
    echo Tests failed - build aborted.
    exit /b 1
)

echo Building DigitalisationPM.exe...
go build -trimpath -ldflags "-s -w -H windowsgui" -o DigitalisationPM.exe .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

echo Done: DigitalisationPM.exe
