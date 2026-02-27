# Product Definition

## Project Name

tf

## Description

A CLI tool that wraps terraform with simplified, improved output.

## Problem Statement

Terraform's verbose output makes it hard to quickly understand what's changing. Resource change blocks are large and noisy, and generating `-target=` flags manually is tedious and error-prone.

## Target Users

Infrastructure engineers and DevOps practitioners using Terraform daily to manage cloud infrastructure.

## Key Goals

1. **Seamless drop-in** — Be a complete drop-in replacement for `terraform`; all subcommands and flags pass through transparently.
2. **Minimal noise** — Collapse verbose plan/apply output to single-line summaries so engineers can quickly scan what's changing.
3. **Easy targeting** — `tf target` automatically generates `-target=` flags for all changed resources, ready to paste into `terraform apply`.

## Key Features

- **Collapsed plan/apply output** — Verbose resource change blocks collapsed to single-line headers.
- **Target flag generation** — `tf target` outputs all changed resources as `-target=` flags.
- **Full terraform passthrough** — All arguments and flags forwarded directly to `terraform`.
- **Version** — `tf ver` prints the current `tf` version.
