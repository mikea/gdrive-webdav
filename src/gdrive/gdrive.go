package gdrive

import (
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/drive/v2"
	"flag"
	"fmt"
	log "github.com/cihub/seelog"
	gocache "github.com/pmylund/go-cache"
	"io"
	"net/http"
	"os/user"
	"path"
	"strings"
	"time"
	"webdav"
)

type GDriveFileSystem struct {
	client    *drive.Service
	transport *oauth.Transport
	cache     *gocache.Cache
}

var (
	tokenFileFlag = flag.String("token-file", "", "OAuth token cache file. ~/.gdrive_token by default.")
)

var config = &oauth.Config{
	Scope:       "https://www.googleapis.com/auth/drive",
	RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
	AuthURL:     "https://accounts.google.com/o/oauth2/auth",
	TokenURL:    "https://accounts.google.com/o/oauth2/token",
}

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
	cacheKeyAbout  = "global:about"
	cacheKeyFile   = "file:"
)

type fileAndPath struct {
	file *drive.File
	path string
}

func NewGDriveFileSystem(clientId string, clientSecret string) *GDriveFileSystem {
	u, err := user.Current()

	tokenFile := u.HomeDir + "/.gdrive_token"

	if *tokenFileFlag != "" {
		tokenFile = *tokenFileFlag
	}

	config.TokenCache = oauth.CacheFile(tokenFile)
	config.ClientId = clientId
	config.ClientSecret = clientSecret

	transport := &oauth.Transport{
		Config:    config,
		Transport: &loggingTransport{http.DefaultTransport},
	}

	obtainToken(transport)

	client, err := drive.New(transport.Client())
	if err != nil {
		log.Errorf("An error occurred creating Drive client: %v\n", err)
		panic(-3)
	}

	fs := &GDriveFileSystem{
		client:    client,
		transport: transport,
		cache:     gocache.New(5*time.Minute, 30*time.Second),
	}
	return fs
}

func (fs *GDriveFileSystem) MkDir(p string) webdav.MkColStatusCode {
	pId := fs.getFileId(p, false)
	if pId != "" {
		log.Errorf("dir already exists: %v", pId)
		return webdav.MkColMethodNotAllowed
	}

	p = strings.TrimRight(p, "/")
	parent := path.Dir(p)
	dir := path.Base(p)

	parentId := fs.getFileId(parent, true)

	if parentId == "" {
		log.Errorf("parent not found")
		return webdav.MkColConflict
	}

	parentRef := &drive.ParentReference{
		Id:     parentId,
		IsRoot: "parent" == "/",
	}

	f := &drive.File{
		MimeType: mimeTypeFolder,
		Title:    dir,
		Parents:  []*drive.ParentReference{parentRef},
	}

	_, err := fs.client.Files.Insert(f).Do()

	if err != nil {
		return webdav.MkColUnknownError
	}

	fs.invalidatePath(p)
	fs.invalidatePath(parent)
	return webdav.MkColCreated
}

func (fs *GDriveFileSystem) Delete(p string) webdav.DeleteStatusCode {
	pId := fs.getFileId(p, false)
	if pId == "" {
		return webdav.DeleteNotFound
	}

	err := fs.client.Files.Delete(pId).Do()
	if err != nil {
		log.Errorf("can't delete file %v", err)
		return webdav.DeleteUnknownError
	}

	fs.invalidatePath(p)
	fs.invalidatePath(path.Dir(p))
	return webdav.DeleteDeleted
}

func (fs *GDriveFileSystem) Put(p string, bytes io.ReadCloser) webdav.StatusCode {
	defer bytes.Close()
	parent := path.Dir(p)
	base := path.Base(p)

	parentId := fs.getFileId(parent, true)

	if parentId == "" {
		log.Errorf("ERROR: Parent not found")
		return webdav.StatusCode(http.StatusConflict) // 409
	}

	parentRef := &drive.ParentReference{
		Id:     parentId,
		IsRoot: "parent" == "/",
	}

	f := &drive.File{
		Title:   base,
		Parents: []*drive.ParentReference{parentRef},
	}

	_, err := fs.client.Files.Insert(f).Media(bytes).Do()
	if err != nil {
		log.Errorf("can't put: %v", err)
		return webdav.StatusCode(500)
	}

	fs.invalidatePath(p)
	fs.invalidatePath(parent)
	return webdav.StatusCode(201)
}

func (fs *GDriveFileSystem) Get(p string) (webdav.StatusCode, io.ReadCloser, int64) {
	pFile := fs.getFile(p, false)
	if pFile == nil {
		return webdav.StatusCode(404), nil, -1
	}

	f := pFile.file
	downloadUrl := f.DownloadUrl
	log.Debug("downloadUrl=", downloadUrl)
	if downloadUrl == "" {
		log.Error("No download url: ", f)
		return webdav.StatusCode(500), nil, -1
	}

	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		log.Error("NewRequest ", err)
		return webdav.StatusCode(500), nil, -1
	}

	resp, err := fs.transport.RoundTrip(req)
	if err != nil {
		log.Error("RoundTrip ", err)
		return webdav.StatusCode(500), nil, -1
	}

	return webdav.StatusCode(200), resp.Body, f.FileSize
}

func (fs *GDriveFileSystem) PropList(p string, depth int, props []string) (webdav.StatusCode, map[string][]webdav.PropertyValue) {
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

		for _, file := range r.Items {
			if ignoreFile(file) {
				continue
			}

			files = append(files, &fileAndPath{file: file, path: path.Join(p, file.Title)})
		}
	}

	return fs.listPropsFromFiles(files, props)
}

func (fs *GDriveFileSystem) Copy(from string, to string, depth int, overwrite bool) webdav.CopyStatusCode {
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

	log.Debug("8", to)
	parentRef := &drive.ParentReference{
		Id: toDirFile.file.Id,
	}

	f := &drive.File{
		Title:   toBase,
		Parents: []*drive.ParentReference{parentRef},
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

func (fs *GDriveFileSystem) Move(from string, to string, overwrite bool) webdav.MoveStatusCode {
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

	parentRef := &drive.ParentReference{
		Id: toDirFile.file.Id,
	}

	f := &drive.File{
		Title:   toBase,
		Parents: []*drive.ParentReference{parentRef},
	}

	_, err := fs.client.Files.Patch(fromFile.file.Id, f).Do()
	if err != nil {
		log.Error("Patch failed: ", err)
		return webdav.CopyUnknownError
	}

	fs.invalidatePath(to)
	return webdav.MoveCreated
}

func (fs *GDriveFileSystem) listPropsFromFiles(files []*fileAndPath, props []string) (webdav.StatusCode, map[string][]webdav.PropertyValue) {
	result := make(map[string][]webdav.PropertyValue)

	for _, fp := range files {
		f := fp.file

		var pValues []webdav.PropertyValue

		for _, p := range props {
			switch p {
			case "getcontentlength":
				pValues = append(pValues, webdav.GetContentLengthPropertyValue(f.FileSize))
			case "displayname":
				pValues = append(pValues, webdav.DisplayNamePropertyValue(f.Title))
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
				t, err := time.Parse(time.RFC3339, f.ModifiedDate)
				if err != nil {
					log.Error("Can't parse modified date ", err, " ", f.ModifiedDate)
					return webdav.StatusCode(500), nil
				}
				pValues = append(pValues, webdav.GetLastModifiedPropertyValue(t.Unix()))
			case "creationdate":
				t, err := time.Parse(time.RFC3339, f.CreatedDate)
				if err != nil {
					log.Error("Can't parse CreationDate date ", err, " ", f.CreatedDate)
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
				pValues = append(pValues, webdav.QuotaAvailableBytesPropertyValue(about.QuotaBytesTotal-about.QuotaBytesUsed))
			case "quota-used-bytes":
				about, err := fs.about()
				if err != nil {
					log.Error("Can't get about info: ", err)
					return webdav.StatusCode(500), nil
				}
				pValues = append(pValues, webdav.QuotaUsedBytesPropertyValue(about.QuotaBytesUsed))
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

func obtainToken(transport *oauth.Transport) {
	t, _ := config.TokenCache.Token()
	if t != nil {
		return
	}

	authUrl := config.AuthCodeURL("state")
	fmt.Printf("Go to the following link in your browser: %v\n", authUrl)

	fmt.Printf("Enter verification code: ")
	var code string
	fmt.Scanln(&code)

	// Read the code, and exchange it for a token.
	_, err := transport.Exchange(code)
	if err != nil {
		log.Errorf("An error occurred exchanging the token: %v\n", err)
		panic(-2)
	}
}

func (fs *GDriveFileSystem) getFileId(p string, onlyFolder bool) string {
	f := fs.getFile(p, onlyFolder)

	if f == nil {
		return ""
	}

	return f.file.Id
}

type FileLookupResult struct {
	fp *fileAndPath
}

func (fs *GDriveFileSystem) getFile(p string, onlyFolder bool) *fileAndPath {
	key := cacheKeyFile + p

	if lookup, found := fs.cache.Get(key); found {
		log.Debug("Reusing cached file: ", p)
		return lookup.(*FileLookupResult).fp
	}

	lookup := &FileLookupResult{fp: fs.getFile0(p, onlyFolder)}
	fs.cache.Set(key, lookup, time.Minute)
	return lookup.fp
}

func (fs *GDriveFileSystem) getFile0(p string, onlyFolder bool) *fileAndPath {
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

	parentId := fs.getFileId(parent, true)
	if parentId == "" {
		// todo: handle errors better
		return nil
	}

	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents and title='%s'", parentId, base)
	if onlyFolder {
		query += " and mimeType='" + mimeTypeFolder + "'"
	}
	q.Q(query)

	r, err := q.Do()

	if err != nil {
		// todo: handle errors better
		log.Errorf("can't list for query %v : ", query, err)
		return nil
	}

	for _, item := range r.Items {
		if ignoreFile(item) {
			continue
		}

		return &fileAndPath{file: item, path: p}
	}

	return nil
}

func ignoreFile(f *drive.File) bool {
	return f.Labels.Trashed
}

func isFolder(f *drive.File) bool {
	return f.MimeType == mimeTypeFolder
}

func (fs *GDriveFileSystem) about() (*drive.About, error) {
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

func (fs *GDriveFileSystem) invalidatePath(p string) {
	fs.cache.Delete(cacheKeyFile + p)
}
