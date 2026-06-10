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

// Token type discriminators. They are stored in the "typ" claim so that an
// access token cannot be replayed where a refresh token is expected (and
// vice versa). All three tokens are signed with the same secret and
// algorithm, so without this discriminator a long-lived refresh token would
// validate as an access token and an access token could mint new ones.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// ErrJWTSecretTooShort is returned by NewJWTService when the supplied
// secret is shorter than MinJWTSecretBytes.
var ErrJWTSecretTooShort = errors.New("auth: JWT secret must be at least 32 bytes")

// ErrInvalidTokenType is returned when a token's "typ" claim does not match
// the type expected by the validating call (e.g. a refresh token passed to
// ValidateToken). Empty/legacy tokens with no "typ" claim are also rejected.
var ErrInvalidTokenType = errors.New("auth: invalid token type")

// JWTService handles JWT token generation and validation
type JWTService struct {
	secretKey       string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	issuer          string
}

// Claims represents the JWT claims structure
type Claims struct {
	jwt.RegisteredClaims
	// TokenType discriminates access vs refresh tokens so they cannot be
	// used interchangeably; see the tokenType* constants.
	TokenType string `json:"typ,omitempty"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role,omitempty"`
	GameID    string `json:"game_id,omitempty"`
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

// keyFuncHS256 returns the secret used to verify a token, but only after
// confirming the token was signed with exactly HS256. A bare
// (*jwt.SigningMethodHMAC) assertion would also accept HS384/HS512 — and an
// attacker who can pick the algorithm header can substitute a weaker (or
// "none") method, so the method must be pinned to the one we sign with.
func (s *JWTService) keyFuncHS256(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodHS256 {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	return []byte(s.secretKey), nil
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
		TokenType: tokenTypeAccess,
		UserID:    userID,
		Username:  username,
		Email:     email,
		Role:      role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// GenerateRefreshToken generates a new refresh token. It carries the
// "refresh" token type so it can never be accepted by ValidateToken.
func (s *JWTService) GenerateRefreshToken(userID string) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   userID,
		},
		TokenType: tokenTypeRefresh,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// ValidateToken validates a JWT access token and returns the claims. Tokens
// whose type is not "access" (including legacy tokens with no type) are
// rejected so a refresh token cannot be replayed as an access token.
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, s.keyFuncHS256)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.TokenType != tokenTypeAccess {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a refresh token. The
// supplied token must be of type "refresh"; an access token (or any other
// type) is rejected so it cannot be used to indefinitely mint new access
// tokens.
func (s *JWTService) RefreshAccessToken(refreshToken string, getUserInfo func(userID string) (username, email, role string, err error)) (string, error) {
	claims, err := s.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	// Get user info
	username, email, role, err := getUserInfo(claims.Subject)
	if err != nil {
		return "", err
	}

	// Generate new access token
	return s.GenerateAccessToken(claims.Subject, username, email, role)
}

// GenerateGameToken generates a game-specific token. Game tokens are
// access-class: they are validated through ValidateToken, so they carry the
// "access" type.
func (s *JWTService) GenerateGameToken(userID, username, gameID string) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   userID,
		},
		TokenType: tokenTypeAccess,
		UserID:    userID,
		Username:  username,
		GameID:    gameID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secretKey))
}

// AccessTokenTTL returns the access token TTL
func (s *JWTService) AccessTokenTTL() time.Duration {
	return s.accessTokenTTL
}

// ValidateRefreshToken validates a refresh token. Tokens whose type is not
// "refresh" (including access tokens and legacy tokens with no type) are
// rejected. The registered claims are returned for callers that only need
// the subject.
func (s *JWTService) ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, s.keyFuncHS256)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}
	if claims.TokenType != tokenTypeRefresh {
		return nil, ErrInvalidTokenType
	}

	return &claims.RegisteredClaims, nil
}
