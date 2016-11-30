package gdrive

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/mikea/gdrive-webdav/webdav"
	gocache "github.com/pmylund/go-cache"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
)

// FileSystem is an instance of GDrive file system.
type FileSystem struct {
	client       *drive.Service
	roundTripper http.RoundTripper
	cache        *gocache.Cache
}

var (
	tokenFileFlag = flag.String("token-file", "", "OAuth token cache file. ~/.gdrive_token by default.")
)

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
	cacheKeyAbout  = "global:about"
	cacheKeyFile   = "file:"
)

type fileAndPath struct {
	file *drive.File
	path string
}

// NewFileSystem creates new gdrive file system.
func NewFileSystem(ctx context.Context, clientID string, clientSecret string) *FileSystem {
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
		tok = getTokenFromWeb(config)
		err = saveToken(tok)
		if err != nil {
			log.Errorf("An error occurred saving token file: %v\n", err)
		}
	}

	httpClient := config.Client(ctx, nil)
	client, err := drive.New(httpClient)
	if err != nil {
		log.Errorf("An error occurred creating Drive client: %v\n", err)
		panic(-3)
	}

	fs := &FileSystem{
		client:       client,
		roundTripper: httpClient.Transport,
		cache:        gocache.New(5*time.Minute, 30*time.Second),
	}
	return fs
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

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Criticalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Criticalf("Unable to retrieve token from web %v", err)
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
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// MkDir creates a directory
func (fs *FileSystem) MkDir(p string) webdav.MkColStatusCode {
	pID := fs.getFileID(p, false)
	if pID != "" {
		log.Errorf("dir already exists: %v", pID)
		return webdav.MkColMethodNotAllowed
	}

	p = strings.TrimRight(p, "/")
	parent := path.Dir(p)
	dir := path.Base(p)

	parentID := fs.getFileID(parent, true)

	if parentID == "" {
		log.Errorf("parent not found")
		return webdav.MkColConflict
	}

	f := &drive.File{
		MimeType: mimeTypeFolder,
		Name:     dir,
		Parents:  []string{parentID},
	}

	_, err := fs.client.Files.Create(f).Do()

	if err != nil {
		return webdav.MkColUnknownError
	}

	fs.invalidatePath(p)
	fs.invalidatePath(parent)
	return webdav.MkColCreated
}

// Delete deletes the file
func (fs *FileSystem) Delete(p string) webdav.DeleteStatusCode {
	pID := fs.getFileID(p, false)
	if pID == "" {
		return webdav.DeleteNotFound
	}

	err := fs.client.Files.Delete(pID).Do()
	if err != nil {
		log.Errorf("can't delete file %v", err)
		return webdav.DeleteUnknownError
	}

	fs.invalidatePath(p)
	fs.invalidatePath(path.Dir(p))
	return webdav.DeleteDeleted
}

// Put uploads the file.
func (fs *FileSystem) Put(p string, bytes io.ReadCloser) webdav.StatusCode {
	defer bytes.Close()
	parent := path.Dir(p)
	base := path.Base(p)

	parentID := fs.getFileID(parent, true)

	if parentID == "" {
		log.Errorf("ERROR: Parent not found")
		return webdav.StatusCode(http.StatusConflict) // 409
	}

	f := &drive.File{
		Name:    base,
		Parents: []string{parentID},
	}

	_, err := fs.client.Files.Create(f).Media(bytes).Do()
	if err != nil {
		log.Errorf("can't put: %v", err)
		return webdav.StatusCode(500)
	}

	fs.invalidatePath(p)
	fs.invalidatePath(parent)
	return webdav.StatusCode(201)
}

// Get downloads the file.
func (fs *FileSystem) Get(p string) (webdav.StatusCode, io.ReadCloser, int64) {
	pFile := fs.getFile(p, false)
	if pFile == nil {
		return webdav.StatusCode(404), nil, -1
	}

	f := pFile.file
	downloadURL := f.WebContentLink
	log.Debug("downloadURL=", downloadURL)
	if downloadURL == "" {
		log.Error("No download url: ", f)
		return webdav.StatusCode(500), nil, -1
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		log.Error("NewRequest ", err)
		return webdav.StatusCode(500), nil, -1
	}

	resp, err := fs.roundTripper.RoundTrip(req)
	if err != nil {
		log.Error("RoundTrip ", err)
		return webdav.StatusCode(500), nil, -1
	}

	return webdav.StatusCode(200), resp.Body, f.Size
}

// PropList fetches file properties.
func (fs *FileSystem) PropList(p string, depth int, props []string) (webdav.StatusCode, map[string][]webdav.PropertyValue) {
	f := fs.getFile(p, false)

	log.Debug("PropList f=", f, " depth=", depth)
	if f == nil {
		log.Debug("Can't find file ", p)
		return webdav.StatusCode(404), nil
	}

	if depth != 0 && depth != 1 {
		log.Error("Unsupported depth ", depth)
		return webdav.StatusCode(500), nil
	}

	files := []*fileAndPath{f}

	if depth == 1 {
		query := fmt.Sprintf("'%s' in parents", f.file.Id)
		r, err := fs.client.Files.List().Q(query).Do()

		if err != nil {
			log.Error("Can't list children ", err)
			return webdav.StatusCode(505), nil
		}

		for _, file := range r.Files {
			if ignoreFile(file) {
				continue
			}

			files = append(files, &fileAndPath{file: file, path: path.Join(p, file.Name)})
		}
	}

	return fs.listPropsFromFiles(files, props)
}

// Copy creates a file copy.
func (fs *FileSystem) Copy(from string, to string, depth int, overwrite bool) webdav.CopyStatusCode {
	log.Debug("DoCopy ", from, " -> ", to)
	to = strings.TrimRight(to, "/")

	fromFile := fs.getFile(from, false)

	log.Debug("1", to)
	if fromFile == nil {
		log.Debug("Src not found: ", from)
		return webdav.CopyNotFound
	}

	log.Debug("2", to)
	toFile := fs.getFile(to, false)

	log.Debug("3", to)
	if !overwrite && toFile != nil {
		log.Debug("Target exists but overwrite is false ", to)
		return webdav.CopyPreconditionFailed
	}

	log.Debug("4", to)
	status := webdav.CopyCreated

	log.Debug("5", to)
	if toFile != nil {
		deleteStatus := fs.Delete(to)
		log.Debug("Deleting target: ", to, " : ", deleteStatus)
		if deleteStatus != webdav.DeleteDeleted {
			log.Debug("Can't delete target folder")
			return webdav.CopyUnknownError
		}
		status = webdav.CopyNoContent
	}

	log.Debug("6", to)
	toDir := path.Dir(to)
	toBase := path.Base(to)

	toDirFile := fs.getFile(toDir, true)
	if toDirFile == nil {
		log.Error("To dir ", toDir, " not found")
		return webdav.CopyConflict
	}

	log.Debug("7", to)
	if isFolder(fromFile.file) {
		log.Error("Directory copy not supported")
		return webdav.CopyUnknownError
	}

	f := &drive.File{
		Name:    toBase,
		Parents: []string{toDirFile.file.Id},
	}

	log.Debug("9", to)
	_, err := fs.client.Files.Copy(fromFile.file.Id, f).Do()
	if err != nil {
		log.Error("Copy failed: ", err)
		return webdav.CopyUnknownError
	}

	log.Debug("10", to)
	fs.invalidatePath(to)
	log.Debug("Copy done: ", status)
	return status
}

// Move moves the file.
func (fs *FileSystem) Move(from string, to string, overwrite bool) webdav.MoveStatusCode {
	fromFile := fs.getFile(from, false)
	toFile := fs.getFile(to, false)

	if !overwrite && toFile != nil {
		return webdav.CopyPreconditionFailed
	}

	if toFile != nil {
		log.Error("Overwrite not supported")
		return webdav.CopyUnknownError
	}

	toDir := path.Dir(to)
	toBase := path.Base(to)

	toDirFile := fs.getFile(toDir, true)
	if toDirFile == nil {
		log.Error("To dir not found")
		return webdav.CopyConflict
	}

	f := &drive.File{
		Name:    toBase,
		Parents: []string{toDirFile.file.Id},
	}

	_, err := fs.client.Files.Update(fromFile.file.Id, f).Do()
	if err != nil {
		log.Error("Patch failed: ", err)
		return webdav.CopyUnknownError
	}

	fs.invalidatePath(to)
	return webdav.MoveCreated
}

func (fs *FileSystem) listPropsFromFiles(files []*fileAndPath, props []string) (webdav.StatusCode, map[string][]webdav.PropertyValue) {
	result := make(map[string][]webdav.PropertyValue)

	for _, fp := range files {
		f := fp.file

		var pValues []webdav.PropertyValue

		for _, p := range props {
			switch p {
			case "getcontentlength":
				pValues = append(pValues, webdav.GetContentLengthPropertyValue(f.Size))
			case "displayname":
				pValues = append(pValues, webdav.DisplayNamePropertyValue(f.Name))
			case "resourcetype":
				b := false
				if isFolder(f) {
					b = true
				}
				pValues = append(pValues, webdav.ResourceTypePropertyValue(b))
			case "getcontenttype":
				s := f.MimeType
				if isFolder(f) {
					s = "httpd/unix-directory"
				}
				pValues = append(pValues, webdav.GetContentTypePropertyValue(s))
			case "getlastmodified":
				t, err := time.Parse(time.RFC3339, f.ModifiedTime)
				if err != nil {
					log.Error("Can't parse modified date ", err, " ", f.ModifiedTime)
					return webdav.StatusCode(500), nil
				}
				pValues = append(pValues, webdav.GetLastModifiedPropertyValue(t.Unix()))
			case "creationdate":
				t, err := time.Parse(time.RFC3339, f.CreatedTime)
				if err != nil {
					log.Error("Can't parse CreationDate date ", err, " ", f.CreatedTime)
					return webdav.StatusCode(500), nil
				}
				pValues = append(pValues, webdav.GetLastModifiedPropertyValue(t.Unix()))
			case "getetag":
				pValues = append(pValues, webdav.GetEtagPropertyValue(""))
			case "quota-available-bytes":
				about, err := fs.about()
				if err != nil {
					log.Error("Can't get about info: ", err)
					return webdav.StatusCode(500), nil
				}
				pValues = append(pValues, webdav.QuotaAvailableBytesPropertyValue(about.StorageQuota.Limit-about.StorageQuota.Usage))
			case "quota-used-bytes":
				about, err := fs.about()
				if err != nil {
					log.Error("Can't get about info: ", err)
					return webdav.StatusCode(500), nil
				}
				pValues = append(pValues, webdav.QuotaUsedBytesPropertyValue(about.StorageQuota.UsageInDrive))
			case "quotaused", "quota":
				// ignore
				continue
			default:
				log.Error("Unsupported property: ", p)
				return webdav.StatusCode(500), nil
			}
		}

		result[fp.path] = pValues
	}

	return webdav.StatusCode(200), result
}

func (fs *FileSystem) getFileID(p string, onlyFolder bool) string {
	f := fs.getFile(p, onlyFolder)

	if f == nil {
		return ""
	}

	return f.file.Id
}

type fileLookupResult struct {
	fp *fileAndPath
}

func (fs *FileSystem) getFile(p string, onlyFolder bool) *fileAndPath {
	key := cacheKeyFile + p

	if lookup, found := fs.cache.Get(key); found {
		log.Debug("Reusing cached file: ", p)
		return lookup.(*fileLookupResult).fp
	}

	lookup := &fileLookupResult{fp: fs.getFile0(p, onlyFolder)}
	fs.cache.Set(key, lookup, time.Minute)
	return lookup.fp
}

func (fs *FileSystem) getFile0(p string, onlyFolder bool) *fileAndPath {
	if strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}

	if p == "" {
		f, err := fs.client.Files.Get("root").Do()
		if err != nil {
			log.Errorf("can't get: %v", err)
			// todo: handle errors better
			return nil
		}
		return &fileAndPath{file: f, path: "/"}
	}

	parent := path.Dir(p)
	base := path.Base(p)

	parentID := fs.getFileID(parent, true)
	if parentID == "" {
		// todo: handle errors better
		return nil
	}

	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents and title='%s'", parentID, base)
	if onlyFolder {
		query += " and mimeType='" + mimeTypeFolder + "'"
	}
	q.Q(query)

	r, err := q.Do()

	if err != nil {
		// todo: handle errors better
		log.Errorf("can't list for query %v : %v", query, err)
		return nil
	}

	for _, file := range r.Files {
		if ignoreFile(file) {
			continue
		}

		return &fileAndPath{file: file, path: p}
	}

	return nil
}

func ignoreFile(f *drive.File) bool {
	return f.Trashed
}

func isFolder(f *drive.File) bool {
	return f.MimeType == mimeTypeFolder
}

func (fs *FileSystem) about() (*drive.About, error) {
	if about, found := fs.cache.Get(cacheKeyAbout); found {
		return about.(*drive.About), nil
	}

	about, err := fs.client.About.Get().Do()
	if err != nil {
		log.Error("Can't get about info: ", err)
		return nil, err
	}
	fs.cache.Set(cacheKeyAbout, about, time.Minute)
	return about, nil
}

func (fs *FileSystem) invalidatePath(p string) {
	fs.cache.Delete(cacheKeyFile + p)
}
