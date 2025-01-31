package gdrive

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"sync"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	tokenFileFlag = flag.String("token-file", "", "OAuth token cache file. ~/.gdrive_token by default.")
	config *oauth2.Config
)

func newHTTPClient(ctx context.Context, clientID string, clientSecret string) *http.Client {
	fmt.Printf("start getting the token\n")
	config = &oauth2.Config{
		RedirectURL:  "http://localhost:8765/callback",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{
							   "https://www.googleapis.com/auth/drive",
							   },
		Endpoint:     google.Endpoint,
	}

	tok, err := getTokenFromFile()
	if err != nil {
		fmt.Printf("fail to get the token from file, start getting it from the web\n")
		tok = getTokenFromWeb(ctx, config)
		fmt.Printf("complete getting the token\n")
		err = saveToken(tok)
		if err != nil {
			log.Errorf("An error occurred saving token file: %v\n", err)
		}
	}

	return config.Client(ctx, tok)
}

func handleMain(w http.ResponseWriter, r *http.Request) {
	url := config.AuthCodeURL("state")
	fmt.Printf("going to the following url: %s\n", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleCallback(w http.ResponseWriter, r *http.Request, tokenCh chan *oauth2.Token, ctx context.Context) {
	r.ParseForm()
	state := r.Form.Get("state")
	if state != "state" {
		http.Error(w, "State invalid", http.StatusBadRequest)
		return
	}
	code := r.Form.Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}
	
	token, err := config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tokenCh <- token
}

func tokenFile() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if *tokenFileFlag != "" {
		return *tokenFileFlag, nil
	}

	return u.HomeDir + "/.gdrive_token", nil
}

func getTokenFromFile() (*oauth2.Token, error) {
	tokenFile, err := tokenFile()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	if err != nil {
		return nil, err
	}
	return t, err
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) *oauth2.Token{
	tokenCh := make(chan *oauth2.Token)
	srv := &http.Server{Addr: ":8765"}
	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)
	go func(){
		defer httpServerExitDone.Done()
		http.HandleFunc("/auth", handleMain)
		http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			handleCallback(w, r, tokenCh, ctx)
		})
		srv.ListenAndServe()
	}()
	token := <-tokenCh

	if err := srv.Shutdown(ctx); err != nil {
        panic(err) // failure/timeout shutting down the server gracefully
    }

    // wait for goroutine started in the goroutine to stop
    httpServerExitDone.Wait()
	return token
}

func saveToken(token *oauth2.Token) error {
	file, err := tokenFile()
	if err != nil {
		return err
	}
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
