# Contributing to sync82

🌐 **Language:** English · [Português](docs/pt-br/contribuindo.md)

Thanks for considering a contribution. This document covers the practical steps: how to build and test the project, what's expected of a pull request, and where to ask questions.

By participating in this project you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Before you start

For anything beyond a small fix (new tools, new CLI commands, schema changes, dependency additions), open an issue first to discuss the approach. This project has a **minimal, justified dependencies** philosophy — see [Concepts — Architecture](docs/en/concepts/architecture.md) and the `require` block in [`go.mod`](go.mod) for the current baseline (`go-sdk`, `modernc.org/sqlite`, and `golang.org/x/text` for the Unicode normalization of full-text search, all pure Go, no cgo). A PR that adds a new dependency without prior discussion is likely to be asked to remove it.

## Development setup

Requirements: a Go toolchain matching the version in [`go.mod`](go.mod).

```bash
git clone https://github.com/oito2/mcp-sync82
cd mcp-sync82
go build ./...
```

Run the same checks CI runs before opening a PR:

```bash
gofmt -l .        # must print nothing
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...   # must report "0 issues."
go build ./...
go test ./... -race
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

All six must pass — [`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs them on every push and pull request against `main` and will block merge otherwise. The tests run on Linux, macOS and Windows (formatting and `govulncheck` on Linux only; golangci-lint runs on Linux for the Linux, Windows and macOS targets, so the files built only for one of them are linted too). golangci-lint uses the repository's `.golangci.yml` and the version pinned by `GOLANGCI_LINT_VERSION` in the workflows, and govulncheck the version pinned by `GOVULNCHECK_VERSION` — bump each there and in the commands above together; a few tests build the real binary and talk to it over stdio — `go test -short ./...` skips those. A test that runs `install` or `uninstall` through `run` must call `isolateClients` (in `cmd/sync82/main_test.go`) first: client detection looks at `PATH` as well as at the home directory, so a temporary `HOME` alone would still let it see, and change, the MCP clients installed on your machine. CI also builds a full set of release artifacts with `scripts/release` (the program the release workflow uses), checks that `self-update` can find, verify and run its binaries, and validates `server.json` with `mcp-publisher` — see [Releasing](#releasing).

## Making a change

1. Fork the repository and create a branch from `main`.
2. Keep the change focused — one logical change per PR. Unrelated cleanups make review slower, not faster.
3. Match existing code style and package layout (see [Concepts — Architecture](docs/en/concepts/architecture.md) for how packages are organized: one tool per file under `internal/tools`, dependency injection for anything touching the network/filesystem/`os.Executable()` outside the store, etc.).
4. Add or update tests for the behavior you changed. This project relies on `go test ./... -race` as the safety net — untested behavior is assumed broken.
5. Update the relevant docs under [`docs/en/`](docs/en/) and its [`docs/pt-br/`](docs/pt-br/) counterpart (see [Documentation](#documentation) below) if you changed a tool's parameters, a CLI command, or the architecture.
6. Run the checks in [Development setup](#development-setup) locally.

## Commit messages

Write a concise summary line explaining *why* the change was made, not just what changed — the diff already shows what changed. Keep related changes in a single commit rather than a string of "fix" follow-ups.

## Documentation

Root-level documents in this repository (README, CONTRIBUTING, CODE_OF_CONDUCT) ship in English (canonical, at the root) and Portuguese (same name, lowercased and translated, inside [`docs/pt-br/`](docs/pt-br/) — `leiame.md`, `contribuindo.md`, `codigo-de-conduta.md` — kept in sync); the `docs/` reference site is split into parallel [`docs/en/`](docs/en/) and [`docs/pt-br/`](docs/pt-br/) trees the same way. If your change affects behavior described in the [Tools Reference](docs/en/reference/tools.md), [CLI Reference](docs/en/reference/cli.md), [Installation Guide](docs/en/getting-started/installation.md), [Architecture](docs/en/concepts/architecture.md), or the README, update both language versions in the same PR — a PR that updates only one will be asked to add the other.

Add an entry under `[Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md) for every user-facing change (internal refactors and test-only changes don't need one). The changelog follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and is kept in English only.

## Pull requests

- Describe what changed and why in the PR description; link the issue it addresses if one exists.
- Keep the PR scoped to the discussed change — large unsolicited refactors are likely to be declined even if the code itself is fine, per this project's preference for minimal, precise changes.
- A maintainer will review, request changes if needed, and merge once CI is green and the discussion is resolved.

## Releasing

Releases are built by `scripts/release` and published by [`.github/workflows/release.yml`](.github/workflows/release.yml). Maintainers only.

**Local dry run** — builds every artifact into `dist/` without publishing anything:

```bash
go run ./scripts/release v1.2.3
```

`dist/` then holds the 6 binaries (`sync82_{linux,darwin,windows}_{amd64,arm64}`, `.exe` on Windows), `sync82.mcpb` (the Claude Desktop bundle), `checksums.txt` and `server.json` (the MCP Registry entry). The version must be `vMAJOR.MINOR.PATCH`; `-allow-prerelease` also accepts a suffix such as `v0.0.0-ci` (that's how CI runs it).

**Publishing** — first, in `CHANGELOG.md`, rename `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD`, add an empty `[Unreleased]` above it, update the comparison links at the bottom, and commit. Then push a `vMAJOR.MINOR.PATCH` tag to trigger the workflow:

```bash
git tag v1.2.3
git push origin v1.2.3
```

| Job | What it does |
|---|---|
| `validate` | Rejects a tag that isn't exactly `vMAJOR.MINOR.PATCH`. |
| `checks` | Runs the CI checks on the tagged commit, on Linux, macOS and Windows (gofmt, golangci-lint and govulncheck on Linux only): vet, build, `go test -race`. |
| `build` | Read-only. Runs `go run ./scripts/release <tag>`, checks that `sync82_linux_amd64 --version` prints the tag, runs the self-update contract test (`TestReleaseContract`) on `dist/`, validates `server.json` with `mcp-publisher validate`, and uploads `dist/` as a workflow artifact. |
| `sign-publish` | The only job with `contents: write`, `id-token: write` and `attestations: write`; it checks out and runs no repository code. Downloads `dist/` (the download fails on a digest mismatch), checks every asset with `sha256sum --check checksums.txt`, signs `checksums.txt` with `cosign sign-blob` (keyless) into `checksums.txt.sigstore.json`, records build provenance attestations for `dist/sync82_*` and `dist/sync82.mcpb`, and creates the GitHub release with every file in `dist/`. |
| `publish-registry` | Downloads `server.json` from the release, logs in to the MCP Registry with GitHub OIDC and publishes it. |

Building and signing are separate jobs so that the code the build runs (the repository, its dependencies and `mcp-publisher`) never holds the permissions that sign, attest or publish. A run that fails before `sign-publish` publishes nothing and can be re-run; a failed `sign-publish` can be re-run on its own for 7 days, while the build artifact exists, and with "Re-run all jobs" after that. `publish-registry` is a separate job so it can be re-run on its own (e.g. after a registry outage) without re-creating the release.

**Pinned tool versions** — before a release, check whether these pins need a bump; Dependabot updates the actions and Go modules, but not these:

| Pin | Where |
|---|---|
| `GOLANGCI_LINT_VERSION` | `env` in `ci.yml` and `release.yml` |
| `GOVULNCHECK_VERSION` | `env` in `ci.yml` and `release.yml` (and the command under [Development setup](#development-setup)) |
| `MCP_PUBLISHER_VERSION`, `MCP_PUBLISHER_SHA256` | the local action `.github/actions/install-mcp-publisher/action.yml` |
| `cosign-release` | the `sigstore/cosign-installer` step of `release.yml` |

**Updating `mcp-publisher`** — the version is pinned by `MCP_PUBLISHER_VERSION` and the SHA-256 of its `mcp-publisher_linux_amd64.tar.gz` by `MCP_PUBLISHER_SHA256`, in the local action [`.github/actions/install-mcp-publisher/action.yml`](.github/actions/install-mcp-publisher/action.yml), which `ci.yml` and `release.yml` both use. Bump the two together; a mismatched hash fails the download check.

## Reporting bugs and requesting features

Open a [GitHub issue](https://github.com/oito2/mcp-sync82/issues) with:

- For bugs: what you ran (`sync82` subcommand or MCP tool call), what you expected, what happened instead, and your `sync82 version` output.
- For features: the problem you're trying to solve, not just the solution you have in mind — see [Before you start](#before-you-start).

## License

By contributing, you agree that your contributions will be licensed under the [GNU General Public License v3.0](LICENSE), the same license that covers the rest of the project.
