package gdrive

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/api/drive/v3"
)

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

func (fi *fileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *fileInfo) Name() string {
	log.Panic("not implemented")
	return ""
}
func (fi *fileInfo) Size() int64 {
	return fi.size
}
func (fi *fileInfo) Mode() os.FileMode {
	log.Panic("not implemented")
	return 0
}
func (fi *fileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi *fileInfo) Sys() interface{} {
	return fi
}
