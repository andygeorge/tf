# Tech Stack

## Language

- **Go 1.22** (module: `github.com/andygeorge/tf`)

## Type

CLI tool — no frontend, no backend framework, no database.

## Key Dependencies

- **Go standard library only** — no external runtime dependencies (see `go.mod`)
- **terraform** — external binary dependency; invoked as a subprocess

## Build Tooling

| Tool | Purpose |
|------|---------|
| `go build` | Compile binary |
| `go test` | Run tests with race detector and coverage |
| `gofmt` | Code formatting |
| `go vet` | Static analysis |
| `staticcheck` | Additional linting (`honnef.co/go/tools/cmd/staticcheck`) |
| `make` | Local build pipeline |

### Make targets

```sh
make check   # fmt + vet + lint + test (full pipeline)
make test    # run tests with race detector and coverage
make build   # compile ./tf binary
make install # install to GOPATH/bin
make clean   # remove build artifacts
```

## Distribution

- **`go install github.com/andygeorge/tf@latest`** — primary install method via Go module proxy
- **Build from source** — `git clone` + `make build`

## Versioning

- Releases tagged `vX.Y.Z` on `main` branch (semver)
- Version injected at build time via `-ldflags "-X main.version=vX.Y.Z"`

## Runtime Requirements

- Go 1.22+ (for build)
- `terraform` binary in `$PATH` (for runtime)
