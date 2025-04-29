package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/alirostami1/gdrive-webdav/include/gdrive"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/webdav"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	addr         = flag.String("addr", "localhost:8765", "listen address")
	clientID     = flag.String("client-id", "", "OAuth client ID")
	clientSecret = flag.String("client-secret", "", "OAuth client secret")
	debug        = flag.Bool("debug", false, "enable debug logging")
	trace        = flag.Bool("trace", false, "enable trace logging")
	authUser     = flag.String("user", "", "Basic-Auth username (empty = no auth)")
	authPass     = flag.String("pass", "", "Basic-Auth password")
)

var (
	driveFSOnce sync.Once // guarantees FS is built only once
	driveFS     webdav.FileSystem
	driveLS     = gdrive.NewLS() // lock system never changes
	oauthCfg    *oauth2.Config
)

func main() {
	flag.Parse()

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	if *trace {
		log.SetLevel(log.TraceLevel)
	} else if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if *clientID == "" || *clientSecret == "" {
		fmt.Fprintln(os.Stderr, "Both --client-id and --client-secret are required. See https://developers.google.com/drive/quickstart-go")
		os.Exit(1)
	}

	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback", *addr)
	oauthCfg = &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/drive"},
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
	}

	http.HandleFunc("/auth", authHandler) // starts the flow
	http.HandleFunc("/oauth2callback", callbackHandler)
	http.HandleFunc("/favicon.ico", notFoundHandler)
	http.Handle("/", basicAuth(http.HandlerFunc(webdavOrRedirect))) // WebDAV after auth

	server := &http.Server{
		Addr:              *addr,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Infof("Listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

// /auth → Google consent screen
func authHandler(w http.ResponseWriter, r *http.Request) {
	state := "drive-webdav" // static is fine for a local CLI tool; use random if you prefer
	url := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, url, http.StatusFound)
}

// /oauth2callback → exchange code → save token → build FS
func callbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if errStr := r.URL.Query().Get("error"); errStr != "" {
		http.Error(w, "OAuth error: "+errStr, http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing ?code parameter", http.StatusBadRequest)
		return
	}

	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := gdrive.SaveToken(token); err != nil {
		log.Errorf("saving token: %v", err)
	}

	driveFSOnce.Do(func() {
		driveFS = gdrive.NewFS(context.Background(), oauthCfg.Client(ctx, token))
	})

	fmt.Fprintln(w, "Authorisation complete – you can now use WebDAV at the root URL (/).")
}

// decides whether we already have a FS or still need auth
func webdavOrRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	driveFSOnce.Do(func() {
		// if a token is already cached from a previous run we can initialise immediately
		if token, err := gdrive.LoadToken(); err == nil {
			driveFS = gdrive.NewFS(context.Background(), oauthCfg.Client(ctx, token))
		}
	})

	if driveFS == nil {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	handler := &webdav.Handler{
		FileSystem: driveFS,
		LockSystem: driveLS,
		Logger: func(req *http.Request, err error) {
			log.Tracef("%+v", req)
			if log.IsLevelEnabled(log.TraceLevel) && err != nil {
				log.Errorf("response error %v", err)
			}
		},
	}
	handler.ServeHTTP(w, r)
}

func notFoundHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func basicAuth(next http.Handler) http.Handler {
	// if authUser is empty we just pass through
	if *authUser == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), []byte(*authUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(*authPass)) != 1 {

			w.Header().Set("WWW-Authenticate", `Basic realm="DriveDAV"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
