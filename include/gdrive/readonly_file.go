package gdrive

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"os"

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
	logger        *slog.Logger
}

func newOpenReadonlyFile(fs *fileSystem, file *drive.File) *openReadonlyFile {
	return &openReadonlyFile{fs: fs, file: file, logger: fs.logger}
}

func (f *openReadonlyFile) Write(_ []byte) (int, error) {
	return -1, errors.New("can't write to readonly file")
}

func (f *openReadonlyFile) Readdir(_ int) ([]os.FileInfo, error) {
	return f.fs.readdir(f.file)
}

func (f *openReadonlyFile) Stat() (os.FileInfo, error) {
	return newFileInfo(f.file, f.logger), nil
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
		f.logger.Error("error downloading file", slog.String("name", f.file.Name), slog.String("error", err.Error()))
		return err
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		f.logger.Error("error reading response body", slog.String("name", f.file.Name), slog.String("error", err.Error()))
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		f.logger.Error("error closing response body", slog.String("name", f.file.Name), slog.String("error", err.Error()))
		return err
	}

	f.size = int64(len(content))
	f.content = content
	f.contentReader = bytes.NewBuffer(content)
	return nil
}

func (f *openReadonlyFile) Read(p []byte) (n int, err error) {
	f.logger.Debug("Read", slog.String("name", f.file.Name), slog.Int("size", len(p)))
	err = f.initContent()

	if err != nil {
		f.logger.Error("error initializing content", slog.String("name", f.file.Name), slog.String("error", err.Error()))
		return 0, err
	}

	n, err = f.contentReader.Read(p)
	if err != nil {
		f.logger.Error("error reading content", slog.String("name", f.file.Name), slog.String("error", err.Error()))
		return 0, err
	}
	f.pos += int64(n)
	return n, err
}

func (f *openReadonlyFile) Seek(offset int64, whence int) (int64, error) {
	f.logger.Debug("Seek", slog.Int64("offset", offset), slog.Int("whence", whence))

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
	return appPropertiesToMap(f.file.AppProperties, f.logger), nil
}

func (f *openReadonlyFile) Patch(_ []webdav.Proppatch) ([]webdav.Propstat, error) {
	panic("should not be called")
}
