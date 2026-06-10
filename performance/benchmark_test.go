package performance

import (
	"testing"
)

func BenchmarkGortexRouter(b *testing.B) {
	suite := NewBenchmarkSuite()
	// Redirect writes to a temp dir so benchmark_db.json is never written to the
	// source tree. Mirrors the pattern used in TestBenchmarkSuite.
	suite.dbPath = b.TempDir() + "/bench.json"

	b.Run("Router", suite.RunRouterBenchmarks)
	b.Run("Context", suite.RunContextBenchmarks)

	// Save results after benchmarks complete
	b.Cleanup(func() {
		if err := suite.SaveResults(); err != nil {
			b.Logf("Failed to save benchmark results: %v", err)
		}
	})
}

func TestBenchmarkSuite(t *testing.T) {
	suite := NewBenchmarkSuite()
	// Redirect writes to a temp dir so the real benchmark_db.json is never touched.
	suite.dbPath = t.TempDir() + "/bench.json"

	b := &testing.B{N: 1}
	suite.benchmarkSimpleRoute(b)

	if len(suite.results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(suite.results))
	}

	if err := suite.SaveResults(); err != nil {
		t.Fatalf("Failed to save results: %v", err)
	}

	latest, err := suite.GetLatestResults()
	if err != nil {
		t.Fatalf("Failed to get latest results: %v", err)
	}

	if len(latest) == 0 {
		t.Error("Expected at least one result")
	}
}
