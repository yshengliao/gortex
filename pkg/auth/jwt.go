// Package auth provides JWT authentication functionality
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MinJWTSecretBytes is the minimum byte length accepted for an HS256
// secret. The HMAC construction depends on the secret having at least the
// output length of the hash (32 bytes for SHA-256); shorter keys reduce
// the effective search space and make brute-force practical.
const MinJWTSecretBytes = 32

// ErrJWTSecretTooShort is returned by NewJWTService when the supplied
// secret is shorter than MinJWTSecretBytes.
var ErrJWTSecretTooShort = errors.New("auth: JWT secret must be at least 32 bytes")

// JWTService handles JWT token generation and validation
type JWTService struct {
	secretKey        string
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
	issuer           string
}

// Claims represents the JWT claims structure
type Claims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"`
	GameID   string `json:"game_id,omitempty"`
}

// NewJWTService creates a new JWT service instance. It returns
// ErrJWTSecretTooShort if secretKey has fewer than MinJWTSecretBytes
// bytes — rejecting weak keys at construction time is safer than failing
// silently and discovering the weakness in a breach post-mortem.
func NewJWTService(secretKey string, accessTTL, refreshTTL time.Duration, issuer string) (*JWTService, error) {
	if len(secretKey) < MinJWTSecretBytes {
		return nil, ErrJWTSecretTooShort
	}
	return &JWTService{
		secretKey:       secretKey,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
		issuer:          issuer,
	}, nil
}

// GenerateAccessToken generates a new access token
func (s *JWTService) GenerateAccessToken(userID, username, email, role string) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   userID,
		},
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// GenerateRefreshToken generates a new refresh token
func (s *JWTService) GenerateRefreshToken(userID string) (string, error) {
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshTokenTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    s.issuer,
		Subject:   userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// ValidateToken validates a JWT token and returns the claims
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a refresh token
func (s *JWTService) RefreshAccessToken(refreshToken string, getUserInfo func(userID string) (username, email, role string, err error)) (string, error) {
	// Validate refresh token
	token, err := jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid refresh token")
	}

	// Get user info
	username, email, role, err := getUserInfo(claims.Subject)
	if err != nil {
		return "", err
	}

	// Generate new access token
	return s.GenerateAccessToken(claims.Subject, username, email, role)
}

// GenerateGameToken generates a game-specific token
func (s *JWTService) GenerateGameToken(userID, username, gameID string) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   userID,
		},
		UserID:   userID,
		Username: username,
		GameID:   gameID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// AccessTokenTTL returns the access token TTL
func (s *JWTService) AccessTokenTTL() time.Duration {
	return s.accessTokenTTL
}

// ValidateRefreshToken validates a refresh token
func (s *JWTService) ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}

	return claims, nil
}