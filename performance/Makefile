.PHONY: help bench report detect all clean build

# Default target
help:
	@echo "Gortex Performance Tools"
	@echo ""
	@echo "Available targets:"
	@echo "  make bench    - Run performance benchmarks"
	@echo "  make report   - Generate performance report"
	@echo "  make detect   - Detect performance bottlenecks"
	@echo "  make all      - Run benchmarks, generate report, and detect bottlenecks"
	@echo "  make clean    - Clean generated files"
	@echo "  make build    - Build the perfcheck CLI tool"

# Build the CLI tool
build:
	@echo "Building perfcheck CLI..."
	@go build -o perfcheck ./cmd/perfcheck

# Run benchmarks
bench: build
	@echo "Running performance benchmarks..."
	@./perfcheck -bench

# Generate report
report: build
	@echo "Generating performance report..."
	@./perfcheck -report

# Detect bottlenecks
detect: build
	@echo "Detecting performance bottlenecks..."
	@./perfcheck -detect

# Run everything
all: build
	@echo "Running complete performance analysis..."
	@./perfcheck -bench -report -detect

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -f perfcheck
	@rm -rf reports/*.md
	@echo "Done. Note: benchmark database preserved for historical tracking"

# Run Go benchmarks directly
gobench:
	@echo "Running Go benchmarks..."
	@go test -bench=. -benchmem -benchtime=10s . | tee benchmark_results.txt

# Compare with baseline
compare: gobench
	@echo "Comparing with baseline..."
	@if [ -f baseline.txt ]; then \
		benchstat baseline.txt benchmark_results.txt; \
	else \
		echo "No baseline found. Creating baseline..."; \
		mv benchmark_results.txt baseline.txt; \
		echo "Baseline created. Run 'make compare' again after changes."; \
	fi

# Update baseline
baseline: gobench
	@echo "Updating baseline..."
	@mv benchmark_results.txt baseline.txt
	@echo "Baseline updated"

# Profile CPU
profile-cpu:
	@echo "Running CPU profile..."
	@go test -bench=BenchmarkGortexRouter -cpuprofile=cpu.prof -benchtime=10s
	@echo "Profile saved to cpu.prof. View with: go tool pprof cpu.prof"

# Profile memory
profile-mem:
	@echo "Running memory profile..."
	@go test -bench=BenchmarkGortexRouter -memprofile=mem.prof -benchtime=10s
	@echo "Profile saved to mem.prof. View with: go tool pprof mem.prof"