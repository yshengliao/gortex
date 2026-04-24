package app

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	appcontext "github.com/yshengliao/gortex/core/context"
)

// TestInjectDependencies_NilFieldReturnsError ensures that a struct field
// tagged with `inject` that remains nil causes injectDependencies to return
// an error rather than silently leaving a nil pointer that would panic later.
func TestInjectDependencies_NilFieldReturnsError(t *testing.T) {
	type FakeService struct{}
	type Handler struct {
		Svc *FakeService `inject:""`
	}

	ctx := appcontext.NewContext()
	h := &Handler{} // Svc is nil

	err := injectDependencies(reflect.ValueOf(h), ctx)
	require.Error(t, err, "injectDependencies must return error for nil inject field")
}

// TestInjectDependencies_NonNilFieldIsOK ensures that a field with inject tag
// that is already populated does not cause an error.
func TestInjectDependencies_NonNilFieldIsOK(t *testing.T) {
	type FakeService struct{}
	type Handler struct {
		Svc *FakeService `inject:""`
	}

	ctx := appcontext.NewContext()
	h := &Handler{Svc: &FakeService{}} // Svc is NOT nil

	err := injectDependencies(reflect.ValueOf(h), ctx)
	require.NoError(t, err, "injectDependencies must not error when inject field is already populated")
}
