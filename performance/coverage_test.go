package performance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- bottleneck_detector.go -------------------------------------------

func TestDefaultThresholdsAreSane(t *testing.T) {
	th := DefaultThresholds()
	assert.Greater(t, th.MaxNsPerOp, int64(0))
	assert.Greater(t, th.MaxAllocsPerOp, int64(0))
	assert.Greater(t, th.MaxBytesPerOp, int64(0))
	assert.Greater(t, th.MaxMemoryUsageMB, int64(0))
	assert.Greater(t, th.MaxGoroutines, 0)
	assert.Greater(t, th.CPUUsagePercent, 0.0)
}

func TestNewBottleneckDetectorAndSetThresholds(t *testing.T) {
	bd := NewBottleneckDetector()
	require.NotNil(t, bd)
	assert.Equal(t, DefaultThresholds(), bd.thresholds)

	custom := Thresholds{MaxNsPerOp: 10, MaxAllocsPerOp: 1, MaxBytesPerOp: 1, MaxMemoryUsageMB: 1, MaxGoroutines: 1, CPUUsagePercent: 1}
	bd.SetThresholds(custom)
	assert.Equal(t, custom, bd.thresholds)
}

func TestCalculateSeverityRatios(t *testing.T) {
	bd := NewBottleneckDetector()
	// ratio > 5 → critical
	assert.Equal(t, "critical", bd.calculateSeverity(600, 100))
	// ratio > 2, <= 5 → high
	assert.Equal(t, "high", bd.calculateSeverity(300, 100))
	// ratio > 1.5, <= 2 → medium
	assert.Equal(t, "medium", bd.calculateSeverity(180, 100))
	// ratio <= 1.5 → low (the caller only invokes this when value>threshold)
	assert.Equal(t, "low", bd.calculateSeverity(120, 100))
}

func TestDetectIncludesBenchmarkAndRuntimeBottlenecks(t *testing.T) {
	bd := NewBottleneckDetector()
	// Tight thresholds so every benchmark result breaches at least one.
	bd.SetThresholds(Thresholds{
		MaxNsPerOp:       1,
		MaxAllocsPerOp:   1,
		MaxBytesPerOp:    1,
		MaxMemoryUsageMB: 1, // heap alloc will exceed 1 MB in any real test run
		MaxGoroutines:    1_000_000,
	})

	results := []BenchmarkResult{
		{Name: "RouteA", NsPerOp: 5000, AllocsPerOp: 20, BytesPerOp: 2048}, // critical (5000/1)
		{Name: "RouteB", NsPerOp: 3, AllocsPerOp: 2, BytesPerOp: 2},        // medium (ratio>1.5)
	}

	det, err := bd.Detect(results)
	require.NoError(t, err)
	require.NotNil(t, det)
	// 3 categories per input × 2 inputs = 6 benchmark bottlenecks, plus a
	// memory runtime bottleneck when heap alloc > 1 MB.
	assert.GreaterOrEqual(t, len(det.Bottlenecks), 6)
	// Suggestions are derived from bottlenecks and deduped.
	assert.NotEmpty(t, det.Suggestions)
	// Bottlenecks are sorted by severity: critical first.
	assert.Equal(t, "critical", det.Bottlenecks[0].Severity)
	// Runtime metrics are captured.
	assert.NotZero(t, det.Metrics.NumCPU)
}

func TestGenerateOptimizationPlan(t *testing.T) {
	bd := NewBottleneckDetector()
	det := &DetectionResult{
		Timestamp: time.Now(),
		Bottlenecks: []DetailedBottleneck{
			{Component: "RouteA", Description: "slow", Severity: "critical", Value: int64(5000), Threshold: int64(1000)},
			{Component: "RouteB", Description: "medium", Severity: "high", Value: int64(2000), Threshold: int64(1000)},
			{Component: "RouteC", Description: "meh", Severity: "low", Value: int64(1200), Threshold: int64(1000)},
		},
	}
	plan := bd.GenerateOptimizationPlan(det)
	assert.Contains(t, plan, "Performance Optimization Plan")
	assert.Contains(t, plan, "Critical Performance Issues")
	assert.Contains(t, plan, "RouteA")
	assert.Contains(t, plan, "RouteB")
}

func TestAnalyzeRuntimeMetricsGoroutineBottleneck(t *testing.T) {
	bd := NewBottleneckDetector()
	bd.SetThresholds(Thresholds{MaxMemoryUsageMB: 1 << 30, MaxGoroutines: 0})

	det := &DetectionResult{
		Bottlenecks: []DetailedBottleneck{},
		Metrics:     bd.captureRuntimeMetrics(),
	}
	bd.analyzeRuntimeMetrics(det)
	// With MaxGoroutines=0 any running goroutine count trips the rule.
	foundGoroutine := false
	for _, b := range det.Bottlenecks {
		if b.Type == "goroutine" {
			foundGoroutine = true
			assert.Equal(t, "high", b.Severity)
		}
	}
	assert.True(t, foundGoroutine)
}

// --- report_generator.go ----------------------------------------------

func TestNewReportGenerator(t *testing.T) {
	rg := NewReportGenerator()
	require.NotNil(t, rg)
	assert.NotNil(t, rg.suite)
	assert.NotEmpty(t, rg.dbPath)
}

func TestCalculateTrendSlopeImproving(t *testing.T) {
	rg := NewReportGenerator()
	// ns/op decreases over time → negative slope → improving trend.
	points := []DataPoint{
		{Timestamp: time.Unix(1, 0), NsPerOp: 1000},
		{Timestamp: time.Unix(2, 0), NsPerOp: 900},
		{Timestamp: time.Unix(3, 0), NsPerOp: 800},
	}
	slope := rg.calculateTrendSlope(points)
	assert.Less(t, slope, 0.0)

	// Fewer than 2 points returns zero without panic.
	assert.Equal(t, 0.0, rg.calculateTrendSlope(nil))
	assert.Equal(t, 0.0, rg.calculateTrendSlope(points[:1]))
}

func TestDetectBottlenecksGroupsByImpact(t *testing.T) {
	rg := NewReportGenerator()
	now := time.Now()
	results := []BenchmarkResult{
		// Old record replaced by the next entry for the same name.
		{Name: "slow", Timestamp: now.Add(-time.Hour), NsPerOp: 500, AllocsPerOp: 1},
		// >10× threshold on both → high impact.
		{Name: "slow", Timestamp: now, NsPerOp: 20_000, AllocsPerOp: 200},
		// 5–10× threshold on ns → medium impact.
		{Name: "meh", Timestamp: now, NsPerOp: 6_000, AllocsPerOp: 11},
		// Just over threshold → low impact.
		{Name: "minor", Timestamp: now, NsPerOp: 1_100, AllocsPerOp: 11},
		// Well under threshold → not a bottleneck at all.
		{Name: "ok", Timestamp: now, NsPerOp: 100, AllocsPerOp: 1},
	}
	bns := rg.detectBottlenecks(results)
	require.Len(t, bns, 3)
	// Sorted by impact, high first.
	assert.Equal(t, "high", bns[0].Impact)
	assert.Equal(t, "medium", bns[1].Impact)
	assert.Equal(t, "low", bns[2].Impact)
}

func TestGenerateComparisonsAndSummary(t *testing.T) {
	rg := NewReportGenerator()
	now := time.Now()
	old := now.Add(-14 * 24 * time.Hour)
	weekAgo := now.Add(-3 * 24 * time.Hour)

	all := []BenchmarkResult{
		// previous baseline (older than 7 days)
		{Name: "A", Timestamp: old, NsPerOp: 1000, AllocsPerOp: 10},
		{Name: "B", Timestamp: old, NsPerOp: 1000, AllocsPerOp: 10},
		// current week data
		{Name: "A", Timestamp: weekAgo, NsPerOp: 500, AllocsPerOp: 5},   // -50% → improved
		{Name: "B", Timestamp: weekAgo, NsPerOp: 2000, AllocsPerOp: 20}, // +100% → degraded
		{Name: "C", Timestamp: weekAgo, NsPerOp: 300, AllocsPerOp: 3},   // new
	}
	weekResults := []BenchmarkResult{all[2], all[3], all[4]}

	comps := rg.generateComparisons(all, weekResults)
	require.Len(t, comps, 3)
	byName := map[string]BenchmarkComparison{}
	for _, c := range comps {
		byName[c.Name] = c
	}
	assert.Equal(t, "improved", byName["A"].Status)
	assert.Equal(t, "degraded", byName["B"].Status)
	assert.Equal(t, "new", byName["C"].Status)

	// Stable comparison when change is within ±5%.
	stableAll := []BenchmarkResult{
		{Name: "S", Timestamp: old, NsPerOp: 1000, AllocsPerOp: 10},
		{Name: "S", Timestamp: weekAgo, NsPerOp: 1020, AllocsPerOp: 10},
	}
	stableComps := rg.generateComparisons(stableAll, []BenchmarkResult{stableAll[1]})
	require.Len(t, stableComps, 1)
	assert.Equal(t, "stable", stableComps[0].Status)

	sum := rg.generateSummary(comps)
	assert.Equal(t, 3, sum.TotalBenchmarks)
	assert.Equal(t, 1, sum.ImprovedCount)
	assert.Equal(t, 1, sum.DegradedCount)
	assert.Greater(t, sum.AverageNsPerOp, int64(0))
}

func TestAnalyzeTrendsNeedsThreePoints(t *testing.T) {
	rg := NewReportGenerator()
	base := time.Now()
	// Only two results — skipped.
	assert.Empty(t, rg.analyzeTrends([]BenchmarkResult{
		{Name: "x", Timestamp: base, NsPerOp: 1},
		{Name: "x", Timestamp: base.Add(time.Second), NsPerOp: 2},
	}))

	// Three ascending results → degrading trend.
	results := []BenchmarkResult{
		{Name: "deg", Timestamp: base, NsPerOp: 100},
		{Name: "deg", Timestamp: base.Add(time.Hour), NsPerOp: 200},
		{Name: "deg", Timestamp: base.Add(2 * time.Hour), NsPerOp: 300},
	}
	trends := rg.analyzeTrends(results)
	require.Len(t, trends, 1)
	assert.Equal(t, "degrading", trends[0].Trend)
	assert.Greater(t, trends[0].TrendSlope, 0.0)
}

func TestGenerateRecommendationsCoversAllBranches(t *testing.T) {
	rg := NewReportGenerator()

	// Every branch: degraded count, high-impact bottleneck, high allocs,
	// degrading trend, and an improved count.
	report := &PerformanceReport{
		Summary: Summary{DegradedCount: 2, ImprovedCount: 1, AverageAllocsPerOp: 20},
		Bottlenecks: []Bottleneck{
			{Component: "route-x", Impact: "high"},
		},
		Trends: []TrendAnalysis{{Trend: "degrading"}},
	}
	recs := rg.generateRecommendations(report)
	joined := strings.Join(recs, " | ")
	assert.Contains(t, joined, "degradation")
	assert.Contains(t, joined, "route-x")
	assert.Contains(t, joined, "allocations per operation")
	assert.Contains(t, joined, "degrading trends")
	assert.Contains(t, joined, "improvement")

	// Empty report → positive fallback message.
	empty := rg.generateRecommendations(&PerformanceReport{})
	require.Len(t, empty, 1)
	assert.Contains(t, empty[0], "stable")
}

func TestGenerateMarkdownContainsKeyFields(t *testing.T) {
	rg := NewReportGenerator()
	report := &PerformanceReport{
		GeneratedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Period:      "Weekly",
		Summary:     Summary{TotalBenchmarks: 3, ImprovedCount: 1, DegradedCount: 1, AverageNsPerOp: 500, AverageAllocsPerOp: 2},
		Benchmarks:  []BenchmarkComparison{{Name: "A", CurrentNsPerOp: 100, PreviousNsPerOp: 200, PercentChange: -50, Status: "improved"}},
		Trends:      []TrendAnalysis{{Name: "A", Trend: "improving", TrendSlope: -0.5}},
		Bottlenecks: []Bottleneck{{Component: "B", Impact: "high", Description: "slow", NsPerOp: 9999, AllocsPerOp: 99}},
		Recommendations: []string{"tighten it up"},
	}
	md, err := rg.generateMarkdown(report)
	require.NoError(t, err)
	assert.Contains(t, md, "Gortex Performance Report")
	assert.Contains(t, md, "Total Benchmarks")
	assert.Contains(t, md, "improving")
	assert.Contains(t, md, "tighten it up")
}

func TestSaveReportWritesFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rg := NewReportGenerator()
	report := &PerformanceReport{
		GeneratedAt: time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		Period:      "Weekly",
	}
	require.NoError(t, rg.SaveReport(report))

	entries, err := os.ReadDir(filepath.Join(dir, "performance", "reports"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, strings.HasPrefix(entries[0].Name(), "performance_report_"))
	assert.True(t, strings.HasSuffix(entries[0].Name(), ".md"))
}

func TestGenerateWeeklyReportEndToEnd(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.json")
	now := time.Now()
	old := now.Add(-14 * 24 * time.Hour)
	weekAgo := now.Add(-2 * 24 * time.Hour)

	payload := []BenchmarkResult{
		{Name: "A", Timestamp: old, NsPerOp: 1000, AllocsPerOp: 10},
		{Name: "A", Timestamp: weekAgo, NsPerOp: 500, AllocsPerOp: 5},
		{Name: "B", Timestamp: weekAgo, NsPerOp: 2000, AllocsPerOp: 20},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dbPath, data, 0o644))

	rg := NewReportGenerator()
	rg.dbPath = dbPath

	report, err := rg.GenerateWeeklyReport()
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "Weekly", report.Period)
	assert.NotEmpty(t, report.Benchmarks)
	assert.NotEmpty(t, report.Recommendations)
}

func TestGenerateComparisonWithFrameworks(t *testing.T) {
	rg := NewReportGenerator()
	out := rg.GenerateComparisonWithFrameworks()
	assert.Contains(t, out, "Framework Comparison")
}

// --- benchmark_suite.go: SaveResults / GetLatestResults ---------------

func TestSaveAndLoadResultsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	suite := &BenchmarkSuite{
		dbPath: filepath.Join(dir, "nested", "db.json"),
	}
	suite.results = []BenchmarkResult{
		{Name: "older", Timestamp: time.Unix(100, 0), NsPerOp: 50},
		{Name: "newer", Timestamp: time.Unix(200, 0), NsPerOp: 60},
		// Duplicate name — only the latest timestamp should survive
		// GetLatestResults.
		{Name: "newer", Timestamp: time.Unix(300, 0), NsPerOp: 70},
	}
	require.NoError(t, suite.SaveResults())

	latest, err := suite.GetLatestResults()
	require.NoError(t, err)
	require.Len(t, latest, 2)
	assert.Equal(t, int64(70), latest["newer"].NsPerOp, "latest by timestamp wins")
}

func TestGetLatestResultsMissingFile(t *testing.T) {
	suite := &BenchmarkSuite{dbPath: filepath.Join(t.TempDir(), "missing.json")}
	_, err := suite.GetLatestResults()
	require.Error(t, err, "missing database must surface as an error")
}
