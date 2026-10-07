# Changelog

All notable changes to sync82 are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.1] - 2026-10-07

### Changed

- **`update_project_memory` is all or nothing.** Every field is checked before anything is written — a `progress` without a date header or an empty field is reported together with every other problem — and all fields are then written in one transaction. Before, the valid fields of a call were written even when another one failed.
- `import_memory` and `sync82 import` skip `README.md`, `CHANGELOG.md`, `LICENSE.md`, `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md` instead of importing them as memory files, and refuse a folder whose `.md` files hold more than 64 MB in total.
- `delete_project` forgets the last used project when it deletes it, and follows a remembered subproject that it promotes to the vault root.

### Security

- `export_memory` never writes through a symlink swapped in after its check, and `import_memory` only reads the file it inspected, so a link placed in a shared folder can't redirect an export or pull another file into the vault.
- A panic in a resource, prompt or completion handler is answered with an internal error instead of crashing the server and ending every client's session.

### Fixed

- Promoting subprojects and deleting their project happen in one transaction, so a failure leaves everything as it was.
- Errors from resource reads, the resource list and completion are worded like tool errors, including the "upgrade sync82" hint for a vault with a newer schema.
- Completion only answers for an argument the prompt or resource template actually has, and `resources/list` rejects a cursor, since it returns everything in one page.
- `search_memory` rejects an invalid `project` or `subproject` name instead of reporting it as not found; its lines no longer keep a trailing carriage return from CRLF content.
- A `## YYYY-MM-DD` example inside a code block is no longer taken as a date header when the block mixes ```` ``` ```` and `~~~` lines.
- `sync82 install` writes `&`, `<` and `>` in other servers' values as they were instead of escaping them, refuses a config file whose `mcpServers` (or equivalent) is not an object instead of replacing it, and treats a file with stray characters after its JSON object as invalid.
- `write_memory` removes `<!-- entry:N -->` lines before checking that the content is not empty.

## [1.1.0] - 2026-10-07

### Added

- `load_project_context`: new `mode` argument — `summary` (default) or `full`.
- `load_project_context`: when dated entries are left out, the response ends with a footer per kind, for example `[older history omitted — progress: 10 of 142 dated entries shown (oldest shown 2026-09-15); ...]`, saying how to load more.
- `load_project_context`: the truncation note names the files that were cut short or left out entirely.
- New tool `edit_entry` (19 tools in total): change one entry of `progress`, `decisions` or a custom append kind by its id, without rewriting the rest of the file. `replace` rewrites it in place, `supersede` appends a new entry and marks the old one as superseded, and `delete` removes it (with `confirm: true`). It refuses a project taken only from the last session.
- `read_memory`: new `with_ids` option that puts a `<!-- entry:N -->` line before each entry of an append-only kind, with the id `edit_entry` takes.
- `search_memory`: JSON results include `entry_id` for matches inside an entry.
- `search_memory`: new `match` argument (`words`, `phrase`, `exact`), a trailing `*` for prefix search (`instal*`), and filters `kinds`, `since` and `until`.
- `check_project_health`: warnings, which never make the project unhealthy, about current-state files older than the newest `progress`/`decisions` entries (`stale`, with a new `stale_days` argument, default 30), files still empty or holding the blank template (`template`), undated log entries (`undated_entries`) and logs over 200 active entries (`large_history`). The JSON report lists them under `warnings`.
- `archive_memory`: new `dry_run` argument that lists the entries that would be archived, and new `summary` argument that adds the agent's summary of them as one active entry, dated today, in the same transaction that archives them.
- MCP resources: each project's and subproject's memory is readable as `sync82://projects/<project>/context` (the default `load_project_context` block) and `sync82://projects/<project>/files/<file>`, from the default vault, read-only. `resources/list` lists the context of every project.
- MCP prompts `start_session` and `end_session`, which tell the agent to load the project memory or save the session to it. The Claude Desktop extension's manifest declares that the server provides prompts (`prompts_generated`).
- MCP tool annotations on every tool: a title, `readOnlyHint`, `destructiveHint`, `idempotentHint` and `openWorldHint: false`, so clients can tell read-only tools from the ones that overwrite or delete memory.
- MCP completion (`completion/complete`) for the `project`, `subproject` and `file` arguments of the prompts and resource templates, from the default vault.
- `notifications/resources/list_changed` after a tool call creates, renames or deletes a project.

### Changed

- **`load_project_context` loads only recent history by default.** In `summary` mode, each log (`progress`, `decisions`, custom append kinds) brings its 10 most recent dated entries, and the response is cut at 40 KB instead of 200 KB. This keeps it under the tool-output limits of MCP clients; Claude Code, for example, saves a result over 50,000 characters to a file instead of showing it. Current-state files (`memory`, `architecture`, `stack`, `next_steps`) still load in full. Use `mode: "full"` for the previous behavior.
- `load_project_context`: in `summary` mode, an explicit `since` lifts the default 10-entry limit, and an explicit `max_entries` or `max_bytes` replaces the mode default.
- **`search_memory` searches by words by default.** In `words` mode it finds the documents and entries holding every word of the query, in any order, ignoring case and the accents of Latin letters (`decisao` finds `Decisão`), and returns the best matches first. Use `match: "exact"` for the previous literal substring search, which is still needed for punctuation such as `100%` or `foo_bar`.
- **The vault schema moves to version 3**, which adds full-text indexes and builds them on the first open. A vault opened by this version can no longer be opened by sync82 1.0.0; upgrade every client that shares the vault.
- New dependency: `golang.org/x/text`, for the Unicode normalization of search.
- A tool call on a vault upgraded by a newer sync82 now says so (`the vault's schema is newer than this sync82 supports; upgrade sync82`) instead of a generic `could not open the vault database`.
- `write_memory`, `append_memory`, `update_project_memory` and `import_memory` remove whole `<!-- entry:N -->` lines from their content, so content read with `with_ids` can be written back without storing the ids.

### Security

- A `.sync82.json` that is a symlink, directory or special file is refused instead of followed: `init_project_memory` could otherwise overwrite any JSON file the user owns through a symlinked `.sync82.json` shipped in a cloned repository.
- `write_memory`, and `update_project_memory` when any of its fields overwrites, refuse a project taken only from the last session, like the other destructive tools; the project must come from `project` or `workspace_root`. A call that only appends may still use it.
- `init_project_memory` validates project and subproject names that come from `.sync82.json` or the last session, so a malformed name can no longer create a project the other tools can't reach.
- MCP resource reads are marked `cacheScope: "private"`.
- The release workflow builds the signed binaries without restoring the Go module and build caches.

### Fixed

- Opening a vault no longer fails for good when it holds several spellings of one project or file name and none in lower case (`Foo` and `FOO`): the name migration renames one of them and keeps the others.
- `--path` followed by another flag (`sync82 import acme ./in --path --all`) is a usage error instead of creating a vault named after that flag; `--path=<value>` still accepts a value starting with `-`.
- A local build from a modified checkout, or a `go install` of an untagged commit, reports version `dev` (it reported the last tag, e.g. `v1.0.0+dirty`), so `self-update` refuses to replace it, as documented.
- `load_project_context` never exceeds `max_bytes`, truncation note included, and a file made of one long line is no longer cut back to almost nothing.
- `search_memory` cuts each matching or context line at 4 KB, so one huge line can't exceed the 1 MB response cap; a `subproject` given with a workspace that has no `.sync82.json` is an error result instead of a search of the whole vault.
- `init_project_memory` no longer reports a workspace's `.sync82.json` as pointing to another project when it names the same project in another case.

## [1.0.0] - 2026-10-06

First public release.

### Added

- MCP server over stdio with 18 tools for persistent, per-project memory stored in an embedded SQLite vault (no cgo, no network access at runtime).
- CLI subcommands `install`, `uninstall`, `self-update`, `export`, `import` and `config`.
- `sync82 install` for Claude Code, Claude Desktop, Antigravity, Codex, OpenCode, Cursor, Zed and Cline.
- Release binaries for Linux, macOS and Windows (amd64/arm64), a Claude Desktop extension (`sync82.mcpb`) and an MCP Registry entry (`io.github.oito2/mcp-sync82`), with SHA-256 checksums, a Sigstore signature and GitHub build provenance attestations.

[Unreleased]: https://github.com/oito2/mcp-sync82/compare/v1.1.1...HEAD
[1.1.1]: https://github.com/oito2/mcp-sync82/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/oito2/mcp-sync82/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/oito2/mcp-sync82/releases/tag/v1.0.0
