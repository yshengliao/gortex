package app

import (
	"github.com/yshengliao/gortex/router"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/pprof"
	"testing"

	"github.com/yshengliao/gortex/context"
	"go.uber.org/zap"
)

// BenchHandler for profiling tests
type BenchHandler struct {
	Logger *zap.Logger
}

func (h *BenchHandler) GET(c context.Context) error {
	return c.JSON(200, map[string]string{"status": "ok"})
}

type BenchHandlersManager struct {
	API *BenchHandler `url:"/api"`
}

func TestProfileRouting(t *testing.T) {
	if os.Getenv("PROFILE") != "1" {
		t.Skip("Skipping profiling test. Set PROFILE=1 to run.")
	}

	// Create CPU profile
	f, err := os.Create("router_cpu.prof")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		t.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// Setup
	r := router.NewGortexRouter()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &BenchHandler{Logger: logger}
	manager := &BenchHandlersManager{API: handler}

	if err := RegisterRoutes(&App{router: r, ctx: ctx}, manager); err != nil {
		t.Fatal(err)
	}

	// Generate load
	req := httptest.NewRequest(http.MethodGet, "/api", nil)

	for i := 0; i < 100000; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != 200 {
			t.Fatalf("Expected 200, got %d", rec.Code)
		}
	}

	fmt.Println("CPU profile written to router_cpu.prof")
	fmt.Println("Run: go tool pprof router_cpu.prof")
}
