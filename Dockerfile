# Build
FROM golang:1.24.2-alpine3.21 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /gdrive-webdav ./cmd/gdrive-webdav/main.go

# Run
FROM alpine:3.21
RUN apk update && apk add ca-certificates \
    && rm -rf /var/cache/apk/*

WORKDIR /root/
COPY --from=build /gdrive-webdav .

EXPOSE 8765
ENTRYPOINT ["./gdrive-webdav" ]
