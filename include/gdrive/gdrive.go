package gdrive

import (
	"net/http"
	"strings"
	"time"

	gocache "github.com/pmylund/go-cache"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
)

type fileAndPath struct {
	file *drive.File
	path string
}

// NewFS creates new gdrive file system.
func NewFS(ctx context.Context, httpClient *http.Client) (webdav.FileSystem, error) {
	client, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	fs := &fileSystem{
		client: client,
		cache:  gocache.New(5*time.Minute, 30*time.Second),
	}
	return fs, nil
}

// NewLS creates new GDrive locking system
func NewLS() webdav.LockSystem {
	return webdav.NewMemLS()
}

func getModTime(file *drive.File) (time.Time, error) {
	modifiedTime := file.ModifiedTime
	if modifiedTime == "" {
		modifiedTime = file.CreatedTime
	}
	if modifiedTime == "" {
		return time.Time{}, nil
	}

	modTime, err := time.Parse(time.RFC3339, modifiedTime)
	if err != nil {
		return time.Time{}, err
	}

	return modTime, nil
}

func ignoreFile(f *drive.File) bool {
	return f.Trashed
}

func normalizePath(p string) string {
	return strings.TrimRight(p, "/")
}
