🌐 [Português](../../pt-br/reference/tools.md) | **English** | 🏠 [Index](../index.md)

---

# Tools Reference

sync82 exposes 18 tools over MCP. Every tool that operates on a specific project accepts the same context arguments — `project`, `subproject`, `workspace_root`, `search_parent_dirs` — resolved through the same [4-tier context resolution](../architecture/context-resolution.md) model, so they're documented once here instead of repeated in every table below.

**Common context arguments** (all optional, present on every project-scoped tool):

| Argument | Type | Description |
|---|---|---|
| `project` | string | Project name. If omitted, auto-discovered from `workspace_root` or the last used project. |
| `subproject` | string | Subproject name, for a component of an existing project. |
| `workspace_root` | string | Path to your project folder, used to auto-discover the project via `.sync82.json`. |
| `search_parent_dirs` | boolean | If true, also look for `.sync82.json` in parent directories above `workspace_root` (useful in monorepos, where the marker file lives at the repo root). Defaults to `false` — only `workspace_root` itself is checked, since a `.sync82.json` found in an ancestor directory you don't control could otherwise silently redirect where memory is stored. |
| `path` | string | Base path where the memory is stored. If left blank, uses the default vault path. To use the default user directory, start the path with `"HOME"` (e.g. `"HOME/custom-vault"`). |

If none of `project`, `subproject`, `workspace_root` resolve to a project and no last-used project is on record, the tool returns a message asking the calling agent for a project name or `workspace_root` instead of failing.

Project and subproject names — whether passed, read from `.sync82.json` or remembered — must start with a letter or digit and contain only letters, digits, hyphens and underscores; any other name is refused with an error result. Names are case-insensitive: project, subproject and kind (`filename`) names are trimmed and lower-cased everywhere, so `Acme` and `acme` are the same project and `Memory` and `memory` the same file, and they are always shown lower-cased (existing vaults are migrated — see [Storage — Names](../architecture/storage.md#names)). Only `create_project`, `init_project_memory` and `import_memory` (except on a dry run) create a vault file; every other tool reports a `path` (or `.sync82.json`/`set-vault` path) where no vault exists instead of creating an empty one there.

Every tool rejects arguments it doesn't define — e.g. `keepDays` instead of `keep_days` — with an error result, and its input schema sets `additionalProperties: false`.

The server identifies itself to clients as `sync82` with its version and a 64×64 PNG icon (embedded as a `data:` URI in `serverInfo`), so clients that show server icons can display it.

Whenever a project is auto-discovered rather than passed explicitly, the tool response's text includes a `[project: ..., from ..., vault: ...]` suffix reporting the resolved vault path — so a custom `path` picked up along the way (e.g. from `.sync82.json`) is never silently invisible.

---

## Project management

### `list_projects`

List every project and subproject currently in the vault.

| Argument | Type | Required | Description |
|---|---|---|---|
| `path` | string | no | Vault path override. |
| `format` | string (`text` \| `json`) | no | `text` (default) for readable text; `json` for a JSON document with the same information, returned as the text and as structured content — see [JSON output](#json-output). |

---

### `create_project`

Create a new project (or subproject of an existing project) in the vault. Safe to call again — an existing project is reported, not duplicated.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project` | string | ✅ | Project name (letters, digits, hyphens, underscores — must start with a letter or digit). |
| `subproject` | string | ❌ | Subproject name. When provided, creates it under `project` instead of at the vault root. Same format rule as `project`. |
| `path` | string | ❌ | Vault path override. |

---

### `delete_project`

Permanently delete a project or subproject from the vault. **Requires `confirm: true`** — the calling agent is instructed to ask the user before setting this.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project` | string | ✅ | Project name to delete. |
| `subproject` | string | ❌ | Subproject name. When provided, only that subproject is deleted. |
| `confirm` | boolean | ✅ | Must be `true` to confirm permanent deletion. |
| `subproject_action` | string | conditionally | One of `cancel`, `promote`, `delete_all`. Required only when deleting a top-level project that has subprojects. |
| `path` | string | ❌ | Vault path override. |

If a top-level project has subprojects and `subproject_action` isn't given, the tool doesn't delete anything — it returns the list of subprojects and asks which action to take: `cancel` (abort), `promote` (move each subproject to the vault root as its own project), or `delete_all` (delete everything).

---

### `rename_project`

Rename a project or subproject in place.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project` | string | ✅ | The project to rename. When `subproject` is also given, renames that subproject instead. |
| `subproject` | string | ❌ | The subproject to rename, if renaming a subproject rather than the top-level project. |
| `new_name` | string | ✅ | The new name (letters, digits, hyphens, underscores — must start with a letter or digit). |
| `path` | string | ❌ | Vault path override. |

Doesn't touch any `.sync82.json` elsewhere on disk that already points at the old name — those keep referring to it until re-initialized (`init_project_memory`) or edited by hand.

---

### `get_vault_config`

Report the current effective vault configuration: active vault path, global config, and (if `workspace_root` is given) the local `.sync82.json` config for that workspace.

| Argument | Type | Required | Description |
|---|---|---|---|
| `workspace_root` | string | ❌ | If provided, the report also includes the local `.sync82.json` for that workspace, if one exists. |
| `search_parent_dirs` | boolean | ❌ | See [Common context arguments](#tools-reference) above — affects whether an ancestor `.sync82.json` is included in the report. |
| `path` | string | ❌ | Vault path override. |

---

## Reading and writing memory

### `list_files`

List every memory file (document or entries kind) recorded for a project.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `metadata` | boolean | ❌ | When `true`, include size, estimated tokens, and last-modified date per file. |
| `path` | string | ❌ | Vault path override. |
| `format` | string (`text` \| `json`) | ❌ | `text` (default) for readable text; `json` for a JSON document with the same information, returned as the text and as structured content — see [JSON output](#json-output). |

---

### `read_memory`

Read a memory file's content. For an append-only kind (`progress`, `decisions`, or a custom append kind), returns every non-archived entry concatenated in date order.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `filename` | string | ✅ | The file/kind to read (e.g. `"memory"`, `"progress"`, or a custom name). |
| `path` | string | ❌ | Vault path override. |

---

### `write_memory`

Overwrite a memory file's entire content. **Destructive**: for append-only kinds (`progress`, `decisions`, or a custom kind created with `append_memory`), this replaces every non-archived entry, not just the latest one — use `append_memory` to add without losing prior entries. Archived entries are kept. A kind keeps the storage it already has: a custom kind created with `append_memory` stays a log of dated entries. For such a kind, the content is split into entries at its date headers (see [Date headers](#date-headers) below).

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `filename` | string | ✅ | The file/kind to overwrite. |
| `content` | string | ✅ | The new full content. |
| `path` | string | ❌ | Vault path override. |

---

### `append_memory`

Append a new dated entry to an append-only memory file (`progress`, `decisions`, or a custom kind). `progress` and `decisions` **require** a `## YYYY-MM-DD` date header somewhere in `content`. Overwrite-style files (`memory`, `architecture`, `stack`, `next_steps`, or a custom kind created with `write_memory`) are rejected with an error result — use `write_memory` for those.

<a id="date-headers"></a>**Date headers:** a date header is `##`, spaces or tabs, then the date on the **same line** (`## 2026-10-06`). Headers inside fenced code blocks (```` ``` ```` or `~~~`) are ignored. If the first header has an invalid date (e.g. `## 2026-13-01`), the first valid header after it is used.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `filename` | string | ✅ | The file/kind to append to. |
| `content` | string | ✅ | The content to append. For `progress`/`decisions`, must contain a `## YYYY-MM-DD` header. |
| `path` | string | ❌ | Vault path override. |

---

### `delete_memory`

Permanently delete a custom memory file. The six standard files (`memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps`) **cannot** be deleted this way — use `write_memory` to clear their content instead. The project must come from `project` or `workspace_root`: a project taken only from the last session is refused.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `filename` | string | ✅ | The custom file/kind to delete. |
| `confirm` | boolean | ✅ | Must be `true` to confirm permanent deletion. Ask the user before setting it. |
| `path` | string | ❌ | Vault path override. |

---

### `archive_memory`

Archive old dated entries from `progress` or `decisions`, keeping only the last N days active. Entries without a date header are **never** archived. The project must come from `project` or `workspace_root`: a project taken only from the last session is refused.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `filename` | string (`progress` \| `decisions`) | ✅ | Which append-only file to archive. |
| `keep_days` | integer | ❌ | Entries older than this many days are archived (default 90, from 1 to 36500). |
| `path` | string | ❌ | Vault path override. |

---

### `search_memory`

Case-insensitive substring search across memory files — accented and other Unicode letters included (`decisão` finds `DECISÃO`). No `project` given searches the whole vault; `project` only searches that project and all its subprojects; `project`+`subproject` searches just that subproject.

| Argument | Type | Required | Description |
|---|---|---|---|
| `query` | string | ✅ | Substring to search for (case-insensitive, literal — not a pattern). Must be a single line. |
| `project` | string | ❌ | Limit the search to this project (and its subprojects, unless `subproject` is also given). |
| `subproject` | string | ❌ | Limit the search to this specific subproject. Requires `project` or `workspace_root`. |
| `workspace_root` | string | ❌ | Used to auto-discover the project **only if** `project` isn't given directly — see the note below. |
| `search_parent_dirs` | boolean | ❌ | See [Common context arguments](#tools-reference) above. |
| `limit` | integer | ❌ | Maximum number of results to return (1–1000, default 100). |
| `offset` | integer | ❌ | Number of results to skip, for pagination (default 0). |
| `context_lines` | integer | ❌ | Number of surrounding lines to include per match (0–20, default 0). |
| `path` | string | ❌ | Vault path override. |
| `format` | string (`text` \| `json`) | ❌ | `text` (default) for readable text; `json` for a JSON document with the same information, returned as the text and as structured content — see [JSON output](#json-output). |

Each match is labeled `project/file:line`, or `project/file[YYYY-MM-DD]:line` for an entry of a dated log (the line is counted within that entry). Results come in a stable order, with a log's entries in reading order, so `offset` pages are consistent. The response is capped at about 1 MB; past that it ends with a note telling you to continue with `offset`. On a very large vault the scan stops after 5000 rows per table or 64 MB of content, and the result says so.

> `search_memory` deliberately never falls back to "the last used project" on its own the way other tools do — an unscoped search should search the whole vault, not silently guess a project. With `workspace_root` and no `project`, a workspace without `.sync82.json` also searches the whole vault, while a `.sync82.json` that can't be read or names an invalid project gives an error result instead of a search.

---

## Session workflow

### `load_project_context`

Load a project's entire memory (every non-blank file) concatenated into one context block, ready to paste into a new session.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `files` | array of strings | ❌ | Only load these specific files/kinds, instead of everything. |
| `since` | string (`YYYY-MM-DD`) | ❌ | Only include dated entries (`progress`, `decisions`, or a custom append kind) on or after this date. Undated entries are always included. Overwrite-style files are unaffected. |
| `max_entries` | integer | ❌ | Only include the most recent N dated entries per append-only kind. Undated entries (such as a title before the first dated entry) are always included and don't count toward N. Overwrite-style files are unaffected. |
| `max_bytes` | integer | ❌ | Maximum size of the response, in bytes (default 204800 = 200 KB, from 1024 to 52428800). |
| `path` | string | ❌ | Vault path override. |

For a long-running project, `progress`/`decisions` grow without bound — `since`/`max_entries` keep the loaded context to recent history instead of the entire session log every time. `memory`, `architecture`, `stack`, and `next_steps` always load in full: they represent current state, not history, so there's nothing dated to filter. They come first, followed by the other files in alphabetical order. A response larger than `max_bytes` is cut at a line break before that size and ends with `[context truncated at N of M bytes — narrow it with "since", "max_entries" or "files", or raise "max_bytes"]`.

---

### `check_project_health`

Report which of the six standard memory files exist for a project. Returns an error result (`isError: true`) when the project is unhealthy — a deliberate signal for the calling agent to act on, not a crash. This holds for `format: "json"` too.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `path` | string | ❌ | Vault path override. |
| `format` | string (`text` \| `json`) | ❌ | `text` (default) for readable text; `json` for a JSON document with the same information, returned as the text and as structured content — see [JSON output](#json-output). |

---

### `init_project_memory`

Guided initialization of a project's memory. This tool's description doubles as an agent playbook — the agent is expected to determine whether the target is a project or subproject (asking the user if unclear), then either auto-detect the project's data from the codebase or ask the user a fixed set of questions. Only files that are empty or still contain the blank template get written — re-running it on an already-initialized project doesn't clobber existing content. With `workspace_root`, it writes `.sync82.json` there — unless one already points at a different project, which is left unchanged and reported. A `path` given as `~/…`, `HOME/…` or an absolute path is recorded in `.sync82.json` as given. When `workspace_root` is given, the last-used project is never used as a fallback.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. `workspace_root` also enables `.sync82.json` auto-discovery for future sessions, and is required when `auto_detect` is true. |
| `auto_detect` | boolean | ❌ | When `true`, analyzes the files at `workspace_root` to infer description, languages, frameworks, and infrastructure automatically. |
| `description` | string | ❌ | What the project does. |
| `goal` | string | ❌ | The main goal or objective. |
| `phase` | string | ❌ | Current phase: `planning` / `mvp` / `active` / `maintenance`. |
| `architecture_overview` | string | ❌ | Brief architecture description. |
| `components` | string | ❌ | Main components, comma-separated. |
| `languages` | string | ❌ | Programming languages used. |
| `frameworks` | string | ❌ | Frameworks and libraries used. |
| `infrastructure` | string | ❌ | Infrastructure and hosting. |
| `next_steps` | string | ❌ | Immediate next tasks, comma or newline separated. |
| `path` | string | ❌ | Vault path override. |

The answer fields together may hold at most 10 MB.

`auto_detect` inspects `README.md`, `package.json`, `composer.json`, `Cargo.toml`, Python project files, `go.mod`, `pom.xml`/`build.gradle`/`build.gradle.kts` (Java/Kotlin), `Gemfile` (Ruby), `*.csproj`/`*.fsproj`/`*.vbproj` (.NET), and common infrastructure markers (Docker, CI configs, etc.) — including PHP/Composer projects (Moodle plugins and similar), detecting frameworks like Laravel, Symfony, and Slim from `require`/`require-dev`. Dependencies are matched by exact name from each manifest's dependency lists (Cargo dependency tables, `Gemfile` `gem` lines, `pom.xml`/Gradle coordinates, `requirements.txt`/`pyproject.toml`/`setup.py` requirement names, `go.mod` `require` lines, `.csproj` `PackageReference`/`FrameworkReference`), so comments and similarly named packages don't produce false detections, and results come in a stable order. It also recognizes `compose.yaml`/`compose.yml`, skips `vendor`, `target`, `venv`, `bin`, `obj` and `__pycache__` when listing components, and reads setext-style README titles while ignoring code fences. It only reads regular files of up to 5 MiB inside `workspace_root`: symlinks pointing outside it, FIFOs and devices are ignored, and detected descriptions are collapsed to a single line.

---

### `update_project_memory`

Save a session's work to the project vault in a single call — the tool most agents are expected to reach for at the end of a working session. It analyzes what changed and writes only the fields that actually changed.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `progress` | string | ❌ | Appended to `progress`. Must contain a `## YYYY-MM-DD` date header. |
| `decisions` | string | ❌ | Appended to `decisions`. Must contain a `## YYYY-MM-DD` date header. |
| `next_steps` | string | ❌ | Overwrites `next_steps` with the full updated content. |
| `memory` | string | ❌ | Overwrites `memory` with the full updated content. |
| `architecture` | string | ❌ | Overwrites `architecture` with the full updated content. |
| `stack` | string | ❌ | Overwrites `stack` with the full updated content. |
| `custom` | array of `{filename, content, mode?}` | ❌ | Custom files outside the six standard ones (naming a standard file is rejected — use its own field). `mode` is `"append"` (default) or `"write"`. |
| `path` | string | ❌ | Vault path override. |

`custom` accepts at most 50 items, and the total content of one call may be at most 10 MB.

Each field is an independent operation — if one fails (e.g. a malformed custom entry), the others still apply; the overall result is only flagged as an error if at least one operation failed.

---

## Export and import

### `export_memory`

Export a project's memory to plain Markdown files (one per kind, e.g. `memory.md`, `progress.md`) on the local filesystem, for browsing or git-diffing outside an MCP client.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `output_dir` | string | ✅ | Directory to write the exported `.md` files into. Must be absolute (or start with `~/` or `HOME/`); otherwise any directory the server process can write to — it is not confined to the vault or workspace. Created (private to the user) if it doesn't exist. |
| `overwrite` | boolean | ❌ | Replace `.md` files that already exist in `output_dir`. Without it, the export is refused — and nothing written — when any destination file exists. A destination that is a symlink or special file is always refused. |
| `path` | string | ❌ | Vault path override. |

A kind with archived entries also gets a `<kind>.archived.md` file holding them, so an export keeps the whole history. There's also a `sync82 export` CLI command that does the same thing without going through an MCP client, plus an `--all` mode that exports every project/subproject in the vault at once — see [CLI Reference — export](./cli.md#export).

---

### `import_memory`

Import a project's memory from plain Markdown files previously produced by `export_memory` — the inverse operation. Creates the project first if it doesn't exist yet.

| Argument | Type | Required | Description |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Standard context arguments. |
| `input_dir` | string | ✅ | Directory to read exported `.md` files from. Must be absolute (or start with `~/` or `HOME/`); otherwise any directory the server process can read — it is not confined to the vault or workspace. Every `"<kind>.md"` file present is imported, and a `"<kind>.archived.md"` file restores that kind's archived entries; files that aren't valid kind names, are empty, exceed 10 MB, or are symlinks or special files are skipped and reported back. At most 256 `.md` files are read per import. |
| `dry_run` | boolean | ❌ | When `true`, report what the import would create and overwrite without writing anything — not even a missing vault file is created. |
| `path` | string | ❌ | Vault path override. |

The import is all or nothing: every file is read and checked first, then everything is written in one transaction, so a failure leaves the vault unchanged. The result lists `New:` (files that create a kind), `Overwritten:` (files that replace an existing kind) and `Skipped:`. Two files that become the same kind once lower-cased (e.g. `Memory.md` and `memory.md`) make the import fail.

**Destructive per kind**: an existing kind's content is overwritten, not merged, the same as `write_memory` (archived entries already in the vault are kept unless the folder has a `<kind>.archived.md` for that kind). The project must come from `project` or `workspace_root`: a project taken only from the last session is refused, so the import never overwrites or recreates a remembered project. Use it to restore a backup, migrate a project between vaults, or seed a fresh vault from a folder of hand-written Markdown. There's also a `sync82 import` CLI command — see [CLI Reference — import](./cli.md#import).

---

## JSON output

`list_projects`, `list_files`, `check_project_health` and `search_memory` accept `format: "json"`. The result's text is then an indented JSON document, and the same object is returned as the MCP result's `structuredContent`. Any other `format` value is rejected with an error result. Context notes (`[project: ..., from ..., vault: ...]`) are not added in JSON mode — the `vault` field carries the resolved path instead. An empty result is an empty array (`[]`), never a "nothing found" sentence. Messages that aren't results — a request for `project`, a missing vault (except for `list_projects`), an invalid argument — stay plain text.

**`list_projects`**

| Field | Type | Description |
|---|---|---|
| `vault` | string | Vault path that was listed. A vault that doesn't exist yet gives an empty `projects` array. |
| `projects` | array | One object per top-level project. |
| `projects[].name` | string | Project name. |
| `projects[].subprojects` | array of strings | Subproject names (`[]` when none). |

**`list_files`**

| Field | Type | Description |
|---|---|---|
| `project` | string | `project` or `project/subproject`. |
| `vault` | string | Resolved vault path. |
| `files` | array | One object per file (kind). |
| `files[].name` | string | File (kind) name. |
| `files[].size_bytes` | integer | Size in bytes — only with `metadata: true`. |
| `files[].estimated_tokens` | integer | Estimated token count — only with `metadata: true`. |
| `files[].last_modified` | string | Last-modified time — only with `metadata: true`. |

**`check_project_health`**

| Field | Type | Description |
|---|---|---|
| `project` | string | `project` or `project/subproject`. |
| `vault` | string | Resolved vault path. |
| `healthy` | boolean | `true` when all six standard files exist (the result is flagged `isError` otherwise). |
| `files` | object | One boolean per standard file: `memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps`. |

**`search_memory`**

| Field | Type | Description |
|---|---|---|
| `query` | string | The query searched for. |
| `results` | array | Matches, in the same stable order as the text output. |
| `results[].project` | string | Project of the match. |
| `results[].subproject` | string | Subproject of the match — omitted for a top-level project. |
| `results[].file` | string | File (kind) of the match. |
| `results[].entry_date` | string | `YYYY-MM-DD` of the dated entry holding the match — omitted for undated content. |
| `results[].line` | integer | Line number (within the entry, for a dated entry). |
| `results[].text` | string | The matching line. |
| `results[].context_before`, `results[].context_after` | array of strings | Surrounding lines — only with `context_lines` > 0, omitted when empty. |
| `next_offset` | integer | `offset` to pass for the next page — omitted when there are no more results. |
| `scan_truncated` | boolean | `true` when the scan stopped early on a very large vault — omitted otherwise. |

---

## The six standard files

Every project gets these six kinds. `decisions` and `progress` are **append-only** (each write adds a new dated entry, never replacing history); the other four are **overwrite-style** (each write replaces the whole file).

| Kind | Style | What goes in it |
|---|---|---|
| `memory` | overwrite | Project name, description, high-level status |
| `architecture` | overwrite | Components and how they relate |
| `stack` | overwrite | Languages, frameworks, infrastructure |
| `decisions` | append, dated | One entry per decision, with the reasoning |
| `progress` | append, dated | One entry per session's worth of completed work |
| `next_steps` | overwrite | The current to-do list for the project |

`delete_memory` refuses to delete any of these six — only custom kinds can be deleted. `write_memory` will overwrite any of them, including the append-only ones (it re-parses the new content into dated sections), for full flexibility when you need it.

---

[← Back to Index](../index.md)
