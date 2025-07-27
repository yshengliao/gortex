package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/transport/http"
)

// Test handlers for auto-initialization
type AutoInitHandlersManager struct {
	Home   *AutoInitHomeHandler   `url:"/"`
	User   *AutoInitUserHandler   `url:"/users/:id"`
	Admin  *AutoInitAdminGroup    `url:"/admin"`
	Static *AutoInitStaticHandler `url:"/static/*"`
	API    *AutoInitAPIGroup      `url:"/api"`
}

type AutoInitHomeHandler struct{}

func (h *AutoInitHomeHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"message": "Home"})
}

type AutoInitUserHandler struct{}

func (h *AutoInitUserHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"user": c.Param("id")})
}

type AutoInitAdminGroup struct {
	Dashboard *AutoInitDashboardHandler `url:"/dashboard"`
	Users     *AutoInitUsersHandler     `url:"/users"`
}

type AutoInitDashboardHandler struct{}

func (h *AutoInitDashboardHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"message": "Dashboard"})
}

type AutoInitUsersHandler struct{}

func (h *AutoInitUsersHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"message": "Users"})
}

type AutoInitStaticHandler struct{}

func (h *AutoInitStaticHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"file": c.Param("*")})
}

type AutoInitAPIGroup struct {
	V1 *AutoInitAPIv1Group `url:"/v1"`
	V2 *AutoInitAPIv2Group `url:"/v2"`
}

type AutoInitAPIv1Group struct {
	Products *AutoInitProductsHandler `url:"/products/:id"`
}

type AutoInitProductsHandler struct{}

func (h *AutoInitProductsHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"product": c.Param("id")})
}

type AutoInitAPIv2Group struct {
	Items *AutoInitItemsHandler `url:"/items/:id"`
}

type AutoInitItemsHandler struct{}

func (h *AutoInitItemsHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"item": c.Param("id")})
}

func TestAutoInitHandlers(t *testing.T) {
	// Test 1: All handlers are nil initially
	t.Run("AutoInitializeAllNilHandlers", func(t *testing.T) {
		handlers := &AutoInitHandlersManager{}

		// Verify all handlers are nil
		assert.Nil(t, handlers.Home)
		assert.Nil(t, handlers.User)
		assert.Nil(t, handlers.Admin)
		assert.Nil(t, handlers.Static)
		assert.Nil(t, handlers.API)

		// Create router and register routes
		r := router.NewGortexRouter()
		ctx := NewContext()

		err := RegisterRoutesFromStruct(r, handlers, ctx)
		require.NoError(t, err)

		// Verify all handlers are now initialized
		assert.NotNil(t, handlers.Home)
		assert.NotNil(t, handlers.User)
		assert.NotNil(t, handlers.Admin)
		assert.NotNil(t, handlers.Static)
		assert.NotNil(t, handlers.API)

		// Verify nested handlers are also initialized
		assert.NotNil(t, handlers.Admin.Dashboard)
		assert.NotNil(t, handlers.Admin.Users)
		assert.NotNil(t, handlers.API.V1)
		assert.NotNil(t, handlers.API.V2)
		assert.NotNil(t, handlers.API.V1.Products)
		assert.NotNil(t, handlers.API.V2.Items)
	})

	// Test 2: Some handlers are already initialized
	t.Run("AutoInitializePartiallyInitializedHandlers", func(t *testing.T) {
		handlers := &AutoInitHandlersManager{
			Home: &AutoInitHomeHandler{}, // Already initialized
			API: &AutoInitAPIGroup{
				V1: &AutoInitAPIv1Group{}, // V1 initialized but not its children
			},
		}

		// Verify initial state
		assert.NotNil(t, handlers.Home)
		assert.Nil(t, handlers.User)
		assert.Nil(t, handlers.Admin)
		assert.NotNil(t, handlers.API)
		assert.NotNil(t, handlers.API.V1)
		assert.Nil(t, handlers.API.V2)
		assert.Nil(t, handlers.API.V1.Products)

		// Create router and register routes
		r := router.NewGortexRouter()
		ctx := NewContext()

		err := RegisterRoutesFromStruct(r, handlers, ctx)
		require.NoError(t, err)

		// Verify all handlers are now initialized
		assert.NotNil(t, handlers.Home)
		assert.NotNil(t, handlers.User)
		assert.NotNil(t, handlers.Admin)
		assert.NotNil(t, handlers.Static)
		assert.NotNil(t, handlers.API)

		// Verify nested handlers
		assert.NotNil(t, handlers.Admin.Dashboard)
		assert.NotNil(t, handlers.Admin.Users)
		assert.NotNil(t, handlers.API.V1)
		assert.NotNil(t, handlers.API.V2)
		assert.NotNil(t, handlers.API.V1.Products)
		assert.NotNil(t, handlers.API.V2.Items)
	})

	// Test 3: Empty handlers struct
	t.Run("AutoInitializeEmptyStruct", func(t *testing.T) {
		type EmptyHandlers struct{}

		handlers := &EmptyHandlers{}
		r := router.NewGortexRouter()
		ctx := NewContext()

		// Should not error on empty struct
		err := RegisterRoutesFromStruct(r, handlers, ctx)
		require.NoError(t, err)
	})

	// Test 4: Handler with no url tag should be ignored
	t.Run("IgnoreHandlersWithoutURLTag", func(t *testing.T) {
		type MixedHandlers struct {
			Public  *AutoInitHomeHandler `url:"/public"`
			Private *AutoInitUserHandler // No url tag, should be ignored
			Admin   *AutoInitAdminGroup  `url:"/admin"`
		}

		handlers := &MixedHandlers{}
		r := router.NewGortexRouter()
		ctx := NewContext()

		err := RegisterRoutesFromStruct(r, handlers, ctx)
		require.NoError(t, err)

		// Only handlers with url tags should be initialized
		assert.NotNil(t, handlers.Public)
		assert.Nil(t, handlers.Private) // Should remain nil
		assert.NotNil(t, handlers.Admin)
	})
}
