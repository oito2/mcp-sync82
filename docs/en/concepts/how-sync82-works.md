🌐 [Português](../../pt-br/concepts/how-sync82-works.md) | **English** | 🏠 [Index](../index.md)

---

# How sync82 works

`sync82` gives your AI assistant a persistent, structured memory for a project — without you ever touching a database directly.

---

## The pipeline

The flow has three stages: **Context resolution** figures out which project (and which vault) a tool call refers to, the **Store** reads/writes SQLite, and the **MCP Server** exposes it all to the AI client as 18 tools over stdio.

```
AI client calls a tool (e.g. update_project_memory)
         │
         ▼  Context resolution
    project/subproject/path resolved through 4 tiers:
    explicit arg > .sync82.json > global config > ask
         │
         ▼  Store
    reads/writes the resolved project's rows in SQLite
    (documents table for overwrite-style kinds,
     entries table for append-only kinds)
         │
         ▼  MCP Server
    returns a CallToolResult over stdio to the AI client
```

---

## Tools, by workflow stage

| Stage | Tools |
| --- | --- |
| **Project management** | `list_projects`, `create_project`, `delete_project`, `rename_project`, `get_vault_config` |
| **Reading & writing memory** | `list_files`, `read_memory`, `write_memory`, `append_memory`, `delete_memory`, `archive_memory`, `search_memory` |
| **Session workflow** | `load_project_context`, `check_project_health`, `init_project_memory`, `update_project_memory` |
| **Export & import** | `export_memory`, `import_memory` |

`update_project_memory` and `load_project_context` are the two tools most sessions actually touch — save at the end, load at the start. The rest exist for setup, search, and maintenance.

For full parameter tables, see the [Tools Reference](../reference/tools.md).

---

## Standard memory files

Every project gets six kinds on `init_project_memory`:

| Kind | Style | What goes in it |
| --- | --- | --- |
| `memory` | overwrite | Project name, description, high-level status |
| `architecture` | overwrite | Components and how they relate |
| `stack` | overwrite | Languages, frameworks, infrastructure |
| `decisions` | append, dated | One entry per decision, with the reasoning |
| `progress` | append, dated | One entry per session's worth of completed work |
| `next_steps` | overwrite | The current to-do list |

Beyond those six, any tool taking a `filename` argument accepts an arbitrary custom name — useful for project-specific files a generic template can't anticipate.

---

## Context resolution

Most tools take optional `project`/`subproject`/`workspace_root` arguments instead of requiring `project` on every call. sync82 resolves the actual project through four tiers — explicit argument, local `.sync82.json`, global "last used project," or asking the agent — see [Context Resolution](../architecture/context-resolution.md) for the full model, including the one deliberate exception (`search_memory` never silently falls back to "last used project").

---

## See also

- [Why sync82?](./why-sync82.md)
- [Architecture](./architecture.md)
- [Tools Reference](../reference/tools.md)
- [Context Resolution](../architecture/context-resolution.md)

---

[🏠 Back to Index](../index.md)
