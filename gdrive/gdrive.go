package gdrive

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"io/ioutil"

	"io"

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

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
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
	log.Debugf("Mkdir %v %v", name, perm)
	name = normalizePath(name)
	pID, err := fs.getFileID(name, false)
	if err != nil && err != os.ErrNotExist {
		log.Error(err)
		return err
	}
	if err == nil {
		log.Errorf("dir already exists: %v", pID)
		return os.ErrExist
	}

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

type openWritableFile struct {
	ctx        context.Context
	fileSystem *fileSystem
	buffer     bytes.Buffer
	size       int64
	name       string
	flag       int
	perm       os.FileMode
}

func (f *openWritableFile) Write(p []byte) (int, error) {
	n, err := f.buffer.Write(p)
	f.size += int64(n)
	return n, err
}

func (f *openWritableFile) Readdir(count int) ([]os.FileInfo, error) {
	panic("not supported")
}

func (f *openWritableFile) Stat() (os.FileInfo, error) {
	return &fileInfo{
		isDir: false,
		size:  f.size,
	}, nil
}

func (f *openWritableFile) Close() error {
	log.Debugf("Close %v", f.name)
	fs := f.fileSystem
	fileID, err := fs.getFileID(f.name, false)
	if err != nil && err != os.ErrNotExist {
		log.Error(err)
		return err
	}

	if fileID != "" {
		err = os.ErrExist
		log.Error(err)
		return err
	}

	parent := path.Dir(f.name)
	base := path.Base(f.name)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		log.Error(err)
		return err
	}

	if parentID == "" {
		err = os.ErrNotExist
		log.Error(err)
		return err
	}

	file := &drive.File{
		Name:    base,
		Parents: []string{parentID},
	}

	_, err = fs.client.Files.Create(file).Media(&f.buffer).Do()
	if err != nil {
		log.Error(err)
		return err
	}

	fs.invalidatePath(f.name)
	fs.invalidatePath(parent)

	log.Debug("Close succesfull ", f.name)
	return nil
}
func (f *openWritableFile) Read(p []byte) (n int, err error) {
	log.Critical("not implemented")
	panic("not implemented")
}
func (f *openWritableFile) Seek(offset int64, whence int) (int64, error) {
	log.Critical("not implemented")
	panic("not implemented")
}

type openReadonlyFile struct {
	fs            *fileSystem
	file          *drive.File
	content       []byte
	size          int64
	pos           int64
	contentReader io.Reader
}

func (f *openReadonlyFile) Write(p []byte) (int, error) {
	log.Critical("not implemented")
	panic("not implemented")
}

func (f *openReadonlyFile) Readdir(count int) ([]os.FileInfo, error) {
	panic("not supported")
}

func (f *openReadonlyFile) Stat() (os.FileInfo, error) {
	return newFileInfo(f.file), nil
}

func (f *openReadonlyFile) Close() error {
	f.content = nil
	f.contentReader = nil
	return nil
}

func (f *openReadonlyFile) initContent() error {
	if f.content != nil {
		return nil
	}

	resp, err := f.fs.client.Files.Get(f.file.Id).Download()
	if err != nil {
		log.Error(err)
		return err
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		log.Error(err)
		return err
	}

	f.size = int64(len(content))
	f.content = content
	f.contentReader = bytes.NewBuffer(content)
	return nil
}

func (f *openReadonlyFile) Read(p []byte) (n int, err error) {
	log.Debug("Read ", len(p))
	err = f.initContent()

	if err != nil {
		log.Error(err)
		return 0, err
	}

	n, err = f.contentReader.Read(p)
	if err != nil {
		log.Error(err)
		return 0, err
	}
	f.pos += int64(n)
	return n, err
}

func (f *openReadonlyFile) Seek(offset int64, whence int) (int64, error) {
	log.Debug("Seek ", offset, whence)

	if whence == 0 {
		// io.SeekStart
		if f.content != nil {
			f.pos = 0
			f.contentReader = bytes.NewBuffer(f.content)
			return 0, nil
		}
		return f.pos, nil
	}

	if whence == 2 {
		// io.SeekEnd
		err := f.initContent()
		if err != nil {
			return 0, err
		}
		f.contentReader = &bytes.Buffer{}
		f.pos = f.size
		return f.pos, nil
	}

	panic("not implemented")
}

func (fs *fileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	log.Debugf("OpenFile %v %v %v", name, flag, perm)
	name = normalizePath(name)

	if flag&os.O_RDWR != 0 {
		if flag != os.O_RDWR|os.O_CREATE|os.O_TRUNC {
			panic("not implemented")
		}

		return &openWritableFile{
			ctx:        ctx,
			fileSystem: fs,
			name:       name,
			flag:       flag,
			perm:       perm,
		}, nil
	}

	if flag == os.O_RDONLY {
		file, err := fs.getFile(name, false)
		if err != nil {
			return nil, err
		}
		return &openReadonlyFile{fs: fs, file: file.file}, nil
	}

	return nil, fmt.Errorf("unsupported open mode: %v", flag)
}

func (fs *fileSystem) RemoveAll(ctx context.Context, name string) error {
	log.Debugf("RemoveAll %v", name)
	name = normalizePath(name)
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
	log.Critical("not implemented")
	panic("not implemented")
}

type fileInfo struct {
	isDir   bool
	modTime time.Time
	size    int64
}

func newFileInfo(file *drive.File) *fileInfo {
	modTime, err := getModTime(file)
	if err != nil {
		log.Error(err)
		panic(err)
	}

	return &fileInfo{
		isDir:   file.MimeType == mimeTypeFolder,
		modTime: modTime,
		size:    file.Size,
	}
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

func (fi *fileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *fileInfo) Name() string {
	log.Critical("not implemented")
	panic("not implemented")
}
func (fi *fileInfo) Size() int64 {
	return fi.size
}
func (fi *fileInfo) Mode() os.FileMode {
	log.Critical("not implemented")
	panic("not implemented")
}
func (fi *fileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi *fileInfo) Sys() interface{} {
	return fi
}

func (fs *fileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	log.Debugf("Stat %v", name)
	f, err := fs.getFile(name, false)

	if err != nil {
		log.Error(err)
		return nil, err
	}

	if f == nil {
		log.Debug("Can't find file ", name)
		return nil, os.ErrNotExist
	}

	return newFileInfo(f.file), nil
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

func (fs *fileSystem) getFile0(p string, onlyFolder bool) (*fileAndPath, error) {
	log.Tracef("getFile0 %v %v", p, onlyFolder)
	p = normalizePath(p)

	if p == "" {
		f, err := fs.client.Files.Get("root").Do()
		if err != nil {
			log.Error(err)
			return nil, err
		}
		return &fileAndPath{file: f, path: "/"}, nil
	}

	parent := path.Dir(p)
	base := path.Base(p)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		log.Errorf("can't locate parent %v error: %v", parent, err)
		return nil, err
	}

	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents and name='%s'", parentID, base)
	if onlyFolder {
		query += " and mimeType='" + mimeTypeFolder + "'"
	}
	q.Q(query)
	log.Tracef("Query: %v", q)

	r, err := q.Do()

	if err != nil {
		log.Error(err)
		return nil, err
	}

	for _, file := range r.Files {
		if ignoreFile(file) {
			continue
		}

		return &fileAndPath{file: file, path: p}, nil
	}

	return nil, os.ErrNotExist
}

func ignoreFile(f *drive.File) bool {
	return f.Trashed
}

func normalizePath(p string) string {
	return strings.TrimRight(p, "/")
}
