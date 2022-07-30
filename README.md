# gdrive-webdav

![Go Workflow](https://github.com/mikea/gdrive-webdav/workflows/Go/badge.svg)

Simple Google Drive => WebDAV bridge.

## Usage

* Build docker image: `docker build -t gdrive-webdav .`
* Obtain OAuth keys and enable GDrive API (https://developers.google.com/drive/v3/web/quickstart/go)
* Run using docker:

  ```bash
  docker run -ti --rm -p 8765:8765 gdrive-webdav --client-id=<client_id> --client-secret=<client_secret>
  ```

* Connect to WebDAV network drive using http://localhost:8765/

## Status

Alpha quality. I trust it my files.

* Linux Nautilus: Readable/Writable
* Linux davfs2: Some issues
* Mac Finder: Read-only
* Cyberduck: Appears to work (works also with Win8)
* Win8: Cannot connect to http://localhost:8765/ , using WIN8 network share builtin webdav support
  * Win8 MiniRedirector Client does not seem to send correct PROPFIND. Missing xml on request body 0 length.

## Development

Use nix to set up development environment:

```bash
nix-shell
go test ./...
go build
```
