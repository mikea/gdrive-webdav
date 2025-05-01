# gdrive-webdav

Simple Google Drive => WebDAV bridge.

## Usage

* Build docker image: `docker build -t gdrive-webdav .`
* Create a project and enable "Google Drive API" (https://developers.google.com/workspace/guides/create-project)
* Configure OAuth consent screen (https://developers.google.com/workspace/guides/configure-oauth-consent), add ".../auth/drive" scope, add yourself to "Test Users".
* Obtain OAuth client ID credentials for *Desktop* Application (https://developers.google.com/workspace/guides/create-credentials#oauth-client-id)
* Run using docker:

  ```bash
  touch .gdrive_token
  docker run -ti --rm -p 8765:8765 -v $(pwd)/.gdrive_token:/root/.gdrive_token gdrive-webdav --client-id=<client_id> --client-secret=<client_secret>
  ```

* Connect to WebDAV network drive using http://localhost:8765/

Supported flags:

* `--host` service host address, `localhost` by default
* `--port` service port address, `8765` by default
* `--log-level` log level, one of `info`, `warn`, `error`, and `debug`, `info` by default
* `--user` username for webdav server, empty means no authentication
* `--pass` password for webdav server

## Status

Alpha quality. I trust it my files.

* Linux Nautilus: Readable/Writable
* Linux davfs2: Some issues
* Mac Finder: Read-only
* Cyberduck: Appears to work (works also with Win8)
* Win8: Cannot connect to http://localhost:8765/ , using WIN8 network share builtin webdav support
  * Win8 MiniRedirector Client does not seem to send correct PROPFIND. Missing xml on request body 0 length.

Litmus test results as of Aug 2022:

```
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
 6. propset............... pass
 7. propget............... pass
 8. propextended.......... pass
 9. propmove.............. pass
10. propget............... pass
11. propdeletes........... pass
12. propget............... pass
13. propreplace........... pass
14. propget............... pass
15. propnullns............ pass
16. propget............... pass
17. prophighunicode....... pass
18. propget............... pass
19. propremoveset......... pass
20. propget............... pass
21. propsetremove......... pass
22. propget............... pass
23. propvalnspace......... pass
24. propwformed........... pass
25. propinit.............. pass
26. propmanyns............ pass
27. propget............... pass
28. propcleanup........... pass
29. finish................ pass
<- summary for `props': of 30 tests run: 29 passed, 1 failed. 96.7%
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
