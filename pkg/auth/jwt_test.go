package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/pkg/auth"
)

func TestJWTService(t *testing.T) {
	service := auth.NewJWTService(
		"test-secret-key",
		time.Hour,
		24*time.Hour,
		"test-issuer",
	)

	t.Run("GenerateAccessToken", func(t *testing.T) {
		token, err := service.GenerateAccessToken("user123", "testuser", "test@example.com", "player")
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the generated token
		claims, err := service.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "user123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "test@example.com", claims.Email)
		assert.Equal(t, "player", claims.Role)
		assert.Equal(t, "test-issuer", claims.Issuer)
	})

	t.Run("GenerateRefreshToken", func(t *testing.T) {
		token, err := service.GenerateRefreshToken("user123")
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("GenerateGameToken", func(t *testing.T) {
		token, err := service.GenerateGameToken("user123", "testuser", "game001")
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the game token
		claims, err := service.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "user123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "game001", claims.GameID)
	})

	t.Run("ValidateToken_Invalid", func(t *testing.T) {
		_, err := service.ValidateToken("invalid-token")
		assert.Error(t, err)
	})

	t.Run("ValidateToken_ExpiredToken", func(t *testing.T) {
		// Create service with very short TTL
		shortService := auth.NewJWTService(
			"test-secret-key",
			1*time.Nanosecond, // Extremely short TTL
			1*time.Hour,
			"test-issuer",
		)

		token, err := shortService.GenerateAccessToken("user123", "testuser", "test@example.com", "player")
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		_, err = shortService.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("RefreshAccessToken", func(t *testing.T) {
		// Generate refresh token
		refreshToken, err := service.GenerateRefreshToken("user123")
		require.NoError(t, err)

		// Use refresh token to get new access token
		newAccessToken, err := service.RefreshAccessToken(refreshToken, func(userID string) (username, email, role string, err error) {
			assert.Equal(t, "user123", userID)
			return "testuser", "test@example.com", "player", nil
		})
		require.NoError(t, err)
		assert.NotEmpty(t, newAccessToken)

		// Validate new access token
		claims, err := service.ValidateToken(newAccessToken)
		require.NoError(t, err)
		assert.Equal(t, "user123", claims.UserID)
		assert.Equal(t, "testuser", claims.Username)
	})

	t.Run("RefreshAccessToken_InvalidToken", func(t *testing.T) {
		_, err := service.RefreshAccessToken("invalid-refresh-token", func(userID string) (username, email, role string, err error) {
			t.Fatal("getUserInfo should not be called")
			return "", "", "", nil
		})
		assert.Error(t, err)
	})
}