# gdrive-webdav

![](https://github.com/mikea/gdrive-webdav/workflows/.github/workflows/go.yml/badge.svg)

Simple Google Drive => WebDAV bridge.

## Building From Source

    go build -i ./main.go

## Usage

* Obtain OAuth keys and enable GDrive API (https://developers.google.com/drive/v3/web/quickstart/go)

## Status

Alpha quality. I trust it my files.

* Linux Nautilus: Readable/Writable
* Linux davfs2: Some issues
* Mac Finder: Read-only
* Cyberduck: Appears to work (works also with Win8)
* Win8: Cannot connect to http://localhost:8765/ , using WIN8 network share builtin webdav support
  * Win8 MiniRedirector Client does not seem to send correct PROPFIND. Missing xml on request body 0 length.