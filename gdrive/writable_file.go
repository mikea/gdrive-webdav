package gdrive

import (
	"bytes"
	"encoding/xml"
	"errors"
	"net/url"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
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
}

func newOpenWritableFile(ctx context.Context, fileSystem *fileSystem, name string, flag int, perm os.FileMode) *openWritableFile {
	return &openWritableFile{
		ctx: ctx, fileSystem: fileSystem, name: name, flag: flag, perm: perm,
	}
}

func (f *openWritableFile) Write(p []byte) (int, error) {
	log.Debugf("Write %v %v", f.name, len(p))
	n, err := f.buffer.Write(p)
	f.size += int64(n)
	return n, err
}

func (f *openWritableFile) Readdir(_count int) ([]os.FileInfo, error) {
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
		log.Errorf("Can't file file %v", f.name)
		return os.ErrNotExist
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

	log.Trace("Close succesfull ", f.name)
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

// DeadPropsHolder interface

func (f *openWritableFile) DeadProps() (map[xml.Name]webdav.Property, error) {
	log.Debugf("DeadProps %v", f.name)
	fileAndPath, err := f.fileSystem.getFile(f.name, false)
	if err != nil {
		return nil, err
	}
	log.Tracef("appProperties %v", fileAndPath.file.AppProperties)
	if len(fileAndPath.file.AppProperties) == 0 {
		return nil, nil
	}
	return appPropertiesToMap(fileAndPath.file.AppProperties), nil
}

func (f *openWritableFile) Patch(props []webdav.Proppatch) ([]webdav.Propstat, error) {
	log.Debugf("Patch %v %v", f.name, props)

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
		log.Error(err)
		return nil, err
	}
	f.fileSystem.invalidatePath(f.name)
	return appPropertiesToList(response.AppProperties), nil
}

func appPropertiesToList(m map[string]string) []webdav.Propstat {
	var props []webdav.Property

	for k, v := range m {
		if len(v) == 0 {
			continue
		}
		k, err := url.QueryUnescape(k)
		if err != nil {
			log.Panicf("unexpected properties: %v %v", m, err)
		}
		sep := strings.Index(k, "!")
		if sep < 0 {
			log.Panicf("unexpected key: %v", k)
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

func appPropertiesToMap(m map[string]string) map[xml.Name]webdav.Property {
	props := make(map[xml.Name]webdav.Property)

	for k, v := range m {
		if len(v) == 0 {
			continue
		}
		k, err := url.QueryUnescape(k)
		if err != nil {
			log.Panicf("unexpected properties: %v %v", m, err)
		}
		sep := strings.Index(k, "!")
		if sep < 0 {
			log.Panicf("unexpected key: %v", k)
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
