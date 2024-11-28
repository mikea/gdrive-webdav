package gdrive

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	gocache "github.com/pmylund/go-cache"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
	"google.golang.org/api/drive/v3"
)

type fileSystem struct {
	client *drive.Service
	cache  *gocache.Cache
}

func (fs *fileSystem) Mkdir(_ctx context.Context, name string, perm os.FileMode) error {
	log.Debugf("Mkdir %v %v", name, perm)
	name = normalizePath(name)
	pID, err := fs.getFileID(name, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
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

func (fs *fileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	log.Debugf("OpenFile %v %v %v", name, flag, perm)
	name = normalizePath(name)

	if flag&os.O_RDWR != 0 {
		return newOpenWritableFile(ctx, fs, name, flag, perm), nil
	}

	if flag == os.O_RDONLY {
		file, err := fs.getFile(name, false)
		if err != nil {
			return nil, err
		}
		return newOpenReadonlyFile(fs, file.file), nil
	}

	log.Errorf("unsupported open mode: %v", flag)
	return nil, fmt.Errorf("unsupported open mode: %v", flag)
}

func (fs *fileSystem) RemoveAll(_ctx context.Context, name string) error {
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

func (fs *fileSystem) Rename(_ctx context.Context, oldName, newName string) error {
	log.Debugf("Rename %v -> %v", oldName, newName)

	newFileAndPath, err := fs.getFile(newName, false)
	if newFileAndPath != nil {
		log.Errorf("file already exists %v", newName)
		return os.ErrExist
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	oldName = strings.TrimSuffix(oldName, "/")
	newName = strings.TrimSuffix(newName, "/")

	if path.Dir(oldName) != path.Dir(newName) {
		log.Panicf("dir change not implemented %v -> %v", oldName, newName)
	}
	fileAndPath, err := fs.getFile(oldName, false)
	if err != nil {
		return err
	}

	file := drive.File{}
	file.Name = path.Base(newName)
	log.Tracef("Files.Update %v %v", fileAndPath.path, file)
	u := fs.client.Files.Update(fileAndPath.file.Id, &file)
	_, err = u.Do()
	if err != nil {
		log.Error(err)
		return err
	}
	fs.invalidatePath(newName)
	fs.invalidatePath(oldName)
	return nil
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
}

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
	q.Fields("files(id, name, appProperties, mimeType, size, modifiedTime, createdTime)")
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

	log.Errorf("Can't file file %v", p)
	return nil, os.ErrNotExist
}

func (fs *fileSystem) readdir(file *drive.File) ([]os.FileInfo, error) {
	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents", file.Id)
	log.Tracef("Query: %v", q)
	q.Q(query)

	r, err := q.Do()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	files := make([]os.FileInfo, len(r.Files))
	for i := range files {
		files[i] = newFileInfo(r.Files[i])
	}
	return files, nil
}
