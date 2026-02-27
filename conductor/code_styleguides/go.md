# Go Style Guide

## Tooling

This project uses the following tools, configured in `Makefile`:

| Tool | Command | Purpose |
|------|---------|---------|
| `gofmt` | `make fmt` | Formatting — reports unformatted files, does not auto-fix |
| `go vet` | `make vet` | Standard static analysis |
| `staticcheck` | `make lint` | Advanced linting (optional; skipped if not installed) |
| `go test -race` | `make test` | Tests with race detector + coverage |

Run the full pipeline before committing:
```sh
make check
```

To auto-format files:
```sh
gofmt -w .
```

## Formatting

- Follow `gofmt` output exactly — no manual overrides.
- Tabs for indentation (Go standard).
- No trailing whitespace.
- One blank line between top-level declarations.

## Naming

| Item | Convention | Example |
|------|-----------|---------|
| Packages | lowercase, single word | `main` |
| Exported identifiers | CamelCase | `RunPlan` |
| Unexported identifiers | camelCase | `parseLine` |
| Constants | CamelCase (exported) / camelCase (unexported) | `MaxRetries` / `defaultTimeout` |
| Acronyms | All caps if exported | `parseURL`, not `parseUrl` |

## Code Organization

- Keep `main.go` thin — delegate logic to focused functions.
- Group related functions together; no strict file-per-type requirement for a small CLI.
- Prefer flat package structure — avoid premature sub-packages.
- One package: `main` (appropriate for a single-binary CLI tool).

## Error Handling

- Return errors; don't panic in library-style functions.
- Use `fmt.Errorf("context: %w", err)` to wrap errors with context.
- Handle errors at the call site — don't ignore them.
- Exit with a non-zero code on failure; preserve terraform's exit code on passthrough.

## Comments

- Exported functions must have a doc comment (`// FunctionName does X`).
- Unexported functions: comment only when the logic isn't self-evident.
- No commented-out code in commits.

## Testing

- Use `testing` package (standard library).
- Test files: `*_test.go` alongside the file under test.
- Table-driven tests preferred for multiple input cases.
- Run with race detector: `go test -race ./...`
- Coverage tracked via `coverage.out` (`go tool cover -func=coverage.out`).

## Dependencies

- **No external runtime dependencies** — standard library only.
- Dev/build tools (staticcheck) are optional and installed separately.
- Keep `go.mod` minimal.

## CLI Conventions

- Write user-facing output to `os.Stdout`.
- Write errors to `os.Stderr`.
- Preserve terraform's exit code when passing through.
- Keep `tf`-specific output minimal — don't pollute the signal.
