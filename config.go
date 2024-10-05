package tusc

import (
	"hash"
	"net/http"
)

type Config struct {
	// ChunkSizeBytes max bytes in chunked file upload
	ChunkSizeBytes int64
	// HTTPMethodOverrides alternate HTTP Methods to be used in case of environmental restrictions
	HTTPMethodOverrides *map[string]string
	// Header custom header values used in all requests e.g. auth
	Header http.Header
	// HttpClient used for all requests e.g. to add proxy
	HttpClient *http.Client
	// Store kv store mapping upload fingerprint to url
	Store Store
	// ChecksumName [optional] Common name of algorithm to use. If set, ChecksumAlgFunc must also be set.
	ChecksumAlg string
	// ChecksumFunc [optional] hash.Hash function to use. If set, ChecksumAlgName must also be set.
	ChecksumFunc *hash.Hash
}

func DefaultConfig() *Config {
	return &Config{
		ChunkSizeBytes:      5 * 1024 * 1024,
		HTTPMethodOverrides: nil,
		Header:              make(http.Header),
		Store:               NewMemoryStore(),
		HttpClient:          &http.Client{},
	}
}

func (c *Config) ValidateAndSetDefaults() error {
	if c.ChunkSizeBytes <= 1 {
		return ErrChuckSize
	}

	if c.HTTPMethodOverrides != nil {
		for k, _ := range *c.HTTPMethodOverrides {
			if k != "PATCH" && k != "DELETE" {
				return ErrBadMethodOverride
			}
		}
	}

	if c.Store == nil {
		return ErrNilStore
	}

	// looks weird but just an xor in the absence of ^ operator
	if (c.ChecksumFunc == nil) != (c.ChecksumAlg == "") {
		return ErrChecksumSetup
	}

	if c.Header == nil {
		c.Header = make(http.Header)
	}

	if c.HttpClient == nil {
		c.HttpClient = &http.Client{}
	}

	return nil
}
