package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	log "github.com/cihub/seelog"
	"github.com/mikea/gdrive-webdav/gdrive"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
)

var (
	addr         = flag.String("addr", ":8765", "WebDAV service address")
	clientID     = flag.String("client-id", "", "OAuth client id")
	clientSecret = flag.String("client-secret", "", "OAuth client secret")
)

func main() {
	defer log.Flush()
	stdFormat()
	flag.Parse()

	if *clientID == "" {
		fmt.Fprintln(os.Stderr, "--client-id is not specified. See https://developers.google.com/drive/quickstart-go for step-by-step guide.")
		return
	}

	if *clientSecret == "" {
		fmt.Fprintln(os.Stderr, "--client-secret is not specified. See https://developers.google.com/drive/quickstart-go for step-by-step guide.")
		return
	}

	handler := &webdav.Handler{
		FileSystem: gdrive.NewFS(context.Background(), *clientID, *clientSecret),
		LockSystem: gdrive.NewLS(),
	}

	http.HandleFunc("/debug/gc", gcHandler)
	http.HandleFunc("/favicon.ico", notFoundHandler)
	http.HandleFunc("/", handler.ServeHTTP)

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
