🌐 [Português](../../pt-br/architecture/storage.md) | **English** | 🏠 [Index](../index.md)

---

# Storage

How the vault is actually stored on disk. Useful if you're debugging a behavior, contributing, or just curious.

---

## Storage model

Everything lives in a single embedded SQLite database file — one file is the whole vault. Default location `~/.sync82/knowledge.db`, overridable via the `SYNC82_DB_PATH` environment variable, `sync82 config set-vault`, a workspace's `.sync82.json`, or a tool call's `path` argument (in that ascending order of priority — see [Context Resolution](./context-resolution.md)). A new vault file, its `-wal`/`-shm` companions and a newly created vault directory are readable by the owning user only (`0600`/`0700`), as is `~/.sync82/config.json`.

Every connection sets `PRAGMA journal_mode=WAL` and `PRAGMA foreign_keys=ON`.

## Tables

```
projects           — the project/subproject tree
documents          — overwrite-style memory files (memory, architecture, stack, next_steps, custom)
entries            — append-only memory entries (progress, decisions, custom)
schema_migrations  — tracks which versioned schema migrations have run
```

**`projects`** — a project is a row with `parent_id NULL`; a subproject is a row whose `parent_id` points at its parent. Only one level of nesting is supported (a subproject can't itself have subprojects). Two partial unique indexes enforce that top-level project names are unique among themselves, and subproject names are unique within their parent — SQL's `NULL != NULL` means a single naive `UNIQUE(parent_id, name)` constraint wouldn't actually prevent two top-level projects sharing a name.

**`documents`** — one row per `(project_id, kind)` for the four overwrite-style kinds (`memory`, `architecture`, `stack`, `next_steps`) plus any custom kind written with `write_memory` in overwrite mode. A write replaces `content` and `updated_at` in place; no revision history is kept — an earlier version of the schema archived every previous version of a document, but nothing ever read it back, so it was removed.

**`entries`** — one row per dated (or undated) entry for the two append-only kinds (`progress`, `decisions`) plus any custom append kind. `entry_date` is `NULL` for undated content — those entries sort last and are never picked up by `archive_memory`. `position` gives stable ordering among entries sharing the same date.

**`schema_migrations`** — every schema version applied to this vault, each recorded once. Version 1 creates the tables above; version 2 lower-cases the project, subproject and kind names already stored (see [Names](#names) below). The migration framework lets a schema change ship safely to vaults that already exist, without re-running non-idempotent statements against them.

## Names

Project, subproject and kind (`filename`) names are case-insensitive: every tool trims and lower-cases them before use, so `Acme` and `acme` are the same project, `Memory` and `memory` the same file, and names are always shown lower-cased. Schema version 2 lower-cases the names in an existing vault on its first open; a name whose lower-case form already exists in the same scope (another top-level project, a sibling subproject, another kind of the same project) is left as is. Entries of an append-only kind are always moved to the lower-cased kind, so its dated log stays in one place.

## Standard vs. custom kinds

Every project gets six standard kinds on `init_project_memory`: `memory`, `architecture`, `stack`, `next_steps` (overwrite-style, stored in `documents`) and `decisions`, `progress` (append-only, stored in `entries`). Beyond those six, any tool that takes a `filename` argument accepts an arbitrary custom name — `write_memory`/`append_memory` decide whether a *new* custom kind is overwrite- or append-style based on which tool created it. `delete_memory` refuses to delete any of the six standard kinds; only custom ones can be removed.

## Concurrency

Every incoming MCP request runs in its own goroutine (the underlying SDK's transport model), so multiple tool calls can execute concurrently against the same vault. `WAL` mode plus `busy_timeout` on every connection is what makes that safe without any additional application-level locking — `store.Manager` caches one `*Store` per resolved vault path so concurrent calls against different vaults don't block each other either.

---

## See also

- [Context Resolution](./context-resolution.md) — how a tool call resolves to a project and a vault path
- [Installer](./installer.md) — how each MCP client gets pointed at a sync82 vault
- [Concepts — Architecture](../concepts/architecture.md) — the bigger picture

---

[🏠 Back to Index](../index.md)
