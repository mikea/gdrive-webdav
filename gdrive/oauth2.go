package gdrive

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/user"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

var (
	tokenFileFlag = flag.String("token-file", "", "OAuth token cache file. ~/.gdrive_token by default.")
)

func newHTTPClient(ctx context.Context, clientID string, clientSecret string) *http.Client {
	config := &oauth2.Config{
		Scopes:      []string{"https://www.googleapis.com/auth/drive"},
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	tok, err := getTokenFromFile()
	if err != nil {
		tok = getTokenFromWeb(ctx, config)
		err = saveToken(tok)
		if err != nil {
			log.Errorf("An error occurred saving token file: %v\n", err)
		}
	}

	return config.Client(ctx, tok)
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
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("Error closing token file: %v", closeErr)
		}
	}()

	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	if err != nil {
		return nil, err
	}
	return t, err
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Panicf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(ctx, code)
	if err != nil {
		log.Panicf("Unable to retrieve token from web %v", err)
	}
	return tok
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
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			log.Printf("Error closing credential file: %v", closeErr)
		}
	}()
	return json.NewEncoder(f).Encode(token)
}
