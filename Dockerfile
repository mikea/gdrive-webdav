# Build
FROM golang:1.24.2-alpine3.21 AS build
COPY . app/
RUN cd app/ && go install cmd/cli

# Run
# FROM debian:stable-slim  
FROM alpine:3.21
# RUN apt update && apt install -y ca-certificates
RUN apk update && apk add ca-certificates \
    && rm -rf /var/cache/apk/*

WORKDIR /root/
# COPY --from=0 /go/bin/gdrive-webdav .
COPY --from=build /go/bin/gdrive-webdav .

EXPOSE 8765
ENTRYPOINT ["./gdrive-webdav" ]
