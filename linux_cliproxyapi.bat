@echo off
chcp 65001 >nul

echo ========================
echo 开始编译 Linux amd64
echo ========================

echo 检测运行环境...

echo %ComSpec%

if /i "%ComSpec%"=="%SystemRoot%\System32\cmd.exe" (
    echo 当前是 CMD
) else (
    echo 当前不是标准 CMD
)


cd /d D:\root\software\ai\CLIProxyAPI

echo 当前目录:
echo %cd%

echo 开始编译exe！
go build -o cli-proxy-api.exe cmd\server\main.go
echo 编译exe成功！

echo 开始编译linux bin！
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0

go build -ldflags="-s -w" -o linux_cliproxyapi cmd\server\main.go

if %errorlevel%==0 (
    echo 编译成功！
) else (
    echo 编译失败！
)

pause