🌐 [Português](../../../pt-br/guides/clients/codex.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with OpenAI Codex

**OpenAI Codex** is OpenAI's CLI agent, configured via a TOML file.

---

## 🛠️ Initial Setup

### 1. Add the server

```bash
sync82 install codex
```

It registers the absolute path of the `sync82` binary you ran; re-running it (e.g. after moving the binary) replaces the entry: it runs `codex mcp get sync82` and, when sync82 is registered, `codex mcp remove sync82` (printing `updated.` instead of `configured.`), then `codex mcp add sync82 -- <absolute-path>`. The `codex` command must be on `PATH` (otherwise it prints `Skipped: codex not detected.`).

Or manually, using the absolute path printed by `which sync82`:

```bash
codex mcp add sync82 -- /usr/local/bin/sync82
```

Note the `--` — Codex requires it to separate its own flags from the command it should run. This writes to `~/.codex/config.toml`:

```toml
[mcp_servers.sync82]
command = "/usr/local/bin/sync82"
```

### 2. Verify the connection

```bash
codex mcp list
```

`sync82` should appear in the list. Start a session and confirm the tools are available.

### 3. Initialize your first project

```
Initialize memory for this project. Analyze the codebase automatically.
```

---

## 💡 Recommended Workflows

Same 19 tools, same prompts as every other client — see [Usage Examples](../workflows/examples.md) for full end-to-end scenarios.

### Resuming a session

Codex supports resuming its most recent conversation:

```bash
codex resume --last
```

Pair this with sync82's own memory: Codex's session resume gives you back the raw conversation, `load_project_context` gives the AI the structured, curated summary — ask for both when starting back up on a task.

Codex's MCP documentation covers tools only, not MCP resources or prompts, so sync82's [resources](../../reference/resources.md) and [MCP prompts](../../reference/mcp-prompts.md) aren't available there; ask for the same things in plain language ("load the project memory", "save this session") and Codex calls the tools.

---

## 🗑️ Removing

```bash
sync82 uninstall codex
```

Runs `codex mcp get sync82` and, when sync82 is registered, `codex mcp remove sync82`. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

### `command not found: sync82`

Codex may not inherit your shell's `PATH`, so a registration that uses the bare command `sync82` (or a path the binary was moved away from) may fail. Run `sync82 install codex` again, or find the absolute path and re-add with it:

```bash
which sync82
# → /usr/local/bin/sync82

codex mcp remove sync82
codex mcp add sync82 -- /usr/local/bin/sync82
```

Or edit `~/.codex/config.toml` directly:

```toml
[mcp_servers.sync82]
command = "/usr/local/bin/sync82"
```

### Server configured but no tools respond

Ask the agent to check the vault:

```
Show me the current vault configuration.
```

This calls `get_vault_config` — if the reported path isn't what you expect, see [Context Resolution](../../architecture/context-resolution.md).

---

## ➡️ Next Steps

- [Claude Code](./claude-code.md)
- [Antigravity](./antigravity.md)
- [OpenCode](./opencode.md)
- [Usage Examples](../workflows/examples.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
