🌐 [Português](../../../pt-br/guides/clients/claude-desktop.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with Claude Desktop

**Claude Desktop** is Anthropic's desktop app for macOS, Windows and Linux (on Linux, a beta for Debian-based distributions such as Ubuntu and Debian, on x86_64 and arm64). sync82 can be added to it in two ways: as a one-click **extension** (the `sync82.mcpb` bundle), or as an entry in `claude_desktop_config.json` pointing at a sync82 binary you installed yourself. Use one or the other, not both.

---

## 🛠️ Option A — Install the extension (`sync82.mcpb`)

Every release ships `sync82.mcpb`, an [MCP Bundle](https://github.com/modelcontextprotocol/mcpb) with the sync82 binaries for macOS (universal), Windows (amd64) and Linux (amd64/arm64) inside — nothing else to install.

1. Download it: <https://github.com/oito2/mcp-sync82/releases/latest/download/sync82.mcpb>
2. Optionally verify it — it's listed in the release's `checksums.txt` and covered by a build provenance attestation (see [Installation — Verifying a Downloaded Binary](../../getting-started/installation.md#-verifying-a-downloaded-binary)).
3. In Claude Desktop, open **Settings → Extensions → Advanced settings → Extension Developer**, click **Install Extension…**, select `sync82.mcpb`, and confirm the installation.

The extension has one optional setting, **Vault database path** (default `~/.sync82/knowledge.db`), passed to the server as `SYNC82_DB_PATH` — see [Configuration Reference](../../reference/configuration.md#claude-desktop-bundle-sync82mcpb).

The bundle's binary is managed by Claude Desktop: update it by installing a newer `sync82.mcpb`, not with `sync82 self-update`, and remove it from **Settings → Extensions**.

---

## 🛠️ Option B — Configure a binary you installed

With the `sync82` binary [installed](../../getting-started/installation.md):

```bash
sync82 install claude-desktop
```

It merges a `sync82` entry with the absolute path of the binary into:

| OS | File |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux (beta) | `$XDG_CONFIG_HOME/Claude/claude_desktop_config.json` (default `~/.config/Claude/claude_desktop_config.json`) — the location the Linux beta uses; Anthropic's documentation doesn't list a Linux path yet |

The client is detected when that `Claude` directory exists; otherwise the target prints `Skipped: claude-desktop not detected.` On any other OS it prints `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).`

Or edit the file by hand — **Settings → Developer → Edit Config** opens it — using the absolute path printed by `which sync82` (`where sync82` on Windows):

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

On Windows, escape the backslashes: `"command": "C:\\Users\\you\\AppData\\Local\\sync82\\sync82.exe"`.

---

## 🔄 Restart and verify

**Quit Claude Desktop completely and reopen it** — closing the window isn't enough; it reads its MCP config only at startup. Then ask:

```
List every project in my sync82 vault.
```

An empty list on a fresh vault means the connection works. Then initialize your first project:

```
Initialize memory for the project in /home/me/code/acme. Analyze the codebase automatically.
```

Claude Desktop has no notion of a "current directory", so give the agent the project's path (or its name) explicitly.

sync82 also exposes [resources](../../reference/resources.md) (each project's memory, readable without a tool call) and two [MCP prompts](../../reference/mcp-prompts.md) (`start_session`, `end_session`). Claude Desktop offers MCP resources and prompts from the menu of the message box; the exact labels change between versions, so look for the sync82 server there. The project list comes from your default vault; give the prompts a `project`, since Claude Desktop has no workspace to find it from.

---

## 🗑️ Removing

```bash
sync82 uninstall claude-desktop
```

This removes the `claude_desktop_config.json` entry (Option B). An extension installed from `sync82.mcpb` (Option A) is removed from **Settings → Extensions**. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

- **sync82 doesn't show up** — make sure you fully quit and reopened Claude Desktop, and that the file is valid JSON. If `sync82 install claude-desktop` reported a failure because the file has comments or trailing commas, add the printed entry by hand.
- **`command` not found** — the path in `command` must be absolute and point at an existing binary; run `sync82 install claude-desktop` again after moving it.
- **Wrong vault** — ask _"Show me the current vault configuration"_ (`get_vault_config`), and see [Context Resolution](../../architecture/context-resolution.md).

---

## ➡️ Next Steps

- [Usage Examples](../workflows/examples.md)
- [Example Prompts](../../prompts.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
