set fallback

default: build

deps:
    mise install
    go get -v -t -d ./...

build:
    go build -v ./...

test:
    go test -v ./...

vet:
    go vet -v ./...

lint:
    golangci-lint run

check: build test vet lint

clean:
    go clean

docker-build:
    docker build -t mikea/gdrive-webdav .
