// Package performance provides performance bottleneck detection
package performance

import (
	"fmt"
	"runtime"
	"runtime/pprof"
	"sort"
	"time"
)

// BottleneckDetector identifies performance bottlenecks in the framework
type BottleneckDetector struct {
	thresholds Thresholds
	profiler   *Profiler
}

// Thresholds defines performance thresholds for bottleneck detection
type Thresholds struct {
	MaxNsPerOp       int64   // Maximum nanoseconds per operation
	MaxAllocsPerOp   int64   // Maximum allocations per operation
	MaxBytesPerOp    int64   // Maximum bytes allocated per operation
	MaxMemoryUsageMB int64   // Maximum memory usage in MB
	MaxGoroutines    int     // Maximum number of goroutines
	CPUUsagePercent  float64 // Maximum CPU usage percentage
}

// DefaultThresholds returns default performance thresholds
func DefaultThresholds() Thresholds {
	return Thresholds{
		MaxNsPerOp:       1000,    // 1 microsecond
		MaxAllocsPerOp:   10,      // 10 allocations
		MaxBytesPerOp:    1024,    // 1KB
		MaxMemoryUsageMB: 100,     // 100MB
		MaxGoroutines:    1000,    // 1000 goroutines
		CPUUsagePercent:  80.0,    // 80% CPU
	}
}

// Profiler handles runtime profiling
type Profiler struct {
	cpuProfile    *pprof.Profile
	memProfile    *pprof.Profile
	goroutineProf *pprof.Profile
}

// DetectionResult represents bottleneck detection results
type DetectionResult struct {
	Timestamp    time.Time
	Bottlenecks  []DetailedBottleneck
	Metrics      RuntimeMetrics
	Suggestions  []Suggestion
}

// DetailedBottleneck provides detailed bottleneck information
type DetailedBottleneck struct {
	Type        string // "cpu", "memory", "allocation", "goroutine"
	Component   string
	Description string
	Severity    string // "critical", "high", "medium", "low"
	Value       interface{}
	Threshold   interface{}
	StackTrace  []string
}

// RuntimeMetrics captures runtime performance metrics
type RuntimeMetrics struct {
	MemStats       runtime.MemStats
	NumGoroutines  int
	NumCPU         int
	GCStats        GCStats
}

// GCStats represents garbage collection statistics
type GCStats struct {
	NumGC          uint32
	PauseTotal     time.Duration
	LastPause      time.Duration
	PauseQuantiles []time.Duration
}

// Suggestion provides optimization suggestions
type Suggestion struct {
	Component   string
	Issue       string
	Solution    string
	Priority    string // "high", "medium", "low"
	CodeExample string
}

// NewBottleneckDetector creates a new bottleneck detector
func NewBottleneckDetector() *BottleneckDetector {
	return &BottleneckDetector{
		thresholds: DefaultThresholds(),
		profiler:   &Profiler{},
	}
}

// SetThresholds updates detection thresholds
func (bd *BottleneckDetector) SetThresholds(t Thresholds) {
	bd.thresholds = t
}

// Detect performs comprehensive bottleneck detection
func (bd *BottleneckDetector) Detect(benchResults []BenchmarkResult) (*DetectionResult, error) {
	result := &DetectionResult{
		Timestamp:   time.Now(),
		Bottlenecks: make([]DetailedBottleneck, 0),
		Suggestions: make([]Suggestion, 0),
	}
	
	// Capture runtime metrics
	result.Metrics = bd.captureRuntimeMetrics()
	
	// Analyze benchmark results
	bd.analyzeBenchmarkResults(benchResults, result)
	
	// Analyze runtime metrics
	bd.analyzeRuntimeMetrics(result)
	
	// Generate suggestions based on bottlenecks
	bd.generateSuggestions(result)
	
	// Sort bottlenecks by severity
	sort.Slice(result.Bottlenecks, func(i, j int) bool {
		severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		return severityOrder[result.Bottlenecks[i].Severity] < severityOrder[result.Bottlenecks[j].Severity]
	})
	
	return result, nil
}

// captureRuntimeMetrics captures current runtime metrics
func (bd *BottleneckDetector) captureRuntimeMetrics() RuntimeMetrics {
	var metrics RuntimeMetrics
	runtime.ReadMemStats(&metrics.MemStats)
	
	metrics.NumGoroutines = runtime.NumGoroutine()
	metrics.NumCPU = runtime.NumCPU()
	
	// Capture GC stats
	metrics.GCStats = GCStats{
		NumGC:      metrics.MemStats.NumGC,
		PauseTotal: time.Duration(metrics.MemStats.PauseTotalNs),
		LastPause:  time.Duration(metrics.MemStats.PauseNs[(metrics.MemStats.NumGC+255)%256]),
	}
	
	return metrics
}

// analyzeBenchmarkResults analyzes benchmark results for bottlenecks
func (bd *BottleneckDetector) analyzeBenchmarkResults(results []BenchmarkResult, detection *DetectionResult) {
	for _, result := range results {
		// Check execution time
		if result.NsPerOp > bd.thresholds.MaxNsPerOp {
			bottleneck := DetailedBottleneck{
				Type:        "cpu",
				Component:   result.Name,
				Description: fmt.Sprintf("High execution time: %d ns/op exceeds threshold of %d ns/op", 
					result.NsPerOp, bd.thresholds.MaxNsPerOp),
				Severity:    bd.calculateSeverity(result.NsPerOp, bd.thresholds.MaxNsPerOp),
				Value:       result.NsPerOp,
				Threshold:   bd.thresholds.MaxNsPerOp,
			}
			detection.Bottlenecks = append(detection.Bottlenecks, bottleneck)
		}
		
		// Check allocations
		if result.AllocsPerOp > bd.thresholds.MaxAllocsPerOp {
			bottleneck := DetailedBottleneck{
				Type:        "allocation",
				Component:   result.Name,
				Description: fmt.Sprintf("High allocation count: %d allocs/op exceeds threshold of %d", 
					result.AllocsPerOp, bd.thresholds.MaxAllocsPerOp),
				Severity:    bd.calculateSeverity(result.AllocsPerOp, bd.thresholds.MaxAllocsPerOp),
				Value:       result.AllocsPerOp,
				Threshold:   bd.thresholds.MaxAllocsPerOp,
			}
			detection.Bottlenecks = append(detection.Bottlenecks, bottleneck)
		}
		
		// Check bytes allocated
		if result.BytesPerOp > bd.thresholds.MaxBytesPerOp {
			bottleneck := DetailedBottleneck{
				Type:        "memory",
				Component:   result.Name,
				Description: fmt.Sprintf("High memory allocation: %d bytes/op exceeds threshold of %d", 
					result.BytesPerOp, bd.thresholds.MaxBytesPerOp),
				Severity:    bd.calculateSeverity(result.BytesPerOp, bd.thresholds.MaxBytesPerOp),
				Value:       result.BytesPerOp,
				Threshold:   bd.thresholds.MaxBytesPerOp,
			}
			detection.Bottlenecks = append(detection.Bottlenecks, bottleneck)
		}
	}
}

// analyzeRuntimeMetrics analyzes runtime metrics for bottlenecks
func (bd *BottleneckDetector) analyzeRuntimeMetrics(detection *DetectionResult) {
	metrics := detection.Metrics
	
	// Check memory usage
	heapUsageMB := int64(metrics.MemStats.HeapAlloc / 1024 / 1024)
	if heapUsageMB > bd.thresholds.MaxMemoryUsageMB {
		bottleneck := DetailedBottleneck{
			Type:        "memory",
			Component:   "Runtime",
			Description: fmt.Sprintf("High heap memory usage: %d MB exceeds threshold of %d MB", 
				heapUsageMB, bd.thresholds.MaxMemoryUsageMB),
			Severity:    bd.calculateSeverity(heapUsageMB, bd.thresholds.MaxMemoryUsageMB),
			Value:       heapUsageMB,
			Threshold:   bd.thresholds.MaxMemoryUsageMB,
		}
		detection.Bottlenecks = append(detection.Bottlenecks, bottleneck)
	}
	
	// Check goroutine count
	if metrics.NumGoroutines > bd.thresholds.MaxGoroutines {
		bottleneck := DetailedBottleneck{
			Type:        "goroutine",
			Component:   "Runtime",
			Description: fmt.Sprintf("High goroutine count: %d exceeds threshold of %d", 
				metrics.NumGoroutines, bd.thresholds.MaxGoroutines),
			Severity:    "high",
			Value:       metrics.NumGoroutines,
			Threshold:   bd.thresholds.MaxGoroutines,
		}
		detection.Bottlenecks = append(detection.Bottlenecks, bottleneck)
	}
	
	// Check GC pause times
	if metrics.GCStats.LastPause > 10*time.Millisecond {
		bottleneck := DetailedBottleneck{
			Type:        "gc",
			Component:   "Garbage Collector",
			Description: fmt.Sprintf("High GC pause time: %v", metrics.GCStats.LastPause),
			Severity:    "medium",
			Value:       metrics.GCStats.LastPause,
			Threshold:   10 * time.Millisecond,
		}
		detection.Bottlenecks = append(detection.Bottlenecks, bottleneck)
	}
}

// calculateSeverity calculates bottleneck severity
func (bd *BottleneckDetector) calculateSeverity(value, threshold int64) string {
	ratio := float64(value) / float64(threshold)
	switch {
	case ratio > 5:
		return "critical"
	case ratio > 2:
		return "high"
	case ratio > 1.5:
		return "medium"
	default:
		return "low"
	}
}

// generateSuggestions generates optimization suggestions
func (bd *BottleneckDetector) generateSuggestions(detection *DetectionResult) {
	suggestionMap := make(map[string]bool) // Avoid duplicate suggestions
	
	for _, bottleneck := range detection.Bottlenecks {
		var suggestion Suggestion
		
		switch bottleneck.Type {
		case "cpu":
			if bottleneck.Component == "MiddlewareChain" {
				suggestion = Suggestion{
					Component: bottleneck.Component,
					Issue:     "High middleware overhead",
					Solution:  "Consider reducing middleware chain length or optimizing middleware functions",
					Priority:  bottleneck.Severity,
					CodeExample: `// Instead of multiple middleware:
router.Use(middleware1, middleware2, middleware3)

// Consider combining into a single optimized middleware:
router.Use(combinedMiddleware)`,
				}
			} else {
				suggestion = Suggestion{
					Component: bottleneck.Component,
					Issue:     "High CPU usage in route handling",
					Solution:  "Profile the specific handler to identify hot spots, consider caching or algorithm optimization",
					Priority:  bottleneck.Severity,
				}
			}
			
		case "allocation":
			suggestion = Suggestion{
				Component: bottleneck.Component,
				Issue:     "Excessive allocations",
				Solution:  "Use object pooling, preallocate slices, or reduce temporary object creation",
				Priority:  bottleneck.Severity,
				CodeExample: `// Use sync.Pool for frequently allocated objects:
var contextPool = sync.Pool{
    New: func() interface{} {
        return &Context{}
    },
}`,
			}
			
		case "memory":
			suggestion = Suggestion{
				Component: bottleneck.Component,
				Issue:     "High memory usage",
				Solution:  "Review data structures, implement pagination, or use streaming for large datasets",
				Priority:  bottleneck.Severity,
			}
			
		case "goroutine":
			suggestion = Suggestion{
				Component: "Concurrency",
				Issue:     "Goroutine leak or excessive concurrency",
				Solution:  "Implement proper goroutine lifecycle management and use worker pools",
				Priority:  "high",
				CodeExample: `// Use worker pool pattern:
type WorkerPool struct {
    workers int
    jobs    chan Job
    wg      sync.WaitGroup
}`,
			}
			
		case "gc":
			suggestion = Suggestion{
				Component: "Memory Management",
				Issue:     "Frequent or long GC pauses",
				Solution:  "Reduce allocation rate, tune GOGC, or use manual memory management for critical paths",
				Priority:  "medium",
			}
		}
		
		// Add unique suggestions only
		key := fmt.Sprintf("%s-%s", suggestion.Component, suggestion.Issue)
		if !suggestionMap[key] && suggestion.Component != "" {
			detection.Suggestions = append(detection.Suggestions, suggestion)
			suggestionMap[key] = true
		}
	}
	
	// Sort suggestions by priority
	sort.Slice(detection.Suggestions, func(i, j int) bool {
		priorityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		return priorityOrder[detection.Suggestions[i].Priority] < priorityOrder[detection.Suggestions[j].Priority]
	})
}

// GenerateOptimizationPlan creates a detailed optimization plan
func (bd *BottleneckDetector) GenerateOptimizationPlan(detection *DetectionResult) string {
	plan := fmt.Sprintf(`# Performance Optimization Plan

Generated: %s

## Critical Issues Requiring Immediate Attention

`, detection.Timestamp.Format("2006-01-02 15:04:05"))

	// Group bottlenecks by severity
	critical := make([]DetailedBottleneck, 0)
	high := make([]DetailedBottleneck, 0)
	other := make([]DetailedBottleneck, 0)
	
	for _, b := range detection.Bottlenecks {
		switch b.Severity {
		case "critical":
			critical = append(critical, b)
		case "high":
			high = append(high, b)
		default:
			other = append(other, b)
		}
	}
	
	// Critical issues
	if len(critical) > 0 {
		plan += "### 🔴 Critical Performance Issues\n\n"
		for _, b := range critical {
			plan += fmt.Sprintf("- **%s**: %s\n  - Current: %v, Threshold: %v\n\n", 
				b.Component, b.Description, b.Value, b.Threshold)
		}
	}
	
	// High priority issues
	if len(high) > 0 {
		plan += "### 🟡 High Priority Issues\n\n"
		for _, b := range high {
			plan += fmt.Sprintf("- **%s**: %s\n", b.Component, b.Description)
		}
		plan += "\n"
	}
	
	// Optimization roadmap
	plan += "## Optimization Roadmap\n\n"
	for i, suggestion := range detection.Suggestions {
		plan += fmt.Sprintf("### Step %d: %s\n", i+1, suggestion.Component)
		plan += fmt.Sprintf("**Issue**: %s\n", suggestion.Issue)
		plan += fmt.Sprintf("**Solution**: %s\n", suggestion.Solution)
		if suggestion.CodeExample != "" {
			plan += fmt.Sprintf("\n**Example**:\n```go\n%s\n```\n", suggestion.CodeExample)
		}
		plan += "\n"
	}
	
	// System metrics summary
	plan += fmt.Sprintf(`## Current System Metrics

- **Goroutines**: %d
- **Heap Memory**: %d MB
- **GC Runs**: %d
- **Last GC Pause**: %v

`, detection.Metrics.NumGoroutines, 
		detection.Metrics.MemStats.HeapAlloc/1024/1024,
		detection.Metrics.GCStats.NumGC,
		detection.Metrics.GCStats.LastPause)
	
	return plan
}