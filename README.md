# tf

tf is a wrapper for [terraform](https://github.com/hashicorp/terraform/) with much-simplified and improved output.

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
```

## Install

Build from source:

```sh
go build -o tf .
```
