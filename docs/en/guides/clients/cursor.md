🌐 [Português](../../../pt-br/guides/clients/cursor.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with Cursor

**Cursor** is an AI code editor with MCP support. sync82 configures it as a **file target**: `sync82 install` merges an entry into Cursor's global `~/.cursor/mcp.json`.

---

## 🛠️ Initial Setup

### 1. Add the server

```bash
sync82 install cursor
```

The target is detected by the presence of the `~/.cursor` directory. It writes the absolute path of the `sync82` binary you ran — re-run it after moving the binary.

### 2. Or configure manually

Edit `~/.cursor/mcp.json` (global, every project), using the absolute path printed by `which sync82` (`where sync82` on Windows):

```json
{
  "mcpServers": {
    "sync82": {
      "type": "stdio",
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

Cursor also reads a project-level `.cursor/mcp.json` with the same shape; `sync82 install` never writes it.

### 3. Verify and initialize

Open **Customize** from Cursor's sidebar and check that `sync82` is listed among the MCP servers and enabled (restart Cursor if it was already running when the file changed). Then ask the agent:

```
Initialize memory for this project. Analyze the codebase automatically.
```

---

## 🗑️ Removing

```bash
sync82 uninstall cursor
```

Removes the `sync82` entry from `~/.cursor/mcp.json`, keeping every other server. Remove an entry you added to a project's `.cursor/mcp.json` by hand. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

- **Server not listed** — restart Cursor, and check that `~/.cursor/mcp.json` is valid JSON. A file with comments or trailing commas is left unchanged by `sync82 install`, which prints the entry to add by hand.
- **`command` not found** — use an absolute path; run `sync82 install cursor` again after moving the binary.
- **Wrong project resolved** — ask _"Show me the current vault configuration"_ (`get_vault_config`), and see [Context Resolution](../../architecture/context-resolution.md).

---

## ➡️ Next Steps

- [Usage Examples](../workflows/examples.md)
- [Example Prompts](../../prompts.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
