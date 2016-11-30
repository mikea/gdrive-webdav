package webdav

// Filesystem interface

import (
	"io"
	"net/http"
)

type StatusCode int

type MkColStatusCode StatusCode

const (
	MkColCreated              MkColStatusCode = 201
	MkColForbidden                            = 403
	MkColMethodNotAllowed                     = 405
	MkColConflict                             = 409
	MkColUnsupportedMediaType                 = 415
	MkColUnknownError                         = 500
	MkColInsufficientStorage                  = 507
)

type CopyStatusCode StatusCode

const (
	CopyCreated             CopyStatusCode = 201
	CopyNoContent                          = 204
	CopyMultiStatus                        = 207
	CopyForbidden                          = 403
	CopyNotFound                           = 404
	CopyConflict                           = 409
	CopyPreconditionFailed                 = 412
	CopyLocked                             = 423
	CopyUnknownError                       = 500
	CopyBadGateway                         = 502
	CopyInsufficientStorage                = 507
)

type MoveStatusCode StatusCode

const (
	MoveCreated            MoveStatusCode = 201
	MoveNoContent                         = 204
	MoveMultiStatus                       = 207
	MoveForbidden                         = 403
	MoveConflict                          = 409
	MovePreconditionFailed                = 412
	MoveLocked                            = 423
	MoveBadGateway                        = 502
)

type DeleteStatusCode StatusCode

const (
	DeleteDeleted      DeleteStatusCode = 200
	DeleteNotFound                      = 404
	DeleteUnknownError                  = 500
)

type FileSystem interface {
	MkDir(path string) MkColStatusCode
	Delete(path string) DeleteStatusCode
	Put(path string, content io.ReadCloser) StatusCode
	Get(path string) (StatusCode, io.ReadCloser, int64)
	PropList(path string, depth int, props []string) (StatusCode, map[string][]PropertyValue)
	Copy(from string, to string, depth int, overwrite bool) CopyStatusCode
	Move(from string, to string, overwrite bool) MoveStatusCode
}

type PropertyValue interface {
	xmlSerializable
}

type GetContentLengthPropertyValue uint64
type GetLastModifiedPropertyValue uint64
type CreationDatePropertyValue uint64
type DisplayNamePropertyValue string
type GetContentTypePropertyValue string
type GetEtagPropertyValue string
type QuotaAvailableBytesPropertyValue uint64
type QuotaUsedBytesPropertyValue uint64
type QuotaPropertyValue uint64
type QuotaUsedPropertyValue uint64
type ResourceTypePropertyValue bool // true for directories, false otherwise

// Handler creates a new webdav handler given the filesystem.
func Handler(fs FileSystem) func(http.ResponseWriter, *http.Request) {
	h := &handler{fs: fs}
	return func(w http.ResponseWriter, r *http.Request) {
		h.handle(w, r)
	}
}
