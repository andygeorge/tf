# Workflow

## TDD Policy

**Moderate** — Tests are encouraged but do not block implementation.

- Write tests for new features and non-trivial logic.
- Bug fixes should include a regression test where practical.
- Run `make test` before committing.
- Tests use the Go standard `testing` package with the race detector enabled.

## Commit Strategy

**Conventional Commits** — all commits follow this format:

```
<type>: <short description>
```

Types:
- `feat:` — new feature
- `fix:` — bug fix
- `chore:` — maintenance, dependencies, tooling
- `docs:` — documentation only
- `test:` — tests only
- `refactor:` — code restructuring without behavior change

Examples:
```
feat: add tf target subcommand
fix: preserve exit code from terraform subprocess
chore: bump go version to 1.22
docs: update README with install instructions
```

## Code Review

**Optional / self-review OK** — Solo project; self-review is sufficient for most changes. Use PRs for significant features or changes that benefit from a structured review record.

## Verification Checkpoints

**Only at track completion** — Manual verification is required when a full feature track is done, not after individual tasks or phases.

Verification includes:
1. `make check` passes (fmt + vet + lint + test)
2. Manual smoke test: `tf plan` and `tf target` behave correctly against a real or mock terraform project
3. README updated if any user-facing behavior changed

## Task Lifecycle

1. Create a track with `/conductor:new-track`
2. Implement tasks in the track's phases
3. Run `make check` after each task
4. Commit with Conventional Commit message
5. Verify at track completion before marking the track done

## Build Pipeline

Always run `make check` before considering a task complete:

```sh
make check   # fmt + vet + lint + test
```
