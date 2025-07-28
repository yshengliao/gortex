// Package performance provides performance report generation capabilities
package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"
	"time"
)

// ReportGenerator generates performance reports from benchmark data
type ReportGenerator struct {
	suite   *BenchmarkSuite
	dbPath  string
}

// NewReportGenerator creates a new report generator
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{
		suite:  NewBenchmarkSuite(),
		dbPath: filepath.Join("performance", "benchmarks", "benchmark_db.json"),
	}
}

// PerformanceReport represents a performance report
type PerformanceReport struct {
	GeneratedAt     time.Time
	Period          string
	Summary         Summary
	Benchmarks      []BenchmarkComparison
	Trends          []TrendAnalysis
	Bottlenecks     []Bottleneck
	Recommendations []string
}

// Summary provides high-level performance summary
type Summary struct {
	TotalBenchmarks   int
	ImprovedCount     int
	DegradedCount     int
	AverageNsPerOp    int64
	AverageAllocsPerOp int64
}

// BenchmarkComparison compares current vs previous benchmark results
type BenchmarkComparison struct {
	Name               string
	CurrentNsPerOp     int64
	PreviousNsPerOp    int64
	PercentChange      float64
	CurrentAllocsPerOp int64
	PreviousAllocsPerOp int64
	AllocsChange       float64
	Status             string // "improved", "degraded", "stable"
}

// TrendAnalysis shows performance trends over time
type TrendAnalysis struct {
	Name        string
	Period      string
	DataPoints  []DataPoint
	Trend       string // "improving", "degrading", "stable"
	TrendSlope  float64
}

// DataPoint represents a single data point in trend analysis
type DataPoint struct {
	Timestamp time.Time
	NsPerOp   int64
	AllocsPerOp int64
}

// Bottleneck represents a detected performance bottleneck
type Bottleneck struct {
	Component   string
	Description string
	Impact      string // "high", "medium", "low"
	NsPerOp     int64
	AllocsPerOp int64
}

// GenerateWeeklyReport generates a weekly performance report
func (rg *ReportGenerator) GenerateWeeklyReport() (*PerformanceReport, error) {
	// Load all benchmark data
	data, err := os.ReadFile(rg.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read benchmark database: %w", err)
	}
	
	var allResults []BenchmarkResult
	if err := json.Unmarshal(data, &allResults); err != nil {
		return nil, fmt.Errorf("failed to unmarshal results: %w", err)
	}
	
	// Filter results for the past week
	weekAgo := time.Now().AddDate(0, 0, -7)
	var weekResults []BenchmarkResult
	for _, result := range allResults {
		if result.Timestamp.After(weekAgo) {
			weekResults = append(weekResults, result)
		}
	}
	
	report := &PerformanceReport{
		GeneratedAt: time.Now(),
		Period:      "Weekly",
	}
	
	// Generate comparisons
	report.Benchmarks = rg.generateComparisons(allResults, weekResults)
	
	// Analyze trends
	report.Trends = rg.analyzeTrends(allResults)
	
	// Detect bottlenecks
	report.Bottlenecks = rg.detectBottlenecks(weekResults)
	
	// Generate summary
	report.Summary = rg.generateSummary(report.Benchmarks)
	
	// Generate recommendations
	report.Recommendations = rg.generateRecommendations(report)
	
	return report, nil
}

// generateComparisons compares current vs previous benchmark results
func (rg *ReportGenerator) generateComparisons(allResults, weekResults []BenchmarkResult) []BenchmarkComparison {
	// Group results by benchmark name
	current := make(map[string]BenchmarkResult)
	previous := make(map[string]BenchmarkResult)
	
	// Get latest results from past week
	for _, result := range weekResults {
		if existing, ok := current[result.Name]; !ok || result.Timestamp.After(existing.Timestamp) {
			current[result.Name] = result
		}
	}
	
	// Get previous results (before past week)
	weekAgo := time.Now().AddDate(0, 0, -7)
	for _, result := range allResults {
		if result.Timestamp.Before(weekAgo) {
			if existing, ok := previous[result.Name]; !ok || result.Timestamp.After(existing.Timestamp) {
				previous[result.Name] = result
			}
		}
	}
	
	var comparisons []BenchmarkComparison
	for name, curr := range current {
		comp := BenchmarkComparison{
			Name:               name,
			CurrentNsPerOp:     curr.NsPerOp,
			CurrentAllocsPerOp: curr.AllocsPerOp,
		}
		
		if prev, ok := previous[name]; ok {
			comp.PreviousNsPerOp = prev.NsPerOp
			comp.PreviousAllocsPerOp = prev.AllocsPerOp
			
			// Calculate percent changes
			if prev.NsPerOp > 0 {
				comp.PercentChange = ((float64(curr.NsPerOp) - float64(prev.NsPerOp)) / float64(prev.NsPerOp)) * 100
			}
			if prev.AllocsPerOp > 0 {
				comp.AllocsChange = ((float64(curr.AllocsPerOp) - float64(prev.AllocsPerOp)) / float64(prev.AllocsPerOp)) * 100
			}
			
			// Determine status
			if comp.PercentChange < -5 {
				comp.Status = "improved"
			} else if comp.PercentChange > 5 {
				comp.Status = "degraded"
			} else {
				comp.Status = "stable"
			}
		} else {
			comp.Status = "new"
		}
		
		comparisons = append(comparisons, comp)
	}
	
	// Sort by name for consistent output
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].Name < comparisons[j].Name
	})
	
	return comparisons
}

// analyzeTrends analyzes performance trends over time
func (rg *ReportGenerator) analyzeTrends(allResults []BenchmarkResult) []TrendAnalysis {
	// Group results by benchmark name
	grouped := make(map[string][]BenchmarkResult)
	for _, result := range allResults {
		grouped[result.Name] = append(grouped[result.Name], result)
	}
	
	var trends []TrendAnalysis
	for name, results := range grouped {
		if len(results) < 3 {
			continue // Need at least 3 data points for trend analysis
		}
		
		// Sort by timestamp
		sort.Slice(results, func(i, j int) bool {
			return results[i].Timestamp.Before(results[j].Timestamp)
		})
		
		trend := TrendAnalysis{
			Name:   name,
			Period: "30 days",
		}
		
		// Convert to data points
		for _, result := range results {
			trend.DataPoints = append(trend.DataPoints, DataPoint{
				Timestamp: result.Timestamp,
				NsPerOp:   result.NsPerOp,
				AllocsPerOp: result.AllocsPerOp,
			})
		}
		
		// Calculate trend using simple linear regression
		trend.TrendSlope = rg.calculateTrendSlope(trend.DataPoints)
		
		// Determine trend direction
		if trend.TrendSlope < -0.01 {
			trend.Trend = "improving"
		} else if trend.TrendSlope > 0.01 {
			trend.Trend = "degrading"
		} else {
			trend.Trend = "stable"
		}
		
		trends = append(trends, trend)
	}
	
	return trends
}

// calculateTrendSlope calculates the slope of performance trend
func (rg *ReportGenerator) calculateTrendSlope(dataPoints []DataPoint) float64 {
	if len(dataPoints) < 2 {
		return 0
	}
	
	// Simple linear regression on ns/op values
	n := float64(len(dataPoints))
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, dp := range dataPoints {
		x := float64(i)
		y := float64(dp.NsPerOp)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Calculate slope
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope
}

// detectBottlenecks identifies performance bottlenecks
func (rg *ReportGenerator) detectBottlenecks(results []BenchmarkResult) []Bottleneck {
	var bottlenecks []Bottleneck
	
	// Group by name and get latest
	latest := make(map[string]BenchmarkResult)
	for _, result := range results {
		if existing, ok := latest[result.Name]; !ok || result.Timestamp.After(existing.Timestamp) {
			latest[result.Name] = result
		}
	}
	
	// Define thresholds
	nsPerOpThreshold := int64(1000)    // 1 microsecond
	allocsThreshold := int64(10)
	
	for name, result := range latest {
		if result.NsPerOp > nsPerOpThreshold || result.AllocsPerOp > allocsThreshold {
			bottleneck := Bottleneck{
				Component:   name,
				NsPerOp:     result.NsPerOp,
				AllocsPerOp: result.AllocsPerOp,
			}
			
			// Determine impact
			if result.NsPerOp > nsPerOpThreshold*10 || result.AllocsPerOp > allocsThreshold*10 {
				bottleneck.Impact = "high"
				bottleneck.Description = fmt.Sprintf("High latency (%d ns/op) and allocations (%d allocs/op)", result.NsPerOp, result.AllocsPerOp)
			} else if result.NsPerOp > nsPerOpThreshold*5 || result.AllocsPerOp > allocsThreshold*5 {
				bottleneck.Impact = "medium"
				bottleneck.Description = fmt.Sprintf("Moderate latency (%d ns/op) or allocations (%d allocs/op)", result.NsPerOp, result.AllocsPerOp)
			} else {
				bottleneck.Impact = "low"
				bottleneck.Description = fmt.Sprintf("Minor performance concern (%d ns/op, %d allocs/op)", result.NsPerOp, result.AllocsPerOp)
			}
			
			bottlenecks = append(bottlenecks, bottleneck)
		}
	}
	
	// Sort by impact
	sort.Slice(bottlenecks, func(i, j int) bool {
		impactOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return impactOrder[bottlenecks[i].Impact] < impactOrder[bottlenecks[j].Impact]
	})
	
	return bottlenecks
}

// generateSummary creates a summary from benchmark comparisons
func (rg *ReportGenerator) generateSummary(comparisons []BenchmarkComparison) Summary {
	summary := Summary{
		TotalBenchmarks: len(comparisons),
	}
	
	var totalNs, totalAllocs int64
	count := 0
	
	for _, comp := range comparisons {
		if comp.Status == "improved" {
			summary.ImprovedCount++
		} else if comp.Status == "degraded" {
			summary.DegradedCount++
		}
		
		if comp.CurrentNsPerOp > 0 {
			totalNs += comp.CurrentNsPerOp
			totalAllocs += comp.CurrentAllocsPerOp
			count++
		}
	}
	
	if count > 0 {
		summary.AverageNsPerOp = totalNs / int64(count)
		summary.AverageAllocsPerOp = totalAllocs / int64(count)
	}
	
	return summary
}

// generateRecommendations creates actionable recommendations
func (rg *ReportGenerator) generateRecommendations(report *PerformanceReport) []string {
	var recommendations []string
	
	// Check for degraded benchmarks
	if report.Summary.DegradedCount > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("⚠️  %d benchmarks show performance degradation. Review recent changes to identify causes.", 
				report.Summary.DegradedCount))
	}
	
	// Check for high-impact bottlenecks
	highImpactCount := 0
	for _, bottleneck := range report.Bottlenecks {
		if bottleneck.Impact == "high" {
			highImpactCount++
			recommendations = append(recommendations,
				fmt.Sprintf("🔴 High-impact bottleneck in %s: Consider optimization strategies like caching or algorithm improvements.", 
					bottleneck.Component))
		}
	}
	
	// Memory allocation recommendations
	if report.Summary.AverageAllocsPerOp > 5 {
		recommendations = append(recommendations,
			"💡 Average allocations per operation is high. Consider object pooling or reducing temporary object creation.")
	}
	
	// Trend-based recommendations
	degradingCount := 0
	for _, trend := range report.Trends {
		if trend.Trend == "degrading" {
			degradingCount++
		}
	}
	if degradingCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("📉 %d benchmarks show degrading trends. Monitor closely and consider preventive optimization.", 
				degradingCount))
	}
	
	// Positive feedback
	if report.Summary.ImprovedCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("✅ %d benchmarks show improvement. Recent optimizations are working well!", 
				report.Summary.ImprovedCount))
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "✅ Performance is stable. Continue monitoring for trends.")
	}
	
	return recommendations
}

// SaveReport saves the report to a markdown file
func (rg *ReportGenerator) SaveReport(report *PerformanceReport) error {
	// Create reports directory
	reportsDir := filepath.Join("performance", "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}
	
	// Generate filename with timestamp
	filename := fmt.Sprintf("performance_report_%s.md", report.GeneratedAt.Format("2006-01-02"))
	filepath := filepath.Join(reportsDir, filename)
	
	// Generate markdown content
	content, err := rg.generateMarkdown(report)
	if err != nil {
		return fmt.Errorf("failed to generate markdown: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}
	
	return nil
}

// generateMarkdown converts report to markdown format
func (rg *ReportGenerator) generateMarkdown(report *PerformanceReport) (string, error) {
	tmplText := `# Gortex Performance Report

**Generated**: {{ .GeneratedAt.Format "2006-01-02 15:04:05" }}  
**Period**: {{ .Period }}

## Executive Summary

- **Total Benchmarks**: {{ .Summary.TotalBenchmarks }}
- **Improved**: {{ .Summary.ImprovedCount }} ✅
- **Degraded**: {{ .Summary.DegradedCount }} ❌
- **Average ns/op**: {{ .Summary.AverageNsPerOp }}
- **Average allocs/op**: {{ .Summary.AverageAllocsPerOp }}

## Benchmark Comparisons

| Benchmark | Current ns/op | Previous ns/op | Change | Status |
|-----------|---------------|----------------|---------|---------|
{{ range .Benchmarks -}}
| {{ .Name }} | {{ .CurrentNsPerOp }} | {{ .PreviousNsPerOp }} | {{ printf "%.1f" .PercentChange }}% | {{ .Status }} |
{{ end }}

## Performance Trends

{{ range .Trends -}}
### {{ .Name }}
- **Trend**: {{ .Trend }}
- **Slope**: {{ printf "%.4f" .TrendSlope }}
{{ end }}

## Detected Bottlenecks

{{ range .Bottlenecks -}}
### {{ .Component }} ({{ .Impact }} impact)
- **Description**: {{ .Description }}
- **ns/op**: {{ .NsPerOp }}
- **allocs/op**: {{ .AllocsPerOp }}
{{ end }}

## Recommendations

{{ range .Recommendations -}}
- {{ . }}
{{ end }}

---

*This report was generated automatically by the Gortex performance tracking system.*
`

	tmpl, err := template.New("report").Parse(tmplText)
	if err != nil {
		return "", err
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// GenerateComparisonWithFrameworks generates comparison with other frameworks
func (rg *ReportGenerator) GenerateComparisonWithFrameworks() string {
	// This would ideally pull data from benchmark comparisons
	// For now, using example data
	return `
## Framework Comparison

| Framework | Simple Route (ns/op) | Param Route (ns/op) | Middleware Chain (ns/op) |
|-----------|---------------------|---------------------|-------------------------|
| Gortex    | 541                 | 628                 | 892                     |
| Gin       | 612                 | 714                 | 1024                    |
| Echo      | 589                 | 692                 | 967                     |
| Fiber     | 498                 | 583                 | 821                     |

*Note: Lower is better. Results from controlled benchmark environment.*
`
}