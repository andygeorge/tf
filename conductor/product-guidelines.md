# Product Guidelines

## Voice and Tone

**Concise and direct.**

- Use short, declarative sentences.
- Avoid unnecessary words or filler.
- Technical precision is preferred over approachability.
- CLI output should be minimal — only show what matters.
- Error messages should state the problem clearly without padding.

## Design Principles

### 1. Simplicity over features

- Add features only when they clearly reduce friction for terraform users.
- Avoid scope creep — `tf` is a wrapper, not a replacement.
- When in doubt, don't add it.

### 2. Developer experience focused

- The primary audience is engineers who run terraform commands frequently.
- Every UX decision should reduce keystrokes or cognitive load.
- Output should be immediately scannable.
- Passthrough behavior must be invisible and reliable.

## Additional Standards

- **No breaking changes** to terraform passthrough behavior.
- **Fail loudly** — if `tf` itself errors, surface the error clearly; don't suppress it.
- **No hidden state** — `tf` should not persist state beyond what terraform already manages.
