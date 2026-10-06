🌐 [Português](../../../pt-br/guides/clients/antigravity.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with Antigravity

**Antigravity** is Google's agentic development platform, available as an IDE and as a CLI (`agy`). Both read the same MCP config file. sync82 configures it as a **file target** — `sync82 install` merges an entry directly into that JSON config, rather than shelling out to an `mcp add` subcommand.

---

## 🛠️ Initial Setup

### 1. Add the server

```bash
sync82 install antigravity
```

The target is detected by the `agy` command on `PATH` or the presence of `~/.gemini/config` or `~/.gemini/antigravity` — a bare `~/.gemini` is not enough; otherwise it prints `Skipped: antigravity not detected.` It writes only `~/.gemini/config/mcp_config.json`, the global config shared by the IDE and the CLI.

Or just run `sync82 install` (no target) to configure every detected client at once, Antigravity included.

### 2. Or configure manually

Edit `~/.gemini/config/mcp_config.json`, with the absolute path of the binary (`which sync82` / `where sync82` prints it):

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

You can also add servers through the IDE's MCP settings or the CLI's interactive `/mcp` manager. Antigravity reads a workspace-level `.agents/mcp_config.json` too, which `sync82 install` never writes; at the time of writing, the Antigravity CLI discovers workspace-level servers but doesn't start them, so CLI users should use the global file.

The `install` command writes the same entry as above, with the absolute path of the `sync82` binary you ran — re-run it after moving the binary. It reads the existing file (a missing or blank one counts as `{}`), merges in this entry under `mcpServers`, and writes it back atomically, keeping the file's permissions and its numbers exactly as written — your other configured servers are preserved. A file that isn't plain JSON (e.g. JSON with comments, or a broken file) is left unchanged: the target fails and prints the entry for you to add by hand.

### 3. Verify and initialize

Restart Antigravity (or start a new CLI session) and check the server in its MCP settings or with `/mcp`, then ask it:

```
Initialize memory for this project. Analyze the codebase automatically.
```

---

## 💡 Recommended Workflows

Same tool set, same prompts as every other client — see [Usage Examples](../workflows/examples.md) for full end-to-end scenarios. A few Antigravity-specific notes:

- **Session save/resume:** Antigravity CLI supports `/chat save <name>` and `/chat resume <name>` — pair that with `update_project_memory` at the end of a session for two layers of continuity (raw chat history plus structured memory).
- Restart the session after editing the config file — Antigravity doesn't hot-reload MCP config.

---

## 🗑️ Removing

```bash
sync82 uninstall antigravity
```

Deletes the `sync82` entry from `~/.gemini/config/mcp_config.json` and from the legacy `~/.gemini/antigravity/mcp_config.json` and `~/.gemini/antigravity-ide/mcp_config.json` (read by older builds), wherever it is present. Remove an entry you added to a workspace's `.agents/mcp_config.json` by hand. See [Uninstallation](../../getting-started/uninstallation.md).

---

## ⚠️ Troubleshooting

### Server doesn't appear after installing

Restart Antigravity — MCP config changes require a fresh session, not just a config write.

### `command not found`

If the entry uses the bare command `sync82` and it isn't on the `PATH` Antigravity inherits (or the binary was moved), run `sync82 install antigravity` again, or edit the config file directly and use an absolute path, as in the example above.

### Server configured in a workspace file doesn't start (CLI)

Move the entry to the global `~/.gemini/config/mcp_config.json` — see the note in step 2.

---

## ➡️ Next Steps

- [Claude Code](./claude-code.md)
- [OpenAI Codex](./codex.md)
- [OpenCode](./opencode.md)
- [Usage Examples](../workflows/examples.md)
- [Tools Reference](../../reference/tools.md)
- [Back to Index](../../index.md)
