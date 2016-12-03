package gdrive

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	gocache "github.com/pmylund/go-cache"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
	"google.golang.org/api/drive/v3"
)

type fileSystem struct {
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

// NewFS creates new gdrive file system.
func NewFS(ctx context.Context, clientID string, clientSecret string) webdav.FileSystem {
	httpClient := newHTTPClient(ctx, clientID, clientSecret)
	client, err := drive.New(httpClient)
	if err != nil {
		log.Errorf("An error occurred creating Drive client: %v\n", err)
		panic(-3)
	}

	fs := &fileSystem{
		client:       client,
		roundTripper: httpClient.Transport,
		cache:        gocache.New(5*time.Minute, 30*time.Second),
	}
	return fs
}

// NewLS creates new GDrive locking system
func NewLS() webdav.LockSystem {
	return webdav.NewMemLS()
}

func (fs *fileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	pID, err := fs.getFileID(name, false)
	if err != os.ErrNotExist {
		log.Errorf("Error: %v", err)
		return err
	}
	if err == nil {
		log.Errorf("dir already exists: %v", pID)
		return os.ErrExist
	}

	name = strings.TrimRight(name, "/")
	parent := path.Dir(name)
	dir := path.Base(name)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		return err
	}

	if parentID == "" {
		log.Errorf("parent not found")
		return os.ErrNotExist
	}

	f := &drive.File{
		MimeType: mimeTypeFolder,
		Name:     dir,
		Parents:  []string{parentID},
	}

	_, err = fs.client.Files.Create(f).Do()
	if err != nil {
		return err
	}

	fs.invalidatePath(name)
	fs.invalidatePath(parent)

	return nil
}

type file struct {
	fileSystem *fileSystem
	buffer     *bytes.Buffer
	name       string
	flag       int
	perm       os.FileMode
}

func (f *file) Write(p []byte) (n int, err error) {
	if f.buffer == nil {
		f.buffer = bytes.NewBuffer(p)
		return n, nil
	}
	return f.buffer.Write(p)
}

func (f *file) Readdir(count int) ([]os.FileInfo, error) {
	panic("not implemented")
}

func (f *file) Stat() (os.FileInfo, error) {
	return &fileInfo{}, nil
}

func (f *file) Close() error {
	fs := f.fileSystem

	if f.buffer != nil {
		fileID, err := fs.getFileID(f.name, false)
		if err != nil {
			return err
		}

		if fileID != "" {
			return os.ErrExist
		}

		parent := path.Dir(f.name)
		base := path.Base(f.name)

		parentID, err := fs.getFileID(parent, true)
		if err != nil {

		}

		if parentID == "" {
			log.Errorf("ERROR: Parent not found")
			return os.ErrNotExist
		}

		file := &drive.File{
			Name:    base,
			Parents: []string{parentID},
		}

		_, err = fs.client.Files.Create(file).Media(f.buffer).Do()
		if err != nil {
			log.Errorf("can't put: %v", err)
			return err
		}

		fs.invalidatePath(f.name)
		fs.invalidatePath(parent)
		return nil
	}

	panic("not implemented")
}
func (f *file) Read(p []byte) (n int, err error) {
	panic("not implemented")
}
func (f *file) Seek(offset int64, whence int) (int64, error) {
	panic("not implemented")
}

func (fs *fileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	return &file{
		fileSystem: fs,
		name:       name,
		flag:       flag,
		perm:       perm,
	}, nil
}

func (fs *fileSystem) RemoveAll(ctx context.Context, name string) error {
	id, err := fs.getFileID(name, false)
	if err != nil {
		return err
	}

	err = fs.client.Files.Delete(id).Do()
	if err != nil {
		log.Errorf("can't delete file %v", err)
		return err
	}

	fs.invalidatePath(name)
	fs.invalidatePath(path.Dir(name))
	return nil

}
func (fs *fileSystem) Rename(ctx context.Context, oldName, newName string) error {
	panic("not implemented")
}

type fileInfo struct {
	file *drive.File
}

func (fi *fileInfo) IsDir() bool {
	return fi.file.MimeType == mimeTypeFolder
}

func (fi *fileInfo) Name() string {
	panic("not implemented")
}
func (fi *fileInfo) Size() int64 {
	panic("not implemented")
}
func (fi *fileInfo) Mode() os.FileMode {
	panic("not implemented")
}
func (fi *fileInfo) ModTime() time.Time {
	panic("not implemented")
}
func (fi *fileInfo) Sys() interface{} {
	panic("not implemented")
}

func (fs *fileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	f, err := fs.getFile(name, false)

	if err != nil {
		return nil, err
	}

	if f == nil {
		log.Debug("Can't find file ", name)
		return nil, os.ErrNotExist
	}

	return &fileInfo{file: f.file}, nil
	// files := []*fileAndPath{f}

	// query := fmt.Sprintf("'%s' in parents", f.file.Id)
	// r, err := fs.client.Files.List().Q(query).Do()

	// if err != nil {
	// 	log.Error("Can't list children ", err)
	// 	return nil, err
	// }

	// for _, file := range r.Files {
	// 	if ignoreFile(file) {
	// 		continue
	// 	}

	// 	files = append(files, &fileAndPath{file: file, path: path.Join(name, file.Name)})
	// }

	// return fs.listPropsFromFiles(files, props)
}

/*

// MkDir creates a directory
func (fs *FileSystem) MkDir(p string) webdav.MkColStatusCode {

}

// Delete deletes the file
func (fs *FileSystem) Delete(p string) webdav.DeleteStatusCode {
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

*/

func (fs *fileSystem) getFileID(p string, onlyFolder bool) (string, error) {
	f, err := fs.getFile(p, onlyFolder)

	if err != nil {
		return "", err
	}

	return f.file.Id, nil
}

func (fs *fileSystem) invalidatePath(p string) {
	fs.cache.Delete(cacheKeyFile + p)
}

type fileLookupResult struct {
	fp  *fileAndPath
	err error
}

func (fs *fileSystem) getFile(p string, onlyFolder bool) (*fileAndPath, error) {
	key := cacheKeyFile + p

	if lookup, found := fs.cache.Get(key); found {
		log.Debug("Reusing cached file: ", p)
		result := lookup.(*fileLookupResult)
		return result.fp, result.err
	}

	fp, err := fs.getFile0(p, onlyFolder)
	lookup := &fileLookupResult{fp: fp, err: err}
	if err != nil {
		fs.cache.Set(key, lookup, time.Minute)
	}
	return lookup.fp, lookup.err
}

func (fs *fileSystem) getFile0(p string, onlyFolder bool) (*fileAndPath, error) {
	if strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}

	if p == "" {
		f, err := fs.client.Files.Get("root").Do()
		if err != nil {
			log.Errorf("E1: %v", err)
			return nil, err
		}
		return &fileAndPath{file: f, path: "/"}, nil
	}

	parent := path.Dir(p)
	base := path.Base(p)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		log.Errorf("E2: %v", err)
		return nil, err
	}

	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents and name='%s'", parentID, base)
	if onlyFolder {
		query += " and mimeType='" + mimeTypeFolder + "'"
	}
	q.Q(query)
	log.Errorf("Query: %v", q)

	r, err := q.Do()

	if err != nil {
		log.Errorf("E4: %v", err)
		return nil, err
	}

	for _, file := range r.Files {
		if ignoreFile(file) {
			continue
		}

		return &fileAndPath{file: file, path: p}, nil
	}
	log.Errorf("E5: %v", err)

	return nil, os.ErrNotExist
}

func ignoreFile(f *drive.File) bool {
	return f.Trashed
}
