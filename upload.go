package tusc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

type Metadata map[string]string

type Upload struct {
	stream io.ReadSeeker
	size   int64
	offset int64

	Fingerprint string
	Metadata    Metadata
}

func NewUpload(_stream io.ReadSeeker, _size int64, _metadata Metadata, _fingerprint *string) (*Upload, error) {
	_, err := _stream.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	if _metadata == nil {
		_metadata = make(Metadata)
	}

	return &Upload{
		stream:      _stream,
		size:        _size,
		Fingerprint: *_fingerprint,
		Metadata:    _metadata,
	}, nil
}

func NewUploadFromFile(_file *os.File, _fingerprint *string) (*Upload, error) {
	if _fingerprint == nil {
		return nil, ErrFingerprintUnset
	}
	fileInfo, err := _file.Stat()
	if err != nil {
		return nil, err
	}

	metadata := Metadata{
		"filename": fileInfo.Name(),
	}

	return NewUpload(_file, fileInfo.Size(), metadata, _fingerprint)
}

func NewUploadFromBytes(_bytes []byte, _fingerprint *string) (*Upload, error) {
	if _fingerprint == nil {
		return nil, ErrFingerprintUnset
	}
	if _bytes == nil {
		return nil, ErrNilUpload
	}
	buffer := bytes.NewReader(_bytes)
	_, err := buffer.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return NewUpload(buffer, buffer.Size(), nil, _fingerprint)
}

func (u *Upload) setOffset(offset int64) {
	u.offset = offset
}

func (u *Upload) Offset() int64 {
	return u.offset
}

func (u *Upload) Size() int64 {
	return u.size
}

// Progress of the current upload as percentage
func (u *Upload) Progress() int64 {
	if u.size == 0 {
		return 100
	}
	return (u.offset * 100) / u.size
}

func (u *Upload) EncodedMetadata() string {
	var encoded []string

	for k, v := range u.Metadata {
		encoded = append(encoded, fmt.Sprintf("%s %s", k, b64encode(v)))
	}

	return strings.Join(encoded, ",")
}

func b64encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
