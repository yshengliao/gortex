// Package main demonstrates Gortex's WebSocket hub with the hardened
// defaults from PR 2: per-frame read limits, an allowed-type whitelist,
// and a message authorizer hook that can drop unwanted traffic.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gorillaWS "github.com/gorilla/websocket"
	"github.com/yshengliao/gortex/core/app"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/transport/websocket"
	"go.uber.org/zap"
)

// ChatHandler upgrades incoming HTTP requests to WebSocket and hands the
// connection to the hub. The `hijack:"ws"` tag tells the router to bypass
// the normal HTTP method fan-out so HandleConnection runs for every verb.
type ChatHandler struct {
	Hub    *websocket.Hub
	Logger *zap.Logger

	upgrader gorillaWS.Upgrader
}

// newChatHandler constructs the handler with an Upgrader that accepts any
// origin — fine for a local demo, never for production.
func newChatHandler(hub *websocket.Hub, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		Hub:    hub,
		Logger: logger,
		upgrader: gorillaWS.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *ChatHandler) HandleConnection(c httpctx.Context) error {
	userID := c.QueryParam("user")
	if userID == "" {
		userID = "anon"
	}
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	client := websocket.NewClient(h.Hub, conn, userID, h.Logger)
	h.Hub.RegisterClient(client)

	go client.WritePump()
	go client.ReadPump()
	return nil
}

// Handlers wires routes declaratively. The chat endpoint opts into
// hijack:"ws" so Gortex treats it as a WebSocket upgrade point.
type Handlers struct {
	Chat *ChatHandler `url:"/chat" hijack:"ws"`
}

// chatAuthorizer enforces two demo-level policies: reject the special
// "banned" user, and require that chat messages carry a non-empty "text"
// field. Real deployments would check a session, a room membership, or
// abuse heuristics here.
func chatAuthorizer(client *websocket.Client, msg *websocket.Message) error {
	if client.UserID == "banned" {
		return websocket.ErrMessageUnauthorized
	}
	if msg.Type == "chat" {
		if text, ok := msg.Data["text"].(string); !ok || text == "" {
			return errors.New("chat message requires a non-empty text field")
		}
	}
	return nil
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	hub := websocket.NewHubWithConfig(logger, websocket.Config{
		MaxMessageBytes:     4 << 10, // 4 KiB — plenty for chat
		AllowedMessageTypes: []string{"chat", "ping"},
		Authorizer:          chatAuthorizer,
	})
	go hub.Run()

	handlers := &Handlers{
		Chat: newChatHandler(hub, logger),
	}

	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		logger.Fatal("failed to create app", zap.Error(err))
	}
	application.OnShutdown(func(ctx context.Context) error {
		return hub.ShutdownWithTimeout(2 * time.Second)
	})

	go func() {
		if err := application.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server exited", zap.Error(err))
		}
	}()
	logger.Info("websocket example listening on :8080 (ws://localhost:8080/chat?user=alice)")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
