package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/alirostami1/gdrive-webdav/include/gdrive"
	"github.com/samber/slog-http"
	"golang.org/x/net/webdav"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	addr         = flag.String("addr", "localhost:8765", "listen address")
	clientID     = flag.String("client-id", "", "OAuth client ID")
	clientSecret = flag.String("client-secret", "", "OAuth client secret")
	logLevel     = flag.String("log-level", "info", "log level (debug, info, warn, error)")
	authUser     = flag.String("user", "", "Basic-Auth username (empty = no auth)")
	authPass     = flag.String("pass", "", "Basic-Auth password")
)

var (
	driveFSOnce sync.Once // guarantees FS is built only once
	driveFS     webdav.FileSystem
	driveLS     = gdrive.NewLS() // lock system never changes
	oauthCfg    *oauth2.Config
)

func ParseLevel(s string) slog.Level {
	var level slog.Level
	var err = level.UnmarshalText([]byte(s))
	if err != nil {
		level = slog.LevelInfo
	}
	return level
}

func main() {
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: ParseLevel(*logLevel),
	}))

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

	mux := http.NewServeMux()

	mux.HandleFunc("/auth", authHandler) // starts the flow
	mux.HandleFunc("/oauth2callback", callbackHandler)
	mux.HandleFunc("/favicon.ico", notFoundHandler)
	mux.Handle("/", basicAuth(http.HandlerFunc(webdavOrRedirect))) // WebDAV after auth

	handler := sloghttp.Recovery(mux)
	handler = sloghttp.New(logger)(handler)

	slog.Info("started htpp server", slog.String("address", *addr))
	if err := http.ListenAndServe(*addr, handler); err != nil {
		slog.Error("HTTP server failed: ", slog.String("error", err.Error()))
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
		slog.Error("failed to save token to file", slog.String("error", err.Error()))
	}

	driveFSOnce.Do(func() {
		df, err := gdrive.NewFS(context.Background(), oauthCfg.Client(ctx, token))
		if err != nil {
			slog.Info("failed to create file system", slog.String("error", err.Error()))
		}
		driveFS = df
	})

	fmt.Fprintln(w, "Authorisation complete – you can now use WebDAV at the root URL (/).")
}

// decides whether we already have a FS or still need auth
func webdavOrRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if token, err := gdrive.LoadToken(); err == nil {
		driveFSOnce.Do(func() {
			// if a token is already cached from a previous run we can initialise immediately
			df, err := gdrive.NewFS(context.Background(), oauthCfg.Client(ctx, token))
			if err != nil {
				slog.Info("failed to create file system", slog.String("error", err.Error()))
				return
			}
			driveFS = df
		})
	}

	if driveFS == nil {
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	handler := &webdav.Handler{
		FileSystem: driveFS,
		LockSystem: driveLS,
		Logger: func(req *http.Request, err error) {
			slog.Error("response error", slog.String("error", err.Error()))
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
