#!/bin/bash
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-w -s"
set GOOS=windows
go build -ldflags="-w -s"
upx -9 filehider
upx -9 filehider.exe