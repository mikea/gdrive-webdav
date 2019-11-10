# Build
FROM golang:latest
COPY . /go/src/github.com/mikea/gdrive-webdav/
RUN go get -v github.com/mikea/gdrive-webdav


# Run
FROM debian:stable-slim  
RUN apt update && apt install -y ca-certificates

WORKDIR /root/
COPY --from=0 /go/bin/gdrive-webdav .

EXPOSE 8765
ENTRYPOINT ["./gdrive-webdav" ]
