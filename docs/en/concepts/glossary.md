🌐 [Português](../../pt-br/concepts/glossary.md) | **English** | 🏠 [Index](../index.md)

---

# Glossary

Technical terms used in the `sync82` documentation, organized alphabetically.

---

## `~/.sync82/config.json`

Global configuration file. Stores an optional custom vault path override (set via `sync82 config set-vault`) and the last project/subproject a tool call named that exists in its vault, together with that vault's path — tier 3 of [context resolution](../architecture/context-resolution.md).

→ See: [Context Resolution](../architecture/context-resolution.md)

---

## `.sync82.json`

Local, per-workspace configuration file. Written by `init_project_memory` when called with a `workspace_root`, so future tool calls made from that same directory auto-resolve to the right project without repeating `project`/`subproject` every time. By default only `workspace_root` itself is checked; pass `search_parent_dirs: true` to also discover one in a parent directory (e.g. a monorepo root).

→ See: [Context Resolution](../architecture/context-resolution.md)

---

## Append-only kind

A memory kind stored in the `entries` table, where each write adds a new dated row instead of replacing existing content. The two standard append-only kinds are `progress` and `decisions`; a custom kind can also be append-only depending on which tool (`append_memory` vs. `write_memory`) created it first.

→ See: [Storage](../architecture/storage.md) · [Tools Reference](../reference/tools.md)

---

## Context resolution

The 4-tier process sync82 uses to determine which project (and subproject) a tool call refers to when `project` isn't given directly: explicit argument → local `.sync82.json` → global "last used project" → ask the calling agent. `search_memory` is the one deliberate exception — it never falls back to the global tier on its own.

→ See: [Context Resolution](../architecture/context-resolution.md)

---

## Custom kind

Any memory file name beyond the six standard ones (`memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps`). Created the first time `write_memory` or `append_memory` is called with a new `filename` — which tool creates it determines whether it's overwrite- or append-style. Unlike the six standard kinds, custom kinds can be removed with `delete_memory`.

---

## Kind

sync82's term for a named memory file within a project — `memory`, `progress`, `architecture`, or any custom name. Every tool that reads or writes memory takes a `filename` argument identifying the kind.

---

## MCP (Model Context Protocol)

Open standard created by Anthropic that defines how AI assistants communicate with external tools and data sources. It allows a single server to be used by any compatible client — Claude Code, OpenAI Codex, OpenCode, Antigravity, and others.

`sync82` is an MCP server specialized in persistent project memory, written in Go and distributed as a single static binary.

→ See: [What is MCP?](./what-is-mcp.md)

---

## Overwrite kind

A memory kind stored in the `documents` table, where each write replaces the entire content in place — no revision history is kept. The four standard overwrite kinds are `memory`, `architecture`, `stack`, and `next_steps`.

→ See: [Storage](../architecture/storage.md)

---

## Project / Subproject

A **project** is a top-level entry in the vault. A **subproject** belongs to exactly one parent project — only one level of nesting is supported. This models a monorepo or plugin ecosystem: one parent project for shared context, one subproject per independently-tracked component.

→ See: [Storage](../architecture/storage.md)

---

## stdio

The only MCP transport mode sync82 supports. The server runs as a subprocess of the AI client, communicating via standard input and output (stdin/stdout). It does not open network ports — running `sync82` with no flags starts this mode.

→ See: [What is MCP?](./what-is-mcp.md#mcp-transports)

---

## Tool (MCP)

An executable function exposed by the MCP server that the AI assistant can explicitly call. sync82 exposes 18 tools, grouped by workflow stage:

| Group | Tools |
| --- | --- |
| Project management | `list_projects`, `create_project`, `delete_project`, `rename_project`, `get_vault_config` |
| Memory read/write | `list_files`, `read_memory`, `write_memory`, `append_memory`, `delete_memory`, `archive_memory`, `search_memory` |
| Session workflow | `load_project_context`, `check_project_health`, `init_project_memory`, `update_project_memory` |
| Export and import | `export_memory`, `import_memory` |

→ See: [Tools Reference](../reference/tools.md)

---

## Vault

The single SQLite database file that stores everything — every project, subproject, and memory file. Default location `~/.sync82/knowledge.db`. A vault can hold any number of unrelated projects; there's no requirement to use one vault per project.

---

[🏠 Back to Index](../index.md)
