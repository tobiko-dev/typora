@echo off
setlocal
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

python make_icon.py assets\Typora-logo.png Typora.ico
if errorlevel 1 exit /b 1

go build -trimpath -ldflags="-s -w -H windowsgui" -o Typora-v4.20-unbranded.exe .
if errorlevel 1 exit /b 1

python embed_icon.py Typora-v4.20-unbranded.exe Typora.ico Typora-v4.20.exe
if errorlevel 1 exit /b 1

echo Built Typora-v4.20.exe
