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

Supported flags:

* `--addr` service address, `:8765` by default
* `--debug`, `--trace` print more information

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
 3. copy_simple........... pass
 4. copy_overwrite........ pass
 5. copy_nodestcoll....... WARNING: COPY to non-existant collection '/litmus/nonesuch' gave '500 Internal Server Error' not 409 (RFC2518:S8.8.5)
    ...................... pass (with 1 warning)
 6. copy_cleanup.......... pass
 7. copy_coll............. pass
 8. copy_shallow.......... WARNING: Could not clean up cdest
    ...................... pass (with 1 warning)
 9. move.................. FAIL (MOVE `/litmus/move' to `/litmus/movedest': Could not read status line: connection was closed by server)
10. move_coll............. FAIL (collection MOVE `/litmus/mvsrc/' to `/litmus/mvdest/': Could not read status line: connection was closed by server)
11. move_cleanup.......... pass
12. finish................ pass
<- summary for `copymove': of 13 tests run: 11 passed, 2 failed. 84.6%
-> 2 warnings were issued.
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

To get test log add `-v $(pwd)/debug.log:/usr/local/share/litmus/debug.log`.
