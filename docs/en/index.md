# Documentation — sync82 🧠

🌐 [Português](../pt-br/index.md) | **English** | 🏠 [Back to README](../../README.md)

---

**sync82** is an MCP server (Model Context Protocol) that gives AI coding agents persistent, per-project memory — decisions, progress, architecture, stack, and next steps — backed by an embedded SQLite vault. It's distributed as a single static binary.

Use this index to navigate through the entire documentation.

---

## 🚀 Getting Started

Set up your environment and get the server running in minutes.

- [Installation](./getting-started/installation.md) — Getting the binary (release or `go install`) and wiring it into your MCP client.
- [Quickstart](./getting-started/quickstart.md) — Connect your assistant and save your first session's memory.
- [Uninstallation](./getting-started/uninstallation.md) — Removing sync82 from every client, deleting its data, and removing the binary.

---

## 🧠 Concepts

Understand what MCP is, why this server exists and how it works internally.

- [What is MCP?](./concepts/what-is-mcp.md) — Introduction to the Model Context Protocol.
- [Why sync82?](./concepts/why-sync82.md) — The problem the server solves and when to use it.
- [How sync82 works](./concepts/how-sync82-works.md) — The flow between your AI agent, the tools, and the vault.
- [Architecture](./concepts/architecture.md) — Storage model, context resolution, and package layout.
- [Glossary](./concepts/glossary.md) — Terms and concepts used in this documentation.

---

## 📖 Guides

### MCP Clients

Configure the server in your preferred AI assistant.

- [Claude Code](./guides/clients/claude-code.md) — Anthropic's CLI, registered through `claude mcp add`.
- [Claude Desktop](./guides/clients/claude-desktop.md) — The `sync82.mcpb` extension, or `claude_desktop_config.json`.
- [Antigravity](./guides/clients/antigravity.md) — Google's IDE and CLI, sharing `~/.gemini/config/mcp_config.json`.
- [OpenAI Codex](./guides/clients/codex.md) — OpenAI's CLI, configured in `~/.codex/config.toml`.
- [OpenCode](./guides/clients/opencode.md) — Open-source TUI agent, global `opencode.json(c)`.
- [Cursor](./guides/clients/cursor.md) — AI editor, global `~/.cursor/mcp.json`.
- [Zed](./guides/clients/zed.md) — Editor with context servers in `settings.json`.
- [Cline](./guides/clients/cline.md) — VS Code extension and CLI, `cline_mcp_settings.json`.

### Workflows

- [Usage examples](./guides/workflows/examples.md) — Real end-to-end sessions, from project setup to handoff.

---

## 💬 Example Prompts

- [Example Prompts](./prompts.md) — Natural-language requests for every capability: setup, sessions, search, JSON output, validation, maintenance, and backups.

---

## 🛠️ Technical Reference

Consult the tools and commands available on the server.

- [Tools](./reference/tools.md) — All 19 MCP tools the AI can call, full parameter reference.
- [Resources](./reference/resources.md) — Project memory as read-only MCP resources (`sync82://projects/...`).
- [MCP Prompts](./reference/mcp-prompts.md) — The `start_session` and `end_session` prompts.
- [CLI](./reference/cli.md) — All `sync82` CLI subcommands: `install`, `uninstall`, `config`, `self-update`, `export`, `import`, `search`, `context`, `health`.
- [Configuration](./reference/configuration.md) — `SYNC82_DB_PATH`, `~/.sync82/config.json`, `.sync82.json`, precedence, transport, and the bundle's settings.

---

## 🔬 Internal Architecture

For those who want to understand or contribute to the server code.

- [Storage](./architecture/storage.md) — The SQLite schema, tables, and standard-vs-custom memory kinds.
- [Context Resolution](./architecture/context-resolution.md) — The 4-tier project/vault-path resolution model.
- [Installer](./architecture/installer.md) — How `sync82 install` and `sync82 uninstall` handle each of the 8 supported MCP clients.

---

## 🆘 Troubleshooting

- [Common Issues](./troubleshooting/common-issues.md) — Connection errors, PATH issues, and vault confusion.

---

> 💡 **Tip:** If you are using Claude Code or another MCP-aware assistant, try asking directly: _"What tools does sync82 provide?"_ — the AI will query the server in real time and respond with the updated list.
