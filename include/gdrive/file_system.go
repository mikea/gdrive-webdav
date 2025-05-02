package gdrive

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	gocache "github.com/pmylund/go-cache"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
	"google.golang.org/api/drive/v3"
)

type fileSystem struct {
	client *drive.Service
	cache  *gocache.Cache
	logger *slog.Logger
}

func (fs *fileSystem) Mkdir(_ context.Context, name string, perm os.FileMode) error {
	fs.logger.Debug("Mkdir", slog.String("name", name), slog.Int("perm", int(perm)))
	name = normalizePath(name)
	pID, err := fs.getFileID(name, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		fs.logger.Error("dir already exists", slog.String("name", name), slog.String("id", pID))
		return os.ErrExist
	}

	parent := path.Dir(name)
	dir := path.Base(name)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		return err
	}

	if parentID == "" {
		fs.logger.Error("parent not found", slog.String("parent", parent))
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
	fs.logger.Debug("OpenFile", slog.String("name", name), slog.Int("flag", flag), slog.Int("perm", int(perm)))
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

	fs.logger.Error("unsupported open mode", slog.Int("flag", flag))
	return nil, fmt.Errorf("unsupported open mode: %v", flag)
}

func (fs *fileSystem) RemoveAll(_ context.Context, name string) error {
	fs.logger.Debug("RemoveAll", slog.String("name", name))
	name = normalizePath(name)
	id, err := fs.getFileID(name, false)
	if err != nil {
		return err
	}

	err = fs.client.Files.Delete(id).Do()
	if err != nil {
		fs.logger.Error("error deleting file", slog.String("name", name), slog.String("error", err.Error()))
		return err
	}

	fs.invalidatePath(name)
	fs.invalidatePath(path.Dir(name))
	return nil
}

func (fs *fileSystem) Rename(_ context.Context, oldName, newName string) error {
	fs.logger.Debug("Rename", slog.String("old", oldName), slog.String("new", newName))

	newFileAndPath, err := fs.getFile(newName, false)
	if newFileAndPath != nil {
		fs.logger.Error("file already exists", slog.String("name", newName))
		return os.ErrExist
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	oldName = strings.TrimSuffix(oldName, "/")
	newName = strings.TrimSuffix(newName, "/")

	if path.Dir(oldName) != path.Dir(newName) {
		fs.logger.Error("dir change not implemented", slog.String("old", oldName), slog.String("new", newName))
	}
	fileAndPath, err := fs.getFile(oldName, false)
	if err != nil {
		return err
	}

	file := drive.File{}
	file.Name = path.Base(newName)
	fs.logger.Debug("Files.Update", slog.String("path", fileAndPath.path), slog.String("name", file.Name))
	u := fs.client.Files.Update(fileAndPath.file.Id, &file)
	_, err = u.Do()
	if err != nil {
		fs.logger.Error("error updating file", slog.String("name", oldName), slog.String("error", err.Error()))
		return err
	}
	fs.invalidatePath(newName)
	fs.invalidatePath(oldName)
	return nil
}

func (fs *fileSystem) Stat(_ context.Context, name string) (os.FileInfo, error) {
	fs.logger.Debug("Stat", slog.String("name", name))
	f, err := fs.getFile(name, false)

	if err != nil {
		fs.logger.Error("error getting file", slog.String("name", name), slog.String("error", err.Error()))
		return nil, err
	}

	if f == nil {
		fs.logger.Debug("Can't find file ", slog.String("name", name))
		return nil, os.ErrNotExist
	}

	return newFileInfo(f.file, fs.logger), nil
}

func (fs *fileSystem) getFileID(p string, onlyFolder bool) (string, error) {
	f, err := fs.getFile(p, onlyFolder)

	if err != nil {
		return "", err
	}

	return f.file.Id, nil
}

func (fs *fileSystem) getFile0(p string, onlyFolder bool) (*fileAndPath, error) {
	fs.logger.Debug("getFile0", slog.String("path", p), slog.Bool("only_folder", onlyFolder))
	p = normalizePath(p)

	if p == "" {
		f, err := fs.client.Files.Get("root").Do()
		if err != nil {
			fs.logger.Error("error getting root file", slog.String("error", err.Error()))
			return nil, err
		}
		return &fileAndPath{file: f, path: "/"}, nil
	}

	parent := path.Dir(p)
	base := path.Base(p)

	parentID, err := fs.getFileID(parent, true)
	if err != nil {
		fs.logger.Error("can't locate parent", slog.String("parent", parent), slog.String("error", err.Error()))
		return nil, err
	}

	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents and name='%s'", parentID, base)
	if onlyFolder {
		query += " and mimeType='" + mimeTypeFolder + "'"
	}
	q.Q(query)
	q.Fields("files(id, name, appProperties, mimeType, size, modifiedTime, createdTime)")
	fs.logger.Debug("getfile0 query", slog.String("query", query))

	r, err := q.Do()
	if err != nil {
		fs.logger.Error("error listing files", slog.String("error", err.Error()))
		return nil, err
	}

	for _, file := range r.Files {
		if ignoreFile(file) {
			continue
		}
		return &fileAndPath{file: file, path: p}, nil
	}

	fs.logger.Debug("can not get file", slog.String("path", p))
	return nil, os.ErrNotExist
}

func (fs *fileSystem) readdir(file *drive.File) ([]os.FileInfo, error) {
	q := fs.client.Files.List()
	query := fmt.Sprintf("'%s' in parents", file.Id)
	fs.logger.Debug("readdir query", slog.String("query", query))
	q.Q(query)

	r, err := q.Do()
	if err != nil {
		fs.logger.Error("error listing files", slog.String("error", err.Error()))
		return nil, err
	}

	files := make([]os.FileInfo, len(r.Files))
	for i := range files {
		files[i] = newFileInfo(r.Files[i], fs.logger)
	}
	return files, nil
}
