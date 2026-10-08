# Changelog

All notable changes to sync82 are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0] - 2026-10-08

### Added

- `read_memory`: new `archived` argument that reads only the archived entries of a log. With `with_ids`, each one comes with its id, so `edit_entry` can replace or delete an archived entry; before, no tool showed those ids.
- `sync82 self-update` verifies the Sigstore signature of the release's `checksums.txt` when `cosign` v3 or later is on `PATH`. The certificate must name this repository's release workflow for exactly the new tag, and a failed verification aborts the update. Without cosign, it warns and relies on the checksum. The new `--require-signature` option refuses to update without a verified signature.

### Changed

- **`sync82 self-update --check` exits with `10` when an update is available**, and with `0` only when the binary is up to date. Before, both cases exited with `0`.
- `search_memory` in `words` and `phrase` mode finds text the full-text index splits or folds differently, such as `100₽`, Cherokee or letters newer than SQLite's Unicode tables. When the index finds nothing for a query holding characters outside the Latin, Greek and Cyrillic scripts, the words are matched inside the text instead, still as whole words (or in order, for a phrase). These results come in reading order, with a note in the text and `substring_fallback: true` in the JSON report.
- **`delete_project` with `subproject_action: "delete_all"` needs `expected_subprojects`**, the number of subprojects shown to the user, checked in the same transaction as the deletion. Without it, the tool answers with the list of subprojects; with another number, it deletes nothing. Before, `delete_all` on a first call deleted subprojects nobody had seen. A project without subprojects is now deleted only while it still has none.
- `sync82 install` and `uninstall` keep the order of the keys in the JSON config files they rewrite, and every value as written; only the indentation changes. Before, keys came out in alphabetical order. A new `sync82` entry goes at the end of the server list.
- **`edit_entry` refuses `supersede` on an archived entry**, with an error result that changes nothing. Before, the replacement was added as an active entry. `replace` and `delete` still work on archived entries.

### Fixed

- `load_project_context` keeps its response within `max_bytes` when `files` names many long files that don't exist: they are counted (`[N requested files not found]`) instead of named when the note would take more than a quarter of `max_bytes`.
- `sync82 install codex` and `uninstall codex` no longer take a failed `codex mcp get` (a broken `config.toml`, a timeout) for "not registered". Only `No MCP server named` means that; any other failure fails the target with Codex's output. Before, `uninstall` could report `not configured` while sync82 was still registered.

### Security

- The release workflow runs the same `cosign verify-blob` check as `self-update` (exact workflow identity and tag) before it publishes a release.

## [1.2.0] - 2026-10-08

### Added

- `read_memory`: new `max_bytes` argument (default 1 MB). A longer file is cut at a line break, with a note giving the sizes, so one large log can no longer produce a response of tens of megabytes.
- CLI: `--` ends the options, so a search query or a value starting with `-` can be given (`sync82 search -- --dry-run`).
- `sync82 install`/`uninstall` honor `CLINE_DIR`, the Cline CLI's configuration directory.
- CLI subcommands `sync82 search`, `sync82 context` and `sync82 health`: the `search_memory`, `load_project_context` and `check_project_health` tools from the command line, with their arguments as flags (`--project`, `--match`, `--kinds`, `--full`, `--json`, `--path`…) and the same output. They only read: they never create a vault and never record the last used project. `sync82 health` exits with `1` when a project is unhealthy.
- `check_project_health`: new `all_projects` argument that checks every project and subproject of the vault, with one status line each and the details of the ones that need attention (`sync82 health --all` in the CLI). Its JSON report is `{vault, healthy, projects: [{project, subproject, healthy, missing, warnings}]}`.
- `list_projects`, `list_files`, `check_project_health` and `search_memory` declare an output schema (`outputSchema`), and every successful result carries its report as `structuredContent`, in `text` format too. A client that shows the agent the structured content, as Claude Code does, gets JSON from these tools whatever the `format`.
- Every subcommand except `help` and `version` accepts `-h`/`--help`, which prints its usage and options without running it. `sync82 --help` lists `serve`.
- `get_vault_config` reports `last_vault_path`, the vault the last used project was remembered in, and, under `local_config`, `vault`, the vault the workspace resolves to.
- `export_memory` and `sync82 export` write a `.sync82-kinds.json` manifest next to the files, naming each file a log or a document; `import_memory` and `sync82 import` read it, so a custom log comes back as a log. A folder without it imports as before.
- `export_memory` and `sync82 export` list the `.md` files left in the folder by an earlier export, of files the project no longer has, since an import would bring them back.

### Changed

- Project, subproject and file (kind) names are limited to 128 characters, and surrounding spaces in a file name are trimmed, as for project names. An export skips, and lists, a file whose name can't be a file name instead of failing.
- `export_memory` with `overwrite: true` refuses a project taken only from the last session, like the other tools that overwrite or delete.
- `rename_project` refuses the current name as `new_name`, and `rename_project`/`delete_project` report a missing project once (`project not found: "x"`).
- Tools that check several arguments list every problem together (`load_project_context`, `check_project_health`, `init_project_memory`, `read_memory`, `update_project_memory`'s custom files).
- `load_project_context` names a requested file that doesn't exist (`[no file named: …]`) instead of answering as if the project had no content; `search_memory` says `No more results … at offset N` past the last page; `archive_memory`'s "nothing to archive" message counts undated entries apart.
- `delete_project` no longer announces a resource list change when it only asks for `subproject_action` or is cancelled.
- CLI usage errors point to the subcommand's own help (`Run 'sync82 search --help'`), and the errors of `search`, `context` and `health` name the CLI flags rather than tool arguments. `install` prints its "Registering …" line only once its arguments are valid.
- A Ctrl-C at a confirmation prompt (`install`, `uninstall`, `self-update`) ends the command at once with `Interrupted; nothing was changed.`
- `self-update` keeps the permission bits of the binary it replaces instead of always setting `0755`.
- **The vault schema moves to version 4**, which rebuilds the entries and projects tables so an id is never given to another row: an `entry_id` read earlier can no longer edit or delete a different entry after a rewrite. Every id is kept. A vault opened by this version can no longer be opened by sync82 1.1.x; upgrade every client that shares the vault.
- `check_project_health`: only a missing current-state file (`memory`, `architecture`, `stack`, `next_steps`) makes a project unhealthy. `progress` and `decisions` with no entry yet are shown as `EMPTY (no entry yet)` with a new `empty_log` warning, so a project `init_project_memory` just set up is healthy. The recommendation names `init_project_memory`, which writes only the missing files.
- `init_project_memory` without `project` or `workspace_root` refuses to write answers (or `auto_detect` results) into the last used project, and says when it initialized that project. With `path`, it no longer re-points a workspace's `.sync82.json` that uses another vault: the file is left unchanged and the result says so.
- `sync82 install`: a commented config file that already holds the wanted sync82 entry counts as installed (`updated.`) instead of needing a manual step on every run. `sync82 uninstall` reports such a file as `manual step needed` instead of `failed`, and its summary counts these separately.
- A `workspace_root` of only spaces is refused instead of being read as absent, which fell back to the last used project.
- A relative `SYNC82_DB_PATH` is made absolute when the process starts.
- `self-update`: `--rollback` can't be combined with `--check` or `--yes` (before, `--check --rollback` rolled back), and an option given twice is a usage error.
- `list_files`, `check_project_health` and `search_memory` return their request for `project` or `workspace_root`, and a missing vault, as an error result (`isError: true`): a tool that declares an output schema must return structured content with every successful result.
- When a call passes a `path` that names another vault than the one the last used project was remembered in, that project is no longer reused there: the call asks for `project` instead. `init_project_memory` follows the same rule.
- `sync82 install` reports a config file with comments or trailing commas as `manual step needed` instead of `failed`, and the summary counts these separately (`Done. N installed, N skipped, N need a manual step, N failed.`). The exit code is still `1`.
- `sync82 uninstall` with no target also cleans a file target whose client is no longer detected when its config still holds a sync82 entry, and says `--purge skipped too` when the client removal is declined with `--purge`.
- `sync82 config get-vault`, with no global vault set, shows the vault used instead and whether it comes from `SYNC82_DB_PATH`.
- A server stopped by `SIGINT`/`SIGTERM` logs the shutdown at `INFO` instead of `ERROR`, and a second Ctrl-C ends the process at once.
- `search_memory` is faster: `exact` mode first narrows the rows with the full-text indexes when the query holds a whole word after its first one (about 3.6× faster on 10,000 entries), and matching lines are found with a cached folding function (about 14× faster per line).
- `check_project_health` and `load_project_context` find which files a project has in one query instead of reading every file's content first, so they no longer slow down as the logs grow.

### Security

- `import_memory` opens each file without following a symlink and without blocking on a named pipe swapped in after its check.
- The project analyzer of `init_project_memory` (`auto_detect`) no longer lists the contents of a `packages/`, `apps/`, `libs/` or `modules/` directory that is a symlink, which could point outside the workspace.
- The release workflow builds the artifacts in a read-only job and signs, attests and publishes them in a separate job that runs no repository code, after checking every asset against `checksums.txt`; only that job holds the write permissions and the OIDC token. The tag pipeline also checks that the binary reports the tag as its version, runs the self-update contract test on the real artifacts, and runs its checks on Linux, macOS and Windows. `govulncheck` is pinned to a version in CI and in the release workflow.
- `self-update` only downloads assets of this repository's release of the new tag (`https://github.com/oito2/mcp-sync82/releases/download/<tag>/…`), and follows redirects only to `github.com`, `objects.githubusercontent.com` and `release-assets.githubusercontent.com` instead of any `*.githubusercontent.com` host, which also serves user content.
- The project analyzer of `init_project_memory` (`auto_detect`) opens marker files without blocking, so a named pipe swapped in for one can't hang the server.

### Fixed

- A client config file or a project file (`README.md`, `package.json`…) starting with a UTF-8 byte order mark is read normally; `uninstall` reports a config file it can't parse instead of taking it for one without sync82.
- On a read-only filesystem, reading the global config no longer fails.
- Renaming, promoting or deleting a subproject while another call deletes its project fails with "not found" instead of reporting success or a database error.
- `list_files` with `metadata` no longer fails when a file is deleted while it lists; `append_memory` refuses content made only of entry id markers before running.
- `self-update` removes the leftovers of an earlier update even when the install path holds characters such as `[`.
- `search_memory` finds words in scripts whose combining marks the full-text index treats as separators — Devanagari (Hindi), vocalized Hebrew, Arabic with harakat, Thai and others — in `words`, `phrase` and `exact` mode. Before, such searches found nothing.
- `search_memory` keeps its whole response, text and structured content together, within about 1 MB; content full of characters JSON escapes (such as `<`) could produce responses of more than 10 MB.
- `init_project_memory` writes the answers of a later call into the documents that still hold the blank template, as documented, instead of reporting that every file already has content.
- Writing one file both as a document and as a log can no longer happen: `update_project_memory` refuses the same custom file given twice, an import keeps a custom log that has a `<kind>.archived.md` file as a log, and any write that would store a file the other way it is stored is refused.
- Splitting a long log into dated entries no longer takes time proportional to the square of its size (a 2 MB log took tens of seconds; `archive_memory` held the vault meanwhile), and a line that starts with inline code such as ```` ```go test``` ```` no longer hides the date headers after it.
- `self-update` flushes the new binary to disk before running it and the directory after the swap, and on Windows retries a rename an antivirus briefly blocks.
- A corrupt `config.json` that is a symlink is repaired by the next update even when that update changes nothing.
- On Windows, `sync82 install claude-desktop` also writes the configuration of Claude Desktop installed as an MSIX package (the claude.ai download and the Microsoft Store), which reads its config from `%LOCALAPPDATA%\Packages\Claude_<id>\LocalCache\Roaming\Claude`; `uninstall` cleans it too.
- On Windows, `self-update` no longer fails when the version kept by the previous update is still running in an MCP client: it is moved aside to `<binary>.bak.old-<n>` and removed by a later update. When even that fails, the message says to restart the MCP clients instead of suggesting elevated privileges.
- On Windows, updating `~/.sync82/config.json` no longer fails while another sync82 process reads it: reads take a shared lock on `config.lock`, and a replacement blocked by another program is retried for up to a second.
- A corrupt `config.json` that is a symlink keeps its link: its content is copied aside instead of the link being renamed.
- On Windows, a `path` starting with `~\` (or `HOME\`, `$HOME\`) is expanded to the home directory, like `~/`.
- A write that races the deletion of its project fails with the project's "not found" error instead of a database foreign-key error, and can't be written to another project that took over its internal id.

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

[Unreleased]: https://github.com/oito2/mcp-sync82/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/oito2/mcp-sync82/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/oito2/mcp-sync82/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/oito2/mcp-sync82/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/oito2/mcp-sync82/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/oito2/mcp-sync82/releases/tag/v1.0.0
