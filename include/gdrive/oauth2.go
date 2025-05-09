package gdrive

import (
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/user"

	"golang.org/x/oauth2"
)

// simple helpers to pull from ENV with a fallback
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var (
	tokenFileFlag = flag.String("token-file",
		getEnv("GWD_TOKEN_FILE", ""),
		"path to token file. ~/.gdrive_token by default.")
)

func tokenFilePath() (string, error) {
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
	tokenFile, err := tokenFilePath()
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

func SaveToken(token *oauth2.Token, logger *slog.Logger) error {
	filePath, err := tokenFilePath()
	if err != nil {
		return err
	}
	logger.Info("saving credential file", slog.String("file_path", filePath))
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
