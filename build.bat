@echo off
echo "building Linux AMD64"
set GOARCH=amd64
set GOOS=linux
go build -ldflags="-w -s"
echo "Building Windows AMD64"
set GOOS=windows
go build -ldflags="-w -s"
echo "upx compressing"
upx -9 filehider
upx -9 filehider.exe
pause