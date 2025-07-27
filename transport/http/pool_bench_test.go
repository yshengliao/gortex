package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpctx "github.com/yshengliao/gortex/transport/http"
)

// BenchmarkContextWithoutPool tests context creation without pooling
func BenchmarkContextWithoutPool(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ctx := httpctx.NewDefaultContext(req, w)
		ctx.SetPath("/test")
		ctx.Set("key", "value")
		_ = ctx.Get("key")
	}
}

// BenchmarkContextWithPool tests context creation with pooling
func BenchmarkContextWithPool(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ctx := httpctx.AcquireContext(req, w)
		ctx.SetPath("/test")
		ctx.Set("key", "value")
		_ = ctx.Get("key")
		httpctx.ReleaseContext(ctx)
	}
}

// BenchmarkSmartParams tests the smart parameter storage
func BenchmarkSmartParams(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()
	
	params := map[string]string{
		"userId": "123",
		"postId": "456",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := httpctx.AcquireContext(req, w)
		httpctx.SetParams(ctx, params)
		_ = ctx.Param("userId")
		_ = ctx.Param("postId")
		httpctx.ReleaseContext(ctx)
	}
}

// BenchmarkManyParams tests performance with many parameters
func BenchmarkManyParams(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/a/b/c/d/e/f", nil)
	w := httptest.NewRecorder()
	
	params := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": "5",
		"f": "6",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := httpctx.AcquireContext(req, w)
		httpctx.SetParams(ctx, params)
		for key := range params {
			_ = ctx.Param(key)
		}
		httpctx.ReleaseContext(ctx)
	}
}