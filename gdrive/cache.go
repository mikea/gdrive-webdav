package gdrive

import (
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	cacheKeyFile = "file:"
	cachePeriod  = time.Minute
)

func (fs *fileSystem) invalidatePath(p string) {
	log.Tracef("invalidatePath %v", p)
	fs.cache.Delete(cacheKeyFile + p)
}

type fileLookupResult struct {
	fp  *fileAndPath
	err error
}

func (fs *fileSystem) getFile(p string, onlyFolder bool) (*fileAndPath, error) {
	log.Tracef("getFile %v %v", p, onlyFolder)
	p = strings.TrimSuffix(p, "/")
	key := cacheKeyFile + p

	if lookup, found := fs.cache.Get(key); found {
		log.Trace("Reusing cached file: ", p)
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
