SHELL := /bin/bash
BUILD_DATE = `date -u +"%a,|%d|%b|%Y|%T|%Z" | tr '|' ' '`
BUILD_VERSION = `cat VERSION`

default: mac

mac:
	GOARCH=amd64 GOOS=darwin go build -o bin/osx/xcp  -ldflags "-X \"main.binaryBuild=$(BUILD_DATE)\" -X main.binaryVersion=$(BUILD_VERSION)"

win:
	GOARCH=amd64 GOOS=windows go build -o bin/win/xcp.exe -ldflags "-X \"main.binaryBuild=$(BUILD_DATE)\" -X \"main.binaryVersion=$(BUILD_VERSION)\""

lin:
	GOARCH=amd64 GOOS=linux go build -o bin/lin/xcp -ldflags "-X \"main.binaryBuild=$(BUILD_DATE)\" -X \"main.binaryVersion=$(BUILD_VERSION)\""

release:
	tar -czvf xcp-${BUILD_VERSION}-lin.tar.gz -C bin/lin/ xcp
	tar -czvf xcp-${BUILD_VERSION}-macos.tar.gz -C bin/osx xcp
	tar -czvf xcp-${BUILD_VERSION}-win.tar.gz -C bin/win xcp.exe
	mkdir -p releases/${BUILD_VERSION} && mv *.tar.gz releases/${BUILD_VERSION}/.

all: mac win lin
