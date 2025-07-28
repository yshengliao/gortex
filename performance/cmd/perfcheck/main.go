// Command perfcheck runs performance analysis on Gortex framework
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/yshengliao/gortex/performance"
)

func main() {
	var (
		runBenchmarks = flag.Bool("bench", false, "Run performance benchmarks")
		generateReport = flag.Bool("report", false, "Generate performance report")
		detectBottlenecks = flag.Bool("detect", false, "Detect performance bottlenecks")
		outputPath = flag.String("output", "", "Output path for reports")
	)
	flag.Parse()

	if !*runBenchmarks && !*generateReport && !*detectBottlenecks {
		fmt.Println("Gortex Performance Checker")
		fmt.Println("\nUsage:")
		fmt.Println("  perfcheck -bench              Run benchmarks")
		fmt.Println("  perfcheck -report             Generate performance report")
		fmt.Println("  perfcheck -detect             Detect bottlenecks")
		fmt.Println("  perfcheck -bench -report      Run benchmarks and generate report")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	suite := performance.NewBenchmarkSuite()

	// Run benchmarks if requested
	if *runBenchmarks {
		fmt.Println("Running performance benchmarks...")
		runBenchmarkSuite(suite)
		
		if err := suite.SaveResults(); err != nil {
			log.Fatalf("Failed to save benchmark results: %v", err)
		}
		fmt.Println("✓ Benchmarks completed and saved")
	}

	// Generate report if requested
	if *generateReport {
		fmt.Println("\nGenerating performance report...")
		generator := performance.NewReportGenerator()
		
		report, err := generator.GenerateWeeklyReport()
		if err != nil {
			log.Fatalf("Failed to generate report: %v", err)
		}
		
		if err := generator.SaveReport(report); err != nil {
			log.Fatalf("Failed to save report: %v", err)
		}
		
		// Print summary
		fmt.Printf("\n📊 Performance Report Summary:\n")
		fmt.Printf("  Total Benchmarks: %d\n", report.Summary.TotalBenchmarks)
		fmt.Printf("  Improved: %d ✅\n", report.Summary.ImprovedCount)
		fmt.Printf("  Degraded: %d ❌\n", report.Summary.DegradedCount)
		fmt.Printf("  Average ns/op: %d\n", report.Summary.AverageNsPerOp)
		fmt.Printf("  Average allocs/op: %d\n", report.Summary.AverageAllocsPerOp)
		
		fmt.Printf("\n✓ Report saved to: performance/reports/performance_report_%s.md\n", 
			report.GeneratedAt.Format("2006-01-02"))
	}

	// Detect bottlenecks if requested
	if *detectBottlenecks {
		fmt.Println("\nDetecting performance bottlenecks...")
		detector := performance.NewBottleneckDetector()
		
		// Get latest benchmark results
		latest, err := suite.GetLatestResults()
		if err != nil {
			log.Fatalf("Failed to get benchmark results: %v", err)
		}
		
		// Convert to slice
		var results []performance.BenchmarkResult
		for _, result := range latest {
			results = append(results, result)
		}
		
		detection, err := detector.Detect(results)
		if err != nil {
			log.Fatalf("Failed to detect bottlenecks: %v", err)
		}
		
		// Print bottlenecks
		if len(detection.Bottlenecks) > 0 {
			fmt.Printf("\n🔍 Detected %d bottlenecks:\n", len(detection.Bottlenecks))
			for _, b := range detection.Bottlenecks {
				icon := getIconForSeverity(b.Severity)
				fmt.Printf("  %s %s (%s): %s\n", icon, b.Component, b.Severity, b.Description)
			}
		} else {
			fmt.Println("\n✅ No significant bottlenecks detected!")
		}
		
		// Print suggestions
		if len(detection.Suggestions) > 0 {
			fmt.Printf("\n💡 Optimization Suggestions:\n")
			for i, s := range detection.Suggestions {
				fmt.Printf("  %d. %s: %s\n", i+1, s.Component, s.Solution)
			}
		}
		
		// Save optimization plan if output path provided
		if *outputPath != "" {
			plan := detector.GenerateOptimizationPlan(detection)
			planPath := *outputPath
			if planPath == "" {
				planPath = "performance/reports/optimization_plan.md"
			}
			
			if err := os.WriteFile(planPath, []byte(plan), 0644); err != nil {
				log.Fatalf("Failed to save optimization plan: %v", err)
			}
			fmt.Printf("\n✓ Optimization plan saved to: %s\n", planPath)
		}
	}
}

func runBenchmarkSuite(suite *performance.BenchmarkSuite) {
	// Create a minimal testing.B to run benchmarks
	benchmarks := []struct {
		name string
		fn   func(*testing.B)
	}{
		{"Router", suite.RunRouterBenchmarks},
		{"Context", suite.RunContextBenchmarks},
	}
	
	for _, bm := range benchmarks {
		fmt.Printf("  Running %s benchmarks...\n", bm.name)
		// This is a simplified version - in production, use testing.Benchmark
		b := &testing.B{N: 1000}
		bm.fn(b)
	}
}

func getIconForSeverity(severity string) string {
	switch severity {
	case "critical":
		return "🔴"
	case "high":
		return "🟡"
	case "medium":
		return "🟠"
	case "low":
		return "🔵"
	default:
		return "⚪"
	}
}