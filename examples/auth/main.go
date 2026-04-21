// Package main demonstrates the Gortex JWT service: login, refresh, and
// a protected route. The secret is loaded from the JWT_SECRET env var and
// must be at least auth.MinJWTSecretBytes (32) bytes — NewJWTService
// refuses shorter keys.
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/pkg/auth"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// fakeUser is the demo account. Production code would look this up from a
// store and compare a bcrypt hash — plain-text compare is used only to
// keep the example self-contained.
type fakeUser struct {
	ID       string
	Username string
	Password string
	Email    string
	Role     string
}

var demoUser = fakeUser{
	ID:       "user-1",
	Username: "alice",
	Password: "s3cret",
	Email:    "alice@example.test",
	Role:     "member",
}

// AuthHandler exposes /auth/login and /auth/refresh.
type AuthHandler struct {
	JWT *auth.JWTService
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (h *AuthHandler) Login(c httpctx.Context) error {
	var req loginReq
	if err := c.Bind(&req); err != nil {
		return httpctx.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Username != demoUser.Username ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(demoUser.Password)) != 1 {
		return httpctx.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	access, err := h.JWT.GenerateAccessToken(demoUser.ID, demoUser.Username, demoUser.Email, demoUser.Role)
	if err != nil {
		return err
	}
	refresh, err := h.JWT.GenerateRefreshToken(demoUser.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tokenResp{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(h.JWT.AccessTokenTTL().Seconds()),
	})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(c httpctx.Context) error {
	var req refreshReq
	if err := c.Bind(&req); err != nil {
		return httpctx.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	access, err := h.JWT.RefreshAccessToken(req.RefreshToken, func(userID string) (string, string, string, error) {
		if userID != demoUser.ID {
			return "", "", "", errors.New("unknown user")
		}
		return demoUser.Username, demoUser.Email, demoUser.Role, nil
	})
	if err != nil {
		return httpctx.NewHTTPError(http.StatusUnauthorized, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"access_token": access,
		"expires_in":   int(h.JWT.AccessTokenTTL().Seconds()),
	})
}

// MeHandler validates the bearer token and echoes the caller's claims.
type MeHandler struct {
	JWT *auth.JWTService
}

func (h *MeHandler) GET(c httpctx.Context) error {
	raw := c.Request().Header.Get("Authorization")
	token := strings.TrimPrefix(raw, "Bearer ")
	if token == "" || token == raw {
		return httpctx.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
	}
	claims, err := h.JWT.ValidateToken(token)
	if err != nil {
		return httpctx.NewHTTPError(http.StatusUnauthorized, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"email":    claims.Email,
		"role":     claims.Role,
	})
}

// AuthGroup mounts both auth endpoints under /auth.
type AuthGroup struct {
	Login   *LoginHandler   `url:"/login"`
	Refresh *RefreshHandler `url:"/refresh"`
}

// LoginHandler and RefreshHandler are thin POST-only shells around
// AuthHandler so the struct-tag router can dispatch by verb.
type LoginHandler struct{ Auth *AuthHandler }

func (h *LoginHandler) POST(c httpctx.Context) error { return h.Auth.Login(c) }

type RefreshHandler struct{ Auth *AuthHandler }

func (h *RefreshHandler) POST(c httpctx.Context) error { return h.Auth.Refresh(c) }

type Handlers struct {
	Auth *AuthGroup `url:"/auth"`
	Me   *MeHandler `url:"/me"`
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		logger.Fatal("JWT_SECRET env var is required (>=32 bytes)")
	}
	jwtSvc, err := auth.NewJWTService(secret, 1*time.Hour, 24*time.Hour, "gortex-example")
	if err != nil {
		logger.Fatal("jwt init failed", zap.Error(err))
	}

	authHandler := &AuthHandler{JWT: jwtSvc}
	handlers := &Handlers{
		Auth: &AuthGroup{
			Login:   &LoginHandler{Auth: authHandler},
			Refresh: &RefreshHandler{Auth: authHandler},
		},
		Me: &MeHandler{JWT: jwtSvc},
	}

	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		logger.Fatal("failed to create app", zap.Error(err))
	}

	go func() {
		if err := application.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server exited", zap.Error(err))
		}
	}()
	logger.Info("auth example listening on :8080")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
