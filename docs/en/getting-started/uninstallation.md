🌐 [Português](../../pt-br/getting-started/uninstallation.md) | **English** | 🏠 [Index](../index.md)

---

# Uninstallation

How to remove sync82 completely: its registration in every MCP client, its own files in `~/.sync82/`, the binary, and the files it leaves in your workspaces. Do the steps in this order — `sync82 uninstall` needs the binary.

---

## 1. Remove sync82 from your MCP clients

```bash
sync82 uninstall            # lists the detected clients, asks "Remove sync82 from all N detected client(s)? [y/N]", removes sync82 from each
sync82 uninstall cursor     # one target only, no confirmation prompt
```

Targets: `claude`, `claude-desktop`, `antigravity`, `codex`, `opencode`, `cursor`, `zed`, `cline` — the same set `sync82 install` writes. With no target, the clients detected on your machine are processed (see [Detection](../architecture/installer.md#detection)), plus any file target whose client was uninstalled but whose config file still holds a `sync82` entry; when there is none, it prints `No supported MCP clients detected. Supported targets: ...`. Name a file target explicitly to clean its config files even after its client was uninstalled. Each target prints one line:

| Line | Meaning |
|---|---|
| `✓  <target> — removed.` | The registration was removed. |
| `⚠  <target> — not configured, skipping` | No sync82 entry was found. |
| `⚠  <target> — nothing removed` | `claude` only had a project-scope registration, which is left unchanged (a `Warning: ...` line follows). |
| `Skipped: <target> not detected.` | An explicit `claude` or `codex` target whose command isn't on `PATH`. |
| `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).` | `claude-desktop` on any other OS. |
| `✗  <target> — failed` | The entry could not be removed; the reason is printed on stderr. |

With no target, the run ends with `Done. N removed, N skipped, N failed.` The command exits with `1` if any target failed (or on an unknown target, or when stdin is closed so the prompt gets no answer), `2` on an unknown flag, `0` otherwise — including when you answer no to the prompt. See [CLI Reference — uninstall](../reference/cli.md#uninstall).

What each target cleans:

| Target | How |
|---|---|
| `claude` | Loops `claude mcp get sync82` and runs `claude mcp remove --scope <scope> sync82` for each scope it reports (`local`, `user`); a project-scope registration is left unchanged with a warning, and the user scope is then removed directly — see [Installer](../architecture/installer.md#removing-existing-cli-registrations). |
| `codex` | Runs `codex mcp get sync82` and, when registered, `codex mcp remove sync82`. |
| `opencode` | sync82 deletes the `sync82` key from `opencode.json`, `opencode.jsonc` and the legacy `config.json` in `$XDG_CONFIG_HOME/opencode` (default `~/.config/opencode`). |
| `claude-desktop`, `antigravity`, `cursor`, `zed`, `cline` | sync82 deletes the `sync82` key from every config file listed in [Installer](../architecture/installer.md#targets), keeping every other key. |

### What `sync82 uninstall` leaves for you

- **Claude Code project scope** — a `sync82` entry in a project's `.mcp.json` is shared with the project and is never removed automatically. Inside each such project run:

  ```bash
  claude mcp remove --scope project sync82
  ```

- **Config files with comments or trailing commas** — a JSONC file (common for Zed's `settings.json` and OpenCode's `opencode.jsonc`) is left unchanged and the target fails with a message naming the file; delete the `sync82` entry from it by hand.
- **Claude Desktop extension** — if you installed the `sync82.mcpb` bundle, remove it from Claude Desktop's **Settings → Extensions**; `sync82 uninstall claude-desktop` only edits `claude_desktop_config.json`.
- **Project-level configs** — sync82 only writes global configs. An entry you added yourself to a project file (for example Cursor's `.cursor/mcp.json` or Antigravity's `.agents/mcp_config.json`) must be removed by hand.

Restart each client afterwards so it stops launching sync82.

---

## 2. Delete sync82's data (`--purge`)

> **This deletes your memory vault.** Export anything you want to keep first, e.g. `sync82 export --all ~/sync82-backup`.

```bash
sync82 uninstall --purge          # all detected targets, then the purge
sync82 uninstall codex --purge    # one target, then the purge
```

After the client step, `--purge` lists the files it will delete from `~/.sync82/` and asks a separate `Delete these files? [y/N]`:

- `knowledge.db`, `knowledge.db-wal`, `knowledge.db-shm` — the default vault
- `config.json`, `config.lock` — the global config
- `config.json.corrupt-*` — config files moved aside after a parse error

Only files that exist are listed; nothing else in the directory is touched, and `~/.sync82/` itself is removed only when it ends up empty. Close every MCP client that uses sync82 first. Answering no prints `Purge cancelled.`; a file that can't be deleted prints `Purge incomplete.` and the command exits with `1`.

Vaults configured elsewhere — `SYNC82_DB_PATH`, `vaultPath`/`lastVaultPath` in `config.json`, and the `path` of a `.sync82.json` in the current directory or a parent — are listed under `Note: these configured vaults are outside the purge and are left untouched:`. Delete them by hand if you want them gone, together with their `-wal` and `-shm` files.

---

## 3. Remove the binary

Delete the binary from wherever you installed it, plus the `<binary>.bak` that `sync82 self-update` keeps next to it:

```bash
# Linux / macOS, release binary
sudo rm -f /usr/local/bin/sync82 /usr/local/bin/sync82.bak

# go install
rm -f "$(go env GOPATH)/bin/sync82" "$(go env GOPATH)/bin/sync82.bak"
```

```powershell
# Windows, release binary
Remove-Item "$env:LOCALAPPDATA\sync82\sync82.exe", "$env:LOCALAPPDATA\sync82\sync82.exe.bak" -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\sync82"
```

On Windows, also remove `%LOCALAPPDATA%\sync82` from your `PATH` if you added it. A `.bak` file only exists after a `self-update`; a `sync82.exe.bak.old-*` file is one that was still running during an update — remove it too (`Remove-Item "$env:LOCALAPPDATA\sync82\sync82.exe.bak.old-*"`).

---

## 4. Remove workspace files

`init_project_memory` writes a `.sync82.json` at each workspace root it initializes. Find and delete them:

```bash
find ~ -name .sync82.json -type f 2>/dev/null
```

Remove any `sync82` instruction you added to a project's `CLAUDE.md` or similar agent instruction file, and any folder you created with `export_memory`/`sync82 export` that you no longer need.

---

## See also

- [Installation](./installation.md)
- [CLI Reference — uninstall](../reference/cli.md#uninstall)
- [Configuration Reference](../reference/configuration.md) — every file sync82 reads and writes
