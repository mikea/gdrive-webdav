FROM golang:latest

COPY . /go/src/github.com/mikea/gdrive-webdav/
CMD go get github.com/mikea/gdrive-webdav/main