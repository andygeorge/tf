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
  # module.api.module.ami.aws_ebs_volume.arm64-ubuntu-22_04 must be replaced
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
go install github.com/andygeorge/tf@latest
```

### Build from source

```sh
git clone https://github.com/andygeorge/tf
cd tf
go build -o tf .
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
```

## Versioning

Releases follow [semver](https://semver.org/) and are tagged `vX.Y.Z` on the `main` branch.
`go install github.com/andygeorge/tf@latest` always gets the latest release.

To build with an explicit version string:

```sh
go build -ldflags "-X main.version=v1.2.3" -o tf .
```
