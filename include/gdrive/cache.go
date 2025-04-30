package gdrive

import (
	"log/slog"
	"strings"
	"time"
)

const (
	cacheKeyFile = "file:"
	cachePeriod  = time.Minute
)

func (fs *fileSystem) invalidatePath(p string) {
	slog.Debug("invalidate cache path", slog.String("path", p))
	fs.cache.Delete(cacheKeyFile + p)
}

type fileLookupResult struct {
	fp  *fileAndPath
	err error
}

func (fs *fileSystem) getFile(p string, onlyFolder bool) (*fileAndPath, error) {
	slog.Debug("getting file", slog.String("path", p), slog.Bool("only_folder", onlyFolder))
	p = strings.TrimSuffix(p, "/")
	key := cacheKeyFile + p

	if lookup, found := fs.cache.Get(key); found {
		slog.Debug("cache hit", slog.String("path", p))
		result := lookup.(*fileLookupResult)
		return result.fp, result.err
	}

	fp, err := fs.getFile0(p, onlyFolder)
	lookup := &fileLookupResult{fp: fp, err: err}
	if err != nil {
		fs.cache.Set(key, lookup, cachePeriod)
	}
	return lookup.fp, lookup.err
}
