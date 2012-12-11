# gofmt -w src/**/*.go
GOARCH=amd64 GOPATH=`pwd` go run src/main/main.go $* 2>&1

