Simple Google Drive => WebDAV bridge.

Usage
==

* Obtain OAuth keys and enable GDrive API (https://developers.google.com/drive/quickstart-go)
* Install go 
* Install go get tools hg and git and set their locations into PATH (https://code.google.com/p/go-wiki/wiki/GoGetTools)
* Install all packages (if any) that ./run.sh reports: `go get -u <package>`
* Run `./run.sh --client-id="<oauth-client-id>" --client-secret="<oauth-client-secret>"`

    
Status
==
Very, very experimental. Though it works.

* Linux Nautilus: Readable/Writable
* Linux davfs2: Some issues
* Mac Finder: Read-only
* Cyberduck: Appears to work (works also with Win8)
* Win8: Cannot connect to http://localhost:8765/ , using WIN8 network share builtin webdav support
  * Win8 MiniRedirector Client does not seem to send correct PROPFIND. Missing xml on request body 0 length.