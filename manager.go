package tusc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
)

type UploadMgr struct {
	client     *Client
	url        string
	upload     *Upload
	offset     int64
	aborted    bool
	uploadSubs []chan Upload
	notifyChan chan bool
}

func NewUploadMgr(_client *Client, _url string, _upload *Upload, _offset int64) (*UploadMgr, error) {
	notifyChan := make(chan bool)

	uploadMgr := &UploadMgr{
		client:     _client,
		url:        _url,
		upload:     _upload,
		offset:     _offset,
		aborted:    false,
		uploadSubs: nil,
		notifyChan: notifyChan,
	}

	go uploadMgr.broadcast()

	return uploadMgr, nil
}

func (um *UploadMgr) broadcast() {
	for range um.notifyChan {
		for _, c := range um.uploadSubs {
			c <- *um.upload
		}
	}
}

func (um *UploadMgr) Abort() {
	um.aborted = true
}

func (um *UploadMgr) Upload() error {
	// if uploading a file that has already been uploaded, below loop would be skipped
	//   and channel would never be notified that it is (already) completed. This ensures
	//   the manager is always notified of a success.
	if um.upload.size == um.offset {
		um.upload.setOffset(um.offset)
		um.notifyChan <- true
		return nil
	}

	for um.offset < um.upload.size && !um.aborted {
		err := um.UploadChunk()

		if err != nil {
			return err
		}
	}

	return nil
}

func (um *UploadMgr) UploadChunk() error {
	_, err := um.upload.stream.Seek(um.offset, io.SeekStart)
	if err != nil {
		return err
	}

	buf := make([]byte, um.client.Config.ChunkSizeBytes)
	size, err := um.upload.stream.Read(buf)
	if err != nil {
		return err
	}

	// buffer size may exceed bytes read, so cap to read size
	checksum, err := um.Checksum(buf[:size])

	body := bytes.NewBuffer(buf[:size])

	offset, err := um.client.uploadChunk(um.url, body, checksum, int64(size), um.offset)
	if err != nil {
		slog.Warn("Unexpected error while uploading chunk: %v", err)
		return err
	}

	um.offset = offset
	um.upload.setOffset(offset)
	um.notifyChan <- true

	return nil
}

func (um *UploadMgr) Checksum(_bytes []byte) (string, error) {
	if um.client.Config.ChecksumFunc == nil || um.client.Config.ChecksumAlg == "" {
		return "", nil
	}

	_, err := (*um.client.Config.ChecksumFunc).Write(_bytes)
	if err != nil {
		return "", err
	}
	checksum := (*um.client.Config.ChecksumFunc).Sum(nil)
	encoded := base64.StdEncoding.EncodeToString(checksum)
	return fmt.Sprintf("%s %s", um.client.Config.ChecksumAlg, encoded), nil
}

func (um *UploadMgr) Subscribe(upload chan Upload) {
	um.uploadSubs = append(um.uploadSubs, upload)
}
