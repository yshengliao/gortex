package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/pkg/auth"
)

func TestNewJWTServiceRejectsShortSecret(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secret string
	}{
		{"empty", ""},
		{"one byte", "a"},
		{"31 bytes", strings.Repeat("a", auth.MinJWTSecretBytes-1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := auth.NewJWTService(tc.secret, time.Hour, time.Hour, "issuer")
			require.ErrorIs(t, err, auth.ErrJWTSecretTooShort)
			assert.Nil(t, svc)
		})
	}
}

func TestNewJWTServiceAcceptsMinLengthSecret(t *testing.T) {
	secret := strings.Repeat("k", auth.MinJWTSecretBytes)
	svc, err := auth.NewJWTService(secret, time.Hour, time.Hour, "issuer")
	require.NoError(t, err)
	require.NotNil(t, svc)
}

func TestMinJWTSecretBytesIs32(t *testing.T) {
	assert.Equal(t, 32, auth.MinJWTSecretBytes)
}
