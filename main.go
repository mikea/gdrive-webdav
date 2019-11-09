package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/mikea/gdrive-webdav/gdrive"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
)

var (
	addr         = flag.String("addr", ":8765", "WebDAV service address")
	clientID     = flag.String("client-id", "", "OAuth client id")
	clientSecret = flag.String("client-secret", "", "OAuth client secret")
)

func main() {
	flag.Parse()

	if *clientID == "" {
		fmt.Fprintln(os.Stderr, "--client-id is not specified. See https://developers.google.com/drive/quickstart-go for step-by-step guide.")
		os.Exit(-1)
	}

	if *clientSecret == "" {
		fmt.Fprintln(os.Stderr, "--client-secret is not specified. See https://developers.google.com/drive/quickstart-go for step-by-step guide.")
		os.Exit(-1)
	}

	handler := &webdav.Handler{
		FileSystem: gdrive.NewFS(context.Background(), *clientID, *clientSecret),
		LockSystem: gdrive.NewLS(),
	}

	http.HandleFunc("/debug/gc", gcHandler)
	http.HandleFunc("/favicon.ico", notFoundHandler)
	http.HandleFunc("/", handler.ServeHTTP)

	log.Info("Listening on: ", *addr)

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Errorf("Error starting HTTP server: %v", err)
		os.Exit(-1)
	}
}

func gcHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("GC")
	runtime.GC()
	w.WriteHeader(200)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
}
