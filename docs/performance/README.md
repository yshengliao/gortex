# Gortex Performance Tracking System

This package provides comprehensive performance tracking, benchmarking, and optimization tools for the Gortex framework.

## Features

- **Automated Benchmarking**: Track performance metrics over time
- **Weekly Reports**: Automated performance report generation
- **Bottleneck Detection**: Identify performance issues automatically
- **Optimization Guidance**: Actionable recommendations for improvements
- **Historical Tracking**: Compare performance across versions

## Quick Start

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./performance

# Run specific benchmark suite
go test -bench=BenchmarkGortexRouter ./performance
```

### Using the CLI Tool

```bash
# Build the tool
go build -o perfcheck ./performance/cmd/perfcheck

# Run benchmarks
./perfcheck -bench

# Generate performance report
./perfcheck -report

# Detect bottlenecks
./perfcheck -detect

# Do everything
./perfcheck -bench -report -detect
```

### Programmatic Usage

```go
import "github.com/yshengliao/gortex/performance"

// Run benchmarks
suite := performance.NewBenchmarkSuite()
testing.Benchmark(func(b *testing.B) {
    suite.RunRouterBenchmarks(b)
})
suite.SaveResults()

// Generate report
generator := performance.NewReportGenerator()
report, _ := generator.GenerateWeeklyReport()
generator.SaveReport(report)

// Detect bottlenecks
detector := performance.NewBottleneckDetector()
results, _ := suite.GetLatestResults()
detection, _ := detector.Detect(results)
```

## Performance Metrics

### Router Benchmarks
- **SimpleRoute**: Basic route matching performance
- **ParameterizedRoute**: Dynamic parameter extraction
- **WildcardRoute**: Wildcard pattern matching
- **NestedGroups**: Group routing performance
- **MiddlewareChain**: Middleware execution overhead

### Context Benchmarks
- **ContextCreation**: Context allocation and pooling
- **ContextParamAccess**: Parameter retrieval performance
- **ContextValueStorage**: Value storage and retrieval
- **ContextPooling**: Pool efficiency under load

## File Structure

```
performance/
├── benchmark_suite.go      # Core benchmarking functionality
├── report_generator.go     # Report generation
├── bottleneck_detector.go  # Performance bottleneck detection
├── test_helpers.go        # Testing utilities
├── OPTIMIZATION_GUIDE.md  # Performance optimization guide
├── benchmarks/           # Benchmark results database
│   └── benchmark_db.json
├── reports/              # Generated reports
│   └── performance_report_*.md
└── cmd/
    └── perfcheck/        # CLI tool
        └── main.go
```

## Benchmark Database

Results are stored in JSON format with the following structure:

```json
{
  "name": "SimpleRoute",
  "timestamp": "2024-01-27T10:30:00Z",
  "ns_per_op": 541,
  "allocs_per_op": 0,
  "bytes_per_op": 0,
  "iterations": 2000000,
  "go_version": "go1.22.0",
  "os": "darwin",
  "arch": "arm64",
  "cpus": 8,
  "gortex_version": "v0.4.0-alpha"
}
```

## Performance Targets

| Metric | Target | Current |
|--------|--------|---------|
| Simple Route | < 600 ns/op | 541 ns/op ✅ |
| Parameterized Route | < 700 ns/op | 628 ns/op ✅ |
| Middleware Chain | < 1000 ns/op | 892 ns/op ✅ |
| Context Creation | < 100 ns/op | 85 ns/op ✅ |
| Zero Allocations | 0 allocs | 0 allocs ✅ |

## Continuous Monitoring

### GitHub Actions Integration

```yaml
name: Performance Check
on:
  pull_request:
  schedule:
    - cron: '0 0 * * 0' # Weekly

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test -bench=. ./performance > benchmark.txt
      - uses: benchmark-action/github-action-benchmark@v1
        with:
          tool: 'go'
          output-file-path: benchmark.txt
```

### Local Development

```bash
# Watch for performance regressions
watch -n 300 './perfcheck -bench -detect'

# Generate weekly reports
cron: 0 9 * * 1 cd /path/to/gortex && ./perfcheck -report
```

## Troubleshooting

### No Benchmark Data
If you see "no benchmark data found", run benchmarks first:
```bash
go test -bench=. ./performance
```

### High Memory Usage
Check the bottleneck detector output:
```bash
./perfcheck -detect
```

### Performance Regression
1. Generate a report to identify the regression
2. Use git bisect to find the problematic commit
3. Profile the specific benchmark
4. Apply optimizations from the guide

## Contributing

When adding new benchmarks:
1. Follow the existing naming convention (BenchmarkComponent_Feature)
2. Always call `b.ReportAllocs()`
3. Include both simple and complex test cases
4. Update the optimization guide with findings

## License

Same as Gortex framework.