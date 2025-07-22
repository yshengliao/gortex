// Package auth provides JWT authentication functionality
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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

// NewJWTService creates a new JWT service instance
func NewJWTService(secretKey string, accessTTL, refreshTTL time.Duration, issuer string) *JWTService {
	return &JWTService{
		secretKey:       secretKey,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
		issuer:          issuer,
	}
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