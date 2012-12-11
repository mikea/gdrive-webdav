package main

import (
	"flag"
	"fmt"
	"gdrive"
	log "github.com/cihub/seelog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"webdav"
)

var (
	addr         = flag.String("addr", ":8765", "WebDAV service address")
	clientId     = flag.String("client-id", "", "OAuth client id")
	clientSecret = flag.String("client-secret", "", "OAuth client secret")
)

func main() {
	defer log.Flush()
	stdFormat()
	flag.Parse()

	if *clientId == "" {
		fmt.Fprintln(os.Stderr, "--client-id is not specified. See https://developers.google.com/drive/quickstart-go for step-by-step guide.")
		return
	}

	if *clientSecret == "" {
		fmt.Fprintln(os.Stderr, "--client-secret is not specified. See https://developers.google.com/drive/quickstart-go for step-by-step guide.")
		return
	}

	fs := gdrive.NewGDriveFileSystem(*clientId, *clientSecret)

	http.HandleFunc("/debug/gc", gcHandler)
	http.HandleFunc("/favicon.ico", notFoundHandler)
	http.HandleFunc("/", webdav.NewHandler(fs))

	fmt.Printf("Listening on %v\n", *addr)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Errorf("Error starting WebDAV server: %v", err)
	}
}

func stdFormat() {
	testConfig := `
<seelog type="sync">
	<outputs formatid="main">
		<console/>
	</outputs>
	<formats>
		<format id="main" format=" %Date %Time - [%LEVEL] - %Msg - (%Func %File)%n"/>
	</formats>
</seelog>`

	logger, _ := log.LoggerFromConfigAsBytes([]byte(testConfig))
	log.ReplaceLogger(logger)
}

func gcHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("GC")
	runtime.GC()
	w.WriteHeader(200)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
}
