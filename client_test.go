package tusc

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	netUrl "net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/tus/tusd/pkg/filestore"
	tusd "github.com/tus/tusd/pkg/handler"
)

type UploadTestSuite struct {
	suite.Suite

	ts    *httptest.Server
	store filestore.FileStore
	url   string
}

func (s *UploadTestSuite) SetupSuite() {
	store := filestore.FileStore{
		Path: os.TempDir(),
	}

	composer := tusd.NewStoreComposer()

	store.UseIn(composer)

	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:                "/uploads/",
		StoreComposer:           composer,
		MaxSize:                 0,
		NotifyCompleteUploads:   false,
		NotifyTerminatedUploads: false,
		RespectForwardedHeaders: true,
	})

	if err != nil {
		panic(err)
	}

	s.store = store
	s.ts = httptest.NewServer(http.StripPrefix("/uploads/", handler))
	s.url = fmt.Sprintf("%s/uploads/", s.ts.URL)
}

func (s *UploadTestSuite) TearDownSuite() {
	s.ts.Close()
}

func (s *UploadTestSuite) TestSmallUploadFromFile() {
	const exampleFileSize = 1024
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file := fmt.Sprintf("%s/%d", os.TempDir(), time.Now().Unix())

	f, err := os.Create(file)
	s.Nil(err)

	defer f.Close()

	err = f.Truncate(exampleFileSize) // 1 MB
	s.Nil(err)

	client, err := NewClient(s.url, nil)
	s.Nil(err)

	fingerprint := "fingerprint-TestSmallUploadFromFile"
	upload, err := NewUploadFromFile(f, &fingerprint)
	s.Nil(err)

	uploadMgr, err := client.CreateUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)

	err = uploadMgr.Upload()
	s.Nil(err)

	up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploadMgr.url))
	s.Nil(err)

	fi, err := up.GetInfo(ctx)
	s.Nil(err)

	s.EqualValues(exampleFileSize, fi.Size)
}

func (s *UploadTestSuite) TestLargeUpload() {
	const exampleFileSize = 1024 * 1024 * 150
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file := fmt.Sprintf("%s/%d", os.TempDir(), time.Now().Unix())

	f, err := os.Create(file)
	s.Nil(err)

	defer f.Close()

	err = f.Truncate(exampleFileSize) // 150 MB
	s.Nil(err)

	client, err := NewClient(s.url, nil)
	s.Nil(err)

	fingerprint := "fingerprint-TestLargeUpload"
	upload, err := NewUploadFromFile(f, &fingerprint)
	s.Nil(err)

	uploadMgr, err := client.CreateUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)

	err = uploadMgr.Upload()
	s.Nil(err)

	up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploadMgr.url))
	s.Nil(err)

	fi, err := up.GetInfo(ctx)
	s.Nil(err)

	s.EqualValues(exampleFileSize, fi.Size)
}

func (s *UploadTestSuite) TestUploadFromBytes() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := NewClient(s.url, nil)
	s.Nil(err)

	fingerprint := "fingerprint-TestUploadFromBytes"
	upload, err := NewUploadFromBytes([]byte("1234567890"), &fingerprint)
	s.Nil(err)

	uploadMgr, err := client.CreateUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)

	err = uploadMgr.Upload()
	s.Nil(err)

	up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploadMgr.url))
	s.Nil(err)

	fi, err := up.GetInfo(ctx)
	s.Nil(err)

	s.EqualValues(10, fi.Size)
}

func (s *UploadTestSuite) TestOverridePatchMethod() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := NewClient(s.url, nil)
	s.Nil(err)

	client.Config.HTTPMethodOverrides = &map[string]string{
		http.MethodPatch: http.MethodPost,
	}
	fingerprint := "fingerprint-TestOverridePatchMethod"
	upload, err := NewUploadFromBytes([]byte("1234567890"), &fingerprint)
	s.Nil(err)

	uploadMgr, err := client.CreateUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)

	err = uploadMgr.Upload()
	s.Nil(err)

	up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploadMgr.url))
	s.Nil(err)

	fi, err := up.GetInfo(ctx)
	s.Nil(err)

	s.EqualValues(10, fi.Size)
}

func (s *UploadTestSuite) TestResumeUpload() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handle := fmt.Sprintf("%s/%d", os.TempDir(), time.Now().Unix())
	f, err := os.Create(handle)
	s.Nil(err)

	defer f.Close()
	const uploadFileSize = 1024 * 1024 * 500
	err = f.Truncate(uploadFileSize)
	s.Nil(err)

	cfg := &Config{
		ChunkSizeBytes: 5 * 1024 * 1024,
		Store:          NewMemoryStore(),
		Header: map[string][]string{
			"X-Extra-Header": []string{"some value"},
		},
	}

	client, err := NewClient(s.url, cfg)
	s.Nil(err)

	fingerprint := "fingerprint-TestResumeUpload"
	upload, err := NewUploadFromFile(f, &fingerprint)
	s.Nil(err)
	s.NotNil(upload)

	uploadMgr, err := client.CreateUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)
	s.NotNil(upload)

	// This will stop the first upload.
	go func() {
		time.Sleep(250 * time.Millisecond)
		uploadMgr.Abort()
	}()
	// this test will fail if the upload completes too quickly, thus we use a (relatively) huge 1GB file
	s.Equalf(false, uploadMgr.aborted, "Expected uploadMgr.aborted to be %v but got %v", false, uploadMgr.aborted)

	err = uploadMgr.Upload()
	s.Equalf(nil, err, "Expected uploadMgr.Upload() to be %v but got %v", nil, err)

	uploadPercentage := uploadMgr.upload.Progress()
	// if upload percentage is 100, aborting is impossible, so this test is run first
	s.NotEqualf(100, uploadPercentage, "Expected upload percentage to be < 100 but got %v", uploadPercentage)
	s.Equalf(true, uploadMgr.aborted, "Expected uploadMgr.aborted to be %v but got %v", true, uploadMgr.aborted)

	uploadMgr, err = client.ResumeUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)

	err = uploadMgr.Upload()
	s.Nil(err)

	up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploadMgr.url))
	s.Nil(err)

	fi, err := up.GetInfo(ctx)
	s.Nil(err)

	s.EqualValues(uploadFileSize, fi.Size)
}

func (s *UploadTestSuite) TestUploadLocation() {
	client, err := NewClient(s.url, nil)
	s.Nil(err)
	sourceURL, err := netUrl.Parse(s.url)
	s.Nil(err)

	s.T().Run("Location is a full URL", func(t *testing.T) {
		location := "https://offby0x01.xyz/upload/123"
		resourceURL, err := client.resolveLocationURL(location)
		s.Nil(err)
		s.EqualValues(location, resourceURL.String())
	})

	s.T().Run("Location is a URL without scheme", func(t *testing.T) {
		location := "//offby0x01.xyz/upload/123"
		resourceURL, err := client.resolveLocationURL(location)
		s.Nil(err)
		s.EqualValues(sourceURL.Scheme+":"+location, resourceURL.String())
	})

	s.T().Run("Location is an absolute path", func(t *testing.T) {
		location := "/upload/123"
		resourceURL, err := client.resolveLocationURL(location)
		s.Nil(err)
		s.EqualValues(sourceURL.Scheme+"://"+sourceURL.Host+location, resourceURL.String())
	})

	s.T().Run("Location is a relative path", func(t *testing.T) {
		location := "somewhere/123"
		resourceURL, err := client.resolveLocationURL(location)
		s.Nil(err)
		s.EqualValues(sourceURL.Scheme+"://"+sourceURL.Host+path.Join(sourceURL.Path, location), resourceURL.String())
	})

}

func (s *UploadTestSuite) TestConcurrentUploads() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	client, err := NewClient(s.url, nil)
	s.Nil(err)

	const uploadFileSize = 1024 * 1024 * 50 // 50 MB

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			// if this runs too quickly, duplicate handles can be generated so also use i
			handle := fmt.Sprintf("%s/%d%d", os.TempDir(), i, time.Now().UnixNano())

			file, err := os.Create(handle)
			s.Nil(err)

			defer file.Close()

			err = file.Truncate(uploadFileSize)
			s.Nil(err)

			fingerprint := "fingerprint-TestConcurrentUploads-" + strconv.Itoa(i)
			upload, err := NewUploadFromFile(file, &fingerprint)
			s.Nil(err)

			uploader, err := client.CreateUpload(upload)
			s.Nil(err)
			s.NotNil(uploader)

			err = uploader.Upload()
			s.Nil(err)

			up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploader.url))
			s.Nil(err)

			fi, err := up.GetInfo(ctx)
			s.Nil(err)

			s.EqualValues(uploadFileSize, fi.Size)
		}()
	}

	wg.Wait()
}

func (s *UploadTestSuite) TestUploadWithChecksum() {
	const exampleFileSize = 1024 * 1024 * 50
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file := fmt.Sprintf("%s/%d", os.TempDir(), time.Now().Unix())

	f, err := os.Create(file)
	s.Nil(err)

	defer f.Close()

	err = f.Truncate(exampleFileSize)
	s.Nil(err)

	hasher := sha1.New()
	config := &Config{
		ChunkSizeBytes: 5 * 1024 * 1024,
		Store:          NewMemoryStore(),
		ChecksumAlg:    "sha1",
		ChecksumFunc:   &hasher,
	}

	client, err := NewClient(s.url, config)
	s.Nil(err)

	fingerprint := "fingerprint-TestUploadWithChecksum"
	upload, err := NewUploadFromFile(f, &fingerprint)
	s.Nil(err)

	uploadMgr, err := client.CreateUpload(upload)
	s.Nil(err)
	s.NotNil(uploadMgr)

	err = uploadMgr.Upload()
	s.Nil(err)

	up, err := s.store.GetUpload(ctx, uploadIDFromURL(uploadMgr.url))
	s.Nil(err)

	fi, err := up.GetInfo(ctx)
	s.Nil(err)

	s.EqualValues(exampleFileSize, fi.Size)
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}

func uploadIDFromURL(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
