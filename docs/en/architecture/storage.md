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
documents_fts      — full-text index over documents.content
entries_fts        — full-text index over entries.body
```

**`projects`** — a project is a row with `parent_id NULL`; a subproject is a row whose `parent_id` points at its parent. Only one level of nesting is supported (a subproject can't itself have subprojects). Two partial unique indexes enforce that top-level project names are unique among themselves, and subproject names are unique within their parent — SQL's `NULL != NULL` means a single naive `UNIQUE(parent_id, name)` constraint wouldn't actually prevent two top-level projects sharing a name.

**`documents`** — one row per `(project_id, kind)` for the four overwrite-style kinds (`memory`, `architecture`, `stack`, `next_steps`) plus any custom kind written with `write_memory` in overwrite mode. A write replaces `content` and `updated_at` in place; no revision history is kept — an earlier version of the schema archived every previous version of a document, but nothing ever read it back, so it was removed.

**`entries`** — one row per dated (or undated) entry for the two append-only kinds (`progress`, `decisions`) plus any custom append kind. `entry_date` is `NULL` for undated content — those entries sort last and are never picked up by `archive_memory`. `position` gives stable ordering among entries sharing the same date. The row's `id` is the entry id that `read_memory` (`with_ids: true`), `search_memory` (JSON `entry_id`) and `edit_entry` use: unique within the vault and unchanged while the row exists, but not stable across a rewrite of the whole kind (`write_memory`, `update_project_memory`) or an `import_memory`, which delete the rows and insert new ones. Ids are never reused: `entries` and `projects` have `AUTOINCREMENT` keys, so a deleted row's id is never given to a new one. `edit_entry` only changes a row whose `project_id` and `kind` match the call, so an id from another project or file is reported as not found.

**`schema_migrations`** — every schema version applied to this vault, each recorded once. Version 1 creates the tables above; version 2 lower-cases the project, subproject and kind names already stored (see [Names](#names) below); version 3 adds the full-text indexes and indexes the existing rows (see [Full-text search](#full-text-search) below); version 4 rebuilds `entries` and `projects` with `AUTOINCREMENT` keys, keeping every id, so ids are never reused. The migration framework lets a schema change ship safely to vaults that already exist, without re-running non-idempotent statements against them.

**`documents_fts`, `entries_fts`** — FTS5 full-text indexes with external content: they hold only the index, keyed by the `id` of the `documents`/`entries` row, and read the text from that row. Both use the `unicode61 remove_diacritics 2` tokenizer, so case and the accents of Latin letters are ignored. See [Full-text search](#full-text-search).

## Full-text search

`search_memory` in `words` and `phrase` mode queries `documents_fts` and `entries_fts` in one SQL statement, ordered by `bm25()` relevance and then by project, kind and reading order, so the order is the same on every call. Archived entries stay indexed and are filtered out by the query. `exact` mode matches rows with a case-insensitive `LIKE`, as before version 3, but first narrows them with the indexes when the query holds a word that must appear whole in any matching row: every word but the first (which the text may extend to the left, as `o_bar` in `foo_barbaz`), the last one as a prefix. Only words in ASCII or the Latin, Greek and Cyrillic blocks are used, where the index is known to split and fold text exactly as sync82 does. A query without such a word — `100%`, a single word, or text in another script — reads every row.

Triggers keep the indexes in sync with every change to their table: `AFTER INSERT`, `AFTER DELETE` (deletes cascaded from a deleted project included) and `AFTER UPDATE OF content`/`body`. Archiving an entry changes only `archived`, so it doesn't touch the index.

The query typed by the user is never passed to FTS5 as is. sync82 splits it into words the way the tokenizer splits text (letters, numbers and the 25 combining accents `unicode61` removes; everything else — other combining marks such as Devanagari vowel signs, Hebrew points or Arabic harakat included — separates words), quotes each word, and keeps only a trailing `*` as a prefix marker, so FTS5 operators can't be injected. Line hits are then computed in Go with a folding function (Unicode decomposition from `golang.org/x/text`, cached per character) that produces the same tokens as the tokenizer; a test compares the two, character by character, over the Latin, Greek and Cyrillic blocks. Elsewhere they can still differ for characters the SQLite build's Unicode tables are older than Go's for (recent letters and symbols such as `₽`): `words` and `phrase` follow the index, so such a word may need `match: "exact"`.

Migration 3 builds the indexes when an existing vault is first opened by a sync82 that has them. An older sync82 refuses to open a vault whose schema version is newer than it supports, rather than writing to it without keeping the indexes in sync — see [Troubleshooting](../troubleshooting/common-issues.md#vault-schema-version-is-newer-than-this-sync82-supports).

## Names

Project, subproject and kind (`filename`) names are case-insensitive: every tool trims and lower-cases them before use, so `Acme` and `acme` are the same project, `Memory` and `memory` the same file, and names are always shown lower-cased. Schema version 2 lower-cases the names in an existing vault on its first open; a name whose lower-case form already exists in the same scope (another top-level project, a sibling subproject, another kind of the same project) is left as is. Entries of an append-only kind are always moved to the lower-cased kind, so its dated log stays in one place.

## Standard vs. custom kinds

Every project gets six standard kinds on `init_project_memory`: `memory`, `architecture`, `stack`, `next_steps` (overwrite-style, stored in `documents`) and `decisions`, `progress` (append-only, stored in `entries`). Beyond those six, any tool that takes a `filename` argument accepts an arbitrary custom name — `write_memory`/`append_memory` decide whether a *new* custom kind is overwrite- or append-style based on which tool created it. `delete_memory` refuses to delete any of the six standard kinds; only custom ones can be removed.

## Concurrency

Every incoming MCP request runs in its own goroutine (the underlying SDK's transport model), so multiple tool calls can execute concurrently against the same vault. Each `*Store` keeps a single database connection, so its calls wait for that connection inside the process instead of contending for SQLite's write lock; `WAL` mode plus `busy_timeout` covers the remaining contention between processes (several clients sharing a vault). `store.Manager` caches one `*Store` per resolved vault path so concurrent calls against different vaults don't block each other either.

Every write looks its project up inside the same transaction that writes, so a project deleted or recreated by another call (or another process) between the lookup and the write can't receive a stale write: the write fails with the project's "not found" error instead of a database constraint error.

Reads don't use a transaction: a read looks the project up and then reads its rows in separate statements. A project deleted in between gives an empty or "not found" result, never another project's rows, since project and entry ids are never reused (`AUTOINCREMENT`, schema version 4).

---

## See also

- [Context Resolution](./context-resolution.md) — how a tool call resolves to a project and a vault path
- [Installer](./installer.md) — how each MCP client gets pointed at a sync82 vault
- [Concepts — Architecture](../concepts/architecture.md) — the bigger picture

---

[🏠 Back to Index](../index.md)
