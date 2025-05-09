package gdrive

import (
	"log/slog"
	"os"
	"time"

	"google.golang.org/api/drive/v3"
)

type fileInfo struct {
	name    string
	isDir   bool
	modTime time.Time
	size    int64
}

func newFileInfo(file *drive.File, logger *slog.Logger) *fileInfo {
	modTime, err := getModTime(file)
	if err != nil {
		logger.Error(
			"failed to parse modified time",
			slog.String("file", file.Name),
			slog.String("modified_time", file.ModifiedTime),
			slog.String("created_time", file.CreatedTime),
			slog.Any("error", err),
		)
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
