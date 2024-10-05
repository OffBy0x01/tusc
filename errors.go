package tusc

import (
	"errors"
)

var (
	ErrChuckSize             = errors.New("chunk size must be greater than zero")
	ErrNilUpload             = errors.New("upload cannot be nil")
	ErrLargeUpload           = errors.New("upload is too large")
	ErrNilStore              = errors.New("store cannot be nil")
	ErrFingerprintUnset      = errors.New("fingerprint unset")
	ErrVersionMismatch       = errors.New("protocol version mismatch")
	ErrOffsetMismatch        = errors.New("upload offset mismatch")
	ErrUploadNotFound        = errors.New("upload not found")
	ErrBadMethodOverride     = errors.New("only 'patch' and 'delete' method overriding supported")
	ErrExtensionNotAvailable = errors.New("extension not available (server)")
	ErrExtensionNotSupported = errors.New("extension not supported (client)")
	ErrChecksumSetup         = errors.New("ChecksumAlgName is required when ChecksumAlgFunc is set")
)
