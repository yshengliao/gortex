package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yshengliao/gortex/pkg/auth"
)

func newTestService(t *testing.T) *auth.JWTService {
	t.Helper()
	svc, err := auth.NewJWTService(testSecret, time.Hour, 24*time.Hour, "test-issuer")
	require.NoError(t, err)
	return svc
}

// A refresh token must not validate as an access token: the two share a
// secret and algorithm, so only the "typ" claim keeps a 7-day refresh token
// from being replayed against access-protected routes.
func TestRefreshTokenRejectedByValidateToken(t *testing.T) {
	svc := newTestService(t)

	refresh, err := svc.GenerateRefreshToken("user123")
	require.NoError(t, err)

	_, err = svc.ValidateToken(refresh)
	assert.ErrorIs(t, err, auth.ErrInvalidTokenType)
}

// An access token must not be accepted where a refresh token is expected,
// otherwise it could be used to indefinitely mint fresh access tokens.
func TestAccessTokenRejectedByRefreshFlows(t *testing.T) {
	svc := newTestService(t)

	access, err := svc.GenerateAccessToken("user123", "alice", "a@example.test", "member")
	require.NoError(t, err)

	t.Run("ValidateRefreshToken", func(t *testing.T) {
		_, err := svc.ValidateRefreshToken(access)
		assert.ErrorIs(t, err, auth.ErrInvalidTokenType)
	})

	t.Run("RefreshAccessToken", func(t *testing.T) {
		_, err := svc.RefreshAccessToken(access, func(string) (string, string, string, error) {
			t.Fatal("getUserInfo must not be called for a non-refresh token")
			return "", "", "", nil
		})
		assert.ErrorIs(t, err, auth.ErrInvalidTokenType)
	})
}

// Game tokens are access-class and must keep validating through ValidateToken.
func TestGameTokenValidatesAsAccess(t *testing.T) {
	svc := newTestService(t)

	game, err := svc.GenerateGameToken("user123", "alice", "game001")
	require.NoError(t, err)

	claims, err := svc.ValidateToken(game)
	require.NoError(t, err)
	assert.Equal(t, "game001", claims.GameID)

	// And it must NOT be usable as a refresh token.
	_, err = svc.ValidateRefreshToken(game)
	assert.ErrorIs(t, err, auth.ErrInvalidTokenType)
}

// signWith re-signs a set of claims with an arbitrary method/key so the
// algorithm-confusion paths can be exercised.
func signWith(t *testing.T, method jwt.SigningMethod, key any, typ string) string {
	t.Helper()
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "test-issuer",
			Subject:   "user123",
		},
		TokenType: typ,
		UserID:    "user123",
	}
	tok := jwt.NewWithClaims(method, claims)
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

// The keyfunc must pin HS256 exactly. A token whose header advertises a
// different HMAC variant (HS384/HS512), "none", or an asymmetric method must
// be rejected even though the secret/claims are otherwise valid.
func TestValidateTokenRejectsAlgorithmConfusion(t *testing.T) {
	svc := newTestService(t)

	t.Run("alg none", func(t *testing.T) {
		// jwt.UnsafeAllowNoneSignatureType is the sentinel key the library
		// requires to emit an unsigned token.
		token := signWith(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "access")
		_, err := svc.ValidateToken(token)
		assert.ErrorContains(t, err, "unexpected signing method")
	})

	t.Run("HS384 same secret", func(t *testing.T) {
		token := signWith(t, jwt.SigningMethodHS384, []byte(testSecret), "access")
		_, err := svc.ValidateToken(token)
		assert.ErrorContains(t, err, "unexpected signing method")
	})

	t.Run("HS512 same secret", func(t *testing.T) {
		token := signWith(t, jwt.SigningMethodHS512, []byte(testSecret), "access")
		_, err := svc.ValidateToken(token)
		assert.ErrorContains(t, err, "unexpected signing method")
	})
}

// The same pinning must hold for the refresh-token keyfunc.
func TestValidateRefreshTokenRejectsAlgorithmConfusion(t *testing.T) {
	svc := newTestService(t)

	none := signWith(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "refresh")
	_, err := svc.ValidateRefreshToken(none)
	assert.ErrorContains(t, err, "unexpected signing method")

	hs384 := signWith(t, jwt.SigningMethodHS384, []byte(testSecret), "refresh")
	_, err = svc.ValidateRefreshToken(hs384)
	assert.ErrorContains(t, err, "unexpected signing method")
}
