package httpclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Config defines HTTP client configuration
type Config struct {
	// Transport settings
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
	
	// Client timeout
	Timeout time.Duration
	
	// Connection settings
	DialTimeout         time.Duration
	TLSHandshakeTimeout time.Duration
	KeepAlive           time.Duration
	
	// TLS configuration
	InsecureSkipVerify bool
	
	// Metrics collection
	EnableMetrics bool
}

// DefaultConfig returns default client configuration
func DefaultConfig() Config {
	return Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     0, // No limit
		IdleConnTimeout:     90 * time.Second,
		Timeout:             30 * time.Second,
		DialTimeout:         10 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		KeepAlive:           30 * time.Second,
		InsecureSkipVerify:  false,
		EnableMetrics:       true,
	}
}

// Client is an HTTP client with connection pooling and metrics
type Client struct {
	*http.Client
	config  Config
	metrics *Metrics
}

// Metrics tracks HTTP client metrics
type Metrics struct {
	// Connection metrics
	ActiveConnections   int64
	IdleConnections     int64
	TotalConnections    int64
	ConnectionReuse     int64
	
	// Request metrics
	TotalRequests       int64
	TotalResponses      int64
	TotalErrors         int64
	
	// Response time tracking
	totalResponseTime   int64
	requestCount        int64
	
	// Status code tracking
	statusCodes         sync.Map // map[int]int64
}

// New creates a new HTTP client with connection pooling
func New(config Config) *Client {
	// Create custom dialer
	dialer := &net.Dialer{
		Timeout:   config.DialTimeout,
		KeepAlive: config.KeepAlive,
	}
	
	// Create transport with connection pooling
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}
	
	client := &Client{
		Client: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		},
		config:  config,
		metrics: &Metrics{},
	}
	
	// Wrap transport with metrics if enabled
	if config.EnableMetrics {
		client.Client.Transport = &metricsTransport{
			base:    transport,
			metrics: client.metrics,
		}
	}
	
	return client
}

// NewDefault creates a new HTTP client with default configuration
func NewDefault() *Client {
	return New(DefaultConfig())
}

// Do performs an HTTP request with metrics tracking
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.config.EnableMetrics {
		atomic.AddInt64(&c.metrics.TotalRequests, 1)
	}
	
	start := time.Now()
	resp, err := c.Client.Do(req)
	elapsed := time.Since(start)
	
	if c.config.EnableMetrics {
		if err != nil {
			atomic.AddInt64(&c.metrics.TotalErrors, 1)
		} else {
			atomic.AddInt64(&c.metrics.TotalResponses, 1)
			atomic.AddInt64(&c.metrics.totalResponseTime, elapsed.Nanoseconds())
			atomic.AddInt64(&c.metrics.requestCount, 1)
			
			// Track status code
			if resp != nil {
				c.trackStatusCode(resp.StatusCode)
			}
		}
	}
	
	return resp, err
}

// DoWithContext performs an HTTP request with context
func (c *Client) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.Do(req.WithContext(ctx))
}

// GetMetrics returns current client metrics
func (c *Client) GetMetrics() ClientMetrics {
	if !c.config.EnableMetrics || c.metrics == nil {
		return ClientMetrics{}
	}
	
	// Get transport metrics
	var transportMetrics TransportMetrics
	if mt, ok := c.Client.Transport.(*metricsTransport); ok {
		if transport, ok := mt.base.(*http.Transport); ok {
			transportMetrics = getTransportMetrics(transport)
		}
	} else if transport, ok := c.Client.Transport.(*http.Transport); ok {
		transportMetrics = getTransportMetrics(transport)
	}
	
	// Calculate average response time
	totalTime := atomic.LoadInt64(&c.metrics.totalResponseTime)
	count := atomic.LoadInt64(&c.metrics.requestCount)
	avgResponseTime := time.Duration(0)
	if count > 0 {
		avgResponseTime = time.Duration(totalTime / count)
	}
	
	// Collect status codes
	statusCodes := make(map[int]int64)
	c.metrics.statusCodes.Range(func(key, value any) bool {
		if code, ok := key.(int); ok {
			if count, ok := value.(*int64); ok {
				statusCodes[code] = atomic.LoadInt64(count)
			}
		}
		return true
	})
	
	return ClientMetrics{
		ActiveConnections:   atomic.LoadInt64(&c.metrics.ActiveConnections),
		IdleConnections:     int64(transportMetrics.IdleConns),
		TotalConnections:    atomic.LoadInt64(&c.metrics.TotalConnections),
		ConnectionReuse:     atomic.LoadInt64(&c.metrics.ConnectionReuse),
		TotalRequests:       atomic.LoadInt64(&c.metrics.TotalRequests),
		TotalResponses:      atomic.LoadInt64(&c.metrics.TotalResponses),
		TotalErrors:         atomic.LoadInt64(&c.metrics.TotalErrors),
		AverageResponseTime: avgResponseTime,
		StatusCodes:         statusCodes,
		TransportMetrics:    transportMetrics,
	}
}

// ClientMetrics contains HTTP client metrics
type ClientMetrics struct {
	// Connection metrics
	ActiveConnections   int64
	IdleConnections     int64
	TotalConnections    int64
	ConnectionReuse     int64
	
	// Request metrics
	TotalRequests       int64
	TotalResponses      int64
	TotalErrors         int64
	AverageResponseTime time.Duration
	
	// Status code distribution
	StatusCodes map[int]int64
	
	// Transport-level metrics
	TransportMetrics TransportMetrics
}

// TransportMetrics contains transport-level metrics
type TransportMetrics struct {
	IdleConns        int
	IdleConnsPerHost map[string]int
}

// metricsTransport wraps http.Transport to collect metrics
type metricsTransport struct {
	base    http.RoundTripper
	metrics *Metrics
}

func (t *metricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Track active connections
	atomic.AddInt64(&t.metrics.ActiveConnections, 1)
	defer atomic.AddInt64(&t.metrics.ActiveConnections, -1)
	
	// Check if this is a reused connection
	if req.Header.Get("Connection") != "close" {
		atomic.AddInt64(&t.metrics.ConnectionReuse, 1)
	} else {
		atomic.AddInt64(&t.metrics.TotalConnections, 1)
	}
	
	return t.base.RoundTrip(req)
}

func (c *Client) trackStatusCode(code int) {
	key := code
	
	// Load or create counter
	val, _ := c.metrics.statusCodes.LoadOrStore(key, new(int64))
	if counter, ok := val.(*int64); ok {
		atomic.AddInt64(counter, 1)
	}
}

func getTransportMetrics(transport *http.Transport) TransportMetrics {
	// Note: These methods are not public in http.Transport
	// In a real implementation, we would need to use reflection or
	// maintain our own connection tracking
	return TransportMetrics{
		IdleConns:        0, // Would need reflection or custom tracking
		IdleConnsPerHost: make(map[string]int),
	}
}

// Close closes idle connections
func (c *Client) Close() {
	if transport, ok := c.Client.Transport.(*metricsTransport); ok {
		if t, ok := transport.base.(*http.Transport); ok {
			t.CloseIdleConnections()
		}
	} else if transport, ok := c.Client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}