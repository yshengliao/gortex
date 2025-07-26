package metrics

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ImprovedCollector is a lightweight metrics collector that maintains
// current state without unbounded memory growth with LRU cardinality limits
type ImprovedCollector struct {
	// Atomic counters for high-frequency metrics
	httpRequestCount     int64
	websocketConnections int64
	
	// Current state (updated periodically)
	mu                   sync.RWMutex
	httpStats           HTTPStats
	websocketStats      WebSocketStats
	systemStats         SystemStats
	businessMetrics     map[string]float64
	lastUpdate          time.Time
	
	// LRU cardinality management
	maxCardinality      int
	lruList            *list.List
	lruMap             map[string]*list.Element
	evictionStats      EvictionStats
}

// EvictionStats tracks metrics eviction information
type EvictionStats struct {
	TotalEvictions     int64     `json:"total_evictions"`
	LastEvictionTime   time.Time `json:"last_eviction_time"`
	EvictedMetrics     []string  `json:"recently_evicted_metrics"`
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key       string
	value     float64
	timestamp time.Time
}

// HTTPStats holds aggregated HTTP statistics
type HTTPStats struct {
	TotalRequests    int64                      `json:"total_requests"`
	RequestsByStatus map[int]int64              `json:"requests_by_status"`
	RequestsByMethod map[string]int64           `json:"requests_by_method"`
	AverageLatency   time.Duration              `json:"average_latency"`
	LastUpdated      time.Time                  `json:"last_updated"`
}

// WebSocketStats holds WebSocket connection statistics
type WebSocketStats struct {
	ActiveConnections int64     `json:"active_connections"`
	TotalMessages     int64     `json:"total_messages"`
	MessagesByType    map[string]int64 `json:"messages_by_type"`
	LastUpdated       time.Time `json:"last_updated"`
}

// SystemStats holds system-level metrics
type SystemStats struct {
	GoroutineCount  int      `json:"goroutine_count"`
	MemoryUsage     uint64   `json:"memory_usage_bytes"`
	LastUpdated     time.Time `json:"last_updated"`
}

// HTTPRequestMetric represents an HTTP request metric
type HTTPRequestMetric struct {
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	Timestamp  time.Time
}

// WebSocketMessageMetric represents a WebSocket message metric
type WebSocketMessageMetric struct {
	Direction   string // "inbound" or "outbound"
	MessageType string
	Size        int64
	Timestamp   time.Time
}

// BusinessMetric represents a custom business metric
type BusinessMetric struct {
	Name      string
	Value     float64
	Tags      map[string]string
	Timestamp time.Time
}

// NewImprovedCollector creates a new lightweight metrics collector
func NewImprovedCollector() *ImprovedCollector {
	return NewImprovedCollectorWithCardinality(10000)
}

// NewImprovedCollectorWithCardinality creates a collector with custom cardinality limit
func NewImprovedCollectorWithCardinality(maxCardinality int) *ImprovedCollector {
	if maxCardinality <= 0 {
		maxCardinality = 10000 // Default value
	}
	
	return &ImprovedCollector{
		businessMetrics: make(map[string]float64),
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

// RecordHTTPRequest records an HTTP request using atomic operations
func (c *ImprovedCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	// Update atomic counter
	atomic.AddInt64(&c.httpRequestCount, 1)
	
	// Update aggregated stats (less frequent, with lock)
	c.mu.Lock()
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
	c.mu.Unlock()
}

func (c *ImprovedCollector) RecordHTTPRequestSize(method, path string, size int64) {
	// For simplified collector, we don't need to track individual request sizes
	// This keeps memory usage constant
}

func (c *ImprovedCollector) RecordHTTPResponseSize(method, path string, size int64) {
	// For simplified collector, we don't need to track individual response sizes
	// This keeps memory usage constant
}

func (c *ImprovedCollector) RecordWebSocketConnection(connected bool) {
	if connected {
		atomic.AddInt64(&c.websocketConnections, 1)
	} else {
		atomic.AddInt64(&c.websocketConnections, -1)
	}
	
	// Update stats
	c.mu.Lock()
	c.websocketStats.ActiveConnections = atomic.LoadInt64(&c.websocketConnections)
	c.websocketStats.LastUpdated = time.Now()
	c.mu.Unlock()
}

func (c *ImprovedCollector) RecordWebSocketMessage(direction string, messageType string, size int64) {
	c.mu.Lock()
	c.websocketStats.TotalMessages++
	
	key := direction + "_" + messageType
	c.websocketStats.MessagesByType[key]++
	c.websocketStats.LastUpdated = time.Now()
	c.mu.Unlock()
}

func (c *ImprovedCollector) RecordBusinessMetric(name string, value float64, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Generate metric key (simple key without complex tag handling to avoid memory growth)
	metricKey := name
	if len(tags) > 0 {
		// Include first tag to maintain some granularity while limiting cardinality
		for k, v := range tags {
			metricKey = fmt.Sprintf("%s{%s=%s}", name, k, v)
			break // Only use first tag to limit cardinality
		}
	}
	
	// Update LRU cache
	c.updateLRUCache(metricKey, value)
	
	// Update business metrics map
	c.businessMetrics[metricKey] = value
}

func (c *ImprovedCollector) RecordGoroutines(count int) {
	c.mu.Lock()
	c.systemStats.GoroutineCount = count
	c.systemStats.LastUpdated = time.Now()
	c.mu.Unlock()
}

func (c *ImprovedCollector) RecordMemoryUsage(bytes uint64) {
	c.mu.Lock()
	c.systemStats.MemoryUsage = bytes
	c.systemStats.LastUpdated = time.Now()
	c.mu.Unlock()
}

// updateLRUCache updates the LRU cache for business metrics
// This method must be called with the mutex already held
func (c *ImprovedCollector) updateLRUCache(key string, value float64) {
	now := time.Now()
	
	// Check if key already exists
	if elem, exists := c.lruMap[key]; exists {
		// Move to front (most recently used)
		c.lruList.MoveToFront(elem)
		// Update value
		entry := elem.Value.(*lruEntry)
		entry.value = value
		entry.timestamp = now
		return
	}
	
	// Check if we need to evict
	if len(c.lruMap) >= c.maxCardinality {
		c.evictLeastRecentlyUsed()
	}
	
	// Add new entry
	entry := &lruEntry{
		key:       key,
		value:     value,
		timestamp: now,
	}
	elem := c.lruList.PushFront(entry)
	c.lruMap[key] = elem
}

// evictLeastRecentlyUsed removes the least recently used metric
// This method must be called with the mutex already held
func (c *ImprovedCollector) evictLeastRecentlyUsed() {
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
	
	// Remove from both list and map
	c.lruList.Remove(elem)
	delete(c.lruMap, evictedKey)
	delete(c.businessMetrics, evictedKey)
	
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
func (c *ImprovedCollector) GetStats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return map[string]any{
		"http":       c.httpStats,
		"websocket":  c.websocketStats,
		"system":     c.systemStats,
		"business":   c.businessMetrics,
		"cardinality": map[string]any{
			"current":    len(c.businessMetrics),
			"max":        c.maxCardinality,
			"evictions":  c.evictionStats,
		},
		"timestamp": time.Now().Unix(),
	}
}

// GetHTTPStats returns HTTP statistics
func (c *ImprovedCollector) GetHTTPStats() HTTPStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.httpStats
}

// GetWebSocketStats returns WebSocket statistics  
func (c *ImprovedCollector) GetWebSocketStats() WebSocketStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.websocketStats
}

// GetSystemStats returns system statistics
func (c *ImprovedCollector) GetSystemStats() SystemStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.systemStats
}

// Reset clears all statistics (useful for testing)
func (c *ImprovedCollector) Reset() {
	atomic.StoreInt64(&c.httpRequestCount, 0)
	atomic.StoreInt64(&c.websocketConnections, 0)
	
	c.mu.Lock()
	c.httpStats = HTTPStats{
		RequestsByStatus: make(map[int]int64),
		RequestsByMethod: make(map[string]int64),
	}
	c.websocketStats = WebSocketStats{
		MessagesByType: make(map[string]int64),
	}
	c.systemStats = SystemStats{}
	c.businessMetrics = make(map[string]float64)
	c.lastUpdate = time.Now()
	
	// Reset LRU cache
	c.lruList = list.New()
	c.lruMap = make(map[string]*list.Element)
	c.evictionStats = EvictionStats{
		EvictedMetrics: make([]string, 0),
	}
	c.mu.Unlock()
}

// GetEvictionStats returns current eviction statistics
func (c *ImprovedCollector) GetEvictionStats() EvictionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictionStats
}

// GetCardinalityInfo returns current cardinality information
func (c *ImprovedCollector) GetCardinalityInfo() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return map[string]any{
		"current_metrics": len(c.businessMetrics),
		"max_cardinality": c.maxCardinality,
		"utilization":     float64(len(c.businessMetrics)) / float64(c.maxCardinality) * 100,
		"evictions":       c.evictionStats,
	}
}