🌐 [Português](../../pt-br/getting-started/quickstart.md) | **English** | 🏠 [Index](../index.md)

---

# Quickstart

This guide will help you connect `sync82` to your AI assistant and save your first session's memory in under 5 minutes.

---

## 1. Connect Your Assistant

The fastest way: let sync82 configure the client for you.

```bash
sync82 install
```

It auto-detects every supported client installed on your machine (Claude Code, Claude Desktop, Antigravity, OpenAI Codex, OpenCode, Cursor, Zed, Cline) and writes the config itself. Pass a target explicitly (e.g. `sync82 install claude`) to configure just one.

### Manual configuration

If you'd rather configure a client by hand — or need to see exactly what `install` writes — here's the equivalent for Claude Code:

```bash
claude mcp add --scope user sync82 -- /usr/local/bin/sync82   # use the path printed by `which sync82`
```

Verify the server was registered:

```bash
claude mcp list
# → sync82: /usr/local/bin/sync82  - ✔ Connected
```

> For the other clients — including exact config file paths and JSON snippets — see the full guides:
> [Claude Code](../guides/clients/claude-code.md) · [Claude Desktop](../guides/clients/claude-desktop.md) · [Antigravity](../guides/clients/antigravity.md) · [OpenAI Codex](../guides/clients/codex.md) · [OpenCode](../guides/clients/opencode.md) · [Cursor](../guides/clients/cursor.md) · [Zed](../guides/clients/zed.md) · [Cline](../guides/clients/cline.md)

---

## 2. Initialize a Project

With the server connected, open a chat with the AI and ask it to set up memory for the project you're in.

**Type in the chat:**

```
Initialize memory for this project. Analyze the codebase automatically.
```

The AI calls `init_project_memory` with `auto_detect: true` — it reads your `README.md`, `go.mod`/`package.json`/`Cargo.toml`/`composer.json`/etc., infers a description, languages, frameworks, and infrastructure, and writes the six standard memory files. It also asks before overwriting anything that already has content.

> Prefer to answer the questions yourself instead of auto-detection? Just say so: _"Set up sync82 memory for this project — I'll answer the questions myself."_

---

## 3. Save Your First Session

At the end of a working session, ask the AI to record what happened.

**Try this prompt:**

```
Save what we did today: implemented the login flow and fixed the
session-timeout bug. We decided to use JWT instead of server-side
sessions because we need stateless auth for the mobile client.
```

The AI calls `update_project_memory` — it appends a dated `progress` entry and a dated `decisions` entry in one call, without touching anything else. This is the tool you'll reach for at the end of almost every session.

---

## 4. Resume in a New Session

In your next session — a fresh chat, no memory of the last one — ask the AI to load what it already knows.

**Try this prompt:**

```
Load the project context before we start.
```

The AI calls `load_project_context`, which concatenates every non-blank memory file (overview, architecture, stack, decisions, progress, next steps) into one block, pasted straight into the conversation. Logs bring their 10 most recent entries by default; ask for the full history when you need it.

---

## 🎯 Next Steps

Now that you are connected, explore the full potential of the server:

- **Multi-component projects:** See [Usage Examples](../guides/workflows/examples.md) for the `project`/`subproject` pattern (e.g. a monorepo or a plugin ecosystem).
- **Check memory health:** Ask the AI to _"Check if this project's memory is healthy"_ — triggers `check_project_health`.
- **Browse the example prompts:** [Example Prompts](../prompts.md) — one ready-to-type request for every capability
- [Back to Index](../index.md)

---

> 💡 **Pro tip:** If the AI says it doesn't know the tool, be explicit: _"Use the MCP tool `search_memory` to find..."_.
