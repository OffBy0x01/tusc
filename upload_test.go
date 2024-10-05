package tusc

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEncodedMetadata(t *testing.T) {
	fingerprint := "fingerprint-TestEncodedMetadata"
	u, err := NewUploadFromBytes([]byte(""), &fingerprint)
	assert.Nil(t, err)
	u.Metadata["filename"] = "foobar.txt"
	enc := u.EncodedMetadata()
	assert.Equal(t, "filename Zm9vYmFyLnR4dA==", enc)
}

func TestNewUploadFromFile(t *testing.T) {
	file := fmt.Sprintf("%s/%d", os.TempDir(), time.Now().Unix())

	f, err := os.Create(file)
	assert.Nil(t, err)

	err = f.Truncate(1048576) // 1 MB
	assert.Nil(t, err)

	fingerprint := "fingerprint-TestNewUploadFromFile"
	u, err := NewUploadFromFile(f, &fingerprint)
	assert.Nil(t, err)
	assert.NotNil(t, u)
	assert.EqualValues(t, 1048576, u.Size())
}
