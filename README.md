# tf (VIBECODED)

tf is an _extremely vibecoded_ wrapper for [terraform](https://github.com/hashicorp/terraform/) with much-simplified and improved output.

## Features

### Collapsed plan/apply output

During `plan` and `apply`, verbose resource change blocks are collapsed to their single-line header:

**Before (`terraform`):**
```
  # module.api.module.ami.aws_ebs_volume.arm64-ubuntu-22_04 must be replaced
-/+ resource "aws_ebs_volume" "arm64-ubuntu-22_04" {
      ~ arn                  = "arn:aws:ec2:us-east-2:..." -> (known after apply)
      ~ create_time          = "2025-11-13T20:45:01Z" -> (known after apply)
        tags                 = {
            "Name" = "arm64-ubuntu-22.04"
        }
      ~ type                 = "gp2" -> (known after apply)
        # (4 unchanged attributes hidden)
    }
```

**After (`tf`):**
```
module.api.module.ami.aws_ebs_volume.arm64-ubuntu-22_04 must be replaced
```

### Interactive diff viewer

When `tf plan` or `tf apply` is run in a terminal (not piped), resource change
blocks are displayed in an interactive viewer after the command completes:

```
 tf interactive diff — 3 resource(s)
  j/k or ↑↓ to navigate  Enter/Space to expand  q to quit

▶ module.api.aws_autoscaling_group.myapp will be destroyed
  module.api.aws_cloudwatch_log_group.myapp-cache will be destroyed
  module.api.aws_launch_template.api will be updated in-place
```

**Keyboard controls:**

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `Enter` / `Space` | Expand / collapse selected resource diff |
| `q` / `Ctrl-C` | Quit viewer |

When `q` is pressed, the viewer exits and the collapsed summaries are printed
to the terminal for scrollback reference.

**Non-interactive fallback:** when stdout is piped or redirected (e.g. in CI),
the plain collapsed output is printed instead:

```sh
tf plan | cat
# module.api.aws_autoscaling_group.myapp will be destroyed
# module.api.aws_cloudwatch_log_group.myapp-cache will be destroyed
# module.api.aws_launch_template.api will be updated in-place
```

### Target flag generation

`tf target` runs `terraform plan` and outputs all changed resources as `-target=` flags, ready to paste into a follow-up `terraform apply` command. Each line ends with `\` for multi-line shell input except the last.

```sh
tf target
# -target=module.api.aws_autoscaling_group.myapp \
# -target=module.api.aws_cloudwatch_log_group.myapp-cache \
# -target=module.api.aws_launch_template.api
```

Additional plan arguments are forwarded:

```sh
tf target -var-file=prod.tfvars
```

### Full terraform passthrough

All arguments and flags are forwarded directly to `terraform`, so `tf` is a complete drop-in replacement.

### Version

```sh
tf ver
# tf v1.2.3
```

## Install

### go install (recommended)

```sh
go install github.com/andygeorge/tf@v1.0.0
```

### Build from source

```sh
git clone https://github.com/andygeorge/tf
cd tf
make build
```

## Usage

```
tf <terraform subcommand> [args...]
```

Examples:

```sh
tf init
tf plan
tf apply
tf plan -target=module.api
tf destroy -auto-approve
tf ver
tf target
tf target -var-file=prod.tfvars
```

## Development

### Local build pipeline

```sh
make check   # fmt + vet + lint + test (full pipeline)
make test    # run tests with race detector and coverage
make build   # compile ./tf binary
make install # install to GOPATH/bin
make clean   # remove build artifacts
make help    # list all targets
```

Optional: install [staticcheck](https://staticcheck.io/) for additional linting:

```sh
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Versioning

Releases follow [semver](https://semver.org/) and are tagged `vX.Y.Z` on the `main` branch.
`go install github.com/andygeorge/tf@latest` always gets the latest release.

To build with an explicit version string:

```sh
make build VERSION=v1.2.3
# or manually:
go build -ldflags "-X main.version=v1.2.3" -o tf .
```
