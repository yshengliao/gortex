# Gortex Optimization Plan

This document outlines a phased plan to improve the Gortex framework based on the initial review. Each phase maps to a separate commit in order to keep the history clear.

## Phase 1 – CLI Project Initialization Fixes
- Correct the `go.mod` generation bug (`GORTEXVersion` variable) in `cmd/gortex/commands/init.go`.
- Add tests to ensure project scaffolding succeeds without template errors.

## Phase 2 – Code Generation Cleanup
- Replace deprecated `strings.Title` usage in `cmd/gortex/commands/generate.go` with `cases.Title` from `golang.org/x/text/cases`.
- Update generator tests accordingly.

## Phase 3 – Configuration Consistency
- Review default configuration prefixes and unify them (e.g. `STMP_` vs `GORTEX_`).
- Document the chosen prefix in the README.

## Phase 4 – CLI Test Expansion
- Add tests for `runDevServer` and other CLI utilities.
- Extend template generation tests to cover all created files.

Each phase should include test updates and a verification run (`go test ./...`) before committing.
