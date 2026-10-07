🌐 [Português](../../../pt-br/guides/clients/opencode.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with OpenCode

**OpenCode** is an open-source AI coding agent with a TUI interface, configured via a global `~/.config/opencode/opencode.jsonc` (or `opencode.json`).

---

## 🛠️ Initial Setup

### 1. Add the server

```bash
sync82 install opencode
```

OpenCode is detected when the `opencode` command is on `PATH`; otherwise the target prints `Skipped: opencode not detected.` sync82 merges its entry, with the absolute path of the `sync82` binary you ran, into the global `$XDG_CONFIG_HOME/opencode/opencode.json` (default `~/.config/opencode/opencode.json`) — or into `opencode.jsonc` when that is the only one of the two that exists — keeping every other key. Re-running it (e.g. after moving the binary) replaces the entry:

```json
{
  "mcp": {
    "sync82": {
      "type": "local",
      "command": ["/usr/local/bin/sync82"],
      "enabled": true
    }
  }
}
```

A file with comments or trailing commas (common in `opencode.jsonc`) is never rewritten: the target fails and prints the entry to add by hand.

Alternatively, register it with OpenCode's own CLI, using the absolute path printed by `which sync82`:

```bash
opencode mcp add sync82 -- /usr/local/bin/sync82
```

Note the `--` — `opencode mcp add <name>` alone is interactive and rejects being called without a command. This also writes to the global `~/.config/opencode/opencode.jsonc` (or `opencode.json`), not to a file in your project (verified with OpenCode 1.17.13).

### 2. Verify and initialize

Start (or restart) `opencode`, check the server is connected, then ask:

```
Initialize memory for this project. Analyze the codebase automatically.
```

---

## 💡 Recommended Workflows

Same 19 tools, same prompts as every other client — see [Usage Examples](../workflows/examples.md) for full end-to-end scenarios.

Because the registration lives in OpenCode's global config, sync82 is available in every project; per-project scoping comes from `init_project_memory workspace_root=<project root>` writing `.sync82.json` at the project root, so every session in that directory auto-resolves without repeating `project`.

---

## 🗑️ Removing

```bash
sync82 uninstall opencode
```

sync82 deletes the `sync82` entry from `opencode.json`, `opencode.jsonc` and the legacy `config.json` in `~/.config/opencode` (or `$XDG_CONFIG_HOME/opencode`), even when the `opencode` command is no longer on `PATH`. A file with comments or trailing commas is left unchanged and the entry must be removed by hand. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

### `command not found`

If the registration uses the bare command `sync82` and it isn't on the `PATH` OpenCode inherits (or the binary was moved), run `sync82 install opencode` again, or edit `~/.config/opencode/opencode.jsonc` (or `opencode.json`) with an absolute path:

```json
{
  "mcp": {
    "sync82": {
      "type": "local",
      "command": ["/usr/local/bin/sync82"]
    }
  }
}
```

### Server doesn't appear after editing the config

Restart `opencode` — config changes require a fresh session.

### Wrong project resolved

```
Show me the current vault configuration.
```

Calls `get_vault_config` — check whether the project directory OpenCode is working in has the `.sync82.json` you expect it to pick up. See [Context Resolution](../../architecture/context-resolution.md).

---

## ➡️ Next Steps

- [Claude Code](./claude-code.md)
- [Antigravity](./antigravity.md)
- [OpenAI Codex](./codex.md)
- [Usage Examples](../workflows/examples.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
