package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appcontext "github.com/yshengliao/gortex/core/context"
	"github.com/yshengliao/gortex/middleware"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

// authTagHandler is a leaf protected by the `auth` middleware tag.
type authTagHandler struct{}

func (authTagHandler) GET(c httpctx.Context) error { return c.NoContent(204) }

type authTaggedManager struct {
	Secure *authTagHandler `url:"/secure" middleware:"auth"`
}

// A route tagged middleware:"auth" with no auth middleware registered must fail
// registration loudly rather than silently exposing the route unprotected.
func TestMiddlewareTagAuthUnresolvedFailsRegistration(t *testing.T) {
	r := newAppTestRouter()
	ctx := appcontext.NewContext()

	err := RegisterRoutesFromStruct(r, &authTaggedManager{}, ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth middleware")
}

// Once an auth middleware is registered in the context, the same handler wires
// up successfully.
func TestMiddlewareTagAuthResolvesWhenRegistered(t *testing.T) {
	r := newAppTestRouter()
	ctx := appcontext.NewContext()

	authMW := middleware.MiddlewareFunc(func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return next
	})
	appcontext.Register(ctx, authMW)

	err := RegisterRoutesFromStruct(r, &authTaggedManager{}, ctx)
	require.NoError(t, err)
}

type rbacTagHandler struct{}

func (rbacTagHandler) GET(c httpctx.Context) error { return c.NoContent(204) }

type rbacTaggedManager struct {
	Admin *rbacTagHandler `url:"/admin" middleware:"rbac"`
}

// rbac has no implementation, so the tag must fail registration with guidance
// to register a custom middleware.
func TestMiddlewareTagRBACFailsRegistration(t *testing.T) {
	r := newAppTestRouter()
	ctx := appcontext.NewContext()

	err := RegisterRoutesFromStruct(r, &rbacTaggedManager{}, ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RBAC is not implemented")
}

type unknownTagHandler struct{}

func (unknownTagHandler) GET(c httpctx.Context) error { return c.NoContent(204) }

type unknownTaggedManager struct {
	Thing *unknownTagHandler `url:"/thing" middleware:"does-not-exist"`
}

// An unknown middleware name must fail registration.
func TestMiddlewareTagUnknownFailsRegistration(t *testing.T) {
	r := newAppTestRouter()
	ctx := appcontext.NewContext()

	err := RegisterRoutesFromStruct(r, &unknownTaggedManager{}, ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown middleware")
}

type rlTagHandler struct{}

func (rlTagHandler) GET(c httpctx.Context) error { return c.NoContent(204) }

type rlTaggedManager struct {
	API *rlTagHandler `url:"/api" ratelimit:"100/min"`
}

// A rate-limit store created from a struct tag must be registered with the app
// and stopped by App.Shutdown, not leaked for the process lifetime.
func TestRateLimitTagStoreStoppedOnShutdown(t *testing.T) {
	a, err := NewApp()
	require.NoError(t, err)

	require.NoError(t, RegisterRoutes(a, &rlTaggedManager{}))

	a.mu.RLock()
	n := len(a.stoppables)
	a.mu.RUnlock()
	require.Equal(t, 1, n, "the struct-tag rate-limit store must be registered for shutdown")

	// Shutdown must drain the stoppables (Stop is idempotent, so a double
	// stop inside the store is harmless even if anything else stops it).
	require.NoError(t, a.Shutdown(context.Background()))

	a.mu.RLock()
	nAfter := len(a.stoppables)
	a.mu.RUnlock()
	assert.Equal(t, 0, nAfter, "Shutdown must drain registered stoppables")
}

// newAppTestRouter is a tiny helper so the tests above read cleanly.
func newAppTestRouter() httpctx.GortexRouter { return httpctx.NewGortexRouter() }
