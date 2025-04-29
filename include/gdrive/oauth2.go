package gdrive

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"

	"golang.org/x/oauth2"
)

var (
	tokenFileFlag = flag.String("token-file", "", "OAuth token cache file. ~/.gdrive_token by default.")
)

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

func LoadToken() (*oauth2.Token, error) {
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

func SaveToken(token *oauth2.Token) error {
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
