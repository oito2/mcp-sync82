🌐 [Português](../../../pt-br/guides/clients/zed.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with Zed

**Zed** is a code editor with a built-in agent that supports MCP servers, which Zed calls *context servers*. sync82 configures it as a **file target**: `sync82 install` merges an entry into the `context_servers` object of Zed's user `settings.json`.

---

## 🛠️ Initial Setup

### 1. Add the server

```bash
sync82 install zed
```

It writes to Zed's user settings file — the target is detected by the presence of its directory:

| OS | File |
|---|---|
| Linux | `$XDG_CONFIG_HOME/zed/settings.json` (default `~/.config/zed/settings.json`) |
| macOS | `~/.config/zed/settings.json` |
| Windows | `%APPDATA%\Zed\settings.json` |

> **Comments in `settings.json`.** Zed's settings file often contains comments. sync82 never rewrites a file with comments or trailing commas: the target fails, the file is left exactly as it was, and the entry is printed for you to paste in by hand (step 2).

### 2. Or configure manually

Open the settings file (command palette: **zed: open settings file**, or edit the file above) and add, using the absolute path printed by `which sync82` (`where sync82` on Windows):

```json
{
  "context_servers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

If `context_servers` already exists, add only the `"sync82"` entry inside it. Zed can also add it from **Settings → AI → MCP Servers → Add Server → Add Local Server**.

### 3. Verify and initialize

Open **Settings → AI → MCP Servers** and check the indicator next to `sync82` shows it running — Zed picks up `settings.json` changes without a restart. Then ask the agent:

```
Initialize memory for this project. Analyze the codebase automatically.
```

---

## 🗑️ Removing

```bash
sync82 uninstall zed
```

Deletes the `sync82` key from `context_servers`. With comments in the file, the target fails and asks you to remove the entry by hand. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

- **`zed — failed` with "contains comments or trailing commas"** — expected for a commented settings file; add (or remove) the printed entry by hand.
- **`command` not found** — use an absolute path; run `sync82 install zed` again after moving the binary.
- **Wrong project resolved** — ask _"Show me the current vault configuration"_ (`get_vault_config`), and see [Context Resolution](../../architecture/context-resolution.md).

---

## ➡️ Next Steps

- [Usage Examples](../workflows/examples.md)
- [Example Prompts](../../prompts.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
