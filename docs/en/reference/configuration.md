🌐 [Português](../../pt-br/reference/configuration.md) | **English** | 🏠 [Index](../index.md)

---

# Configuration Reference

Every source of configuration sync82 reads. sync82 has **no command-line flags** for the server and **one environment variable**; everything else lives in two small JSON files. For how these sources combine on a tool call, see [Context Resolution](../architecture/context-resolution.md).

---

## Overview

| Source | Scope | Written by | What it sets |
|---|---|---|---|
| `SYNC82_DB_PATH` | process (per MCP client entry) | you, or the Claude Desktop bundle | default vault path |
| `~/.sync82/config.json` | user (every client, every project) | `sync82 config set-vault`/`unset-vault`, and tool calls | global vault path, last-used project |
| `.sync82.json` | one workspace | `init_project_memory` (with `workspace_root`), or you | project, subproject, vault path for that workspace |
| `path` tool argument / `--path` flag | one call | the agent, or you on the CLI | vault path for that call |
| MCPB `user_config.db_path` | Claude Desktop extension | Claude Desktop's extension settings | passed to the server as `SYNC82_DB_PATH` |

---

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `SYNC82_DB_PATH` | `~/.sync82/knowledge.db` | Vault file used when no `path` argument, `.sync82.json` `path` or global `vaultPath` applies. A leading `~`, `~/`, `HOME`, `HOME/`, `$HOME` or `$HOME/` is expanded to the home directory. Read once when the server (or `export`/`import`) starts. |

To set it for one client, add it to that client's server entry, e.g. in a `mcpServers`-style config:

```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": [],
      "env": { "SYNC82_DB_PATH": "~/vaults/work.db" }
    }
  }
}
```

`sync82 uninstall --purge` reads `SYNC82_DB_PATH` too, only to list that vault as one it leaves untouched.

---

## `~/.sync82/config.json` (global config)

Created on demand in `~/.sync82/` (directory mode `0700`, file mode `0600`). A missing file is the same as an empty config.

| Field | Type | Written by | Description |
|---|---|---|---|
| `vaultPath` | string | `sync82 config set-vault <path>` / removed by `unset-vault` | Global vault path override, stored as an absolute path. |
| `lastProject` | string | tool calls | Last project a tool call named that exists in its vault — tier 3 of context resolution. |
| `lastSubproject` | string | tool calls | Subproject of `lastProject`, if any. |
| `lastVaultPath` | string | tool calls | Absolute path of the vault `lastProject` lives in. A project resolved from the global config is opened in this vault unless the call passes `path`. |

Every field is omitted when empty. Example:

```json
{
  "lastProject": "acme",
  "lastSubproject": "api",
  "lastVaultPath": "/home/user/.sync82/knowledge.db",
  "vaultPath": "/home/user/vaults/work.db"
}
```

**`config.lock`** — every update of `config.json` holds an exclusive advisory lock on `~/.sync82/config.lock` (created with mode `0600`), so several sync82 processes (one per MCP client, plus the CLI) never overwrite each other's update. An update that changes nothing doesn't rewrite the file.

**Corrupt file recovery** — when an update finds a `config.json` that is empty, whitespace-only or not valid JSON, it renames it to `config.json.corrupt-<unix-timestamp>` (keeping its permissions) and continues from an empty config. Until the next update, tools that only read the file log the parse error and skip the global tiers. See [Troubleshooting](../troubleshooting/common-issues.md#a-configjsoncorrupt--file-appeared-in-sync82).

---

## `.sync82.json` (workspace config)

A marker file at the root of a workspace, read when a tool call passes `workspace_root` (and, with `search_parent_dirs: true`, looked for in up to 64 parent directories). `init_project_memory` writes it (mode `0644`) when called with `workspace_root`; you can also write it by hand.

| Field | Type | Required | Description |
|---|---|---|---|
| `project` | string | yes | Project this workspace maps to. A file without it is ignored. |
| `subproject` | string | no | Subproject of `project`. An explicit `subproject` argument on the call wins over it. |
| `path` | string | no | Vault path for this workspace. `~`/`HOME`/`$HOME` prefixes are expanded; a **relative** path is resolved against the directory that holds the `.sync82.json`, not the server's working directory. |

```json
{
  "project": "moodle",
  "subproject": "mod_quiz",
  "path": "../vaults/moodle.db"
}
```

A `.sync82.json` that exists but can't be parsed makes the call ask for `project` and say why — it never falls back to the last-used project. `init_project_memory` and `search_memory` return an error result instead, and `init_project_memory` never overwrites that file.

---

## Precedence

**Project:** explicit `project` argument → `.sync82.json` at `workspace_root` → `lastProject` in `config.json` (only when neither `project` nor `workspace_root` is given) → the tool asks for one.

**Vault path:**

1. `path` argument on the tool call (`--path` on `export`/`import`).
2. `path` in the resolved `.sync82.json`.
3. `vaultPath` in `config.json` (`sync82 config set-vault`).
4. `SYNC82_DB_PATH`.
5. `~/.sync82/knowledge.db`.

When the project itself came from `lastProject`, its `lastVaultPath` is used instead of steps 2–5. Full rules and diagrams: [Context Resolution](../architecture/context-resolution.md).

---

## Transport

sync82 serves MCP over **stdio only**: running `sync82` with no arguments starts the server. There is no HTTP, SSE or Streamable HTTP transport and no flag or variable to enable one.

The server supports MCP protocol revisions `2024-11-05` through `2026-07-28` and negotiates the newest one the client also supports. A single incoming JSON-RPC message may be at most **64 MiB**, enough for the largest valid tool call (10 MB of content) even after JSON escaping; a larger message closes the connection.

---

## Claude Desktop bundle (`sync82.mcpb`)

The `sync82.mcpb` bundle declares one optional user setting, shown by Claude Desktop in the extension's settings:

| Key | Title | Type | Default | Effect |
|---|---|---|---|---|
| `db_path` | Vault database path | string | `~/.sync82/knowledge.db` | Passed to the server as `SYNC82_DB_PATH`. A leading `~` is expanded; the file is created if missing. |

Because it is delivered as `SYNC82_DB_PATH`, a `vaultPath` set with `sync82 config set-vault` and a workspace's `.sync82.json` still take precedence over it.

---

## See also

- [CLI Reference — config](./cli.md#config) — `set-vault`, `get-vault`, `unset-vault`
- [Context Resolution](../architecture/context-resolution.md) — how these sources combine
- [Tools Reference](./tools.md) — the `path`, `workspace_root` and `search_parent_dirs` arguments
