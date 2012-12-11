Simple Google Drive => WebDAV bridge.

Usage
==

* Obtain OAuth keys and enable GDrive API (https://developers.google.com/drive/quickstart-go)
* Install go 
* Run `./run.sh --client-id="<oauth-client-id>" --client-secret="<oauth-client-secret>"`
* Install all packages (if any) that ./run.sh reports: `go get -u <package>`
    
Status
==
Very, very experimental. Though it works.

* Linux Nautilus: Readable/Writable
* Linux davfs2: Some issues
* Mac Finder: Read-only
* Cyberduck: Appears to work 