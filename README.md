# sync82

<p align="center">
  <img src="docs/img/github-header.png" alt="sync82 by OITO2 Labs — Persistent and shared memory among AI agents" width="100%">
</p>

[![CI](https://github.com/oito2/mcp-sync82/actions/workflows/ci.yml/badge.svg)](https://github.com/oito2/mcp-sync82/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/oito2/mcp-sync82.svg)](https://pkg.go.dev/github.com/oito2/mcp-sync82)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Release](https://img.shields.io/github/v/release/oito2/mcp-sync82?sort=semver)](https://github.com/oito2/mcp-sync82/releases)
[![License](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Code: AI-Assisted](https://img.shields.io/badge/Code-AI--Assisted-blueviolet)](#ai-usage-in-this-project)

🌐 **Language:** English · [Português](docs/pt-br/leiame.md)

`sync82` is a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that gives an AI coding agent persistent, structured memory of a software project across sessions — goals, architecture, tech stack, decisions, and progress, all stored locally and recalled automatically the next time the agent opens the project.

## Table of Contents

- [Overview](#overview)
- [Prerequisites & Quick Installation](#prerequisites--quick-installation)
- [Client Setup](#client-setup)
- [Update & Maintenance](#update--maintenance)
- [Documentation](#documentation)
- [AI Usage in This Project](#ai-usage-in-this-project)
- [License](#license)

## Overview

Memory is stored in an embedded SQLite database and ships as a single self-contained binary — no runtime toolchain, no npm package, just `sync82` on your `PATH`. The agent reads and writes it through MCP tools; you talk to the agent in plain language.

- **Six standard memory files per project**, plus any custom file, for projects and their subprojects (monorepos, plugin ecosystems).
- **Automatic project discovery** — a `.sync82.json` in the workspace means the agent never has to name the project again.
- **19 MCP tools** — load a whole project's context in one call, save a session in one call, search the whole vault, archive old history, export/import plain Markdown — plus read-only MCP resources with each project's memory and `start_session`/`end_session` prompts.
- **One-step client setup** — `sync82 install` registers sync82 in Claude Code, Claude Desktop, Antigravity, Codex, OpenCode, Cursor, Zed and Cline; `sync82 uninstall` removes it.
- **Local and private** — stdio transport only, no network service, one SQLite file per vault.
- **Verifiable releases** — SHA-256 checksums, a Sigstore signature and GitHub build provenance attestations; a Claude Desktop extension (`sync82.mcpb`) and an [MCP Registry](https://registry.modelcontextprotocol.io/) entry.

| File | Kind | Purpose |
|---|---|---|
| `memory` | overwrite | Project overview: name, description, goal |
| `architecture` | overwrite | Components and how they fit together |
| `stack` | overwrite | Languages, frameworks, infrastructure |
| `decisions` | append-only, dated | Decisions made and why |
| `progress` | append-only, dated | Work completed, session by session |
| `next_steps` | overwrite | What to do next |

### Tools (19)

The AI agent calls these over MCP — it never touches the database directly. Full parameter reference in [Tools Reference](docs/en/reference/tools.md).

| Tool | What it does |
|---|---|
| `list_projects` | List every project and subproject in the vault |
| `create_project` | Create a new project or subproject |
| `delete_project` | Permanently delete a project or subproject (requires confirmation) |
| `rename_project` | Rename a project or subproject in place |
| `get_vault_config` | Report the active vault path and config |
| `list_files` | List every memory file recorded for a project |
| `read_memory` | Read a memory file's content |
| `write_memory` | Overwrite a memory file's entire content |
| `append_memory` | Append a dated entry to `progress`, `decisions`, or a custom append kind |
| `delete_memory` | Delete a custom memory file (the six standard files are protected) |
| `edit_entry` | Replace, supersede or delete one entry of `progress`, `decisions`, or a custom append kind |
| `archive_memory` | Archive old dated entries, keeping only the last N days active, optionally with a summary of them |
| `search_memory` | Search memory files by words (accents and case ignored, best matches first), phrase or exact text |
| `load_project_context` | Load a project's memory (current state plus recent history) into one context block |
| `check_project_health` | Report which of the six standard files exist, and warn about files that look out of date |
| `init_project_memory` | Guided initialization, with optional auto-detection from the codebase |
| `update_project_memory` | Save a session's work (progress, decisions, next steps, etc.) in one call |
| `export_memory` | Export a project's memory to plain `.md` files on disk |
| `import_memory` | Import a project's memory from plain `.md` files — the inverse of `export_memory` |

`list_projects`, `list_files`, `check_project_health` and `search_memory` also return JSON (`format: "json"`). Besides tools, sync82 serves each project's memory as [MCP resources](docs/en/reference/resources.md) (`sync82://projects/<project>/context`) and two [MCP prompts](docs/en/reference/mcp-prompts.md), `start_session` and `end_session`. Not sure what to type to your agent? See [Example Prompts](docs/en/prompts.md).

### CLI commands

| Command | What it does |
|---|---|
| `sync82` (no args) | Start the MCP server over stdio — this is what your client launches |
| `sync82 install [target]` | Wire sync82 into one or all supported MCP clients |
| `sync82 uninstall [target] [--purge]` | Remove sync82 from one or all clients; `--purge` also deletes `~/.sync82` files after a separate confirmation |
| `sync82 config set-vault\|get-vault\|unset-vault` | Manage the global vault path override |
| `sync82 self-update [--check] [--yes] [--require-signature] \| --rollback` | Check GitHub Releases and update the binary in place, verifying the signature when `cosign` is installed (`--rollback` restores the previous version) |
| `sync82 export <project> [subproject] <output-dir>` | Dump a project's memory to plain `.md` files (`--all` for the whole vault) |
| `sync82 import <project> [subproject] <input-dir> [--dry-run]` | Restore a project's memory from plain `.md` files (the inverse of `export`); `--dry-run` only reports what would change |
| `sync82 search <query> [--project P] [--json]` | Search the memory, like the `search_memory` tool |
| `sync82 context <project> [subproject] [--full]` | Print a project's memory, like the `load_project_context` tool |
| `sync82 health <project> [subproject] \| --all` | Check a project, or every project, for missing files and out-of-date memory |
| `sync82 help` / `--help` / `-h` | Print the list of subcommands |
| `sync82 version` / `--version` / `-v` | Print the installed version |

Full flags, exit codes, and examples for every command: [CLI Reference](docs/en/reference/cli.md).

## Prerequisites & Quick Installation

**Prerequisites:** Linux, macOS, or Windows (amd64/arm64) — a release binary needs nothing else; building from source needs a Go toolchain matching [`go.mod`](go.mod) (1.26+). You'll also need an MCP client (see [Client Setup](#client-setup)). Claude Desktop users can skip this section and install the [`.mcpb` extension](#client-setup) instead.

Two ways to get the binary on any OS — either works, but don't mix update mechanisms (see [CLI Reference — self-update](docs/en/reference/cli.md#self-update)).

### Linux

**Prebuilt binary** (no Go toolchain needed):

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/sync82_linux_amd64   # or sync82_linux_arm64
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing   # must print "sync82_linux_amd64: OK"
chmod +x sync82_linux_amd64
sudo mv sync82_linux_amd64 /usr/local/bin/sync82
```

**From source** (requires Go 1.26+ — download it from [go.dev/dl](https://go.dev/dl/); distribution packages such as Debian/Ubuntu's `golang-go` are usually older):

```bash
go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

### macOS

**Prebuilt binary** (no Go toolchain needed):

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/sync82_darwin_arm64   # Intel: sync82_darwin_amd64
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt
grep ' sync82_darwin_arm64$' checksums.txt | shasum -a 256 -c   # must print "sync82_darwin_arm64: OK"
chmod +x sync82_darwin_arm64
sudo mv sync82_darwin_arm64 /usr/local/bin/sync82
```

**From source** (requires a Go toolchain — `brew install go`):

```bash
go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

### Windows

**Prebuilt binary** (no Go toolchain needed — PowerShell):

```powershell
$base = "https://github.com/oito2/mcp-sync82/releases/latest/download"
Invoke-WebRequest -Uri "$base/sync82_windows_amd64.exe" -OutFile sync82_windows_amd64.exe
Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile checksums.txt
$expected = ((Select-String -Path checksums.txt -SimpleMatch "sync82_windows_amd64.exe").Line -split '\s+')[0]
if ((Get-FileHash sync82_windows_amd64.exe -Algorithm SHA256).Hash -ne $expected) { throw "checksum mismatch" }
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\sync82" | Out-Null
Move-Item sync82_windows_amd64.exe "$env:LOCALAPPDATA\sync82\sync82.exe"
# add $env:LOCALAPPDATA\sync82 to PATH: System Properties > Environment Variables
```

**From source** (requires a Go toolchain — [installer](https://go.dev/dl/)):

```powershell
go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

---

Every release ships a `checksums.txt` (verified in the steps above), a Sigstore bundle signing it (`checksums.txt.sigstore.json`), and a GitHub build provenance attestation for every binary and the `.mcpb` bundle (`gh attestation verify <file> --repo oito2/mcp-sync82`) — see the [Installation Guide](docs/en/getting-started/installation.md#-verifying-a-downloaded-binary).

`go install`/`go build` produces the final binary directly, ready to run — make sure `$(go env GOPATH)/bin` (or `%GOBIN%`/`$GOBIN`) is on your `PATH`; check with `which sync82` (`where sync82` on Windows).

## Client Setup

Register sync82 in every supported client detected on your machine, in one step:

```bash
sync82 install            # lists the detected clients, asks to confirm, configures each of them
sync82 install claude     # configure a single client instead
```

Targets: `claude`, `claude-desktop`, `antigravity`, `codex`, `opencode`, `cursor`, `zed`, `cline`. Each client is registered with the absolute path of the `sync82` binary you ran, so run `sync82 install` again if you move the binary. Each target prints `configured.` or `updated.`; a client that isn't detected (its command on `PATH` or its config directory) is skipped.

**Claude Code** by hand (user scope, every project):

```bash
claude mcp add --scope user sync82 -- /usr/local/bin/sync82   # the path printed by `which sync82`
claude mcp list
```

**Claude Desktop** — no binary needed: download [`sync82.mcpb`](https://github.com/oito2/mcp-sync82/releases/latest/download/sync82.mcpb) and install it from Claude Desktop's **Settings → Extensions → Advanced settings → Extension Developer → Install Extension…**. sync82 is also listed in the [MCP Registry](https://registry.modelcontextprotocol.io/) as `io.github.oito2/mcp-sync82`.

Per-client guides, with manual configuration and troubleshooting: [Claude Code](docs/en/guides/clients/claude-code.md) · [Claude Desktop](docs/en/guides/clients/claude-desktop.md) · [Antigravity](docs/en/guides/clients/antigravity.md) · [Codex](docs/en/guides/clients/codex.md) · [OpenCode](docs/en/guides/clients/opencode.md) · [Cursor](docs/en/guides/clients/cursor.md) · [Zed](docs/en/guides/clients/zed.md) · [Cline](docs/en/guides/clients/cline.md). Any other client just needs `command` set to the binary's absolute path — see [Installer](docs/en/architecture/installer.md#clients-not-in-this-list).

## Update & Maintenance

```bash
sync82 self-update --check   # report whether a newer release exists (exit code 10 when one does)
sync82 self-update           # download, verify (signature with cosign, SHA-256) and install the latest release
sync82 self-update --rollback   # restore the previous version, kept as <binary>.bak
```

`self-update` works on a release binary or a `go install .../sync82@vX.Y.Z` build. If you installed with `go install`, update with `go install github.com/oito2/mcp-sync82/cmd/sync82@latest` instead — don't mix the two. The Claude Desktop extension is updated by installing a newer `sync82.mcpb`. A release can upgrade the vault schema (1.1.0 and 1.2.0 do); once the new version has opened a vault, the previous binary — including one restored by `--rollback` — refuses that vault, so update every client that shares it and don't roll back past such a release (see [Troubleshooting](docs/en/troubleshooting/common-issues.md#vault-schema-version-is-newer-than-this-sync82-supports)).

To remove sync82 from every detected client, run `sync82 uninstall` (add `--purge` to also delete the default vault and config in `~/.sync82`) — see [Uninstallation](docs/en/getting-started/uninstallation.md) for the complete removal, binary included.

## Documentation

The [documentation site](docs/en/index.md) has the full detail (also in [Portuguese](docs/pt-br/index.md)):

- [Installation](docs/en/getting-started/installation.md) · [Quickstart](docs/en/getting-started/quickstart.md) · [Uninstallation](docs/en/getting-started/uninstallation.md)
- [Concepts — Architecture](docs/en/concepts/architecture.md) and [internals](docs/en/architecture/context-resolution.md)
- Reference: [Tools](docs/en/reference/tools.md) · [Resources](docs/en/reference/resources.md) · [MCP Prompts](docs/en/reference/mcp-prompts.md) · [CLI](docs/en/reference/cli.md) · [Configuration](docs/en/reference/configuration.md)
- [Example Prompts](docs/en/prompts.md) · [Usage Examples](docs/en/guides/workflows/examples.md)
- [Troubleshooting](docs/en/troubleshooting/common-issues.md) · [Changelog](CHANGELOG.md)

**Contributing:** see [`CONTRIBUTING.md`](CONTRIBUTING.md) for the development workflow, the checks CI runs (`go build ./...`, `go vet ./...`, `gofmt -l .`, golangci-lint, `go test ./... -race`, `govulncheck`), and how releases are published. Everyone participating is expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## AI Usage in This Project

This project was developed with the assistance of generative AI tools:

- **Scope:** Generation of boilerplate, unit tests and refactoring of helper functions.

- **Oversight:** All generated code was manually reviewed, tested and validated before integration.

## License

GPL-3.0 — see [LICENSE](LICENSE).
