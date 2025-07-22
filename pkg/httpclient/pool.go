package httpclient

import (
	"fmt"
	"sync"
	"time"
)

// Pool manages a pool of HTTP clients with different configurations
type Pool struct {
	mu      sync.RWMutex
	clients map[string]*Client
	factory ClientFactory
}

// ClientFactory creates clients with specific configurations
type ClientFactory func(name string) *Client

// NewPool creates a new client pool
func NewPool() *Pool {
	return &Pool{
		clients: make(map[string]*Client),
		factory: defaultFactory,
	}
}

// NewPoolWithFactory creates a new client pool with custom factory
func NewPoolWithFactory(factory ClientFactory) *Pool {
	return &Pool{
		clients: make(map[string]*Client),
		factory: factory,
	}
}

// defaultFactory creates clients with default configuration
func defaultFactory(name string) *Client {
	config := DefaultConfig()
	
	// Customize based on client name
	switch name {
	case "internal":
		config.Timeout = 5 * time.Second
		config.MaxIdleConnsPerHost = 20
	case "external":
		config.Timeout = 30 * time.Second
		config.MaxIdleConnsPerHost = 5
	case "long-poll":
		config.Timeout = 5 * time.Minute
		config.MaxIdleConnsPerHost = 2
	}
	
	return New(config)
}

// Get returns a client from the pool, creating it if necessary
func (p *Pool) Get(name string) *Client {
	// Try read lock first
	p.mu.RLock()
	client, exists := p.clients[name]
	p.mu.RUnlock()
	
	if exists {
		return client
	}
	
	// Create new client
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Double-check after acquiring write lock
	if client, exists = p.clients[name]; exists {
		return client
	}
	
	// Create new client
	client = p.factory(name)
	p.clients[name] = client
	
	return client
}

// GetDefault returns the default client
func (p *Pool) GetDefault() *Client {
	return p.Get("default")
}

// Set adds or replaces a client in the pool
func (p *Pool) Set(name string, client *Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Close old client if exists
	if old, exists := p.clients[name]; exists {
		old.Close()
	}
	
	p.clients[name] = client
}

// Remove removes a client from the pool and closes it
func (p *Pool) Remove(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if client, exists := p.clients[name]; exists {
		client.Close()
		delete(p.clients, name)
	}
}

// Close closes all clients in the pool
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for name, client := range p.clients {
		client.Close()
		delete(p.clients, name)
	}
}

// GetMetrics returns metrics for a specific client
func (p *Pool) GetMetrics(name string) (ClientMetrics, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	client, exists := p.clients[name]
	if !exists {
		return ClientMetrics{}, fmt.Errorf("client %s not found", name)
	}
	
	return client.GetMetrics(), nil
}

// GetAllMetrics returns metrics for all clients
func (p *Pool) GetAllMetrics() map[string]ClientMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	metrics := make(map[string]ClientMetrics)
	for name, client := range p.clients {
		metrics[name] = client.GetMetrics()
	}
	
	return metrics
}

// Size returns the number of clients in the pool
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return len(p.clients)
}

// Names returns all client names in the pool
func (p *Pool) Names() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	names := make([]string, 0, len(p.clients))
	for name := range p.clients {
		names = append(names, name)
	}
	
	return names
}