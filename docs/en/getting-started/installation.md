🌐 [Português](../../pt-br/getting-started/installation.md) | **English** | 🏠 [Index](../index.md)

---

# Installation Guide

**sync82** ships as a single static binary — no runtime, no dependency manager, no `node_modules`. Download a release, or build it from source if you have Go installed.

---

## 📋 Prerequisites

| Component | Minimum version | Notes |
| :--- | :--- | :--- |
| Go (source builds only) | matches [`go.mod`](../../../go.mod) | Not needed if you download a release binary |
| Operating system | Linux, macOS, Windows | Prebuilt binaries for amd64/arm64 |

You will also need a **compatible MCP client** to interact with the server. `sync82 install` auto-configures any of these it detects:

- [Claude Code](../guides/clients/claude-code.md)
- [Claude Desktop](../guides/clients/claude-desktop.md) — also installable as a one-click extension, see [below](#-claude-desktop-the-mcpb-bundle)
- [Antigravity](../guides/clients/antigravity.md)
- [OpenAI Codex](../guides/clients/codex.md)
- [OpenCode](../guides/clients/opencode.md)
- [Cursor](../guides/clients/cursor.md)
- [Zed](../guides/clients/zed.md)
- [Cline](../guides/clients/cline.md)

---

## 🚀 Installing sync82

Two ways to get the binary on any OS — either works, but don't mix update mechanisms (see [CLI Reference — self-update](../reference/cli.md#self-update)).

### 🐧 Linux

**Option A — download a release binary (recommended, no Go toolchain needed):**

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/sync82_linux_amd64   # or sync82_linux_arm64
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing   # must print "sync82_linux_amd64: OK"
chmod +x sync82_linux_amd64
sudo mv sync82_linux_amd64 /usr/local/bin/sync82
```

**Option B — build from source:**

```bash
# needs Go 1.26+; Debian/Ubuntu's golang-go package is usually older,
# so install a current toolchain from https://go.dev/dl/ if `go version` is below 1.26

go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

### 🍎 macOS

**Option A — download a release binary (recommended, no Go toolchain needed):**

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/sync82_darwin_arm64   # Intel: sync82_darwin_amd64
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt
grep ' sync82_darwin_arm64$' checksums.txt | shasum -a 256 -c   # must print "sync82_darwin_arm64: OK"
chmod +x sync82_darwin_arm64
sudo mv sync82_darwin_arm64 /usr/local/bin/sync82
```

**Option B — build from source:**

```bash
# Homebrew, if you don't already have a Go toolchain matching go.mod:
brew install go

go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

### 🪟 Windows

**Option A — download a release binary (recommended, no Go toolchain needed):**

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

**Option B — build from source:**

```powershell
# no Go toolchain matching go.mod yet? download the installer from https://go.dev/dl/

go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

---

Building from source (Option B on any OS) installs `sync82` into `$(go env GOPATH)/bin` (or `$GOBIN`/`%GOBIN%`) — make sure that directory is on your `PATH`.

> There's no separate "build step" to run afterward — `go build`/`go install` produces the final, directly runnable binary. `CGO_ENABLED=0` works fine too, since the SQLite driver (`modernc.org/sqlite`) is pure Go.

### 🔒 Verifying a Downloaded Binary

Every release ships a `checksums.txt` with the SHA-256 of every binary. The steps above already check it before the binary is moved into place: `sha256sum -c` (Linux), `shasum -a 256 -c` (macOS) or `Get-FileHash` (Windows) must succeed before you continue.

#### Verifying the Signature (Optional)

`checksums.txt` is itself signed by the release workflow with a keyless [Sigstore](https://www.sigstore.dev/) signature, published as `checksums.txt.sigstore.json`. With [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/) v3 installed:

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt.sigstore.json
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/oito2/mcp-sync82/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

`Verified OK` means `checksums.txt` was produced by this repository's release workflow from a version tag.

#### Verifying the Build Provenance (Optional)

Every binary and the `sync82.mcpb` bundle also carry a [GitHub artifact attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations) (SLSA build provenance) recording the workflow run that built them. With the [GitHub CLI](https://cli.github.com/) installed and logged in:

```bash
gh attestation verify sync82_linux_amd64 --repo oito2/mcp-sync82
```

Pass the file you downloaded (`sync82_darwin_arm64`, `sync82_windows_amd64.exe`, `sync82.mcpb`, ...). A successful verification confirms the file was built by this repository's GitHub Actions.

> `sync82 self-update` verifies each download against `checksums.txt` (SHA-256) only; it doesn't check the signature or the attestation.

### Confirm it's reachable

```bash
which sync82   # or `where sync82` on Windows
```

If that prints nothing, the binary isn't on `PATH` yet — add its directory to `PATH` so you can run `sync82` from your own shell. MCP client configs don't depend on it: `sync82 install` registers the binary's absolute path.

---

## 🔌 Registering the MCP Server

sync82 can configure your MCP client for you:

```bash
sync82 install [target]
```

Run it without a target to list the supported clients detected on your machine (`Detected the following MCP clients:`) and, after `Install sync82 into all N detected client(s)? [y/N]`, configure each of them — a client is detected when its command is on `PATH` or its config directory exists (see [Detection](../architecture/installer.md#detection)); with none detected, it prints `No supported MCP clients detected. Supported targets: ...`. Or pass one target explicitly: `claude`, `claude-desktop`, `antigravity`, `codex`, `opencode`, `cursor`, `zed`, `cline`. An explicit target whose client isn't detected is skipped (`Skipped: <target> not detected.`), not treated as an error. Each configured target prints `configured.` for a new registration or `updated.` for an existing one. Every target is registered with the absolute path of the `sync82` binary you ran — run `sync82 install` again after moving the binary. See [Installer](../architecture/installer.md#targets) for the exact file each target writes, and the client guides above if you'd rather configure a client manually.

To undo it, run `sync82 uninstall` — see [Uninstallation](./uninstallation.md).

### 📦 Claude Desktop: the `.mcpb` bundle

Claude Desktop users can skip the binary entirely: every release ships `sync82.mcpb`, a Claude Desktop extension bundle with the sync82 binaries for macOS, Windows and Linux inside.

1. Download <https://github.com/oito2/mcp-sync82/releases/latest/download/sync82.mcpb>.
2. In Claude Desktop, open **Settings → Extensions → Advanced settings → Extension Developer**, click **Install Extension…** and select the file.
3. Optionally set **Vault database path** in the extension's settings (default `~/.sync82/knowledge.db`).

The bundle is updated by installing a newer `sync82.mcpb`, not with `sync82 self-update`. It's also published in the [MCP Registry](https://registry.modelcontextprotocol.io/) as `io.github.oito2/mcp-sync82`. See the [Claude Desktop guide](../guides/clients/claude-desktop.md).

---

## ⚙️ Picking a Vault Location (Optional)

By default, sync82 stores everything in one SQLite file at `~/.sync82/knowledge.db`. To use a different path for every session without passing `path` on every tool call, set it once:

```bash
sync82 config set-vault /path/to/your/vault.db
```

See [Context Resolution](../architecture/context-resolution.md) for the full priority order between this, the `SYNC82_DB_PATH` environment variable, a workspace's `.sync82.json`, and an explicit `path` argument.

---

## 🔍 Verifying the Installation

After installing and registering the MCP client, verify the server works by asking your AI assistant:

```
List every project in my sync82 vault.
```

The AI will call the `list_projects` tool. An empty list is expected on a fresh vault — it confirms the connection and vault path are both working.

---

## ⬆️ Keeping sync82 Up To Date

```bash
sync82 self-update --check   # report whether a newer release exists, without installing it
sync82 self-update           # download, verify, and install the latest release
```

Downloads are checksum-verified against the release's `checksums.txt` before the running binary is ever replaced; the previous version is kept as `<binary>.bak`, and `sync82 self-update --rollback` restores it. `self-update` works on a release binary or one built with `go install .../sync82@vX.Y.Z`; only a local build from a source checkout or a `go install` of an untagged commit (version `dev`) refuses to run it — re-run `go install .../sync82@latest` instead.

> **Pick one update mechanism and stick with it** — the two paths have no awareness of each other. See [CLI Reference — self-update](../reference/cli.md#self-update) for details.

---

## ➡️ Next steps

With the server installed, configure your MCP client:

- [Configure Claude Code](../guides/clients/claude-code.md)
- [Configure Claude Desktop](../guides/clients/claude-desktop.md)
- [Configure Antigravity](../guides/clients/antigravity.md)
- [Configure OpenAI Codex](../guides/clients/codex.md)
- [Configure OpenCode](../guides/clients/opencode.md)
- [Configure Cursor](../guides/clients/cursor.md)
- [Configure Zed](../guides/clients/zed.md)
- [Configure Cline](../guides/clients/cline.md)

Or jump straight to usage:

- [Quickstart](./quickstart.md)
- [Back to Index](../index.md)
