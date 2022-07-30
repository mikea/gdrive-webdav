# gdrive-webdav

![Go Workflow](https://github.com/mikea/gdrive-webdav/workflows/Go/badge.svg)

Simple Google Drive => WebDAV bridge.

## Usage

* Build docker image: `docker build -t gdrive-webdav .`
* Create a project and enable "Drive API" (https://developers.google.com/workspace/guides/create-project)
* Obtain OAuth client ID credentials for *Desktop* Application (https://developers.google.com/workspace/guides/create-credentials#oauth-client-id)
* Run using docker:

  ```bash
  touch .gdrive_token
  docker run -ti --rm -p 8765:8765 -v $(pwd)/.gdrive_token:/root/.gdrive_token gdrive-webdav --client-id=<client_id> --client-secret=<client_secret>
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
golangci-lint run
```

## Testing

You can use litmus tests to test the implementation:

```bash
docker build -t litmus litmus && docker run -ti litmus http://localhost:1234/
```
