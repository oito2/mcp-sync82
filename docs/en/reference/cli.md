🌐 [Português](../../pt-br/reference/cli.md) | **English** | 🏠 [Index](../index.md)

---

# CLI Reference

`sync82` is both the MCP server binary and its own CLI. All commands below are subcommands of the same `sync82` binary — there's nothing else to install.

**Argument parsing:** flags can be given as `--path value` or `--path=value`, anywhere on the line. An unknown option (e.g. `--force`), an empty or repeated `--path`, and extra arguments (e.g. `sync82 version x`, `sync82 install claude codex`) are **usage errors**: the command does nothing, prints `Error: <reason>` followed by `Run 'sync82 --help' for usage.` on stderr, and exits with code `2`. An unknown subcommand (e.g. `sync82 instal`) exits with code `1` and prints the help, and a flag-like first argument (e.g. `sync82 --bogus`) is a usage error. A confirmation prompt that gets no answer because stdin is closed (e.g. `sync82 install </dev/null`) is an error (code `1`), never read as "no".

## `sync82` (serve)

Running the binary with no arguments starts the MCP server over stdio. This is what your MCP client actually launches — you won't normally run this by hand.

```bash
sync82
```

- Registers all [18 tools](./tools.md).
- Resolves the default vault path from the `SYNC82_DB_PATH` environment variable, or `~/.sync82/knowledge.db` if unset.
- Logs to stderr only — stdout is reserved for the JSON-RPC protocol.
- Stops cleanly on `SIGINT`/`SIGTERM` (as do the other subcommands): the server closes its open vaults and exits with code `0`.

## `help` / `--help` / `-h`

Prints the list of subcommands and exits — no vault or MCP client is touched.

```bash
sync82 help
sync82 --help
sync82 -h
```

An unrecognized subcommand prints the same usage list (to stderr) before exiting with code `1`.

## `version` / `--version` / `-v`

Prints the installed version and exits.

```bash
sync82 version
sync82 --version
sync82 -v
```

Release builds print the version injected at build time into `github.com/oito2/mcp-sync82/internal/version.Current`. A binary built by `go install github.com/oito2/mcp-sync82/cmd/sync82@vX.Y.Z` reports `vX.Y.Z` (read from the module build info). Only a local build from a source checkout prints `dev` — see [`self-update`](#self-update) below for why that matters.

## `install`

Wires sync82 into one or all supported MCP clients' configs.

```bash
sync82 install            # list the detected clients, ask to confirm, configure each of them
sync82 install <target>   # configure a single target, no confirmation prompt
```

`install` takes at most one target.

Every target is registered with the **absolute path** of the running `sync82` binary (symlinks resolved), not the bare command `sync82`, so the binary doesn't need to be on the MCP client's `PATH`. `install` prints `Registering <path> — run "sync82 install" again after moving the binary.` first: if you move the binary later, run `sync82 install` again. Re-running it is safe — it replaces the existing registration.

**Targets:** `claude`, `codex` (each via its own `mcp add` subcommand, after removing any existing registration), `claude-desktop`, `antigravity`, `opencode`, `cursor`, `zed`, `cline` (each via reading, merging, and writing a JSON config file). See [Installation](../getting-started/installation.md) or the [Installer internals](../architecture/installer.md#targets) for the exact file each target touches on each OS.

A client is **detected** when its command is on `PATH` or its config directory exists (per target — see [Detection](../architecture/installer.md#detection)). Running `sync82 install` with no target acts only on detected clients: it prints `Detected the following MCP clients:` with one `  - <target>` line each, then asks `Install sync82 into all N detected client(s)? [y/N]` on stdin before touching anything (any answer other than `y`/`yes` prints `Aborted.`; a closed stdin is an error that suggests `sync82 install <target>`). When nothing is detected it prints `No supported MCP clients detected. Supported targets: ...` and exits with `0`. An explicit target whose client isn't detected is **skipped**, not treated as an error: `Skipped: <target> not detected.` On an OS other than macOS, Windows and Linux, `claude-desktop` prints `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).` Each target that succeeds prints `✓  <target> — configured.` for a new registration or `✓  <target> — updated.` when sync82 was already registered (for `claude` and `codex`: a previous registration was removed first); with no target, the run ends with `Done. N installed, N skipped, N failed.` A config file with comments or trailing commas is never rewritten: that target fails and prints the entry to add by hand.

**Exit codes:**

| Code | Meaning |
|---|---|
| `0` | Every target installed or skipped cleanly (or no client was detected, or the user declined the confirmation prompt) |
| `1` | An unrecognized target name, a closed stdin at the confirmation prompt, or at least one target failed |
| `2` | Usage error: a flag or more than one target was given |

## `uninstall`

Removes sync82's registration from one or all supported MCP clients, and optionally deletes sync82's own files.

```bash
sync82 uninstall                    # list the detected clients, ask to confirm, remove sync82 from each
sync82 uninstall <target>           # one target, no confirmation prompt
sync82 uninstall [target] --purge   # then also delete ~/.sync82 files, after a separate confirmation
sync82 uninstall --help             # print the usage
```

`uninstall` takes at most one target — the same targets as [`install`](#install) — and the `--purge` flag. With no target, it acts only on detected clients, like `install`: it prints `Detected the following MCP clients:` and asks `Remove sync82 from all N detected client(s)? [y/N]`; the run ends with `Done. N removed, N skipped, N failed.` When nothing is detected it prints `No supported MCP clients detected. Supported targets: ...`, and `--purge` still runs. Each target prints `✓  <target> — removed.`, `⚠  <target> — not configured, skipping` (nothing registered), `⚠  <target> — nothing removed` (`claude` with only a project-scope registration), `Skipped: <target> not detected.` (an explicit `claude` or `codex` target whose command isn't on `PATH`), `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).`, or `✗  <target> — failed`.

- `claude` loops `claude mcp get sync82` and runs `claude mcp remove --scope <scope> sync82` for each scope it reports (`local`, `user`). A project-scope registration (`.mcp.json`) is left unchanged with a `Warning: sync82 is also registered in project scope (.mcp.json, shared with the project) for <dir>; it was left unchanged and, inside that project, it takes precedence over any user-scope registration. To remove it, run from that directory: claude mcp remove --scope project sync82`, and the user scope is then removed directly. `install` does the same before `claude mcp add`.
- `codex` runs `codex mcp get sync82` and, when registered, `codex mcp remove sync82`.
- File targets (`opencode` included) have the `sync82` key deleted from every config file that holds one, keeping every other key — even when an explicitly named file target's client is no longer detected. A file with comments or trailing commas is left unchanged and the target fails with a message to remove the entry by hand.

**`--purge`** runs after the targets. It notes the vaults configured outside `~/.sync82` (`SYNC82_DB_PATH`, `vaultPath`/`lastVaultPath` in `config.json`, the `path` of a `.sync82.json` in the current directory or a parent) — those are **never deleted** — then lists the existing files among `knowledge.db`, `knowledge.db-wal`, `knowledge.db-shm`, `config.json`, `config.lock` and `config.json.corrupt-*` in `~/.sync82`, and asks `Delete these files? [y/N]`. It prints `Purge complete.`, `Purge cancelled.`, or `Purge incomplete.` when a file couldn't be deleted. `~/.sync82` itself is removed only when left empty. Close every MCP client using sync82 first.

**Exit codes:**

| Code | Meaning |
|---|---|
| `0` | Every target removed or skipped cleanly, and the purge (if any) completed or was declined — also when the user declined the first confirmation prompt |
| `1` | An unknown target, a closed stdin at a confirmation prompt, at least one target failed, or an incomplete purge |
| `2` | Usage error: an unknown flag or more than one target |

See [Uninstallation](../getting-started/uninstallation.md) for a complete removal, binary included.

## `config`

Manages the global vault path override recorded in `~/.sync82/config.json` — the same file that tracks the last-used project for [context resolution](../architecture/context-resolution.md).

```bash
sync82 config set-vault <path>    # use a non-default vault database for every session
sync82 config get-vault           # show the currently configured path, if any
sync82 config unset-vault         # go back to the default (~/.sync82/knowledge.db)
```

`<path>` accepts the same `HOME`/`$HOME`/`~` expansion as every tool's optional `path` argument, and is stored as an absolute path (a relative one is resolved against the current directory). It applies to every tool call and to the `export`/`import` commands; an explicit `path` argument (or `--path`), or a workspace's `.sync82.json`, still wins over it, and it wins over `SYNC82_DB_PATH` and the default.

Updates to `config.json` from several sync82 processes at once (e.g. two MCP clients) are serialized with a lock file, `~/.sync82/config.lock`, and an update that changes nothing doesn't rewrite the file. A `config.json` that is empty or not valid JSON is moved aside to `config.json.corrupt-<unix-timestamp>` on the next update (`set-vault`/`unset-vault`, or a tool call that records the last-used project), and a fresh config is started — see [Troubleshooting](../troubleshooting/common-issues.md#a-configjsoncorrupt--file-appeared-in-sync82).

**Exit codes:** `0` on success, `1` on a config read/write failure, `2` (usage error) on an unknown or missing `config` subcommand, a missing `set-vault` path, or extra arguments.

## `self-update`

Checks GitHub Releases for a newer version and replaces the binary currently on disk, in place. Versions are compared by full semver precedence, pre-release identifiers included: `v1.2.3` is newer than `v1.2.3-rc.1`, `rc.10` newer than `rc.2`, and build metadata (`+...`) is ignored — so a `go install` build at a pseudo-version past a tag is not offered that tag again.

```bash
sync82 self-update            # check, show current → latest, confirm, then update
sync82 self-update --check    # check only, don't download or install anything
sync82 self-update --yes      # skip the confirmation prompt (also accepts -y)
sync82 self-update --rollback # swap back to the previous version kept by the last update
```

Any other option is a usage error (exit code `2`). Without `--yes`, a closed stdin at the `Update now? [y/N]` prompt is an error (exit code `1`) that suggests `--yes`.

`self-update` only works on a binary that knows its version — a release build, or a `go install github.com/oito2/mcp-sync82/cmd/sync82@vX.Y.Z` build. A local build from a source checkout reports `dev` and refuses to run it. The update:

1. Fetches the latest release from the GitHub API (User-Agent `sync82/<version>`, 30 s timeout; a GitHub rate-limit response is reported as such).
2. Downloads the matching platform binary (`sync82_<os>_<arch>`, `.exe` on Windows, 5 min timeout) and `checksums.txt` — only HTTPS URLs on `github.com` or `*.githubusercontent.com` are accepted, for the initial URL and for every redirect it follows (at most 10). The binary is staged in a temporary `.sync82-update-*` directory **next to the running binary**, so the final rename never crosses filesystems (e.g. a tmpfs `/tmp`).
3. Verifies the SHA-256 checksum before touching anything. The checksum detects a corrupted or truncated download; `checksums.txt` comes from the same release, so its Sigstore signature is not checked here — to confirm a release was built by this repository, verify it by hand as shown in [Installation — Verifying the Signature](../getting-started/installation.md#verifying-the-signature-optional).
4. Runs the new binary with `--version` (10 s timeout); it must print exactly the release tag, otherwise nothing is changed.
5. Renames the running binary to `<binary>.bak`, then renames the new one into place — if that second step fails, the original is restored. This works the same on Windows, where a running `.exe` can be renamed.

On success it prints where the previous version is kept and how to undo the update with `sync82 self-update --rollback`. Because the download is staged next to the binary, `self-update` needs write permission on the binary's directory; a permission error suggests re-running with elevated privileges or reinstalling via `go install`.

`sync82 self-update --rollback` swaps the binary with `<binary>.bak`, after checking that the backup runs (`--version`). Running it again swaps back. It fails with exit code `1` if there is no backup.

The update takes effect the **next time** something launches `sync82` fresh (e.g. your MCP client's next restart) — the currently running server process is unaffected.

**Exit codes:**

| Code | Meaning |
|---|---|
| `0` | Already up to date, update available (with `--check`), update completed, rollback completed, or the user declined the confirmation prompt |
| `1` | Error (network failure, checksum mismatch, dev build, permission denied, closed stdin at the prompt, ...) |
| `2` | Usage error: an unknown option |

> **Pick one update mechanism and stick with it.** If you installed via `go install` and then run `self-update`, the binary is no longer "managed" by `go install` — running `go install .../sync82@latest` again later silently overwrites it back. The two paths have no awareness of each other.

## `export`

Dumps a project's memory to plain Markdown files on disk — the CLI equivalent of the [`export_memory`](./tools.md#export_memory) tool, for scripting or one-off backups without going through an MCP client.

```bash
sync82 export <project> [subproject] <output-dir>
sync82 export <project> [subproject] <output-dir> --path <vault>
sync82 export --all <output-dir>                    # every project/subproject in the vault
sync82 export --all <output-dir> --path <vault>
```

Writes one `.md` file per kind (`memory.md`, `progress.md`, etc.) into `<output-dir>`, in the same flat-file format the project used before moving to SQLite. Existing files there are overwritten, but a destination that is a symlink or special file is refused. A kind with archived entries also gets a `<kind>.archived.md` file holding them. The vault must already exist — a vault path where none exists is an error, not an empty export. `--all` skips naming a project entirely — it walks every top-level project and subproject in the vault, writing each into its own `<output-dir>/<project>[/<subproject>]` subfolder, for a full vault backup in one command.

**Exit codes:** `0` on success (including "nothing to export"), `2` (usage error) on a missing or extra argument or an unknown option, `1` on a missing vault, an unknown project, a project name that isn't a valid folder name, or a filesystem error.

## `import`

Restores a project's memory from plain Markdown files — the inverse of `export`, and the CLI equivalent of the [`import_memory`](./tools.md#import_memory) tool.

```bash
sync82 import <project> [subproject] <input-dir>
sync82 import <project> [subproject] <input-dir> --path <vault>
sync82 import <project> [subproject] <input-dir> --dry-run   # report what would happen, write nothing (not even a new vault)
```

Reads every `"<kind>.md"` file in `<input-dir>` and writes it into `<project>` (creating it first if it doesn't exist yet). `<project>` and `[subproject]` must start with a letter or digit and contain only letters, digits, hyphens and underscores. Symlinks, special files and files over 10 MB are skipped. A `"<kind>.archived.md"` file restores that kind's archived entries. Files that aren't valid kind names, or are empty, are skipped and listed in the output. **Destructive per kind**: an existing kind's content is overwritten, not merged; archived entries already in the vault are kept unless the folder has a `<kind>.archived.md` for that kind. The import is all or nothing: every file is read and checked first, then everything is written in one transaction, so a failure leaves the vault unchanged. The output lists `New:` (files that create a kind), `Overwritten:` (files that replace an existing kind) and `Skipped:`; with `--dry-run` it shows the same report without writing anything. Two files that become the same kind once lower-cased (e.g. `Memory.md` and `memory.md`) make the import fail.

```bash
# Full round trip: back up a project, then restore it into a fresh vault
sync82 export acme ./backup
sync82 import acme ./backup --path ./restored-vault.db
```

**Exit codes:** `0` on success, `2` (usage error) on a missing or extra argument, an unknown option or an invalid project/subproject name, `1` on a missing `<input-dir>` or a filesystem error.

---

[← Back to Index](../index.md)
