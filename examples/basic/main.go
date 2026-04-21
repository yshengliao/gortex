// Package main demonstrates the minimum Gortex setup: struct-tag
// routing, the built-in binder, and the default middleware chain
// (recovery, request-id, logger, CORS, error handler, gzip).
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/yshengliao/gortex/core/app"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// Todo is the example domain model.
type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// store is an in-memory map guarded by a mutex — enough for the demo.
type store struct {
	mu    sync.Mutex
	next  int
	items map[int]*Todo
}

func newStore() *store {
	return &store{items: make(map[int]*Todo)}
}

func (s *store) list() []*Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Todo, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *store) get(id int) (*Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	return t, ok
}

func (s *store) add(title string) *Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	t := &Todo{ID: s.next, Title: title}
	s.items[t.ID] = t
	return t
}

func (s *store) update(id int, title *string, done *bool) (*Todo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.items[id]
	if !ok {
		return nil, false
	}
	if title != nil {
		t.Title = *title
	}
	if done != nil {
		t.Done = *done
	}
	return t, true
}

func (s *store) delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return false
	}
	delete(s.items, id)
	return true
}

// TodosHandler mounts at /todos.
type TodosHandler struct {
	Store *store
}

func (h *TodosHandler) GET(c httpctx.Context) error {
	return c.JSON(http.StatusOK, h.Store.list())
}

type createReq struct {
	Title string `json:"title"`
}

func (h *TodosHandler) POST(c httpctx.Context) error {
	var req createReq
	if err := c.Bind(&req); err != nil {
		return httpctx.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Title == "" {
		return httpctx.NewHTTPError(http.StatusBadRequest, "title is required")
	}
	return c.JSON(http.StatusCreated, h.Store.add(req.Title))
}

// TodoHandler mounts at /todos/:id.
type TodoHandler struct {
	Store *store
}

func (h *TodoHandler) idOrError(c httpctx.Context) (int, error) {
	raw := c.Param("id")
	if raw == "" {
		return 0, httpctx.NewHTTPError(http.StatusBadRequest, "id is required")
	}
	var id int
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch < '0' || ch > '9' {
			return 0, httpctx.NewHTTPError(http.StatusBadRequest, "id must be numeric")
		}
		id = id*10 + int(ch-'0')
	}
	return id, nil
}

func (h *TodoHandler) GET(c httpctx.Context) error {
	id, err := h.idOrError(c)
	if err != nil {
		return err
	}
	t, ok := h.Store.get(id)
	if !ok {
		return httpctx.NewHTTPError(http.StatusNotFound, "todo not found")
	}
	return c.JSON(http.StatusOK, t)
}

type updateReq struct {
	Title *string `json:"title,omitempty"`
	Done  *bool   `json:"done,omitempty"`
}

func (h *TodoHandler) PATCH(c httpctx.Context) error {
	id, err := h.idOrError(c)
	if err != nil {
		return err
	}
	var req updateReq
	if err := c.Bind(&req); err != nil {
		return httpctx.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	t, ok := h.Store.update(id, req.Title, req.Done)
	if !ok {
		return httpctx.NewHTTPError(http.StatusNotFound, "todo not found")
	}
	return c.JSON(http.StatusOK, t)
}

func (h *TodoHandler) DELETE(c httpctx.Context) error {
	id, err := h.idOrError(c)
	if err != nil {
		return err
	}
	if !h.Store.delete(id) {
		return httpctx.NewHTTPError(http.StatusNotFound, "todo not found")
	}
	return c.NoContent(http.StatusNoContent)
}

// Handlers binds the declarative routes to concrete types. The Store
// field on each handler is populated directly below — Gortex's
// `inject:""` DI facility is documented as a TODO, so wiring by hand is
// the reliable approach for now.
type Handlers struct {
	Todos *TodosHandler `url:"/todos"`
	Todo  *TodoHandler  `url:"/todos/:id"`
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	s := newStore()
	handlers := &Handlers{
		Todos: &TodosHandler{Store: s},
		Todo:  &TodoHandler{Store: s},
	}

	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		logger.Fatal("failed to create app", zap.Error(err))
	}

	go func() {
		if err := application.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server exited", zap.Error(err))
		}
	}()
	logger.Info("basic example listening on :8080")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
