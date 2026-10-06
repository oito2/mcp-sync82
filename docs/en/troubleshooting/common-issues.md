🇧🇷 [Português (BR)](../../pt-br/troubleshooting/common-issues.md) | **English** | 🏠 [Index](../index.md)

---

# Troubleshooting

Running into issues with `sync82`? This page covers the most common errors with direct solutions.

---

## 🔍 Initial Diagnosis

Before investigating any specific problem, run this step first — it resolves most cases:

**Check if the server is connected:**

In your AI client's chat, run its MCP status command (e.g. `/mcp` in Claude Code). If `sync82` appears connected with 18 tools, the server is working — the problem is in vault/project resolution, not the connection.

If it does not appear, the problem is in the server configuration — see [Connection and PATH Errors](#-connection-and-path-errors) below.

If it's connected but resolving the wrong project (or none), ask:

```
Show me the current vault configuration.
```

This calls `get_vault_config`, which reports the active vault path, global config, and (with `workspace_root`) the local `.sync82.json` — most configuration problems become obvious from this output. See [Context Resolution](../architecture/context-resolution.md) for how each field is derived.

---

## 🚫 Connection and PATH Errors

### The AI client cannot find `sync82`

**Symptom:** the AI client reports it cannot connect to the server, or the server appears as "Connecting..." and never completes.

**Cause:** sync82 is a single binary — the client just needs its absolute path when it's not on the `PATH` the client inherits (IDEs and some CLIs don't always inherit your interactive shell's `PATH`). `sync82 install` already registers the absolute path, but that path goes stale if the binary is moved afterwards.

**Solution:** find the absolute path and use it explicitly in the server configuration.

```bash
which sync82
# → /usr/local/bin/sync82
```

**Claude Code (`~/.claude.json`):**
```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82"
    }
  }
}
```

**Antigravity (`~/.gemini/config/mcp_config.json`) — and the same `mcpServers` shape for Claude Desktop, Cline and Cursor (Cursor adds `"type": "stdio"`):**
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

**OpenAI Codex (`~/.codex/config.toml`):**
```toml
[mcp_servers.sync82]
command = "/usr/local/bin/sync82"
```

**OpenCode (global `~/.config/opencode/opencode.jsonc` or `opencode.json`):**
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

**Zed (`settings.json`):** the same entry under `"context_servers"` instead of `"mcpServers"`.

The exact file for each client and OS is listed in [Installer — Targets](../architecture/installer.md#targets).

> Easiest fix of all: run `sync82 install [target]` again — it writes the absolute path itself.

---

### Server stuck at "Connecting..."

**Symptom:** The server appears but never leaves the "Connecting..." state. No tools are listed.

**Solution:** Restart the client session — MCP config changes require a fresh session in every client, not just a config write.

---

### `sync82 install` skips a client you have installed

**Symptom:** `sync82 install` prints `No supported MCP clients detected. Supported targets: ...`, leaves a client out of `Detected the following MCP clients:`, or `sync82 install <target>` prints `Skipped: <target> not detected.`

**Solution:** A client is detected only when its command is on `PATH` (`claude`, `codex`, `opencode`, `agy`) or its config directory exists — see [Detection](../architecture/installer.md#detection). Put the command on `PATH`, or configure the client by hand following its [client guide](../guides/clients/claude-code.md).

---

### `claude`: could not determine the scope of the sync82 registration

**Symptom:** `sync82 install claude` or `sync82 uninstall claude` fails with ``[claude] could not determine the scope of the sync82 registration from `claude mcp get sync82`; remove it manually with `claude mcp remove --scope <scope> sync82` `` or ``[claude] sync82 is still registered in the <scope> scope after `claude mcp remove --scope <scope> sync82` ``.

**Solution:** sync82 couldn't read the scope from `claude mcp get sync82`, or the removal didn't take effect. Run `claude mcp get sync82` to see where sync82 is registered, remove it with `claude mcp remove --scope <scope> sync82` (`local`, `user` or, from the project's directory, `project`), then run `sync82 install claude` again.

---

## 🛠️ Build and Runtime Errors

### `command not found: sync82` after building from source

**Symptom:** `go build -o sync82 ./cmd/sync82` succeeds, but running `sync82` fails.

**Cause:** the binary was written to the current directory, which usually isn't on `PATH`.

**Solution:**
```bash
sudo mv sync82 /usr/local/bin/
# or, if you used `go install`:
export PATH="$(go env GOPATH)/bin:$PATH"
```

---

### Permission denied opening the vault

**Symptom:** A tool call fails with a generic "internal error: could not open the vault database" — the actual filesystem path is deliberately never included in the message sent back to the agent (only logged server-side, to stderr).

**Cause:** The user running `sync82` doesn't have write permission on the vault's parent directory.

**Solution:**
```bash
mkdir -p ~/.sync82
chmod u+rwx ~/.sync82
```

Or point at a writable location instead:
```bash
sync82 config set-vault /a/writable/path/vault.db
```

---

## 🔄 Project and Vault Confusion

### The AI resolves the wrong project, or asks for one it should already know

**Symptom:** You expected `.sync82.json` auto-discovery to kick in, but the agent asks for `project` anyway.

**Cause:** either `init_project_memory` was never run with a `workspace_root` in this directory (so `.sync82.json` was never written), or you're in a different directory than the one it was written to — by default, resolution only checks `workspace_root` itself, never a parent or sibling directory. If your `.sync82.json` lives at a monorepo root above the current directory, pass `search_parent_dirs: true` to opt into looking there.

**Solution:**
```
Initialize memory for this project, using this directory as the workspace root.
```

Or check what's actually configured:
```
Show me the current vault configuration for this workspace.
```

---

### Two AI clients seem to have different memory for the same project

**Symptom:** Something saved in Claude Code doesn't show up when you ask Codex about it.

**Cause:** the two clients are pointed at different vaults — most likely one has a `SYNC82_DB_PATH`/`config set-vault` override the other doesn't.

**Solution:** ask each client to report `get_vault_config` and compare the `path` reported by both. Standardize on one (usually via `sync82 config set-vault`, since that's shared across every client on the machine, not per-client).

---

### `.sync82.json` or `~/.sync82/` appearing in `git status`

**Symptom:** `git status` shows a new, untracked `.sync82.json` file in your project.

**Solution:** Add it to `.gitignore` if the auto-discovery config is machine-specific (it usually is — it's meant to be local convenience, not something to share via git):

```gitignore
.sync82.json
```

`~/.sync82/` lives outside any project directory, so it never shows up in `git status` on its own.

---

### A `config.json.corrupt-*` file appeared in `~/.sync82/`

**Symptom:** `~/.sync82/` contains a file named `config.json.corrupt-<unix-timestamp>`, and the vault override (`sync82 config get-vault`) or the last-used project is gone.

**Cause:** `~/.sync82/config.json` was empty or not valid JSON (e.g. a hand edit with a typo, or a disk that filled up). On the next update — `sync82 config set-vault`/`unset-vault`, or a tool call that records the last-used project — sync82 moved the broken file aside under that name and started a fresh config.

**Solution:** open the `.corrupt-*` file, recover the values you need, and set them again (e.g. `sync82 config set-vault <path>`). Delete the `.corrupt-*` file once you no longer need it — sync82 never reads it.

---

## ❓ Common Questions

### The AI says it doesn't know sync82 or its tools

**Symptom:** The AI responds that it doesn't have access to the server or doesn't know what sync82 is.

**Solution:** Be more explicit in your instruction:

```
Use the load_project_context tool to load memory for this project.
```

```
Use the search_memory tool to find mentions of "authentication".
```

Naming the tool explicitly ensures the AI uses it instead of responding with generic knowledge.

---

## 📝 Reporting a New Bug

If your problem is not listed here:

1. Ask your assistant to run `get_vault_config` and copy the full output.
2. Note which AI client you are using and your OS/platform.
3. Open an **Issue** on GitHub: [github.com/oito2/mcp-sync82/issues](https://github.com/oito2/mcp-sync82/issues)
4. Describe the steps to reproduce the error.

---

> **Tip:** Restarting the IDE or the AI client process resolves most MCP server hang issues — especially after editing configuration files like `~/.claude.json`, `~/.gemini/config/mcp_config.json`, `~/.codex/config.toml`, or `opencode.json`.

---

[🏠 Back to Index](../index.md)
