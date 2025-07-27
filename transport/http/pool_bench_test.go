package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yshengliao/gortex/transport/http"
)

// BenchmarkContextWithoutPool tests context creation without pooling
func BenchmarkContextWithoutPool(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ctx := context.NewContext(req, w)
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
		ctx := context.AcquireContext(req, w)
		ctx.SetPath("/test")
		ctx.Set("key", "value")
		_ = ctx.Get("key")
		context.ReleaseContext(ctx)
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
		ctx := context.AcquireContext(req, w)
		context.SetParams(ctx, params)
		_ = ctx.Param("userId")
		_ = ctx.Param("postId")
		context.ReleaseContext(ctx)
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
		ctx := context.AcquireContext(req, w)
		context.SetParams(ctx, params)
		for key := range params {
			_ = ctx.Param(key)
		}
		context.ReleaseContext(ctx)
	}
}