package gdrive

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/drive/v3"
)

type fileInfo struct {
	name    string
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
		name:    file.Name,
		isDir:   file.MimeType == mimeTypeFolder,
		modTime: modTime,
		size:    file.Size,
	}
}

func (fi *fileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *fileInfo) Name() string {
	return fi.name
}
func (fi *fileInfo) Size() int64 {
	return fi.size
}
func (fi *fileInfo) Mode() os.FileMode {
	if fi.isDir {
		return os.ModeDir | 0o777
	}
	return 0o777
}
func (fi *fileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi *fileInfo) Sys() interface{} {
	return fi
}
