package webdav

// Filesystem interface

import (
	"io"
	"net/http"
)

// StatusCode is HTTP result code for all webdav operations
type StatusCode int

// MkColStatusCode is status code for MkCol operation
type MkColStatusCode StatusCode

// Standard MkCol errors
const (
	MkColCreated              MkColStatusCode = 201
	MkColForbidden                            = 403
	MkColMethodNotAllowed                     = 405
	MkColConflict                             = 409
	MkColUnsupportedMediaType                 = 415
	MkColUnknownError                         = 500
	MkColInsufficientStorage                  = 507
)

// CopyStatusCode is the status code for Copy
type CopyStatusCode StatusCode

// Copy status codes
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

// MoveStatusCode is the status code for Move
type MoveStatusCode StatusCode

// Move status codes
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

// DeleteStatusCode is the staus code for Delete
type DeleteStatusCode StatusCode

// Delete stauts codes
const (
	DeleteDeleted      DeleteStatusCode = 200
	DeleteNotFound                      = 404
	DeleteUnknownError                  = 500
)

// FileSystem is interface for webdav file system implementation
type FileSystem interface {
	MkDir(path string) MkColStatusCode
	Delete(path string) DeleteStatusCode
	Put(path string, content io.ReadCloser) StatusCode
	Get(path string) (StatusCode, io.ReadCloser, int64)
	PropList(path string, depth int, props []string) (StatusCode, map[string][]PropertyValue)
	Copy(from string, to string, depth int, overwrite bool) CopyStatusCode
	Move(from string, to string, overwrite bool) MoveStatusCode
}

// PropertyValue interface for properties
type PropertyValue interface {
	xmlSerializable
}

// GetContentLengthPropertyValue tag type
type GetContentLengthPropertyValue uint64

// GetLastModifiedPropertyValue tag type
type GetLastModifiedPropertyValue uint64

// CreationDatePropertyValue tag type
type CreationDatePropertyValue uint64

// DisplayNamePropertyValue tag type
type DisplayNamePropertyValue string

// GetContentTypePropertyValue tag type
type GetContentTypePropertyValue string

// GetEtagPropertyValue tag type
type GetEtagPropertyValue string

// QuotaAvailableBytesPropertyValue tag type
type QuotaAvailableBytesPropertyValue uint64

// QuotaUsedBytesPropertyValue tag type
type QuotaUsedBytesPropertyValue uint64

// QuotaPropertyValue tag type
type QuotaPropertyValue uint64

// QuotaUsedPropertyValue tag type
type QuotaUsedPropertyValue uint64

// ResourceTypePropertyValue tag type
type ResourceTypePropertyValue bool // true for directories, false otherwise

// Handler creates a new webdav handler given the filesystem.
func Handler(fs FileSystem) func(http.ResponseWriter, *http.Request) {
	h := &handler{fs: fs}
	return func(w http.ResponseWriter, r *http.Request) {
		h.handle(w, r)
	}
}
