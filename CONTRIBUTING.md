# Contributing

Thanks for your interest in contributing to **vcf-toolkit**.

## Requirements

- Go **1.24.2** or newer
- Node **18+** (only needed if you work on the npm wrapper)

## Development workflow

1. Fork the repo and create a feature branch.
2. Make your changes with clear, focused commits.
3. Run tests before opening a PR:

```/dev/null/tests.sh#L1-L1
 go test ./...
```

4. Format Go code:

```/dev/null/format.sh#L1-L1
 go fmt ./...
```

## Pull requests

- Describe the change and the problem it solves.
- Keep PRs small and focused when possible.
- Include tests when adding or changing behavior.
