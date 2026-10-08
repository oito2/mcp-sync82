🌐 [Português](../pt-br/prompts.md) | **English** | 🏠 [Index](./index.md)

---

# Example Prompts

Requests you can type to your AI agent, in plain language, to make it use sync82 — once sync82 is [installed](./getting-started/installation.md) and registered in your client. You never need to name a tool or write JSON; the agent picks the tool and its arguments. Every tool and argument mentioned below is detailed in the [Tools Reference](./reference/tools.md).

**About "this project":** inside a workspace, the agent usually passes the folder it's working in as `workspace_root`, so sync82 finds the project through that folder's `.sync82.json` (written the first time you initialize memory there). In a client without a working folder (e.g. Claude Desktop), name the project or give its path.

---

## 🚀 Setup Prompts

### Initialize memory automatically

- **Expected parameters:** an open workspace (the project folder).
- **Example:**
  > "Initialize memory for this project. Analyze the codebase automatically."
- **Expected output:** the agent calls `init_project_memory` with `workspace_root` and `auto_detect: true`. sync82 infers a description, languages, frameworks and infrastructure from files like `README.md`, `go.mod` or `package.json`, writes the six standard files (only those still empty or blank), and creates `.sync82.json` so later sessions find the project on their own.

### Initialize memory by answering questions

- **Expected parameters:** an open workspace; your answers (description, goal, phase, architecture, components, languages, frameworks, infrastructure, next steps).
- **Example:**
  > "Set up sync82 memory for this project. I'll answer the questions myself."
- **Expected output:** the agent asks the questions one at a time, then calls `init_project_memory` with your answers and `workspace_root`.

### Track a component as a subproject

- **Expected parameters:** the parent project's name and the subproject's name.
- **Example:**
  > "This is a plugin for the Moodle project I'm already tracking. Set up memory for it as a subproject called mod_quiz under moodle."
- **Expected output:** `init_project_memory` (or `create_project`) with `project: "moodle"`, `subproject: "mod_quiz"`; the parent is created first if it doesn't exist.

### Create an empty project

- **Expected parameters:** the project's name.
- **Example:**
  > "Create a sync82 project called billing-api, I'll fill it in later."
- **Expected output:** `create_project` with `project: "billing-api"`. Asking again for an existing project reports it instead of duplicating it.

### Keep a project in a separate vault

- **Expected parameters:** the vault file's path.
- **Example:**
  > "Initialize memory for this project, but store it in ~/vaults/client-x.db instead of the default vault."
- **Expected output:** `init_project_memory` with `path: "~/vaults/client-x.db"`; the path is recorded in `.sync82.json`, so later calls in this workspace use that vault too.

### Use a marker file from the repository root (monorepo)

- **Expected parameters:** a subfolder of a repository whose root has a `.sync82.json`.
- **Example:**
  > "I'm in packages/web of our monorepo. Load the project memory — the sync82 marker is at the repo root, look in parent folders."
- **Expected output:** the agent passes `search_parent_dirs: true`, so sync82 looks for `.sync82.json` in up to 64 parent directories and loads that project's memory.

---

## 🔁 Session Workflow Prompts

### Load the project at the start of a session

- **Expected parameters:** none (the current workspace), or the project's name.
- **Example:**
  > "Load the project context before we start."
- **Expected output:** `load_project_context` returns every non-blank memory file in one block: `memory`, `architecture`, `stack` and `next_steps` first and in full, then the other files in alphabetical order. Logs such as `progress` and `decisions` bring only their 10 most recent dated entries; a footer says how many were left out.

### Start or end a session with a shortcut

- **Expected parameters:** optionally the project (and subproject) name.
- **Example:** in Claude Code, type `/mcp__sync82__start_session acme` at the start and `/mcp__sync82__end_session acme` at the end; or attach `@sync82:sync82://projects/acme/context` to any message.
- **Expected output:** the [MCP prompts](./reference/mcp-prompts.md) send the agent the same request as "load the project context" / "save this session", so it calls `load_project_context` or `update_project_memory`. The [resource](./reference/resources.md) attaches the project memory to the message without any tool call.

### Load the whole history

- **Expected parameters:** none (the current workspace), or the project's name.
- **Example:**
  > "Load this project's full memory, with the entire progress and decisions history."
- **Expected output:** `load_project_context` with `mode: "full"`: every entry of every log, cut at 200 KB with a note if it's larger.

### Load only recent history

- **Expected parameters:** a date, or a number of entries.
- **Example:**
  > "Load this project's memory, but only progress and decisions from the last two weeks."
- **Expected output:** `load_project_context` with `since` set to the date two weeks ago (or `max_entries` for "the last 5 entries"). Overview, architecture, stack and next steps still load in full; undated entries are always included.

### Load only some files

- **Expected parameters:** the files you want.
- **Example:**
  > "Just load the architecture and the next steps for this project."
- **Expected output:** `load_project_context` with `files: ["architecture", "next_steps"]`.

### Save the session's work

- **Expected parameters:** what you did (the agent can summarize the conversation).
- **Example:**
  > "Save what we did today: implemented the archive_memory tool and wrote its tests."
- **Expected output:** `update_project_memory` with a `progress` entry under today's `## YYYY-MM-DD` header; earlier entries stay untouched.

### Record a decision

- **Expected parameters:** the decision and its reason.
- **Example:**
  > "We decided to use SQLite instead of a flat-file store because we needed real transactions. Save that."
- **Expected output:** `update_project_memory` with a dated `decisions` entry.

### Update several things at once

- **Expected parameters:** what changed.
- **Example:**
  > "Update the memory — we finished the migration and now the next steps are writing docs and testing on staging."
- **Expected output:** one `update_project_memory` call with `progress` and `next_steps` (and any other field that changed). Each field is applied independently; the result says which ones failed, if any.

### Keep a custom log or note

- **Expected parameters:** the custom file's name and its content.
- **Example:**
  > "Add today's deploy to a 'deploys' log in the project memory: v1.4.0 to production, no incidents."
- **Expected output:** `append_memory` (or `update_project_memory` with a `custom` item in `append` mode) adds a new entry (agents usually give it a `## YYYY-MM-DD` header) to the custom `deploys` file, creating it as a log on first use.

### Read one file

- **Expected parameters:** which file.
- **Example:**
  > "What's the current architecture of this project?"
- **Expected output:** `read_memory` with `filename: "architecture"`.

### Rewrite a file completely

- **Expected parameters:** the new content (or what to change).
- **Example:**
  > "The stack file is outdated — rewrite it: Go 1.26, SQLite via modernc.org/sqlite, GitHub Actions for CI."
- **Expected output:** `write_memory` with `filename: "stack"` and the full new content, replacing the old one.

### Fix a wrong entry

- **Expected parameters:** which entry (its date or what it says) and the correction.
- **Example:**
  > "Yesterday's progress entry says we moved to Postgres, but we stayed on SQLite. Fix that entry."
- **Expected output:** `read_memory` with `filename: "progress"` and `with_ids: true` to find the entry's id, then `edit_entry` with `action: "replace"` and the corrected text. Only that entry changes; the rest of the log stays as it was.

### Record that a decision changed

- **Expected parameters:** the old decision and the new one.
- **Example:**
  > "We decided to drop the REST API in favor of gRPC. Mark the old REST decision as superseded."
- **Expected output:** `edit_entry` with `action: "supersede"` on the old decision's id and the new decision as `content`. The new decision is added as an entry, and the old one gets a `> Superseded by entry N on YYYY-MM-DD.` line, so both stay in the history.

### Remove a duplicated entry

- **Expected parameters:** which entry is the duplicate.
- **Example:**
  > "The last two progress entries are the same — delete the duplicate."
- **Expected output:** after you confirm, `edit_entry` with `action: "delete"` and `confirm: true` on one of the two ids.

### Fix or remove an archived entry

- **Expected parameters:** the log (`progress`, `decisions` or a custom log) and what to change in an entry that was archived.
- **Example:**
  > "One of the progress entries we archived last year says we shipped to production in March, but it was April. Fix it in the archive."
- **Expected output:** `read_memory` with `filename: "progress"`, `archived: true` and `with_ids: true` lists the archived entries with their ids, then `edit_entry` with `action: "replace"` (or `"delete"` with `confirm: true`, after you confirm) on that id. The entry stays archived; `supersede` is refused on archived entries.

---

## 🔍 Analysis and Search Prompts

### Search the current project

- **Expected parameters:** what to look for.
- **Example:**
  > "Show me every decision we've made about authentication."
- **Expected output:** `search_memory` with `kinds: ["decisions"]` and a query such as `"auth*"` (a prefix, so it also finds "authentication" and "authorization"), scoped to the current project and its subprojects. The best matches come first; each hit is labeled `project/file:line` (`project/file[YYYY-MM-DD]:line` for a dated entry).

### Search the whole vault

- **Expected parameters:** what to look for.
- **Example:**
  > "Search everywhere in the vault for how we've handled database migrations before — I don't remember which project it was in."
- **Expected output:** `search_memory` without `project`, which deliberately covers every project in the vault.

### Search without remembering the exact words

- **Expected parameters:** a few words of what you remember, in any order, with or without accents.
- **Example:**
  > "Find where we wrote about the decisao on the config path for the installer."
- **Expected output:** `search_memory` with words such as `"decisao config installer"` (the default `words` mode): every document or entry holding all of them, in any order, with `decisão`/`Decisão` matched by `decisao`, best matches first.

### Search for an exact phrase or a literal string

- **Expected parameters:** the phrase, or the literal text (a path, an identifier, a version).
- **Example:**
  > "Find the exact phrase 'one connection per vault'." / "Search for the literal string `SetMaxOpenConns(1)`."
- **Expected output:** `search_memory` with `match: "phrase"` for the phrase (the words in that order), or `match: "exact"` for a literal string with punctuation.

### Search one period of history

- **Expected parameters:** what to look for; the period.
- **Example:**
  > "What did we decide about the release workflow in September 2026?"
- **Expected output:** `search_memory` with `kinds: ["decisions"]`, `since: "2026-09-01"` and `until: "2026-09-30"`. Only dated entries in that range are searched.

### Search with surrounding lines, page by page

- **Expected parameters:** the query; how many lines of context; page size.
- **Example:**
  > "Search this project for 'retry' with 3 lines of context around each match, 20 results at a time."
- **Expected output:** `search_memory` with `context_lines: 3` and `limit: 20`; asking for "the next page" repeats it with `offset: 20`.

### List projects and subprojects

- **Expected parameters:** none.
- **Example:**
  > "What projects do I have in sync82? Include the subprojects."
- **Expected output:** `list_projects` returns the project tree of the vault.

### List a project's files with sizes

- **Expected parameters:** none (the current project).
- **Example:**
  > "List every memory file we have for this project, with sizes."
- **Expected output:** `list_files` with `metadata: true`: each file with its size in bytes, estimated tokens and last-modified time.

### Find out which vault is in use

- **Expected parameters:** none.
- **Example:**
  > "Where is my vault actually stored right now?"
- **Expected output:** `get_vault_config` reports the active vault path, the global config and, with the current workspace, its `.sync82.json`.

---

## 🧾 Structured Output (JSON) Prompts

`list_projects`, `list_files`, `check_project_health` and `search_memory` accept `format: "json"`, which returns a JSON document (also sent as MCP structured content) instead of readable text — useful when the agent will process the result further.

### Project tree as JSON

- **Expected parameters:** none.
- **Example:**
  > "Give me the list of sync82 projects as JSON."
- **Expected output:** `list_projects` with `format: "json"`: `{"vault": ..., "projects": [{"name": ..., "subprojects": [...]}]}`.

### File inventory as JSON

- **Expected parameters:** none (the current project).
- **Example:**
  > "Return this project's memory files with their sizes as JSON, so we can build a table from it."
- **Expected output:** `list_files` with `metadata: true` and `format: "json"`.

### Health report as JSON

- **Expected parameters:** none (the current project).
- **Example:**
  > "Check this project's memory health and give me the result as JSON."
- **Expected output:** `check_project_health` with `format: "json"`: `{"project", "vault", "healthy", "files": {"memory": true, ...}}`.

### Search results as JSON

- **Expected parameters:** the query.
- **Example:**
  > "Search the vault for 'rate limit' and return the matches as JSON."
- **Expected output:** `search_memory` with `format: "json"`: each result with `project`, `file`, `line`, `text` and, when present, `entry_date` and context lines; `next_offset` when more results exist.

---

## ✅ Testing and Validation Prompts

### Check that the standard files exist

- **Expected parameters:** none (the current project).
- **Example:**
  > "Is this project's memory healthy? Are all the standard files there?"
- **Expected output:** `check_project_health` lists the six standard files and whether each exists. An unhealthy project is returned as an error result — a deliberate signal for the agent to fill the missing files, not a crash.

### Check whether the memory is up to date

- **Expected parameters:** none (the current project); optionally how many days count as old.
- **Example:**
  > "Is this project's memory up to date? Anything that looks stale or never filled in?"
- **Expected output:** `check_project_health` (with `stale_days` if you gave a number of days). Its `Warnings:` list names current-state files older than the newest progress/decisions entries, files still empty or holding the blank template, undated log entries and logs long enough to archive. The agent can then offer to update each file, for example by rewriting `architecture` from what the recent decisions say.

### Check every project at once

- **Expected parameters:** none; optionally another vault.
- **Example:**
  > "Check the memory health of every project in my vault. Which ones need attention?"
- **Expected output:** `check_project_health` with `all_projects: true`: one `HEALTHY`, `UNHEALTHY` or `WARNINGS` line per project and subproject, a count of each, then the missing files and warnings of the ones that need attention. It is an error result when any project is unhealthy.

### Confirm the connection after installing

- **Expected parameters:** none.
- **Example:**
  > "List every project in my sync82 vault."
- **Expected output:** `list_projects`; an empty list on a fresh vault confirms the server and the vault path both work.

### Preview an import before running it

- **Expected parameters:** the folder to import from.
- **Example:**
  > "Before restoring my backup from ~/backups/acme, show me what would be overwritten."
- **Expected output:** `import_memory` with `input_dir: "~/backups/acme"` and `dry_run: true`: nothing is written; the result lists `New:`, `Overwritten:` and `Skipped:` files. Ask again without the preview to import.

---

## 🧹 Maintenance Prompts

### Archive old entries

- **Expected parameters:** which log (`progress` or `decisions`) and how many days to keep.
- **Example:**
  > "Archive progress entries older than 6 months, we don't need them cluttering the context anymore."
- **Expected output:** `archive_memory` with `filename: "progress"` and `keep_days: 180`. Undated entries are never archived. The project must be named or come from the workspace, not only from the last session.

### Archive old entries and keep a summary

- **Expected parameters:** which log and how many days to keep (or a date).
- **Example:**
  > "Archive the decisions older than 90 days, but keep a short summary of them so we don't lose the context."
- **Expected output:** `archive_memory` with `dry_run: true` to list the entries that would go, `read_memory` to read them, then `archive_memory` again with `summary` set to the summary the agent wrote. The old entries are archived and the summary is added as one entry dated today, headed with how many entries it covers and their date range — both in one step.

### Delete a custom file

- **Expected parameters:** the custom file's name; your confirmation.
- **Example:**
  > "Delete the old 'testing-notes' file, we don't use that anymore."
- **Expected output:** the agent confirms with you, then calls `delete_memory` with `confirm: true`. The six standard files can't be deleted this way.

### Delete a project or subproject

- **Expected parameters:** the project (and subproject); your confirmation; for a project with subprojects, what to do with them.
- **Example:**
  > "We're done with the legacy-portal project. Delete it, and promote its subprojects to standalone projects."
- **Expected output:** after your confirmation, `delete_project` with `confirm: true` and `subproject_action: "promote"` (or `cancel`). Without `subproject_action`, sync82 lists the subprojects and asks. To delete the subprojects too, the agent shows you their list first and passes `subproject_action: "delete_all"` with `expected_subprojects`, their count; if the project has another number of subprojects by then, nothing is deleted and you get the new list.

### Rename a project

- **Expected parameters:** the current and the new name.
- **Example:**
  > "This project was renamed from borg-82 to sync82, update it here too."
- **Expected output:** `rename_project` with `new_name: "sync82"`. A `.sync82.json` elsewhere that still names the old project must be re-initialized or edited.

---

## 💾 Backup and Migration Prompts

### Export a project to Markdown

- **Expected parameters:** the destination folder; whether existing files may be replaced.
- **Example:**
  > "Export this project's memory to ~/notes/acme-memory so I can commit it to git. Overwrite what's there."
- **Expected output:** `export_memory` with `output_dir` and `overwrite: true`: one `.md` file per kind, plus `<kind>.archived.md` for archived entries and a `.sync82-kinds.json` manifest; `.md` files left by an earlier export are listed.

### Restore a project from Markdown

- **Expected parameters:** the folder to import from.
- **Example:**
  > "Restore this project's memory from ~/backups/acme."
- **Expected output:** `import_memory` with `input_dir`: creates the project if needed and overwrites each kind found in the folder, all in one transaction.

### Move a project to another vault

- **Expected parameters:** a temporary folder; the destination vault's path.
- **Example:**
  > "Copy the acme project into the vault at ~/vaults/work.db: export it to /tmp/acme first, then import it there."
- **Expected output:** `export_memory` from the current vault, then `import_memory` with `path: "~/vaults/work.db"`.

For a backup of the **whole** vault in one step, use the CLI: `sync82 export --all <output-dir>` — see [CLI Reference — export](./reference/cli.md#export).

---

## ➡️ Next Steps

- [Usage Examples](./guides/workflows/examples.md) — full end-to-end sessions
- [Tools Reference](./reference/tools.md) — every tool and argument
- [Back to Index](./index.md)
