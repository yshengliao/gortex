# WebSocket Example - Real-time Communication

This example demonstrates WebSocket support using the `hijack:"ws"` struct tag.

## Key Features

### 1. WebSocket Route with Hijack Tag
```go
type HandlersManager struct {
    // Regular HTTP endpoints
    Home   *HomeHandler   `url:"/"`
    Status *StatusHandler `url:"/status"`
    
    // WebSocket endpoint with hijack tag
    WS     *WSHandler     `url:"/ws" hijack:"ws"`
}
```

The `hijack:"ws"` tag tells the framework this route handles WebSocket upgrades.

### 2. Built-in Hub Pattern
```go
// Create WebSocket hub
wsHub := hub.NewHub(logger)
go wsHub.Run()

// Hub manages all connected clients
// - Automatic client tracking
// - Message broadcasting
// - Graceful shutdown support
```

### 3. WebSocket Metrics
```go
metrics := hub.GetMetrics()
// Returns:
// - Current connections
// - Total connections  
// - Messages sent/received
// - Message rates
```

## Running the Example

```bash
go run main.go
```

Then open http://localhost:8082 in your browser.

## Testing WebSocket

1. **Browser Client**: Open http://localhost:8082 for interactive chat
2. **Check Status**: 
```bash
curl http://localhost:8082/status
```

3. **Broadcast Message**:
```bash
curl -X POST http://localhost:8082/api/broadcast \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello everyone!"}'
```

## WebSocket Flow

1. Client connects to `/ws`
2. Framework upgrades HTTP → WebSocket
3. Hub registers the client
4. Client can send/receive messages
5. On disconnect, hub cleans up

## Graceful Shutdown

The example includes WebSocket graceful shutdown:
```go
application.OnShutdown(func(ctx context.Context) error {
    return wsHub.ShutdownWithTimeout(5 * time.Second)
})
```

This ensures all clients receive proper close messages before shutdown.

## Production Considerations

1. **Authentication**: Add JWT validation before upgrade
2. **Rate Limiting**: Limit messages per client
3. **Message Size**: Set max message size limits
4. **Compression**: Enable per-message compression
5. **Rooms/Channels**: Implement topic-based messaging

This example shows how Gortex makes WebSocket as simple as adding a struct tag!