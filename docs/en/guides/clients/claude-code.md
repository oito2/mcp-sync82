🌐 [Português](../../../pt-br/guides/clients/claude-code.md) | **English** | 🏠 [Index](../../index.md)

---

# Using with Claude Code

**Claude Code** is Anthropic's CLI for AI-assisted development, with native MCP protocol support via stdio.

---

## 🛠️ Initial Setup

### 1. Add the server

The fastest way — let sync82 configure it for you:

```bash
sync82 install claude
```

It registers sync82 in **user scope** (available in every project) with the absolute path of the binary — so re-running it is safe, and it's what to do after moving the binary. The `claude` command must be on `PATH` (otherwise it prints `Skipped: claude not detected.`).

Before `claude mcp add`, it removes every existing `sync82` registration: it runs `claude mcp get sync82` and `claude mcp remove --scope <scope> sync82` for the scope reported (`local`, `user`), repeating until none is left, and prints `updated.` instead of `configured.` when it removed one. A **project-scope** registration (`.mcp.json`, shared with the project) is never removed; it prints:

```text
  Warning: sync82 is also registered in project scope (.mcp.json, shared with the project) for <current directory>; it was left unchanged and, inside that project, it takes precedence over any user-scope registration. To remove it, run from that directory: claude mcp remove --scope project sync82
```

and then removes the user-scope registration directly. If the scope can't be read from `claude mcp get`, or a scope is still reported after its removal, the target fails — see [Troubleshooting](../../troubleshooting/common-issues.md#claude-could-not-determine-the-scope-of-the-sync82-registration).

Or add it manually, using the absolute path printed by `which sync82`:

```bash
claude mcp add --scope user sync82 -- /usr/local/bin/sync82
```

Claude will create or update `~/.claude.json`. From this point on, every time you start `claude`, it will automatically connect to the MCP server in the background.

### 2. Verify the connection

Inside a Claude Code session, run:

```
/mcp
```

You will see `sync82` listed as connected with all 18 tools available. If the server does not appear, see [Troubleshooting](#troubleshooting) below.

### 3. Initialize your first project

In the first session in a workspace, ask Claude:

```
Initialize memory for this project. Analyze the codebase automatically.
```

Claude will call `init_project_memory` with `workspace_root` set to your current directory — this also writes `.sync82.json`, so every future session in this workspace auto-resolves the right project without you naming it again.

---

## 📄 Enhancing with CLAUDE.md

Claude Code automatically reads the `CLAUDE.md` file at the project root when starting each session. Since `load_project_context` still needs to be called explicitly, a short pointer in `CLAUDE.md` saves you from typing it every time:

```markdown
# Project Context

At the start of a session, load memory from sync82 (`load_project_context`)
before doing anything else. At the end of a session, save what changed with
`update_project_memory`.
```

---

## 💡 Recommended Workflows

### Starting a session

```
Load the project context before we start.
```

Claude calls `load_project_context` and gains the project's full recorded history — overview, architecture, stack, decisions, progress, next steps — in one call.

### Recording a decision mid-session

```
We decided to use SQLite instead of a flat-file store because we
needed real transactions. Save that.
```

Claude calls `update_project_memory` with a `decisions` field — appended with today's date, existing decisions untouched.

### Ending a session

```
Save what we did today.
```

Claude summarizes the session and calls `update_project_memory`, usually with `progress` and, if anything changed, `next_steps`.

### Searching past work

```
Have we handled retries anywhere before? Search the whole vault,
not just this project.
```

Claude calls `search_memory` with no `project` — an unscoped search deliberately covers the whole vault.

### Multi-component projects

```
This is a plugin for the moodle project I'm already tracking. Set up
memory for it as a subproject called mod_quiz.
```

Claude calls `create_project` (or `init_project_memory`) with `project: "moodle"`, `subproject: "mod_quiz"`. See [Usage Examples](../workflows/examples.md) for the full pattern.

---

## 🗑️ Removing

```bash
sync82 uninstall claude
```

Removes the local-scope and user-scope registrations the same way `install` does, printing `✓  claude — removed.`, or `⚠  claude — nothing removed` followed by the warning above when only a project-scope registration was found. A project-scope entry (`.mcp.json`, shared with the project) is never removed automatically — run `claude mcp remove --scope project sync82` inside each project that has one. See [Uninstallation](../../getting-started/uninstallation.md).

---

<a id="troubleshooting"></a>

## ⚠️ Troubleshooting

### First step: verify the connection

Run `/mcp` inside the Claude session. If `sync82` does not appear as connected, the problem is in the server configuration, not your prompt.

### `sync82: command not found`

Claude Code does not inherit your shell's PATH in every environment, so a registration that uses the bare command `sync82` may fail, and so does one pointing at a path the binary was moved away from. Run `sync82 install claude` again, or replace the registration by hand with the absolute path:

```bash
which sync82
# → /usr/local/bin/sync82

claude mcp remove --scope user sync82
claude mcp add --scope user sync82 -- /usr/local/bin/sync82
```

### Permission errors

```bash
ls -l $(which sync82)
chmod +x $(which sync82)
```

### Wrong vault or empty project list

```
Show me the current vault configuration.
```

Claude calls `get_vault_config`, which reports the active vault path, global config, and (with `workspace_root`) the local `.sync82.json` — see [Context Resolution](../../architecture/context-resolution.md) if the resolved path is unexpected.

---

## ➡️ Next Steps

- [Antigravity](./antigravity.md) — equivalent guide for Google's IDE and CLI
- [OpenAI Codex](./codex.md) — OpenAI's CLI with TOML configuration
- [OpenCode](./opencode.md) — open-source agent
- [Usage Examples](../workflows/examples.md) — real-world workflows
- [Tools Reference](../../reference/tools.md) — complete parameters for all tools
- [Back to Index](../../index.md)
