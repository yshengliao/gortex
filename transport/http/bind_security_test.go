package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

// buildOversizedJSON builds a JSON payload just above the 1 MiB limit.
func buildOversizedJSON() *bytes.Reader {
	// `{"k":"..."}` with value of ~1.1 MiB total
	padding := strings.Repeat("x", 1<<20+1)
	payload := `{"k":"` + padding + `"}`
	return bytes.NewReader([]byte(payload))
}

// buildOversizedXML builds an XML payload just above the 1 MiB limit.
func buildOversizedXML() *bytes.Reader {
	padding := strings.Repeat("x", 1<<20+1)
	payload := `<Root><K>` + padding + `</K></Root>`
	return bytes.NewReader([]byte(payload))
}

func TestBind_OversizedJSON(t *testing.T) {
	body := buildOversizedJSON()
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ctx := httpctx.NewDefaultContext(req, rec).(*httpctx.DefaultContext)

	var dest map[string]any
	err := ctx.Bind(&dest)

	// Must return an error — either "request body too large" or similar.
	require.Error(t, err, "Bind() must reject oversized JSON body")
	assert.NotNil(t, err)
}

func TestBind_OversizedXML(t *testing.T) {
	body := buildOversizedXML()
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()

	ctx := httpctx.NewDefaultContext(req, rec).(*httpctx.DefaultContext)

	type Root struct {
		K string `xml:"K"`
	}
	var dest Root
	err := ctx.Bind(&dest)

	require.Error(t, err, "Bind() must reject oversized XML body")
	assert.NotNil(t, err)
}

func TestBind_NormalJSON(t *testing.T) {
	body := strings.NewReader(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ctx := httpctx.NewDefaultContext(req, rec).(*httpctx.DefaultContext)

	var dest map[string]any
	err := ctx.Bind(&dest)

	require.NoError(t, err)
	assert.Equal(t, "test", dest["name"])
}
