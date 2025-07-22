# Gortex Optimization Plan

This document outlines a phased plan to improve the Gortex framework based on the initial review. Each phase maps to a separate commit in order to keep the history clear.

## Phase 1 – CLI Project Initialization Fixes *(Completed)*
- Corrected the `go.mod` generation bug (`GORTEXVersion` variable) in `cmd/gortex/commands/init.go`.
- Added tests to ensure project scaffolding succeeds without template errors.

## Phase 2 – Code Generation Cleanup *(Completed)*
- Replaced deprecated `strings.Title` usage in `cmd/gortex/commands/generate.go` with `cases.Title` from `golang.org/x/text/cases`.
- Updated generator tests accordingly.

## Phase 3 – Configuration Consistency *(Completed)*
- Reviewed default configuration prefixes and unified them (e.g. `STMP_` vs `GORTEX_`).
- Documented the chosen prefix in the README.

## Phase 4 – CLI Test Expansion (In Progress)
- Increase coverage of CLI commands:
  - exercise `gortex server` via `runDevServer` with various flags
  - test the `init` and `generate` subcommands through Cobra's command API
  - verify help output and error handling
- Extend template generation tests to ensure all files are created and contain expected placeholders
- Consolidate test helpers for easier future additions

## Additional Improvements
- Replace any remaining deprecated functions across the codebase with their modern equivalents (e.g., `strings.Title` -> `cases.Title`)
- Review the codebase for other outdated APIs and update accordingly

Each phase should include test updates and a verification run (`go test ./...`) before committing.
