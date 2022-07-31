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

[Litmus](http://webdav.org/neon/litmus/) test results as of Jul 30 2022:

```text
-> running `basic':
 0. init.................. pass
 1. begin................. pass
 2. options............... pass
 3. put_get............... pass
 4. put_get_utf8_segment.. pass
 5. put_no_parent......... pass
 6. mkcol_over_plain...... pass
 7. delete................ pass
 8. delete_null........... pass
 9. delete_fragment....... pass
10. mkcol................. pass
11. mkcol_again........... pass
12. delete_coll........... pass
13. mkcol_no_parent....... pass
14. mkcol_with_body....... pass
15. finish................ pass
<- summary for `basic': of 16 tests run: 16 passed, 0 failed. 100.0%
-> running `copymove':
 0. init.................. pass
 1. begin................. pass
 2. copy_init............. pass
 3. copy_simple........... FAIL (simple resource COPY:
Could not read status line: connection was closed by server)
 4. copy_overwrite........ FAIL (COPY-on-existing with 'Overwrite: F' MUST fail with 412 (RFC4918:10.6):
Could not read status line: connection was closed by server)
 5. copy_nodestcoll....... WARNING: COPY to non-existant collection '/litmus/nonesuch' gave 'Could not read status line: connection was closed by server' not 409 (RFC2518:S8.8.5)
    ...................... pass (with 1 warning)
 6. copy_cleanup.......... pass
 7. copy_coll............. FAIL (collection COPY `/litmus/ccsrc/' to `/litmus/ccdest/': Could not read status line: connection was closed by server)
 8. copy_shallow.......... FAIL (MKCOL on `/litmus/ccsrc/': 405 Method Not Allowed)
 9. move.................. FAIL (MOVE `/litmus/move' to `/litmus/movedest': Could not read status line: connection was closed by server)
10. move_coll............. FAIL (collection COPY `/litmus/mvsrc/' to `/litmus/mvdest2/', depth infinity: Could not read status line: connection was closed by server)
11. move_cleanup.......... pass
12. finish................ pass
<- summary for `copymove': of 13 tests run: 7 passed, 6 failed. 53.8%
-> 1 warning was issued.
```


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
docker build -t litmus litmus && docker run -ti --network=host litmus http://localhost:8765/
```

Running single group of tests:

```bash
docker run -ti --network=host --entrypoint=/usr/local/libexec/litmus/copymove litmus http://localhost:8765/
```

Evailable tests: basic, copymove, http, locks, props.
