FROM golang:latest
MAINTAINER mike.aizatsky@gmail.com

# Following is not necessary, but helps to speed up rebuilds.
RUN go get github.com/golang/lint/golint \
           github.com/cihub/seelog \
           github.com/pmylund/go-cache \
           golang.org/x/oauth2 \
           google.golang.org/api/drive/v3

COPY . /go/src/github.com/mikea/gdrive-webdav/


RUN go get -v github.com/mikea/gdrive-webdav/main
RUN golint -set_exit_status github.com/mikea/gdrive-webdav/...

CMD go run github.com/mikea/gdrive-webdav/main