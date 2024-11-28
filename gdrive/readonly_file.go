package gdrive

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/webdav"
	"google.golang.org/api/drive/v3"
)

type openReadonlyFile struct {
	fs            *fileSystem
	file          *drive.File
	content       []byte
	size          int64
	pos           int64
	contentReader io.Reader
}

func newOpenReadonlyFile(fs *fileSystem, file *drive.File) *openReadonlyFile {
	return &openReadonlyFile{fs: fs, file: file}
}

func (f *openReadonlyFile) Write(_p []byte) (int, error) {
	return -1, errors.New("can't write to readonly file")
}

func (f *openReadonlyFile) Readdir(_count int) ([]os.FileInfo, error) {
	return f.fs.readdir(f.file)
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
	log.Debugf("Read %v %v", f.file.Name, len(p))
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
	log.Debugf("Seek %v %v", offset, whence)

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

// DeadPropsHolder interface

func (f *openReadonlyFile) DeadProps() (map[xml.Name]webdav.Property, error) {
	if len(f.file.AppProperties) == 0 {
		return nil, nil
	}
	return appPropertiesToMap(f.file.AppProperties), nil
}

func (f *openReadonlyFile) Patch(_props []webdav.Proppatch) ([]webdav.Propstat, error) {
	panic("should not be called")
}
