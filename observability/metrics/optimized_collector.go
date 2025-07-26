package metrics

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// OptimizedCollector is an improved version with better concurrency characteristics
type OptimizedCollector struct {
	// Atomic counters for high-frequency metrics
	httpRequestCount     int64
	websocketConnections int64
	
	// HTTP and WebSocket stats (separate mutexes for better concurrency)
	httpMu             sync.RWMutex
	httpStats          HTTPStats
	
	websocketMu        sync.RWMutex
	websocketStats     WebSocketStats
	
	systemMu           sync.RWMutex
	systemStats        SystemStats
	
	// Business metrics using sync.Map for better concurrent performance
	businessMetrics    sync.Map // map[string]float64
	businessMu         sync.RWMutex // Only for LRU operations and eviction stats
	lastUpdate         time.Time
	
	// LRU cardinality management
	maxCardinality     int
	lruList           *list.List
	lruMap            map[string]*list.Element
	evictionStats     EvictionStats
	currentCount      int64 // Atomic counter for current metric count
}

// NewOptimizedCollector creates a new optimized metrics collector
func NewOptimizedCollector() *OptimizedCollector {
	return NewOptimizedCollectorWithCardinality(10000)
}

// NewOptimizedCollectorWithCardinality creates an optimized collector with custom cardinality limit
func NewOptimizedCollectorWithCardinality(maxCardinality int) *OptimizedCollector {
	if maxCardinality <= 0 {
		maxCardinality = 10000 // Default value
	}
	
	return &OptimizedCollector{
		httpStats: HTTPStats{
			RequestsByStatus: make(map[int]int64),
			RequestsByMethod: make(map[string]int64),
		},
		websocketStats: WebSocketStats{
			MessagesByType: make(map[string]int64),
		},
		lastUpdate:     time.Now(),
		maxCardinality: maxCardinality,
		lruList:       list.New(),
		lruMap:        make(map[string]*list.Element),
		evictionStats: EvictionStats{
			EvictedMetrics: make([]string, 0),
		},
	}
}

// RecordHTTPRequest records an HTTP request with optimized locking
func (c *OptimizedCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	// Update atomic counter (no lock needed)
	atomic.AddInt64(&c.httpRequestCount, 1)
	
	// Update aggregated stats with minimal lock time
	c.httpMu.Lock()
	c.httpStats.TotalRequests = atomic.LoadInt64(&c.httpRequestCount)
	c.httpStats.RequestsByStatus[statusCode]++
	c.httpStats.RequestsByMethod[method]++
	
	// Simple rolling average for latency (last 100 requests)
	if c.httpStats.AverageLatency == 0 {
		c.httpStats.AverageLatency = duration
	} else {
		c.httpStats.AverageLatency = (c.httpStats.AverageLatency*99 + duration) / 100
	}
	c.httpStats.LastUpdated = time.Now()
	c.httpMu.Unlock()
}

func (c *OptimizedCollector) RecordHTTPRequestSize(method, path string, size int64) {
	// For simplified collector, we don't need to track individual request sizes
	// This keeps memory usage constant
}

func (c *OptimizedCollector) RecordHTTPResponseSize(method, path string, size int64) {
	// For simplified collector, we don't need to track individual response sizes  
	// This keeps memory usage constant
}

func (c *OptimizedCollector) RecordWebSocketConnection(connected bool) {
	if connected {
		atomic.AddInt64(&c.websocketConnections, 1)
	} else {
		atomic.AddInt64(&c.websocketConnections, -1)
	}
	
	// Update stats with minimal lock
	c.websocketMu.Lock()
	c.websocketStats.ActiveConnections = atomic.LoadInt64(&c.websocketConnections)
	c.websocketStats.LastUpdated = time.Now()
	c.websocketMu.Unlock()
}

func (c *OptimizedCollector) RecordWebSocketMessage(direction string, messageType string, size int64) {
	c.websocketMu.Lock()
	c.websocketStats.TotalMessages++
	
	key := direction + "_" + messageType
	c.websocketStats.MessagesByType[key]++
	c.websocketStats.LastUpdated = time.Now()
	c.websocketMu.Unlock()
}

func (c *OptimizedCollector) RecordBusinessMetric(name string, value float64, tags map[string]string) {
	// Generate metric key
	metricKey := name
	if len(tags) > 0 {
		// Include first tag to maintain some granularity while limiting cardinality
		for k, v := range tags {
			metricKey = fmt.Sprintf("%s{%s=%s}", name, k, v)
			break // Only use first tag to limit cardinality
		}
	}
	
	// Check if this is a new metric (atomic check)
	_, exists := c.businessMetrics.Load(metricKey)
	isNewMetric := !exists
	
	// Store in sync.Map (highly concurrent)
	c.businessMetrics.Store(metricKey, value)
	
	// Handle LRU and cardinality only if it's a new metric
	if isNewMetric {
		newCount := atomic.AddInt64(&c.currentCount, 1)
		
		// Only acquire business lock for LRU operations
		c.businessMu.Lock()
		c.updateLRUCache(metricKey, value)
		
		// Check cardinality limit
		if newCount > int64(c.maxCardinality) {
			c.evictLeastRecentlyUsed()
		}
		c.businessMu.Unlock()
	} else {
		// For existing metrics, update LRU without full lock
		c.businessMu.Lock()
		c.updateLRUCacheExisting(metricKey, value)
		c.businessMu.Unlock()
	}
}

func (c *OptimizedCollector) RecordGoroutines(count int) {
	c.systemMu.Lock()
	c.systemStats.GoroutineCount = count
	c.systemStats.LastUpdated = time.Now()
	c.systemMu.Unlock()
}

func (c *OptimizedCollector) RecordMemoryUsage(bytes uint64) {
	c.systemMu.Lock()
	c.systemStats.MemoryUsage = bytes
	c.systemStats.LastUpdated = time.Now()
	c.systemMu.Unlock()
}

// updateLRUCache adds a new metric to LRU cache (must be called with businessMu held)
func (c *OptimizedCollector) updateLRUCache(key string, value float64) {
	now := time.Now()
	
	// Add new entry
	entry := &lruEntry{
		key:       key,
		value:     value,
		timestamp: now,
	}
	elem := c.lruList.PushFront(entry)
	c.lruMap[key] = elem
}

// updateLRUCacheExisting updates existing metric in LRU cache (must be called with businessMu held)
func (c *OptimizedCollector) updateLRUCacheExisting(key string, value float64) {
	if elem, exists := c.lruMap[key]; exists {
		// Move to front (most recently used)
		c.lruList.MoveToFront(elem)
		// Update value
		entry := elem.Value.(*lruEntry)
		entry.value = value
		entry.timestamp = time.Now()
	}
}

// evictLeastRecentlyUsed removes the least recently used metric (must be called with businessMu held)
func (c *OptimizedCollector) evictLeastRecentlyUsed() {
	if c.lruList.Len() == 0 {
		return
	}
	
	// Get least recently used element (back of list)
	elem := c.lruList.Back()
	if elem == nil {
		return
	}
	
	entry := elem.Value.(*lruEntry)
	evictedKey := entry.key
	
	// Remove from LRU structures
	c.lruList.Remove(elem)
	delete(c.lruMap, evictedKey)
	
	// Remove from sync.Map
	c.businessMetrics.Delete(evictedKey)
	atomic.AddInt64(&c.currentCount, -1)
	
	// Update eviction stats
	c.evictionStats.TotalEvictions++
	c.evictionStats.LastEvictionTime = time.Now()
	
	// Keep track of recently evicted metrics (max 10)
	c.evictionStats.EvictedMetrics = append(c.evictionStats.EvictedMetrics, evictedKey)
	if len(c.evictionStats.EvictedMetrics) > 10 {
		c.evictionStats.EvictedMetrics = c.evictionStats.EvictedMetrics[1:]
	}
}

// GetStats returns current statistics snapshot
func (c *OptimizedCollector) GetStats() map[string]any {
	// Read each section with minimal lock time
	c.httpMu.RLock()
	httpStats := c.httpStats
	c.httpMu.RUnlock()
	
	c.websocketMu.RLock()
	websocketStats := c.websocketStats
	c.websocketMu.RUnlock()
	
	c.systemMu.RLock()
	systemStats := c.systemStats
	c.systemMu.RUnlock()
	
	// Convert sync.Map to regular map for response
	businessMetrics := make(map[string]float64)
	c.businessMetrics.Range(func(key, value interface{}) bool {
		businessMetrics[key.(string)] = value.(float64)
		return true
	})
	
	c.businessMu.RLock()
	evictionStats := c.evictionStats
	c.businessMu.RUnlock()
	
	currentCount := atomic.LoadInt64(&c.currentCount)
	
	return map[string]any{
		"http":       httpStats,
		"websocket":  websocketStats,
		"system":     systemStats,
		"business":   businessMetrics,
		"cardinality": map[string]any{
			"current":    currentCount,
			"max":        c.maxCardinality,
			"evictions":  evictionStats,
		},
		"timestamp": time.Now().Unix(),
	}
}

// GetHTTPStats returns HTTP statistics
func (c *OptimizedCollector) GetHTTPStats() HTTPStats {
	c.httpMu.RLock()
	defer c.httpMu.RUnlock()
	return c.httpStats
}

// GetWebSocketStats returns WebSocket statistics  
func (c *OptimizedCollector) GetWebSocketStats() WebSocketStats {
	c.websocketMu.RLock()
	defer c.websocketMu.RUnlock()
	return c.websocketStats
}

// GetSystemStats returns system statistics
func (c *OptimizedCollector) GetSystemStats() SystemStats {
	c.systemMu.RLock()
	defer c.systemMu.RUnlock()
	return c.systemStats
}

// Reset clears all statistics (useful for testing)
func (c *OptimizedCollector) Reset() {
	atomic.StoreInt64(&c.httpRequestCount, 0)
	atomic.StoreInt64(&c.websocketConnections, 0)
	atomic.StoreInt64(&c.currentCount, 0)
	
	c.httpMu.Lock()
	c.httpStats = HTTPStats{
		RequestsByStatus: make(map[int]int64),
		RequestsByMethod: make(map[string]int64),
	}
	c.httpMu.Unlock()
	
	c.websocketMu.Lock()
	c.websocketStats = WebSocketStats{
		MessagesByType: make(map[string]int64),
	}
	c.websocketMu.Unlock()
	
	c.systemMu.Lock()
	c.systemStats = SystemStats{}
	c.systemMu.Unlock()
	
	// Clear sync.Map
	c.businessMetrics.Range(func(key, value interface{}) bool {
		c.businessMetrics.Delete(key)
		return true
	})
	
	c.businessMu.Lock()
	c.lastUpdate = time.Now()
	c.lruList = list.New()
	c.lruMap = make(map[string]*list.Element)
	c.evictionStats = EvictionStats{
		EvictedMetrics: make([]string, 0),
	}
	c.businessMu.Unlock()
}

// GetEvictionStats returns current eviction statistics
func (c *OptimizedCollector) GetEvictionStats() EvictionStats {
	c.businessMu.RLock()
	defer c.businessMu.RUnlock()
	return c.evictionStats
}

// GetCardinalityInfo returns current cardinality information
func (c *OptimizedCollector) GetCardinalityInfo() map[string]any {
	currentCount := atomic.LoadInt64(&c.currentCount)
	
	c.businessMu.RLock()
	evictionStats := c.evictionStats
	c.businessMu.RUnlock()
	
	return map[string]any{
		"current_metrics": currentCount,
		"max_cardinality": c.maxCardinality,
		"utilization":     float64(currentCount) / float64(c.maxCardinality) * 100,
		"evictions":       evictionStats,
	}
}