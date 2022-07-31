package gdrive

import (
	"bytes"
	"errors"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/api/drive/v3"
)

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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
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
	log.Panic("not implemented", p)
	return -1, nil
}

func (f *openWritableFile) Seek(offset int64, whence int) (int64, error) {
	log.Panic("not implemented", offset, whence)
	return -1, nil
}
