#!/bin/bash

# Gortex Benchmark Script
# This script runs benchmarks and compares results between branches

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BENCHMARK_DIR="$PROJECT_ROOT/.benchmark"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
BASE_BRANCH="main"
BENCH_COUNT=10
BENCH_TIME="10s"
BENCH_PATTERN="."

# Function to print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -b, --base BRANCH     Base branch to compare against (default: main)"
    echo "  -c, --count N         Number of benchmark runs (default: 10)"
    echo "  -t, --time TIME       Benchmark duration (default: 10s)"
    echo "  -p, --pattern PATTERN Benchmark pattern to run (default: .)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Compare current branch with main"
    echo "  $0 -b develop         # Compare with develop branch"
    echo "  $0 -p Router          # Run only Router benchmarks"
    echo "  $0 -c 5 -t 5s         # Quick benchmark run"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--base)
            BASE_BRANCH="$2"
            shift 2
            ;;
        -c|--count)
            BENCH_COUNT="$2"
            shift 2
            ;;
        -t|--time)
            BENCH_TIME="$2"
            shift 2
            ;;
        -p|--pattern)
            BENCH_PATTERN="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Create benchmark directory
mkdir -p "$BENCHMARK_DIR"

# Get current branch
CURRENT_BRANCH=$(git branch --show-current)
if [ -z "$CURRENT_BRANCH" ]; then
    CURRENT_BRANCH=$(git rev-parse --short HEAD)
fi

echo -e "${GREEN}Gortex Benchmark Comparison${NC}"
echo "================================"
echo "Base branch: $BASE_BRANCH"
echo "Current branch: $CURRENT_BRANCH"
echo "Benchmark pattern: $BENCH_PATTERN"
echo "Count: $BENCH_COUNT, Time: $BENCH_TIME"
echo ""

# Check if benchstat is installed
if ! command -v benchstat &> /dev/null; then
    echo -e "${YELLOW}Installing benchstat...${NC}"
    go install golang.org/x/perf/cmd/benchstat@latest
fi

# Save current state
echo -e "${YELLOW}Saving current state...${NC}"
STASH_NEEDED=false
if [ -n "$(git status --porcelain)" ]; then
    STASH_NEEDED=true
    git stash push -m "benchmark-script-stash"
fi

# Function to run benchmarks
run_benchmarks() {
    local output_file=$1
    echo -e "${YELLOW}Running benchmarks...${NC}"
    
    # Run benchmarks for each major package
    packages=(
        "./http/router"
        "./http/context"
        "./http/middleware"
        "./websocket/hub"
        "./observability"
        "./validation"
    )
    
    > "$output_file"  # Clear file
    
    for pkg in "${packages[@]}"; do
        if [ -d "${pkg#./}" ]; then
            echo "  Benchmarking $pkg..."
            go test -bench="$BENCH_PATTERN" -benchmem -count="$BENCH_COUNT" -benchtime="$BENCH_TIME" -run=^$ "$pkg" >> "$output_file" 2>&1 || true
        fi
    done
}

# Checkout base branch and run benchmarks
echo -e "${GREEN}Running benchmarks on base branch ($BASE_BRANCH)...${NC}"
git checkout "$BASE_BRANCH" --quiet
run_benchmarks "$BENCHMARK_DIR/base.txt"

# Checkout current branch and run benchmarks
echo -e "${GREEN}Running benchmarks on current branch ($CURRENT_BRANCH)...${NC}"
git checkout "$CURRENT_BRANCH" --quiet
run_benchmarks "$BENCHMARK_DIR/current.txt"

# Compare results
echo ""
echo -e "${GREEN}Comparing results...${NC}"
echo "================================"

# Run benchstat
benchstat "$BENCHMARK_DIR/base.txt" "$BENCHMARK_DIR/current.txt" > "$BENCHMARK_DIR/comparison.txt"

# Display results
cat "$BENCHMARK_DIR/comparison.txt"

# Check for regressions
echo ""
echo -e "${GREEN}Checking for regressions...${NC}"
echo "================================"

REGRESSION_FOUND=false
while IFS= read -r line; do
    # Check for significant performance regression (>10%)
    if echo "$line" | grep -E "\+[1-9][0-9]\.[0-9]+%" > /dev/null; then
        percentage=$(echo "$line" | grep -oE "\+[0-9]+\.[0-9]+%" | tr -d '+%')
        if (( $(echo "$percentage > 10" | bc -l) )); then
            echo -e "${RED}⚠️  Performance regression detected: $line${NC}"
            REGRESSION_FOUND=true
        fi
    fi
    
    # Check for significant memory regression (>20%)
    if echo "$line" | grep -E "allocs/op.*\+[2-9][0-9]\.[0-9]+%" > /dev/null; then
        echo -e "${RED}⚠️  Memory allocation regression detected: $line${NC}"
        REGRESSION_FOUND=true
    fi
done < "$BENCHMARK_DIR/comparison.txt"

if [ "$REGRESSION_FOUND" = false ]; then
    echo -e "${GREEN}✅ No significant regressions found!${NC}"
fi

# Generate summary report
echo ""
echo -e "${GREEN}Generating summary report...${NC}"

cat > "$BENCHMARK_DIR/summary.md" << EOF
# Benchmark Comparison Report

**Date**: $(date -u '+%Y-%m-%d %H:%M:%S UTC')  
**Base Branch**: $BASE_BRANCH  
**Current Branch**: $CURRENT_BRANCH  
**Pattern**: $BENCH_PATTERN  

## Summary

$(if [ "$REGRESSION_FOUND" = true ]; then
    echo "⚠️ **Performance regressions detected**"
else
    echo "✅ **No significant regressions found**"
fi)

## Full Comparison

\`\`\`
$(cat "$BENCHMARK_DIR/comparison.txt")
\`\`\`

## Top Performance Changes

### Improvements
\`\`\`
$(grep -E "\-[0-9]+\.[0-9]+%" "$BENCHMARK_DIR/comparison.txt" | sort -k5 -n | head -5 || echo "No improvements found")
\`\`\`

### Regressions
\`\`\`
$(grep -E "\+[0-9]+\.[0-9]+%" "$BENCHMARK_DIR/comparison.txt" | sort -k5 -nr | head -5 || echo "No regressions found")
\`\`\`
EOF

echo "Summary report saved to: $BENCHMARK_DIR/summary.md"

# Restore original state
if [ "$STASH_NEEDED" = true ]; then
    echo -e "${YELLOW}Restoring original state...${NC}"
    git stash pop --quiet
fi

# Exit with error if regressions found
if [ "$REGRESSION_FOUND" = true ]; then
    exit 1
fi