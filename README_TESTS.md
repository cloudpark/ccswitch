# ccswitch Test Suite

This project includes comprehensive tests for the ccswitch CLI tool.

## Test Files

Tests live alongside the code they cover, one `*_test.go` file per package:

- **cmd/integration_test.go** - integration tests for the CLI commands (requires git)
- **internal/config/config_test.go** - unit tests for global config load/save/defaults
- **internal/repoconfig/repoconfig_test.go** - unit tests for the repo-level `.ccswitch.yaml` loader/scaffolder
- **internal/trust/trust_test.go** - unit tests for the post-create-hook trust store
- **internal/hooks/hooks_test.go** - unit tests for running post-create commands
- **internal/git/worktree_test.go** - unit + integration tests for worktree/branch parsing and real git operations (requires git)
- **internal/session/manager_test.go** - integration tests for session creation/checkout (requires git)
- **internal/utils/slugify_test.go** - unit tests for the `Slugify()` function
- **bash_wrapper_test.sh** - tests for the generated shell wrapper function (command passthrough, session creation output parsing)

## Running Tests

### All Tests
```bash
go test -v -cover ./...
```

### Via Make
```bash
make test        # unit + integration (see caveat below)
make test-unit    # a name-filtered subset of unit tests
make test-integration  # a name-filtered subset of integration tests
make test-docker  # tests in a clean Docker environment
make coverage     # generate coverage.html
make test-bash    # run bash_wrapper_test.sh
```

> **Note:** `test-unit`/`test-integration` filter by test name via `go test -run <regex>` (see the `Makefile`). That regex is a manually maintained allowlist and can lag behind newly added tests in newer packages - if you add a test and it doesn't seem to run under `make test`, run `go test ./...` directly to confirm it passes, and consider updating the regex in the `Makefile`.

### Coverage Report
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Bash Wrapper Tests
```bash
./bash_wrapper_test.sh
```

## Benchmarks

The test suite includes benchmarks for performance-critical functions:
```bash
go test -bench=. ./...
```
