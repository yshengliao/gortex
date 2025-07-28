// Package performance provides comprehensive benchmarking and performance tracking for Gortex framework
package performance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	httpctx "github.com/yshengliao/gortex/transport/http"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name              string    `json:"name"`
	Timestamp         time.Time `json:"timestamp"`
	NsPerOp           int64     `json:"ns_per_op"`
	AllocsPerOp       int64     `json:"allocs_per_op"`
	BytesPerOp        int64     `json:"bytes_per_op"`
	Iterations        int       `json:"iterations"`
	GoVersion         string    `json:"go_version"`
	OS                string    `json:"os"`
	Arch              string    `json:"arch"`
	CPUs              int       `json:"cpus"`
	GortexVersion     string    `json:"gortex_version"`
	MemStats          MemStats  `json:"mem_stats"`
}

// MemStats captures memory statistics
type MemStats struct {
	Alloc        uint64 `json:"alloc"`
	TotalAlloc   uint64 `json:"total_alloc"`
	Sys          uint64 `json:"sys"`
	NumGC        uint32 `json:"num_gc"`
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapInuse    uint64 `json:"heap_inuse"`
	StackInuse   uint64 `json:"stack_inuse"`
}

// BenchmarkSuite manages benchmark execution and result storage
type BenchmarkSuite struct {
	results []BenchmarkResult
	dbPath  string
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite() *BenchmarkSuite {
	return &BenchmarkSuite{
		results: make([]BenchmarkResult, 0),
		dbPath:  filepath.Join("performance", "benchmarks", "benchmark_db.json"),
	}
}

// RunRouterBenchmarks executes router performance benchmarks
func (bs *BenchmarkSuite) RunRouterBenchmarks(b *testing.B) {
	b.Run("SimpleRoute", bs.benchmarkSimpleRoute)
	b.Run("ParameterizedRoute", bs.benchmarkParameterizedRoute)
	b.Run("WildcardRoute", bs.benchmarkWildcardRoute)
	b.Run("NestedGroups", bs.benchmarkNestedGroups)
	b.Run("MiddlewareChain", bs.benchmarkMiddlewareChain)
}

// benchmarkSimpleRoute tests simple route matching performance
func (bs *BenchmarkSuite) benchmarkSimpleRoute(b *testing.B) {
	router := httpctx.NewGortexRouter()
	handler := func(c httpctx.Context) error {
		return nil
	}

	// Register routes
	router.GET("/", handler)
	router.GET("/users", handler)
	router.GET("/posts", handler)
	router.GET("/api/v1/health", handler)

	req := NewTestRequest("GET", "/users", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		w := NewTestResponseRecorder()
		router.ServeHTTP(w, req)
	}

	bs.recordResult(b, "SimpleRoute")
}

// benchmarkParameterizedRoute tests parameterized route performance
func (bs *BenchmarkSuite) benchmarkParameterizedRoute(b *testing.B) {
	router := httpctx.NewGortexRouter()
	handler := func(c httpctx.Context) error {
		_ = c.Param("id")
		return nil
	}

	router.GET("/users/:id", handler)
	router.GET("/posts/:id/comments/:cid", handler)
	
	req := NewTestRequest("GET", "/users/123", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		w := NewTestResponseRecorder()
		router.ServeHTTP(w, req)
	}

	bs.recordResult(b, "ParameterizedRoute")
}

// benchmarkWildcardRoute tests wildcard route performance
func (bs *BenchmarkSuite) benchmarkWildcardRoute(b *testing.B) {
	router := httpctx.NewGortexRouter()
	handler := func(c httpctx.Context) error {
		_ = c.Param("*")
		return nil
	}

	router.GET("/static/*", handler)
	
	req := NewTestRequest("GET", "/static/css/main.css", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		w := NewTestResponseRecorder()
		router.ServeHTTP(w, req)
	}

	bs.recordResult(b, "WildcardRoute")
}

// benchmarkNestedGroups tests nested group routing performance
func (bs *BenchmarkSuite) benchmarkNestedGroups(b *testing.B) {
	router := httpctx.NewGortexRouter()
	handler := func(c httpctx.Context) error {
		return nil
	}

	api := router.Group("/api")
	v1 := api.Group("/v1")
	v1.GET("/users", handler)
	v1.GET("/posts", handler)
	
	v2 := api.Group("/v2")
	v2.GET("/users", handler)
	v2.GET("/posts", handler)
	
	req := NewTestRequest("GET", "/api/v2/users", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		w := NewTestResponseRecorder()
		router.ServeHTTP(w, req)
	}

	bs.recordResult(b, "NestedGroups")
}

// benchmarkMiddlewareChain tests middleware chain performance
func (bs *BenchmarkSuite) benchmarkMiddlewareChain(b *testing.B) {
	router := httpctx.NewGortexRouter()
	
	// Create middleware chain
	middleware1 := func(next httpctx.HandlerFunc) httpctx.HandlerFunc {
		return func(c httpctx.Context) error {
			c.Response().Header().Set("X-Middleware-1", "true")
			return next(c)
		}
	}
	
	middleware2 := func(next httpctx.HandlerFunc) httpctx.HandlerFunc {
		return func(c httpctx.Context) error {
			c.Response().Header().Set("X-Middleware-2", "true")
			return next(c)
		}
	}
	
	middleware3 := func(next httpctx.HandlerFunc) httpctx.HandlerFunc {
		return func(c httpctx.Context) error {
			c.Response().Header().Set("X-Middleware-3", "true")
			return next(c)
		}
	}
	
	handler := func(c httpctx.Context) error {
		return c.String(200, "OK")
	}

	router.Use(middleware1, middleware2, middleware3)
	router.GET("/test", handler)
	
	req := NewTestRequest("GET", "/test", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		w := NewTestResponseRecorder()
		router.ServeHTTP(w, req)
	}

	bs.recordResult(b, "MiddlewareChain")
}

// RunContextBenchmarks executes context operation benchmarks
func (bs *BenchmarkSuite) RunContextBenchmarks(b *testing.B) {
	b.Run("ContextCreation", bs.benchmarkContextCreation)
	b.Run("ContextParamAccess", bs.benchmarkContextParamAccess)
	b.Run("ContextValueStorage", bs.benchmarkContextValueStorage)
	b.Run("ContextPooling", bs.benchmarkContextPooling)
}

// benchmarkContextCreation tests context creation performance
func (bs *BenchmarkSuite) benchmarkContextCreation(b *testing.B) {
	req := NewTestRequest("GET", "/test", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		w := NewTestResponseRecorder()
		ctx := httpctx.AcquireContext(req, w)
		httpctx.ReleaseContext(ctx)
	}

	bs.recordResult(b, "ContextCreation")
}

// benchmarkContextParamAccess tests parameter access performance
func (bs *BenchmarkSuite) benchmarkContextParamAccess(b *testing.B) {
	req := NewTestRequest("GET", "/users/123", nil)
	w := NewTestResponseRecorder()
	ctx := httpctx.AcquireContext(req, w)
	
	// Set params
	params := map[string]string{
		"id": "123",
		"action": "view",
		"format": "json",
	}
	httpctx.SetParams(ctx, params)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = ctx.Param("id")
		_ = ctx.Param("action")
		_ = ctx.Param("format")
	}

	httpctx.ReleaseContext(ctx)
	bs.recordResult(b, "ContextParamAccess")
}

// benchmarkContextValueStorage tests context value storage performance
func (bs *BenchmarkSuite) benchmarkContextValueStorage(b *testing.B) {
	req := NewTestRequest("GET", "/test", nil)
	w := NewTestResponseRecorder()
	ctx := httpctx.AcquireContext(req, w)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		ctx.Set("user_id", 12345)
		ctx.Set("tenant_id", "tenant-123")
		ctx.Set("request_id", "req-abc-123")
		
		_ = ctx.Get("user_id")
		_ = ctx.Get("tenant_id")
		_ = ctx.Get("request_id")
	}

	httpctx.ReleaseContext(ctx)
	bs.recordResult(b, "ContextValueStorage")
}

// benchmarkContextPooling tests context pooling efficiency
func (bs *BenchmarkSuite) benchmarkContextPooling(b *testing.B) {
	req := NewTestRequest("GET", "/test", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := NewTestResponseRecorder()
			ctx := httpctx.AcquireContext(req, w)
			ctx.Set("test", "value")
			_ = ctx.Get("test")
			httpctx.ReleaseContext(ctx)
		}
	})

	bs.recordResult(b, "ContextPooling")
}

// recordResult records benchmark result with system information
func (bs *BenchmarkSuite) recordResult(b *testing.B, name string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	result := BenchmarkResult{
		Name:          name,
		Timestamp:     time.Now(),
		NsPerOp:       b.Elapsed().Nanoseconds() / int64(b.N),
		AllocsPerOp:   0, // Will be filled from benchmark output
		BytesPerOp:    0, // Will be filled from benchmark output
		Iterations:    b.N,
		GoVersion:     runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		CPUs:          runtime.NumCPU(),
		GortexVersion: "v0.4.0-alpha", // TODO: Get from version file
		MemStats: MemStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
			HeapAlloc:  m.HeapAlloc,
			HeapSys:    m.HeapSys,
			HeapInuse:  m.HeapInuse,
			StackInuse: m.StackInuse,
		},
	}
	
	bs.results = append(bs.results, result)
}

// SaveResults saves benchmark results to database
func (bs *BenchmarkSuite) SaveResults() error {
	// Ensure directory exists
	dir := filepath.Dir(bs.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Load existing results
	var existingResults []BenchmarkResult
	if data, err := os.ReadFile(bs.dbPath); err == nil {
		json.Unmarshal(data, &existingResults)
	}
	
	// Append new results
	existingResults = append(existingResults, bs.results...)
	
	// Save all results
	data, err := json.MarshalIndent(existingResults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}
	
	if err := os.WriteFile(bs.dbPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}
	
	return nil
}

// GetLatestResults returns the most recent benchmark results for each test
func (bs *BenchmarkSuite) GetLatestResults() (map[string]BenchmarkResult, error) {
	data, err := os.ReadFile(bs.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read benchmark database: %w", err)
	}
	
	var allResults []BenchmarkResult
	if err := json.Unmarshal(data, &allResults); err != nil {
		return nil, fmt.Errorf("failed to unmarshal results: %w", err)
	}
	
	// Get latest result for each benchmark
	latest := make(map[string]BenchmarkResult)
	for _, result := range allResults {
		if existing, ok := latest[result.Name]; !ok || result.Timestamp.After(existing.Timestamp) {
			latest[result.Name] = result
		}
	}
	
	return latest, nil
}