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
