package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/pkg/pool"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

type HandlersManager struct {
	Process *ProcessHandler `url:"/process"`
	Metrics *MetricsHandler `url:"/metrics"`
}

type ProcessHandler struct {
	Logger         *zap.Logger
	BufferPool     *pool.BufferPool
	ByteSlicePool  *pool.ByteSlicePool
	ResponsePool   *pool.ObjectPool[*ProcessResponse]
}

type ProcessResponse struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Buffer    []byte                 `json:"-"` // Internal use only
}

func (h *ProcessHandler) Json(c echo.Context) error {
	// Get buffer from pool for JSON processing
	buf := h.BufferPool.Get()
	defer h.BufferPool.Put(buf)
	
	// Simulate JSON processing
	data := map[string]interface{}{
		"message": "Processing with buffer pool",
		"size":    buf.Cap(),
		"reused":  true,
	}
	
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(data); err != nil {
		return response.Error(c, http.StatusInternalServerError, "Encoding failed")
	}
	
	return c.JSONBlob(http.StatusOK, buf.Bytes())
}

func (h *ProcessHandler) Binary(c echo.Context) error {
	// Get byte slice from pool
	size := 4096
	data := h.ByteSlicePool.Get(size)
	defer h.ByteSlicePool.Put(data)
	
	// Simulate binary data processing
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	
	// Process data (e.g., compression, encryption)
	processed := processData(data)
	
	return c.Blob(http.StatusOK, "application/octet-stream", processed)
}

func (h *ProcessHandler) Batch(c echo.Context) error {
	count := 10
	results := make([]*ProcessResponse, 0, count)
	
	// Process multiple items using object pool
	for i := 0; i < count; i++ {
		// Get response object from pool
		resp := h.ResponsePool.Get()
		
		// Use the object
		resp.ID = fmt.Sprintf("item-%d", i)
		resp.Timestamp = time.Now()
		resp.Data = map[string]interface{}{
			"index":     i,
			"processed": true,
		}
		
		// Get temporary buffer for processing
		tempBuf := h.ByteSlicePool.Get(1024)
		copy(tempBuf, []byte(fmt.Sprintf("data-%d", i)))
		resp.Buffer = tempBuf
		
		results = append(results, resp)
		
		// Return buffer to pool
		h.ByteSlicePool.Put(tempBuf)
	}
	
	// Return response objects to pool after sending response
	defer func() {
		for _, resp := range results {
			h.ResponsePool.Put(resp)
		}
	}()
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"count":   count,
		"results": results,
	})
}

type MetricsHandler struct {
	BufferPool    *pool.BufferPool
	ByteSlicePool *pool.ByteSlicePool
	ResponsePool  *pool.ObjectPool[*ProcessResponse]
}

func (h *MetricsHandler) GET(c echo.Context) error {
	bufferMetrics := h.BufferPool.GetMetrics()
	byteMetrics := h.ByteSlicePool.GetMetrics()
	responseMetrics := h.ResponsePool.GetMetrics()
	
	// Format byte slice metrics
	byteSliceStats := make(map[string]interface{})
	totalActive := int64(0)
	totalWasted := int64(0)
	
	for size, metric := range byteMetrics {
		byteSliceStats[fmt.Sprintf("%d_bytes", size)] = map[string]interface{}{
			"gets":         metric.TotalGet,
			"puts":         metric.TotalPut,
			"news":         metric.TotalNew,
			"active":       metric.CurrentActive,
			"bytes_wasted": metric.TotalBytesWasted,
			"reuse_rate":   calculateReuseRate(metric.TotalGet, metric.TotalNew),
		}
		totalActive += metric.CurrentActive
		totalWasted += metric.TotalBytesWasted
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"buffer_pool": map[string]interface{}{
			"total_gets":            bufferMetrics.TotalGet,
			"total_puts":            bufferMetrics.TotalPut,
			"total_news":            bufferMetrics.TotalNew,
			"current_active":        bufferMetrics.CurrentActive,
			"reuse_rate":            bufferMetrics.ReuseRate,
			"total_bytes_allocated": bufferMetrics.TotalBytesAllocated,
			"largest_buffer":        bufferMetrics.LargestBuffer,
		},
		"byte_slice_pool": map[string]interface{}{
			"sizes":         byteSliceStats,
			"total_active":  totalActive,
			"total_wasted":  totalWasted,
		},
		"response_pool": map[string]interface{}{
			"total_gets":     responseMetrics.TotalGet,
			"total_puts":     responseMetrics.TotalPut,
			"total_news":     responseMetrics.TotalNew,
			"current_active": responseMetrics.CurrentActive,
			"reuse_rate":     calculateReuseRate(responseMetrics.TotalGet, responseMetrics.TotalNew),
		},
	})
}

func processData(data []byte) []byte {
	// Simulate some processing
	result := make([]byte, len(data))
	for i := range data {
		result[i] = data[i] ^ 0xFF // Simple XOR
	}
	return result
}

func calculateReuseRate(totalGet, totalNew int64) float64 {
	if totalGet == 0 {
		return 0
	}
	return float64(totalGet-totalNew) / float64(totalGet)
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	// Create pools
	bufferPool := pool.NewBufferPool()
	byteSlicePool := pool.NewByteSlicePool()
	responsePool := pool.NewObjectPool(
		func() *ProcessResponse {
			return &ProcessResponse{
				Data: make(map[string]interface{}),
			}
		},
		func(resp **ProcessResponse) {
			(*resp).ID = ""
			(*resp).Timestamp = time.Time{}
			// Clear map
			for k := range (*resp).Data {
				delete((*resp).Data, k)
			}
			(*resp).Buffer = nil
		},
	)
	
	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug"
	
	// Create handlers
	handlers := &HandlersManager{
		Process: &ProcessHandler{
			Logger:        logger,
			BufferPool:    bufferPool,
			ByteSlicePool: byteSlicePool,
			ResponsePool:  responsePool,
		},
		Metrics: &MetricsHandler{
			BufferPool:    bufferPool,
			ByteSlicePool: byteSlicePool,
			ResponsePool:  responsePool,
		},
	}
	
	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		logger.Fatal("Failed to create application", zap.Error(err))
	}
	
	logger.Info("Memory pool example started", 
		zap.String("address", cfg.Server.Address))
	logger.Info("Try these endpoints:",
		zap.String("json", "POST http://localhost:8080/process/json"),
		zap.String("binary", "POST http://localhost:8080/process/binary"),
		zap.String("batch", "POST http://localhost:8080/process/batch"),
		zap.String("metrics", "GET http://localhost:8080/metrics"))
	logger.Info("Memory pools reduce GC pressure and improve performance")
	
	// Start server
	if err := application.Run(); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}