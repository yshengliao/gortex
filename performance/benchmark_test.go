package performance

import (
	"testing"
)

func BenchmarkGortexRouter(b *testing.B) {
	suite := NewBenchmarkSuite()
	
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
	// Run a simple test to ensure benchmark suite works
	suite := NewBenchmarkSuite()
	
	// Create a minimal benchmark
	b := &testing.B{N: 1}
	suite.benchmarkSimpleRoute(b)
	
	if len(suite.results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(suite.results))
	}
	
	// Test saving and loading
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