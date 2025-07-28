# GitHub Actions Workflows

This directory contains CI/CD workflows for the Gortex framework.

## Workflows

### static-analysis.yml

Runs various static analysis tools on the codebase:

- **Context Propagation Check**: Custom analyzer that ensures proper context.Context usage
- **golangci-lint**: Comprehensive Go linting with multiple linters
- **go vet**: Go's built-in static analyzer
- **go fmt**: Ensures code formatting consistency
- **gosec**: Security vulnerability scanner

This workflow runs on:
- All pushes to main/master/develop branches
- All pull requests targeting these branches

#### Context Checker

The context checker is a custom static analyzer that ensures:
- Functions accepting context.Context properly propagate it to called functions
- Long-running operations check for context cancellation
- Loops in context-aware functions have cancellation check points

Failed checks will:
- Block PR merging
- Post detailed reports as PR comments
- Upload analysis artifacts

### ci.yml

Main continuous integration workflow that:

- **Tests**: Runs all tests with race detection and coverage
- **Multi-version**: Tests against Go 1.22, 1.23, and 1.24
- **Examples**: Builds all example applications
- **Cross-platform**: Builds for Linux, macOS, Windows (amd64 and arm64)
- **Integration Tests**: Runs with PostgreSQL service container

### benchmark.yml

Performance regression testing for pull requests:

- **Automated Comparison**: Compares PR performance against base branch
- **Statistical Analysis**: Uses benchstat for reliable comparisons
- **PR Comments**: Posts detailed performance reports on PRs
- **Regression Detection**: Fails if performance degrades >10%
- **Memory Tracking**: Monitors allocation changes

Features:
- Runs 10 iterations for statistical significance
- Uploads benchmark artifacts for analysis
- Provides clear performance impact visualization
- Configurable thresholds via benchmark-thresholds.yml

### benchmark-continuous.yml

Continuous performance monitoring on main branch:

- **Scheduled Runs**: Weekly performance baseline updates
- **Historical Tracking**: Stores results in gh-pages branch
- **Visualization**: Generates performance trend graphs
- **Profiling**: Optional CPU and memory profiling
- **Alerts**: Notifies on performance regressions

Features:
- Integration with github-action-benchmark
- Custom metrics extraction for key operations
- Profile generation for deep performance analysis
- Automatic performance dashboards

## Configuration Files

### .golangci.yml

Configuration for golangci-lint with:
- Comprehensive linter selection
- Custom rules for the project
- Exclusions for test files and generated code

## Configuration Files

### benchmark-thresholds.yml

Defines performance regression thresholds:
- Global defaults for time, memory, and allocations
- Package-specific overrides for critical paths
- Individual benchmark customization

## Scripts

### scripts/benchmark.sh

Local benchmark comparison tool:

```bash
# Compare with main branch
./scripts/benchmark.sh

# Compare with specific branch
./scripts/benchmark.sh -b develop

# Run specific benchmarks
./scripts/benchmark.sh -p Router

# Quick run with fewer iterations
./scripts/benchmark.sh -c 5 -t 5s
```

## Running Locally

To run the context checker locally:

```bash
# Build the tool
go build -o bin/contextcheck ./internal/analyzer/cmd/contextcheck

# Run on specific packages
./bin/contextcheck ./core/...

# Run on entire project
./bin/contextcheck ./...
```

To run golangci-lint locally:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linting
golangci-lint run

# Run with specific linters
golangci-lint run --enable-all
```

To run benchmarks locally:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run with benchstat comparison
./scripts/benchmark.sh

# Run specific package benchmarks
go test -bench=. -benchmem -count=10 ./http/router
```

## Adding New Checks

To add new static analysis checks:

1. Create analyzer in `internal/analyzer/`
2. Add to static-analysis.yml workflow
3. Configure as required/optional check
4. Update this README

## Troubleshooting

### Context Checker False Positives

If the context checker reports false positives, you can:

1. Add `//nolint:contextcheck` comment to suppress specific lines
2. Update the analyzer logic in `internal/analyzer/context_checker.go`
3. File an issue with the false positive example

### Workflow Failures

Check the workflow logs in the Actions tab for detailed error messages. Common issues:

- **Timeout**: Increase timeout in workflow or golangci.yml
- **OOM**: Reduce parallelism or exclude large packages
- **Permission errors**: Check GITHUB_TOKEN permissions