package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveMaxMultipartBytesDefault(t *testing.T) {
	SetDefaultMaxMultipartBytes(0) // ensure clean state
	t.Cleanup(func() { SetDefaultMaxMultipartBytes(0) })

	assert.Equal(t, DefaultMaxMultipartBytes, effectiveMaxMultipartBytes())
}

func TestSetDefaultMaxMultipartBytesOverridesDefault(t *testing.T) {
	t.Cleanup(func() { SetDefaultMaxMultipartBytes(0) })

	SetDefaultMaxMultipartBytes(1 << 20)
	assert.Equal(t, int64(1<<20), effectiveMaxMultipartBytes())

	SetDefaultMaxMultipartBytes(-1) // negative/zero restores default
	assert.Equal(t, DefaultMaxMultipartBytes, effectiveMaxMultipartBytes())
}
