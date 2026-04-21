package http_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	httpctx "github.com/yshengliao/gortex/transport/http"
)

func newCtx(t *testing.T, method, target string) (*httpctx.DefaultContext, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	return httpctx.NewDefaultContext(req, rec).(*httpctx.DefaultContext), rec
}

func TestRedirectRejectsUnsafeTargets(t *testing.T) {
	cases := []struct {
		name   string
		target string
	}{
		{"protocol-relative", "//evil.com/phish"},
		{"absolute-http", "http://evil.com/"},
		{"absolute-https", "https://evil.com/"},
		{"javascript-scheme", "javascript:alert(1)"},
		{"data-scheme", "data:text/html,<script>"},
		{"relative-no-slash", "evil.com"},
		{"empty", ""},
		{"crlf-injection", "/ok\r\nSet-Cookie: x=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newCtx(t, http.MethodGet, "/")
			err := c.Redirect(http.StatusFound, tc.target)
			require.Error(t, err)
			assert.Equal(t, httpctx.ErrUnsafeRedirectURL, err)
			assert.Empty(t, rec.Header().Get("Location"))
		})
	}
}

func TestRedirectAcceptsSafeTargets(t *testing.T) {
	cases := []string{"/", "/dashboard", "/users/42?x=1", "/a/b/c"}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			c, rec := newCtx(t, http.MethodGet, "/")
			err := c.Redirect(http.StatusSeeOther, target)
			require.NoError(t, err)
			assert.Equal(t, target, rec.Header().Get("Location"))
			assert.Equal(t, http.StatusSeeOther, rec.Code)
		})
	}
}

func TestRedirectRejectsInvalidCode(t *testing.T) {
	c, _ := newCtx(t, http.MethodGet, "/")
	err := c.Redirect(200, "/ok")
	assert.Equal(t, httpctx.ErrInvalidRedirectCode, err)
}

func TestFileRejectsTraversal(t *testing.T) {
	cases := []string{
		"../../../../etc/passwd",
		"foo/../../secret",
		"./..",
		"",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			c, _ := newCtx(t, http.MethodGet, "/")
			err := c.File(p)
			require.Error(t, err)
			assert.Equal(t, httpctx.ErrUnsafeFilePath, err)
		})
	}
}

func TestFileServesTrustedPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hi"), 0o600))

	c, rec := newCtx(t, http.MethodGet, "/")
	require.NoError(t, c.File(path))
	assert.Equal(t, "hi", rec.Body.String())
}

func TestFileFSRejectsEscapes(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("root")},
		"static/app.js": &fstest.MapFile{Data: []byte("js")},
	}
	cases := []string{"../etc/passwd", "/etc/passwd", "./foo", "static/../../etc"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := newCtx(t, http.MethodGet, "/")
			err := c.FileFS(fsys, name)
			require.Error(t, err)
			assert.Equal(t, httpctx.ErrUnsafeFilePath, err)
		})
	}
}

func TestFileFSServesValidPath(t *testing.T) {
	fsys := fstest.MapFS{
		"static/app.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	}
	c, rec := newCtx(t, http.MethodGet, "/")
	require.NoError(t, c.FileFS(fsys, "static/app.js"))
	assert.Equal(t, "console.log(1)", rec.Body.String())
}
