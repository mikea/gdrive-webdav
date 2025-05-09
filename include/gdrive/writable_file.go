package gdrive

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
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
	logger     *slog.Logger
}

func newOpenWritableFile(ctx context.Context, fileSystem *fileSystem, name string, flag int, perm os.FileMode) *openWritableFile {
	return &openWritableFile{
		ctx: ctx, fileSystem: fileSystem, name: name, flag: flag, perm: perm, logger: fileSystem.logger,
	}
}

func (f *openWritableFile) Write(p []byte) (int, error) {
	f.logger.Debug("Write", slog.String("name", f.name), slog.Int("len", len(p)))
	n, err := f.buffer.Write(p)
	f.size += int64(n)
	return n, err
}

func (f *openWritableFile) Readdir(_ int) ([]os.FileInfo, error) {
	f.logger.Error("Readdir not implemented")
	panic("not supported")
}

func (f *openWritableFile) Stat() (os.FileInfo, error) {
	return &fileInfo{
		isDir: false,
		size:  f.size,
	}, nil
}

func (f *openWritableFile) Close() error {
	f.logger.Debug("Close", slog.String("name", f.name), slog.Int("len", f.buffer.Len()))
	fs := f.fileSystem
	fileID, err := fs.getFileID(f.name, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		f.logger.Error("error getting file ID", slog.String("name", f.name), slog.String("error", err.Error()))
		return err
	}

	if fileID != "" {
		err = os.ErrExist
		f.logger.Error("file already exists", slog.String("name", f.name), slog.String("id", fileID))
		return err
	}

	parent := path.Dir(f.name)
	base := path.Base(f.name)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		f.logger.Error("error getting parent ID", slog.String("parent", parent), slog.String("error", err.Error()))
		return err
	}

	if parentID == "" {
		f.logger.Error("parent not found", slog.String("parent", parent))
		return os.ErrNotExist
	}

	file := &drive.File{
		Name:    base,
		Parents: []string{parentID},
	}

	_, err = fs.client.Files.Create(file).Media(&f.buffer).Do()
	if err != nil {
		f.logger.Error("error creating file", slog.String("name", f.name), slog.String("error", err.Error()))
		return err
	}

	fs.invalidatePath(f.name)
	fs.invalidatePath(parent)

	f.logger.Debug("Close", slog.String("name", f.name), slog.Int("len", f.buffer.Len()))
	return nil
}

func (f *openWritableFile) Read(p []byte) (n int, err error) {
	f.logger.Error("Read not implemented", slog.String("name", f.name), slog.Int("len", len(p)))
	return -1, nil
}

func (f *openWritableFile) Seek(offset int64, whence int) (int64, error) {
	f.logger.Error("Seek not implemented", slog.String("name", f.name), slog.Int64("offset", offset), slog.Int("whence", whence))
	return -1, nil
}

// DeadPropsHolder interface

func (f *openWritableFile) DeadProps() (map[xml.Name]webdav.Property, error) {
	f.logger.Debug("DeadProps", slog.String("name", f.name))
	fileAndPath, err := f.fileSystem.getFile(f.name, false)
	if err != nil {
		return nil, err
	}
	f.logger.Debug("DeadProps", slog.String("name", f.name), slog.Any("props", fileAndPath.file.AppProperties))
	if len(fileAndPath.file.AppProperties) == 0 {
		return nil, nil
	}
	return appPropertiesToMap(fileAndPath.file.AppProperties, f.logger), nil
}

func (f *openWritableFile) Patch(props []webdav.Proppatch) ([]webdav.Propstat, error) {
	f.logger.Debug("Patch", slog.String("name", f.name), slog.Any("props", props))

	appProperties := make(map[string]string)
	for i := range props {
		for j := range props[i].Props {
			prop := props[i].Props[j]
			key := url.QueryEscape(prop.XMLName.Space + "!" + prop.XMLName.Local)
			if len(prop.InnerXML) > 0 {
				appProperties[key] = string(prop.InnerXML)
			} else {
				// todo: this should be nil, but defined types don't let me.
				appProperties[key] = ""
			}
		}
	}
	if len(appProperties) == 0 {
		return nil, nil
	}

	file := drive.File{
		AppProperties: appProperties,
	}

	fileID, err := f.fileSystem.getFileID(f.name, false)
	if err != nil {
		return nil, err
	}
	u := f.fileSystem.client.Files.Update(fileID, &file)
	u.Fields("appProperties")
	response, err := u.Do()
	if err != nil {
		f.logger.Error("error updating file", slog.String("name", f.name), slog.String("error", err.Error()))
		return nil, err
	}
	f.fileSystem.invalidatePath(f.name)
	return appPropertiesToList(response.AppProperties, f.logger), nil
}

func appPropertiesToList(m map[string]string, logger *slog.Logger) []webdav.Propstat {
	var props []webdav.Property

	for k, v := range m {
		if len(v) == 0 {
			continue
		}
		k, err := url.QueryUnescape(k)
		if err != nil {
			logger.Error("unexpected properties", slog.String("key", k), slog.String("value", v))
			panic(fmt.Sprintf("unexpected properties: %v %v", m, err))
		}
		sep := strings.Index(k, "!")
		if sep < 0 {
			logger.Error("unexpected key", slog.String("key", k))
		}
		ns := k[:sep]
		n := k[sep+1:]
		prop := webdav.Property{
			XMLName:  xml.Name{Space: ns, Local: n},
			InnerXML: []byte(v),
		}
		props = append(props, prop)
	}

	propstat := webdav.Propstat{
		Props:  props,
		Status: 200,
	}
	return []webdav.Propstat{propstat}
}

func appPropertiesToMap(m map[string]string, logger *slog.Logger) map[xml.Name]webdav.Property {
	props := make(map[xml.Name]webdav.Property)

	for k, v := range m {
		if len(v) == 0 {
			continue
		}
		k, err := url.QueryUnescape(k)
		if err != nil {
			logger.Error("unexpected properties", slog.String("key", k), slog.String("value", v))
			panic(fmt.Sprintf("unexpected properties: %v %v", m, err))
		}
		sep := strings.Index(k, "!")
		if sep < 0 {
			logger.Error("unexpected key", slog.String("key", k))
		}
		ns := k[:sep]
		n := k[sep+1:]
		prop := webdav.Property{
			XMLName:  xml.Name{Space: ns, Local: n},
			InnerXML: []byte(v),
		}
		props[prop.XMLName] = prop
	}
	return props
}
