# Contributing to CallToVerify

Thanks for your interest. CallToVerify is an open-source ecosystem, and contributions of every
kind are welcome: code, documentation, bug reports, and ideas.

## Ground rules

- Be respectful. We follow the [Code of Conduct](CODE_OF_CONDUCT.md).
- Open an issue before starting large work so we can align on direction.
- Never report a security vulnerability in a public issue. See [SECURITY.md](SECURITY.md).

## Monorepo layout

This is a polyglot monorepo. Each component is self-contained with its own toolchain and README:

| Path | Stack | Build / test |
|---|---|---|
| `coordinator/` | Go | `go build ./...`, `go test ./...`, `go vet ./...` |
| `receiver-android/` | Kotlin | Gradle (planned) |
| `receiver-pi/` | Python | `pytest` (planned) |
| `sdk-server-node/` | Node / TS | `npm test` (planned) |
| `widget-web/` | JS | `npm test` (planned) |

Only `coordinator/` is runnable today. Other directories carry READMEs describing scope.

## Development setup

```bash
# Bring up Postgres + Redis + Coordinator
docker compose up --build

# Or run the Coordinator directly
cd coordinator
go run ./cmd/coordinator
```

Configuration is via environment variables (`CTV_*`); see `coordinator/README.md`.

## Branching and commits

- Branch off `main`. Use descriptive names, e.g. `feat/sms-matching`, `fix/heartbeat-timeout`.
- We use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`,
  `docs:`, `refactor:`, `test:`, `chore:`. Scope by component where useful, e.g.
  `feat(coordinator): add session expiry sweep`.
- Keep PRs focused. One logical change per PR.

## Developer Certificate of Origin (DCO)

By contributing you certify the [DCO](https://developercertificate.org/). Sign off each commit:

```bash
git commit -s -m "feat(coordinator): add session expiry sweep"
```

This adds a `Signed-off-by` line. It is how you assert you have the right to submit the code
under the project's Apache-2.0 license.

## Pull request checklist

- [ ] Code builds and tests pass for the component you touched.
- [ ] New behavior has tests where practical.
- [ ] Public APIs and config are documented.
- [ ] Commits are signed off (`-s`) and follow Conventional Commits.

## Code style

- **Go:** `gofmt` / `go vet` clean. Standard library first; justify new dependencies.
- **JS/TS:** Prettier defaults, two-space indent.
- **Python:** `black` + `ruff`.
- General: see `.editorconfig`. LF line endings, final newline, no trailing whitespace.

## License of contributions

All contributions are licensed under [Apache-2.0](LICENSE), the project license.
