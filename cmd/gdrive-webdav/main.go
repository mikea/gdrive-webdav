package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/alirostami1/gdrive-webdav/include/gdrive"
	sloghttp "github.com/samber/slog-http"
	"golang.org/x/net/webdav"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	// shared across handlers
	rootCtx    context.Context
	rootCancel context.CancelFunc

	driveFSOnce sync.Once // guarantees FS is built only once
	driveFS     webdav.FileSystem
	driveLS     = gdrive.NewLS() // lock system never changes
	oauthCfg    *oauth2.Config
)

// simple helpers to pull from ENV with a fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func getEnvAsInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

var (
	host = flag.String(
		"host",
		getEnv("GWD_HOST", "localhost"),
		"host address (env: GWD_HOST)",
	)
	port = flag.Int(
		"addr",
		getEnvAsInt("GWD_PORT", 8765),
		"port (env: GWD_PORT)",
	)
	clientID = flag.String(
		"client-id",
		getEnv("GWD_CLIENT_ID", ""),
		"OAuth client ID (env: GWD_CLIENT_ID)",
	)
	clientSecret = flag.String(
		"client-secret",
		getEnv("GWD_CLIENT_SECRET", ""),
		"OAuth client secret (env: GWD_CLIENT_SECRET)",
	)
	logLevel = flag.String(
		"log-level",
		getEnv("GWD_LOG_LEVEL", "info"),
		"log level (debug, info, warn, error) (env: GWD_LOG_LEVEL)",
	)
	authUser = flag.String(
		"user",
		getEnv("GWD_USER", ""),
		"Basic-Auth username (empty = no auth) (env: GWD_USER)",
	)
	authPass = flag.String(
		"pass",
		getEnv("GWD_PASS", ""),
		"Basic-Auth password (env: GWD_PASS)",
	)
)

func ParseLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		slog.Error("unknown log level, falling back to info",
			slog.String("chosen-level", s),
		)
		return slog.LevelInfo
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

	// root context → cancelled on SIGINT/SIGTERM
	rootCtx, rootCancel = context.WithCancel(context.Background())
	defer rootCancel()

	// capture Ctrl+C / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	redirectURL := fmt.Sprintf("http://%s%d/oauth2callback", *host, *port)
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

	addr := net.JoinHostPort(*host, fmt.Sprint(*port))

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-quit
		slog.Info("shutdown signal received, closing server")
		// cancel any background operations
		rootCancel()

		// give outstanding requests up to 10s to finish
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("HTTP server shutdown with error", slog.String("error", err.Error()))
		}
	}()

	slog.Info("started htpp server", slog.String("address", fmt.Sprintf("http://%s", addr)))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("HTTP server shutdown with error", slog.String("error", err.Error()))
	}
	slog.Info("server exited cleanly")
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
		df, err := gdrive.NewFS(rootCtx, oauthCfg.Client(ctx, token))
		if err != nil {
			slog.Info("failed to create file system", slog.String("error", err.Error()))
		}
		driveFS = df
	})

	fmt.Fprintln(w, "Authorisation complete – you can now use WebDAV at the root URL (/).")
}

// decides whether we already have a FS or still need auth
func webdavOrRedirect(w http.ResponseWriter, r *http.Request) {
	if token, err := gdrive.LoadToken(); err == nil {
		// if a token is already cached from a previous run we can initialise immediately
		driveFSOnce.Do(func() {
			df, err := gdrive.NewFS(rootCtx, oauthCfg.Client(rootCtx, token))
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
			if err != nil {
				slog.Error("error happened in webdav handler", slog.String("error", err.Error()))
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
