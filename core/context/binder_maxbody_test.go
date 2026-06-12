package context

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpctx "github.com/yshengliao/gortex/transport/http"
)

type sizedPayload struct {
	Name string `json:"name"`
}

func newPostJSON(t *testing.T, body string) httpctx.Context {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	return httpctx.NewDefaultContext(req, rec)
}

func TestBinderRejectsOversizedJSON(t *testing.T) {
	big := make([]byte, 128)
	for i := range big {
		big[i] = 'a'
	}
	body := `{"name":"` + string(big) + `"}`

	pb := NewParameterBinder()
	pb.SetMaxJSONBodyBytes(32) // absurdly low to force the limit

	c := newPostJSON(t, body)

	params := &sizedPayload{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json decode")
}

func TestBinderAcceptsJSONUnderLimit(t *testing.T) {
	body := bytes.NewReader([]byte(`{"name":"hi"}`))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := httpctx.NewDefaultContext(req, rec)

	pb := NewParameterBinder()
	params := &sizedPayload{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.NoError(t, err)
	assert.Equal(t, "hi", params.Name)
}

func TestBinderTolratesEmptyJSONBody(t *testing.T) {
	c := newPostJSON(t, "")
	pb := NewParameterBinder()
	params := &sizedPayload{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.NoError(t, err)
	assert.Empty(t, params.Name)
}

func TestBinderPropagatesMalformedJSON(t *testing.T) {
	c := newPostJSON(t, `{"name":`)
	pb := NewParameterBinder()
	params := &sizedPayload{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json decode")
}

func TestSetMaxJSONBodyBytesRestoresDefaultOnZero(t *testing.T) {
	pb := NewParameterBinder()
	pb.SetMaxJSONBodyBytes(0)
	assert.Equal(t, DefaultMaxJSONBodyBytes, pb.maxJSONBodyBytes)

	pb.SetMaxJSONBodyBytes(-5)
	assert.Equal(t, DefaultMaxJSONBodyBytes, pb.maxJSONBodyBytes)

	pb.SetMaxJSONBodyBytes(123)
	assert.Equal(t, int64(123), pb.maxJSONBodyBytes)
}

// TestDefaultMaxJSONBodyBytesIs1MiB verifies the constant matches the 1 MiB
// limit documented in CLAUDE.md and enforced by transport/http/default.go.
func TestDefaultMaxJSONBodyBytesIs1MiB(t *testing.T) {
	assert.Equal(t, int64(1<<20), DefaultMaxJSONBodyBytes,
		"DefaultMaxJSONBodyBytes must be 1 MiB to match transport/http DefaultMaxBodyBytes")
}

// TestBinderAcceptsJSONWithCharsetContentType verifies that a Content-Type of
// "application/json; charset=utf-8" (and similar variants) is recognised for
// JSON body binding, not silently skipped (prefix matching, not exact match).
func TestBinderAcceptsJSONWithCharsetContentType(t *testing.T) {
	body := `{"name":"charset-test"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	c := httpctx.NewDefaultContext(req, rec)

	pb := NewParameterBinder()
	params := &sizedPayload{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.NoError(t, err)
	assert.Equal(t, "charset-test", params.Name)
}

// TestBinderExplicitTagMismatchReturnsError verifies that a type-conversion
// failure on a field with an explicit "bind" tag is surfaced as an error
// rather than silently leaving the field at its zero value.
func TestBinderExplicitTagMismatchReturnsError(t *testing.T) {
	type Req struct {
		Age int `bind:"age,query"`
	}
	req := httptest.NewRequest(http.MethodGet, "/?age=abc", nil)
	rec := httptest.NewRecorder()
	c := httpctx.NewDefaultContext(req, rec)

	pb := NewParameterBinder()
	params := &Req{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.Error(t, err, "explicit bind tag with invalid value should return error")
	assert.Contains(t, err.Error(), "Age")
}

// TestBinderAutoTagMismatchStaysZero verifies that a type-conversion failure
// on a field without an explicit "bind" tag stays lenient: the field is left
// at its zero value and no error is returned.
func TestBinderAutoTagMismatchStaysZero(t *testing.T) {
	type Req struct {
		Count int `json:"count"` // no explicit bind tag — auto-detected from json tag
	}
	req := httptest.NewRequest(http.MethodGet, "/?count=notanint", nil)
	rec := httptest.NewRecorder()
	c := httpctx.NewDefaultContext(req, rec)

	pb := NewParameterBinder()
	params := &Req{}
	err := pb.bindStruct(c, reflect.ValueOf(params))
	require.NoError(t, err, "auto-bound field with invalid value should not error")
	assert.Equal(t, 0, params.Count, "field should remain at zero value")
}
