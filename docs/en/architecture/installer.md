🌐 [Português](../../pt-br/architecture/installer.md) | **English** | 🏠 [Index](../index.md)

---

# Installer

How `sync82 install` and `sync82 uninstall` configure each of the 8 supported MCP client targets. The target list lives in `internal/installer/targets.go`; the commands are wired in `cmd/sync82/install.go` and `cmd/sync82/uninstall.go`.

---

## Two kinds of targets

- **CLI targets** (`claude`, `codex`) are installed by removing any existing registration and then running the client's own `mcp add` subcommand, which inherits your terminal's stdin/stdout/stderr — if that command prompts for anything, you'll see it directly.
- **File targets** (`claude-desktop`, `antigravity`, `opencode`, `cursor`, `zed`, `cline`) are installed by reading the client's JSON config (a missing or blank file counts as `{}`), merging in a `sync82` entry, and writing it back atomically, keeping every other key, the file's permissions, a symlink at that path, and its numbers exactly as written.

Which targets count as installed on your machine is decided per target — see [Detection](#detection).

`<absolute-path>` below is the absolute path of the running `sync82` binary, with symlinks resolved — not the bare command `sync82` — so the binary doesn't need to be on the client's `PATH`. `install` prints `Registering <path> — run "sync82 install" again after moving the binary.` before touching any target.

---

## Targets

| Target | Kind | Install | Config files |
|---|---|---|---|
| `claude` | CLI | [Remove existing registrations](#removing-existing-cli-registrations), then `claude mcp add --scope user sync82 -- <absolute-path>` | `~/.claude.json`, written by Claude Code |
| `claude-desktop` | file | `mcpServers.sync82 = {command, args}` | macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`; Windows: `%APPDATA%\Claude\claude_desktop_config.json`; Linux: `$XDG_CONFIG_HOME/Claude/claude_desktop_config.json` (default `~/.config/Claude/claude_desktop_config.json`); any other OS is skipped with `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).` |
| `antigravity` | file | `mcpServers.sync82 = {command, args}` | Install writes only `~/.gemini/config/mcp_config.json`; uninstall also cleans the legacy `~/.gemini/antigravity/mcp_config.json` and `~/.gemini/antigravity-ide/mcp_config.json` |
| `codex` | CLI | `codex mcp get sync82`, then `codex mcp remove sync82` when it is registered, then `codex mcp add sync82 -- <absolute-path>` | `~/.codex/config.toml`, written by Codex |
| `opencode` | file | `mcp.sync82 = {"type": "local", "command": ["<absolute-path>"], "enabled": true}` | Install writes `$XDG_CONFIG_HOME/opencode/opencode.json` (default `~/.config/opencode/opencode.json`), or `opencode.jsonc` when that is the only one of the two that exists; uninstall cleans `opencode.json`, `opencode.jsonc` and the legacy `config.json` in that directory |
| `cursor` | file | `mcpServers.sync82 = {type: "stdio", command, args}` | `~/.cursor/mcp.json` |
| `zed` | file | `context_servers.sync82 = {command, args}` | Linux: `$XDG_CONFIG_HOME/zed/settings.json` (default `~/.config/zed/settings.json`); macOS: `~/.config/zed/settings.json`; Windows: `%APPDATA%\Zed\settings.json` |
| `cline` | file | `mcpServers.sync82 = {command, args}` | VS Code extension: `<VS Code user dir>/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`, where the user dir is `~/.config/Code/User` (Linux, honoring `$XDG_CONFIG_HOME`), `~/Library/Application Support/Code/User` (macOS) or `%APPDATA%\Code\User` (Windows); Cline CLI: `$CLINE_MCP_SETTINGS_PATH` when set to an absolute path, otherwise `~/.cline/data/settings/cline_mcp_settings.json`, or `$CLINE_DATA_DIR/settings/cline_mcp_settings.json` when that variable is set; uninstall also cleans the data-directory file when the override is set |

`%APPDATA%` falls back to `<home>\AppData\Roaming` when unset, and `$XDG_CONFIG_HOME` is only used when it is an absolute path.

`cline` writes only the settings file of the variant that is installed — both when both are.

The `{command, args}` entry is `{"command": "<absolute-path>", "args": []}`.

### Detection

A target is **detected** when its command is on `PATH` or one of its config directories exists (`DetectCmd`/`DetectDirs` in `targets.go`); a target unsupported on the current OS is never detected.

| Target | Detected when |
|---|---|
| `claude` | `claude` is on `PATH` |
| `claude-desktop` | Claude Desktop's config directory exists (`~/Library/Application Support/Claude`, `%APPDATA%\Claude` or `$XDG_CONFIG_HOME/Claude`) |
| `antigravity` | `agy` is on `PATH`, or `~/.gemini/config` or `~/.gemini/antigravity` exists (a bare `~/.gemini` is not enough) |
| `codex` | `codex` is on `PATH` |
| `opencode` | `opencode` is on `PATH` |
| `cursor` | `~/.cursor` exists |
| `zed` | Zed's config directory exists (the directory of its `settings.json` above) |
| `cline` | the extension's `saoudrizwan.claude-dev` directory, the CLI's `~/.cline` (or `$CLINE_DATA_DIR`), or the folder of `$CLINE_MCP_SETTINGS_PATH` exists |

---

## Install flow

1. With no target, `install` acts only on detected clients: it prints `Detected the following MCP clients:` with one `  - <target>` line each and asks `Install sync82 into all N detected client(s)? [y/N]` (any other answer than `y`/`yes` prints `Aborted.`). When nothing is detected, it prints `No supported MCP clients detected. Supported targets: <targets>` and exits with `0` without asking. With one target, it runs only that one, without asking. More than one target is a usage error (exit code `2`); an unknown target exits with `1`.
2. An unsupported OS prints `Skipped: <target> (<reason>).` — only `claude-desktop` has one: `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).`; an explicit target whose client isn't detected prints `Skipped: <target> not detected.` Neither is an error.
3. Otherwise it installs and prints `✓  <target> — configured.` for a new registration, or `✓  <target> — updated.` when sync82 was already registered — for `claude` and `codex`, when a previous registration was [removed](#removing-existing-cli-registrations) first; for file targets, when a config file already had a `sync82` entry. Warnings are printed after the status line.
4. With no target, it ends with `Done. N installed, N skipped, N failed.` and exits with `1` if any target failed.

Re-running it is safe: `claude` and `codex` remove every existing registration before `mcp add`, since their `mcp add` refuses to replace one; file targets overwrite the `sync82` entry.

### Removing existing CLI registrations

Both `install` and `uninstall` start a CLI target by removing sync82's existing registrations (`internal/installer/cliregistration.go`):

- **`codex`** runs `codex mcp get sync82` once and, when it exits with `0`, `codex mcp remove sync82`.
- **`claude`** loops `claude mcp get sync82`, which reports only the registration that takes precedence (local, then project, then user). It reads the scope from the output and runs `claude mcp remove --scope <scope> sync82` (`local`, `user`), repeating until `claude mcp get` fails.
  - A **project-scope** registration (`.mcp.json`) is never removed. It prints `Warning: sync82 is also registered in project scope (.mcp.json, shared with the project) for <current directory>; it was left unchanged and, inside that project, it takes precedence over any user-scope registration. To remove it, run from that directory: claude mcp remove --scope project sync82`, and the user scope hidden behind it is then removed directly with `claude mcp remove --scope user sync82` (a `No MCP server named` output means there was nothing to remove).
  - When the scope can't be read from the output, the target fails with ``could not determine the scope of the sync82 registration from `claude mcp get sync82`; remove it manually with `claude mcp remove --scope <scope> sync82` ``; when a scope is reported again after its removal, with ``sync82 is still registered in the <scope> scope after `claude mcp remove --scope <scope> sync82` ``. Both are printed on stderr prefixed with `[claude]`, and `mcp add` isn't run.

**JSON with comments.** A config file that only parses after removing comments and trailing commas (JSONC — common for Zed's `settings.json`) is never rewritten: the target fails, no backup is made, and the entry to add by hand is printed on stderr. A file that doesn't parse at all fails the same way.

---

## Uninstall flow

`sync82 uninstall [target] [--purge]` mirrors install: no target prints `Detected the following MCP clients:` and asks `Remove sync82 from all N detected client(s)? [y/N]` (declining ends the run, `--purge` included); one target runs without asking. When nothing is detected, it prints `No supported MCP clients detected. Supported targets: <targets>` and `--purge` still runs.

- **CLI targets** (`claude`, `codex`) — an explicit target whose command isn't on `PATH` prints `Skipped: <target> not detected.`; otherwise its registrations are [removed](#removing-existing-cli-registrations). It prints `✓  <target> — removed.` when something was removed, `⚠  <target> — nothing removed` when only a project-scope `claude` registration was found (followed by its warning), and `⚠  <target> — not configured, skipping` when nothing was registered.
- **File targets** delete the `sync82` key from every config file that has one — for `antigravity`, all three `mcp_config.json` files; for `opencode`, `opencode.json`, `opencode.jsonc` and `config.json`; for `cline`, both the extension and CLI settings files — keeping every other key. An explicitly named file target is cleaned even when its client is no longer detected. A JSONC file is left unchanged and the target fails with a message to remove the entry by hand.

Each target prints `✓  <target> — removed.`, `⚠  <target> — not configured, skipping`, `⚠  <target> — nothing removed`, a `Skipped: ...` line, or `✗  <target> — failed`; with no target, the run ends with `Done. N removed, N skipped, N failed.`

### `--purge`

After the targets, `--purge` (implemented in `internal/installer/purge.go`):

1. Lists the vaults configured away from the default — `SYNC82_DB_PATH`, `vaultPath` and `lastVaultPath` from `~/.sync82/config.json`, and the `path` of a `.sync82.json` found in the current directory or a parent — under `Note: these configured vaults are outside the purge and are left untouched:`. They are never deleted.
2. Lists the existing files among `~/.sync82/knowledge.db`, `knowledge.db-wal`, `knowledge.db-shm`, `config.json`, `config.lock` and every `config.json.corrupt-*` (or prints `Nothing to purge in <home>/.sync82.`).
3. Asks `Delete these files? [y/N]`. No prints `Purge cancelled.`; yes deletes them, removes `~/.sync82` if it is left empty, and prints `Purge complete.` — or `Purge incomplete.` plus one error per file that couldn't be removed.

**Exit codes:** `0` when nothing failed (including a declined prompt), `1` for an unknown target, a closed stdin at a prompt, a failed target, or an incomplete purge, `2` (usage error) for an unknown flag or more than one target.

---

## Clients not in this list

Any other MCP client can be wired up manually: its config just needs a server entry along these lines (adjust the surrounding JSON structure to match your client's format):

```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

No `npx`-style wrapper, no arguments — just the absolute path of the binary (find it with `which sync82`, or `where sync82` on Windows), so it works even when the client doesn't inherit your shell's `PATH`.

---

## Safety in tests

`RunInstall` and `RunUninstall` receive `homeDir` and `targets []installer.Target` as explicit parameters rather than reading `os.UserHomeDir()`/`installer.Targets` directly, and every target resolves its paths and presence through an `installer.Env` (OS, home directory, environment lookup, `PATH` lookup). This is deliberate: automated tests run on the same machine where `claude`/`codex`/`opencode` are genuinely on `PATH`, so if the commands used the real values directly, the test suite would run real `mcp add`/`mcp remove` commands and edit your actual client configs. With the injection, tests use a fake target list and home directory and never touch real configs. Only the production wiring in `cmd/sync82/main.go` passes the real values.

---

## See also

- [Installation Guide](../getting-started/installation.md) — the user-facing walkthrough
- [Uninstallation](../getting-started/uninstallation.md) — complete removal, step by step
- [CLI Reference — install](../reference/cli.md#install) and [uninstall](../reference/cli.md#uninstall) — exit codes and flags
- [Client guides](../guides/clients/claude-code.md) — per-client setup and troubleshooting

---

[🏠 Back to Index](../index.md)
