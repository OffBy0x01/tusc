package tusc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfigChunkSizeZero(t *testing.T) {
	c := Config{
		ChunkSizeBytes: 0,
	}

	assert.NotNil(t, c.ValidateAndSetDefaults())
}

func TestConfigDefaultValid(t *testing.T) {
	c := DefaultConfig()
	assert.Nil(t, c.ValidateAndSetDefaults())
}
