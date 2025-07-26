package metrics

import (
	"container/list"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// ShardedCollector uses sharded locks to reduce contention
type ShardedCollector struct {
	// Atomic counters for high-frequency metrics
	httpRequestCount     int64
	websocketConnections int64
	
	// HTTP stats with minimal locking
	httpMu             sync.RWMutex
	httpStats          HTTPStats
	
	// WebSocket stats with minimal locking
	websocketMu        sync.RWMutex
	websocketStats     WebSocketStats
	
	// System stats with minimal locking
	systemMu           sync.RWMutex
	systemStats        SystemStats
	lastUpdate         time.Time
	
	// Business metrics with sharded locks
	shardCount         int
	shards            []*metricShard
	maxCardinality     int
	globalEvictionStats EvictionStats
	globalMu           sync.RWMutex // For global eviction stats only
}

// metricShard holds a subset of business metrics with its own lock
type metricShard struct {
	mu              sync.RWMutex
	metrics         map[string]float64
	lruList        *list.List
	lruMap         map[string]*list.Element
	evictionStats  EvictionStats
	currentCount   int64
	maxCount       int64 // Max metrics per shard
}

// NewShardedCollector creates a collector with sharded business metrics
func NewShardedCollector() *ShardedCollector {
	return NewShardedCollectorWithCardinality(10000)
}

// NewShardedCollectorWithCardinality creates a sharded collector with custom cardinality limit
func NewShardedCollectorWithCardinality(maxCardinality int) *ShardedCollector {
	if maxCardinality <= 0 {
		maxCardinality = 10000
	}
	
	// Use number of CPUs as shard count for optimal performance
	shardCount := 16 // Fixed number for predictable performance
	maxPerShard := int64(maxCardinality / shardCount)
	if maxPerShard < 1 {
		maxPerShard = 1
	}
	
	shards := make([]*metricShard, shardCount)
	for i := 0; i < shardCount; i++ {
		shards[i] = &metricShard{
			metrics:   make(map[string]float64),
			lruList:   list.New(),
			lruMap:    make(map[string]*list.Element),
			maxCount:  maxPerShard,
			evictionStats: EvictionStats{
				EvictedMetrics: make([]string, 0),
			},
		}
	}
	
	return &ShardedCollector{
		httpStats: HTTPStats{
			RequestsByStatus: make(map[int]int64),
			RequestsByMethod: make(map[string]int64),
		},
		websocketStats: WebSocketStats{
			MessagesByType: make(map[string]int64),
		},
		lastUpdate:     time.Now(),
		shardCount:     shardCount,
		shards:        shards,
		maxCardinality: maxCardinality,
		globalEvictionStats: EvictionStats{
			EvictedMetrics: make([]string, 0),
		},
	}
}

// hashKey returns a consistent shard index for a metric key
func (c *ShardedCollector) hashKey(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % c.shardCount
}

// RecordHTTPRequest records an HTTP request (same as original)
func (c *ShardedCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	atomic.AddInt64(&c.httpRequestCount, 1)
	
	c.httpMu.Lock()
	c.httpStats.TotalRequests = atomic.LoadInt64(&c.httpRequestCount)
	c.httpStats.RequestsByStatus[statusCode]++
	c.httpStats.RequestsByMethod[method]++
	
	if c.httpStats.AverageLatency == 0 {
		c.httpStats.AverageLatency = duration
	} else {
		c.httpStats.AverageLatency = (c.httpStats.AverageLatency*99 + duration) / 100
	}
	c.httpStats.LastUpdated = time.Now()
	c.httpMu.Unlock()
}

func (c *ShardedCollector) RecordHTTPRequestSize(method, path string, size int64) {
	// No-op for simplified collector
}

func (c *ShardedCollector) RecordHTTPResponseSize(method, path string, size int64) {
	// No-op for simplified collector
}

func (c *ShardedCollector) RecordWebSocketConnection(connected bool) {
	if connected {
		atomic.AddInt64(&c.websocketConnections, 1)
	} else {
		atomic.AddInt64(&c.websocketConnections, -1)
	}
	
	c.websocketMu.Lock()
	c.websocketStats.ActiveConnections = atomic.LoadInt64(&c.websocketConnections)
	c.websocketStats.LastUpdated = time.Now()
	c.websocketMu.Unlock()
}

func (c *ShardedCollector) RecordWebSocketMessage(direction string, messageType string, size int64) {
	c.websocketMu.Lock()
	c.websocketStats.TotalMessages++
	
	key := direction + "_" + messageType
	c.websocketStats.MessagesByType[key]++
	c.websocketStats.LastUpdated = time.Now()
	c.websocketMu.Unlock()
}

// RecordBusinessMetric records a business metric using sharded locks
func (c *ShardedCollector) RecordBusinessMetric(name string, value float64, tags map[string]string) {
	// Generate metric key
	metricKey := name
	if len(tags) > 0 {
		for k, v := range tags {
			metricKey = fmt.Sprintf("%s{%s=%s}", name, k, v)
			break
		}
	}
	
	// Determine shard
	shardIndex := c.hashKey(metricKey)
	shard := c.shards[shardIndex]
	
	// Only lock the specific shard
	shard.mu.Lock()
	defer shard.mu.Unlock()
	
	// Check if metric exists
	_, exists := shard.metrics[metricKey]
	
	// Update metric value
	shard.metrics[metricKey] = value
	
	// Handle LRU for this shard
	if exists {
		// Update existing entry in LRU
		if elem, found := shard.lruMap[metricKey]; found {
			shard.lruList.MoveToFront(elem)
			entry := elem.Value.(*lruEntry)
			entry.value = value
			entry.timestamp = time.Now()
		}
	} else {
		// New metric
		atomic.AddInt64(&shard.currentCount, 1)
		
		// Check if we need to evict from this shard
		if shard.currentCount > shard.maxCount {
			c.evictFromShard(shard)
		}
		
		// Add to LRU
		entry := &lruEntry{
			key:       metricKey,
			value:     value,
			timestamp: time.Now(),
		}
		elem := shard.lruList.PushFront(entry)
		shard.lruMap[metricKey] = elem
	}
}

// evictFromShard evicts the least recently used metric from a specific shard
// Must be called with shard lock held
func (c *ShardedCollector) evictFromShard(shard *metricShard) {
	if shard.lruList.Len() == 0 {
		return
	}
	
	// Get least recently used element
	elem := shard.lruList.Back()
	if elem == nil {
		return
	}
	
	entry := elem.Value.(*lruEntry)
	evictedKey := entry.key
	
	// Remove from shard
	shard.lruList.Remove(elem)
	delete(shard.lruMap, evictedKey)
	delete(shard.metrics, evictedKey)
	atomic.AddInt64(&shard.currentCount, -1)
	
	// Update shard eviction stats
	shard.evictionStats.TotalEvictions++
	shard.evictionStats.LastEvictionTime = time.Now()
	shard.evictionStats.EvictedMetrics = append(shard.evictionStats.EvictedMetrics, evictedKey)
	if len(shard.evictionStats.EvictedMetrics) > 5 { // Keep fewer per shard
		shard.evictionStats.EvictedMetrics = shard.evictionStats.EvictedMetrics[1:]
	}
	
	// Update global stats (minimal lock)
	c.globalMu.Lock()
	c.globalEvictionStats.TotalEvictions++
	c.globalEvictionStats.LastEvictionTime = time.Now()
	c.globalEvictionStats.EvictedMetrics = append(c.globalEvictionStats.EvictedMetrics, evictedKey)
	if len(c.globalEvictionStats.EvictedMetrics) > 10 {
		c.globalEvictionStats.EvictedMetrics = c.globalEvictionStats.EvictedMetrics[1:]
	}
	c.globalMu.Unlock()
}

func (c *ShardedCollector) RecordGoroutines(count int) {
	c.systemMu.Lock()
	c.systemStats.GoroutineCount = count
	c.systemStats.LastUpdated = time.Now()
	c.systemMu.Unlock()
}

func (c *ShardedCollector) RecordMemoryUsage(bytes uint64) {
	c.systemMu.Lock()
	c.systemStats.MemoryUsage = bytes
	c.systemStats.LastUpdated = time.Now()
	c.systemMu.Unlock()
}

// GetStats aggregates statistics from all shards
func (c *ShardedCollector) GetStats() map[string]any {
	// Get non-sharded stats
	c.httpMu.RLock()
	httpStats := c.httpStats
	c.httpMu.RUnlock()
	
	c.websocketMu.RLock()
	websocketStats := c.websocketStats
	c.websocketMu.RUnlock()
	
	c.systemMu.RLock()
	systemStats := c.systemStats
	c.systemMu.RUnlock()
	
	// Aggregate business metrics from all shards
	businessMetrics := make(map[string]float64)
	var totalCurrentCount int64
	
	for _, shard := range c.shards {
		shard.mu.RLock()
		for k, v := range shard.metrics {
			businessMetrics[k] = v
		}
		totalCurrentCount += atomic.LoadInt64(&shard.currentCount)
		shard.mu.RUnlock()
	}
	
	c.globalMu.RLock()
	evictionStats := c.globalEvictionStats
	c.globalMu.RUnlock()
	
	return map[string]any{
		"http":       httpStats,
		"websocket":  websocketStats,
		"system":     systemStats,
		"business":   businessMetrics,
		"cardinality": map[string]any{
			"current":    totalCurrentCount,
			"max":        c.maxCardinality,
			"evictions":  evictionStats,
		},
		"timestamp": time.Now().Unix(),
	}
}

// GetHTTPStats returns HTTP statistics
func (c *ShardedCollector) GetHTTPStats() HTTPStats {
	c.httpMu.RLock()
	defer c.httpMu.RUnlock()
	return c.httpStats
}

// GetWebSocketStats returns WebSocket statistics
func (c *ShardedCollector) GetWebSocketStats() WebSocketStats {
	c.websocketMu.RLock()
	defer c.websocketMu.RUnlock()
	return c.websocketStats
}

// GetSystemStats returns system statistics
func (c *ShardedCollector) GetSystemStats() SystemStats {
	c.systemMu.RLock()
	defer c.systemMu.RUnlock()
	return c.systemStats
}

// Reset clears all statistics
func (c *ShardedCollector) Reset() {
	atomic.StoreInt64(&c.httpRequestCount, 0)
	atomic.StoreInt64(&c.websocketConnections, 0)
	
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
	c.lastUpdate = time.Now()
	c.systemMu.Unlock()
	
	// Reset all shards
	for _, shard := range c.shards {
		shard.mu.Lock()
		shard.metrics = make(map[string]float64)
		shard.lruList = list.New()
		shard.lruMap = make(map[string]*list.Element)
		shard.evictionStats = EvictionStats{
			EvictedMetrics: make([]string, 0),
		}
		atomic.StoreInt64(&shard.currentCount, 0)
		shard.mu.Unlock()
	}
	
	c.globalMu.Lock()
	c.globalEvictionStats = EvictionStats{
		EvictedMetrics: make([]string, 0),
	}
	c.globalMu.Unlock()
}

// GetEvictionStats returns aggregated eviction statistics
func (c *ShardedCollector) GetEvictionStats() EvictionStats {
	c.globalMu.RLock()
	defer c.globalMu.RUnlock()
	return c.globalEvictionStats
}

// GetCardinalityInfo returns current cardinality information
func (c *ShardedCollector) GetCardinalityInfo() map[string]any {
	var totalCurrentCount int64
	
	for _, shard := range c.shards {
		totalCurrentCount += atomic.LoadInt64(&shard.currentCount)
	}
	
	c.globalMu.RLock()
	evictionStats := c.globalEvictionStats
	c.globalMu.RUnlock()
	
	return map[string]any{
		"current_metrics": totalCurrentCount,
		"max_cardinality": c.maxCardinality,
		"utilization":     float64(totalCurrentCount) / float64(c.maxCardinality) * 100,
		"evictions":       evictionStats,
	}
}