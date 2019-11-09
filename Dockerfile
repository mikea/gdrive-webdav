FROM golang:latest
MAINTAINER mike.aizatsky@gmail.com

COPY . /go/src/github.com/mikea/gdrive-webdav/

RUN go get -v github.com/mikea/gdrive-webdav

EXPOSE 8765

ENTRYPOINT ["gdrive-webdav" ]