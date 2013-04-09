# gofmt -w src/**/*.go
set GOPATH=%CD%
go run src/main/main.go %* 2>&1


