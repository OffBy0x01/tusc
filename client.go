package tusc

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	netUrl "net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	ProtocolVersion = "1.0.0"
)

type Option struct {
	// Extensions
	concatenation       bool
	checksum            bool
	checksumTrailer     bool
	creation            bool
	creationDeferLength bool
	creationWithUpload  bool
	expiration          bool
	// max upload size
	maxSizeBytes int64
}

type Client struct {
	Version string
	Config  *Config
	BaseUrl string

	// server-reported/controlled settings
	Option *Option
}

func NewClient(_baseUrl string, _config *Config) (*Client, error) {
	if _baseUrl == "" {
		return nil, errors.New("BaseUrl cannot be empty")
	}
	if _config == nil {
		_config = DefaultConfig()
	} else if err := _config.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	client := &Client{
		Config:  _config,
		BaseUrl: _baseUrl,
		Version: ProtocolVersion,
	}

	if err := client.options(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Do(_req *http.Request) (*http.Response, error) {
	for k, v := range c.Config.Header {
		_req.Header[k] = v
	}

	_req.Header.Set("Tus-Resumable", ProtocolVersion)

	if c.Config.HTTPMethodOverrides == nil {
		return c.Config.HttpClient.Do(_req)
	} else if method, ok := (*c.Config.HTTPMethodOverrides)[_req.Method]; ok {
		_req.Header.Set("X-HTTP-Method-Override", _req.Method)
		_req.Method = method
	}

	return c.Config.HttpClient.Do(_req)
}

func (c *Client) options() error {
	req, err := http.NewRequest(http.MethodOptions, c.BaseUrl, nil)
	if err != nil {
		return err
	}

	res, err := c.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		slog.Warn("options unsupported or unreachable, extensions may fail without warning")
		return nil
	}

	if tusVersions := res.Header.Get("Tus-Versions"); tusVersions != "" && !slices.Contains(commaSplitTrim(tusVersions), ProtocolVersion) {
		return fmt.Errorf("unsupported tus version: '%s', server supports: %s", ProtocolVersion, tusVersions)
	}

	c.Option = &Option{}
	extensions := commaSplitTrim(res.Header.Get("Tus-Extension"))
	for _, extension := range extensions {
		switch strings.ToLower(extension) {
		case "concatenation":
			c.Option.concatenation = true
		case "creation":
			c.Option.creation = true
		case "creation-defer-length":
			c.Option.creationDeferLength = true
		case "creation-with-upload":
			c.Option.creationWithUpload = true
		case "expiration":
			c.Option.expiration = true
		case "checksum":
			if c.Config.ChecksumAlg == "" {
				continue
			}
			algorithms := commaSplitTrim(res.Header.Get("Tus-Checksum-Algorithm"))
			if !slices.Contains(algorithms, c.Config.ChecksumAlg) {
				return errors.New("ChecksumAlgName " + c.Config.ChecksumAlg + " not supported")
			}
			c.Option.checksum = true
		case "checksum-trailer":
			c.Option.checksumTrailer = true
		case "termination":
		default:
			return errors.New("unknown extension: " + extension)
		}
	}

	if maxSizeBytes := res.Header.Get("Tus-Max-Size"); maxSizeBytes != "" {
		c.Option.maxSizeBytes, err = strconv.ParseInt(maxSizeBytes, 10, 64)
		return err
	}

	return nil
}

func commaSplitTrim(_s string) []string {
	re := regexp.MustCompile(`\s*,+\s*`)
	parts := re.Split(_s, -1)
	var split []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			split = append(split, trimmed)
		}
	}
	return split
}

func (c *Client) CreateUpload(_upload *Upload) (*UploadMgr, error) {
	if c.Option != nil && !c.Option.creation {
		return nil, ErrExtensionNotAvailable
	}
	if _upload == nil {
		return nil, ErrNilUpload
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Length", "0")
	req.Header.Set("Upload-Length", strconv.FormatInt(_upload.size, 10))
	req.Header.Set("Upload-Metadata", _upload.EncodedMetadata())

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusCreated:
		location := res.Header.Get("Location")

		url, err := c.resolveLocationURL(location)
		if err != nil {
			return nil, err
		}

		c.Config.Store.Set(_upload.Fingerprint, url.String())

		return NewUploadMgr(c, url.String(), _upload, 0)
	case http.StatusPreconditionFailed:
		return nil, ErrVersionMismatch
	case http.StatusRequestEntityTooLarge:
		return nil, ErrLargeUpload
	default:
		return nil, newClientError(res)
	}
}

func (c *Client) getUploadOffset(_url string) (int64, error) {
	req, err := http.NewRequest("HEAD", _url, nil)
	if err != nil {
		return 0, err
	}

	res, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		i, err := strconv.ParseInt(res.Header.Get("Upload-Offset"), 10, 64)
		if err == nil {
			return i, nil
		}
		return 0, err
	case http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		// upload doesn't exist
		return 0, ErrUploadNotFound
	case http.StatusPreconditionFailed:
		return 0, ErrVersionMismatch
	default:
		return 0, newClientError(res)
	}
}

func (c *Client) ResumeUpload(_upload *Upload) (*UploadMgr, error) {
	if _upload == nil {
		return nil, ErrNilUpload
	}
	if len(_upload.Fingerprint) == 0 {
		return nil, ErrFingerprintUnset
	}
	url, found := c.Config.Store.Get(_upload.Fingerprint)
	if !found {
		return nil, ErrUploadNotFound
	}

	offset, err := c.getUploadOffset(url)
	if err != nil {
		return nil, err
	}

	return NewUploadMgr(c, url, _upload, offset)
}

// CreateOrResumeUpload resumes the upload if already created or creates a new upload in the server.
func (c *Client) CreateOrResumeUpload(_upload *Upload) (*UploadMgr, error) {
	if _upload == nil {
		return nil, ErrNilUpload
	}

	uploadMgr, err := c.ResumeUpload(_upload)

	if err == nil {
		return uploadMgr, err
	} else if errors.Is(err, ErrUploadNotFound) {

		return c.CreateUpload(_upload)
	}

	return nil, err
}

func (c *Client) uploadChunk(_url string, _buf io.Reader, _checksum string, _size int64, _offset int64) (int64, error) {

	req, err := http.NewRequest(http.MethodPatch, _url, _buf)
	if err != nil {
		return -1, err
	}

	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Content-Length", strconv.FormatInt(_size, 10))
	req.Header.Set("Upload-Offset", strconv.FormatInt(_offset, 10))
	if c.Option != nil && c.Option.checksum {
		req.Header.Set("Tus-Checksum-Algorithm", _checksum)
	}

	res, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusNoContent:
		if offset, err := strconv.ParseInt(res.Header.Get("Upload-Offset"), 10, 64); err == nil {
			return offset, nil
		}
		return 0, err

	case http.StatusConflict:
		return 0, ErrOffsetMismatch
	case http.StatusPreconditionFailed:
		return 0, ErrVersionMismatch
	case http.StatusRequestEntityTooLarge:
		return 0, ErrLargeUpload
	default:
		return 0, newClientError(res)
	}
}

func (c *Client) resolveLocationURL(location string) (*netUrl.URL, error) {
	baseURL, err := netUrl.Parse(c.BaseUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid Base URL '%s'", c.BaseUrl)
	}

	locationURL, err := netUrl.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("invalid Location header value for Creation '%s': %s", location, err.Error())
	}
	newURL := baseURL.ResolveReference(locationURL)
	return newURL, nil
}

func newClientError(res *http.Response) error {
	body, _ := io.ReadAll(res.Body)
	return fmt.Errorf("%d: %s", res.StatusCode, string(body))
}
