package http_test

import (
	"net/http/httptest"
	"testing"

	ghttp "github.com/yshengliao/gortex/transport/http"
)

// ---- 以下測試案例從已刪除的 router_test.go 遷移而來 ----
// 原測試驗證的是 NewRouter() (radix tree)；此版本驗證相同行為在
// NewGortexRouter() (segment trie，生產實作) 上同樣成立。

func TestGortexRouter_ParamExtraction(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/users/:id", func(c ghttp.Context) error {
		return c.String(200, "User: "+c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "User: 123" {
		t.Errorf("expected 'User: 123', got %q", rec.Body.String())
	}
}

func TestGortexRouter_MultipleParams(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/users/:id/posts/:postId", func(c ghttp.Context) error {
		return c.String(200, "User: "+c.Param("id")+", Post: "+c.Param("postId"))
	})

	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	want := "User: 123, Post: 456"
	if rec.Body.String() != want {
		t.Errorf("expected %q, got %q", want, rec.Body.String())
	}
}

func TestGortexRouter_Wildcard(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/static/*filepath", func(c ghttp.Context) error {
		return c.String(200, "File: "+c.Param("filepath"))
	})

	req := httptest.NewRequest("GET", "/static/css/style.css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGortexRouter_NotFound(t *testing.T) {
	r := ghttp.NewGortexRouter()
	r.GET("/exists", func(c ghttp.Context) error {
		return c.String(200, "Found")
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGortexRouter_AllHTTPMethods(t *testing.T) {
	r := ghttp.NewGortexRouter()

	r.GET("/test", func(c ghttp.Context) error { return c.String(200, "GET") })
	r.POST("/test", func(c ghttp.Context) error { return c.String(200, "POST") })
	r.PUT("/test", func(c ghttp.Context) error { return c.String(200, "PUT") })
	r.DELETE("/test", func(c ghttp.Context) error { return c.String(200, "DELETE") })
	r.PATCH("/test", func(c ghttp.Context) error { return c.String(200, "PATCH") })
	r.HEAD("/test", func(c ghttp.Context) error { return c.String(200, "HEAD") })
	r.OPTIONS("/test", func(c ghttp.Context) error { return c.String(200, "OPTIONS") })

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/test", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != 200 {
			t.Errorf("%s: expected 200, got %d", method, rec.Code)
		}
	}
}

func TestGortexRouter_NestedGroups(t *testing.T) {
	r := ghttp.NewGortexRouter()

	api := r.Group("/api")
	api.GET("/users", func(c ghttp.Context) error { return c.String(200, "API Users") })

	v1 := api.Group("/v1")
	v1.GET("/products", func(c ghttp.Context) error { return c.String(200, "API V1 Products") })

	cases := []struct {
		path string
		code int
		body string
	}{
		{"/api/users", 200, "API Users"},
		{"/api/v1/products", 200, "API V1 Products"},
		{"/api/v1/notfound", 404, ""},
	}

	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != tc.code {
			t.Errorf("%s: expected %d, got %d", tc.path, tc.code, rec.Code)
		}
		if tc.code == 200 && rec.Body.String() != tc.body {
			t.Errorf("%s: expected %q, got %q", tc.path, tc.body, rec.Body.String())
		}
	}
}
