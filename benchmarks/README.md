# Gortex Metrics Performance Benchmarks

This directory contains performance benchmarks and baseline results for the Gortex metrics collection system.

## Benchmark Overview

The benchmark suite tests performance across multiple dimensions:

### 1. Core Benchmark Scenarios

#### **High Concurrency Writes**
- Tests multiple workers writing metrics simultaneously
- Measures lock contention and throughput under load
- **ShardedCollector**: ~266 ns/op (2.2x faster than ImprovedCollector)
- **ImprovedCollector**: ~596 ns/op

#### **High Cardinality Tags**
- Tests performance with many different tag combinations
- Simulates real-world high-cardinality scenarios
- Both collectors perform similarly (~500-560 ns/op)

#### **Mixed Read/Write Workloads**
- 25% read operations, 75% write operations
- Simulates dashboard + metric collection scenarios
- **ImprovedCollector**: Better read performance (~305 ns/op)
- **ShardedCollector**: Higher read overhead due to shard aggregation (~5694 ns/op)

#### **HTTP Request Recording**
- Tests realistic HTTP metrics recording
- Both collectors achieve ~62 ns/op with zero allocations

#### **Contentious Key Access**
- Worst-case scenario: multiple workers updating same metrics
- **ShardedCollector**: 55% faster (~108 ns/op vs ~238 ns/op)

### 2. Scalability Benchmarks

Tests performance scaling from 1 to 32 concurrent workers:

| Workers | ImprovedCollector | ShardedCollector | Improvement |
|---------|-------------------|------------------|-------------|
| 1       | 143.3 ns/op       | 184.7 ns/op      | -29%        |
| 2       | 188.3 ns/op       | 135.0 ns/op      | +28%        |
| 4       | 243.9 ns/op       | 97.2 ns/op       | +60%        |
| 8       | 272.7 ns/op       | 119.8 ns/op      | +56%        |
| 16      | 440.6 ns/op       | 303.2 ns/op      | +31%        |

**Key Finding**: ShardedCollector scales much better with concurrent workers.

### 3. Memory Benchmarks

Tests memory allocation patterns and footprint:
- **Memory allocations per operation**: 15-24 bytes typically
- **Allocation frequency**: 1-4 allocations per metric recording
- **Memory pressure handling**: Both collectors handle LRU eviction efficiently

### 4. Cardinality Limit Benchmarks

Tests performance under different cardinality limits (100, 1K, 10K):
- Performance degrades gracefully as limits increase
- LRU eviction adds ~100-200ns overhead when limits are exceeded

## Baseline Results (M1 Mac)

Current baseline established from main branch:

```
BenchmarkMetricsCollectors/ImprovedCollector/HighConcurrencyWrites-8    	1846021	595.8 ns/op
BenchmarkMetricsCollectors/ShardedCollector/HighConcurrencyWrites-8     	4324012	266.5 ns/op
BenchmarkMetricsCollectors/ImprovedCollector/SingleThreadedBaseline-8   	9968097	120.6 ns/op
BenchmarkMetricsCollectors/ShardedCollector/SingleThreadedBaseline-8    	8057266	148.7 ns/op
```

## Performance Recommendations

### Use ShardedCollector When:
- ✅ High-concurrency web applications (>4 concurrent workers)
- ✅ High write throughput requirements
- ✅ CPU-intensive applications with available cores
- ✅ Lock contention is a bottleneck

### Use ImprovedCollector When:
- ✅ Low-traffic applications (single-threaded or minimal concurrency)
- ✅ Read-heavy workloads (dashboards, monitoring UIs)
- ✅ Memory-constrained environments
- ✅ Simple deployment scenarios

## CI Integration

### Automated Performance Tracking

The benchmark suite is integrated into CI (`/.github/workflows/benchmarks.yml`) with:

1. **Regression Detection**: Fails CI if performance degrades >10%
2. **Baseline Comparison**: Uses `benchstat` to compare against main branch
3. **PR Comments**: Automatic benchmark results posted to pull requests
4. **Baseline Updates**: Main branch automatically updates baseline results

### Performance Thresholds

| Metric | Warning Threshold | Failure Threshold |
|--------|-------------------|-------------------|
| Latency Regression | >5% | >10% |
| Memory Increase | >10% | >20% |
| Allocation Count | >20% | >50% |

### Running Benchmarks Locally

```bash
# Run all benchmarks
go test -bench=. -benchmem ./observability/metrics/

# Run specific benchmark
go test -bench=BenchmarkMetricsCollectors -benchmem ./observability/metrics/

# Compare with baseline
go install golang.org/x/perf/cmd/benchstat@latest
benchstat benchmarks/baseline-metrics.txt current-run.txt

# Memory profiling
go test -bench=BenchmarkMetricsCollectors -memprofile=mem.prof ./observability/metrics/
go tool pprof mem.prof
```

## Benchmark Maintenance

### Adding New Benchmarks

1. Add benchmark functions to `benchmark_test.go`
2. Follow naming convention: `BenchmarkMetrics*`
3. Use realistic data patterns and workloads
4. Include memory profiling with `-benchmem`
5. Update this README with new baseline results

### Updating Baselines

Baselines are automatically updated when changes are merged to main. Manual updates:

```bash
go test -bench=BenchmarkMetricsCollectors -benchmem -count=5 \
  ./observability/metrics/ > benchmarks/baseline-metrics.txt
```

### Interpreting Results

- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)  
- **allocs/op**: Number of allocations per operation (lower is better)

Significant changes:
- **>10% regression**: Requires investigation and optimization
- **>20% improvement**: Document the optimization technique
- **Changed allocation pattern**: Review for memory leaks or efficiency gains

## Historical Performance Data

Performance improvements achieved:

- **v0.4.0**: ShardedCollector implementation (+60% concurrency performance)
- **v0.3.0**: LRU cardinality limits (bounded memory usage)
- **v0.2.0**: Atomic counters for HTTP metrics (+40% HTTP recording performance)

---

*Last Updated: 2025/07/26*  
*Baseline Platform: Apple M1, Go 1.24*