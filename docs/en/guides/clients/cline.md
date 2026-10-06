🌐 [Português](../../../pt-br/guides/clients/cline.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with Cline

**Cline** is an AI coding agent available as a VS Code extension and as a CLI. Both keep their MCP servers in a `cline_mcp_settings.json` file. sync82 configures it as a **file target**: `sync82 install` merges an entry into the settings file of each Cline variant installed.

---

## 🛠️ Initial Setup

### 1. Add the server

```bash
sync82 install cline
```

It writes to every variant it finds:

| Variant | Detected by | File |
|---|---|---|
| VS Code extension | the extension's `saoudrizwan.claude-dev` storage directory | Linux: `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` (honors `$XDG_CONFIG_HOME`); macOS: `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`; Windows: `%APPDATA%\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json` |
| Cline CLI (and the shared Cline data directory) | `~/.cline` (or `$CLINE_DATA_DIR`, or the folder of `$CLINE_MCP_SETTINGS_PATH`) | `$CLINE_MCP_SETTINGS_PATH` when it is set to an absolute path; otherwise `~/.cline/data/settings/cline_mcp_settings.json`, or `$CLINE_DATA_DIR/settings/cline_mcp_settings.json` when `CLINE_DATA_DIR` is set |

Cline documents `~/.cline` as shared by its IDE extensions, CLI and SDK, and `~/.cline/data/settings/cline_mcp_settings.json` as the CLI's MCP settings file; the VS Code `globalStorage` file is where the extension has kept its servers. sync82 writes every file whose directory exists. Only the stable VS Code's storage directory is checked.

### 2. Or configure manually

In VS Code, click the **MCP Servers** icon in the Cline panel's toolbar, open the **Configure** tab and click **Configure MCP Servers**, which opens the settings file the extension uses; for the CLI, edit the file above. Add, using the absolute path printed by `which sync82` (`where sync82` on Windows):

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

### 3. Verify and initialize

Check that `sync82` appears in Cline's **MCP Servers** view. Then ask:

```
Initialize memory for this project. Analyze the codebase automatically.
```

---

## 🗑️ Removing

```bash
sync82 uninstall cline
```

Deletes the `sync82` entry from both settings files, wherever it is present. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

- **`Skipped: cline not detected.`** — neither the extension's storage directory, nor `~/.cline` (or `$CLINE_DATA_DIR`), nor the folder of `$CLINE_MCP_SETTINGS_PATH` exists; open Cline once so it creates its storage, or configure it manually.
- **`command` not found** — use an absolute path; run `sync82 install cline` again after moving the binary.
- **Wrong project resolved** — ask _"Show me the current vault configuration"_ (`get_vault_config`), and see [Context Resolution](../../architecture/context-resolution.md).

---

## ➡️ Next Steps

- [Usage Examples](../workflows/examples.md)
- [Example Prompts](../../prompts.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
