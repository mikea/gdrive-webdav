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

[Litmus](http://webdav.org/neon/litmus/) test results as of Jul 2022:

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
 8. copy_shallow.......... pass
 9. move.................. pass
10. move_coll............. pass
11. move_cleanup.......... pass
12. finish................ pass
<- summary for `copymove': of 13 tests run: 13 passed, 0 failed. 100.0%
-> 1 warning was issued.
-> running `props':
 0. init.................. pass
 1. begin................. pass
 2. propfind_invalid...... pass
 3. propfind_invalid2..... FAIL (PROPFIND with invalid namespace declaration in body (see FAQ) got 207 response not 400)
 4. propfind_d0........... pass
 5. propinit.............. pass
 6. propset............... FAIL (PROPPATCH on `/litmus/prop': Could not read status line: connection was closed by server)
 7. propget............... SKIPPED
 8. propextended.......... pass
 9. propmove.............. SKIPPED
10. propget............... SKIPPED
11. propdeletes........... SKIPPED
12. propget............... SKIPPED
13. propreplace........... SKIPPED
14. propget............... SKIPPED
15. propnullns............ SKIPPED
16. propget............... SKIPPED
17. prophighunicode....... SKIPPED
18. propget............... SKIPPED
19. propremoveset......... SKIPPED
20. propget............... SKIPPED
21. propsetremove......... SKIPPED
22. propget............... SKIPPED
23. propvalnspace......... SKIPPED
24. propwformed........... pass
25. propinit.............. pass
26. propmanyns............ FAIL (PROPPATCH on `/litmus/prop': Could not read status line: connection was closed by server)
27. propget............... FAIL (No value given for property {http://example.com/kappa}somename)
28. propcleanup........... pass
29. finish................ pass
-> 16 tests were skipped.
<- summary for `props': of 14 tests run: 10 passed, 4 failed. 71.4%
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
