🌐 [Português](../../pt-br/concepts/architecture.md) | **English** | 🏠 [Index](../index.md)

---

# Architecture

Overview of the components of `sync82`, the data flow, and the storage model.

---

## Project structure

```
mcp-sync82/
├── go.mod, go.sum        ← module definition and dependencies
├── CONTRIBUTING.md        ← dev setup, CI checks, PR process
├── docs/                 ← full documentation (en/ and pt-br/), images and icons (img/)
├── .github/workflows/    ← ci.yml (checks + release contract) and release.yml (tag → release → MCP Registry)
├── scripts/release/      ← the release builder: `go run ./scripts/release vX.Y.Z` writes dist/
├── cmd/sync82/          ← main.go — CLI dispatch: serve (default), install, uninstall, config, self-update, export, import
└── internal/
    ├── server/           ← MCP server construction, tool registration, server icon, JSON-RPC error handling
    ├── tools/            ← one file per tool — the 18 tools documented in reference/tools.md; registry.go lists them (tools.Registered)
    ├── store/            ← SQLite persistence: projects, documents, entries, schema_migrations
    ├── config/           ← global config (~/.sync82/config.json) and local config (.sync82.json) discovery
    ├── analyzer/         ← stack/description auto-detection, used by init_project_memory's auto_detect
    ├── installer/        ← the 8-target MCP client installer and uninstaller (incl. --purge)
    ├── selfupdate/       ← self-update: GitHub Releases check, checksum verification, binary replacement
    ├── cli/               ← the `config set-vault|get-vault|unset-vault` subcommand
    ├── fsutil/            ← shared atomic-file-write helper
    ├── binpath/           ← absolute, symlink-resolved path of the running binary (install, self-update)
    ├── version/           ← the version string (injected at release build time, or read from build info)
    ├── logging/           ← stderr-only logger
    └── */                 ← one `_test.go` suite per package (go test -race ./...)
```

---

## Data flow

```
1. AI client calls a tool (e.g. update_project_memory)
   └─ internal/tools/context.go resolves project/subproject/path
      through the 4-tier model (explicit arg > .sync82.json >
      global config > ask)

2. Tool calls internal/store
   └─ store.Manager caches one *Store per resolved vault path
   └─ overwrite-style kinds (memory, architecture, stack,
      next_steps) go to the documents table
   └─ append-only kinds (progress, decisions) go to the
      entries table, one row per dated entry

3. internal/server wraps the result
   └─ a Validate() failure becomes isError:true (agent sees a
      helpful message, connection stays healthy)
   └─ an Execute() failure becomes a JSON-RPC protocol error,
      with internal details (e.g. filesystem paths) stripped
      before it reaches the agent — logged server-side instead

4. Result returned to the AI client over stdio
   └─ listing tools called with format: "json" also return
      the same data as structuredContent
```

The server's tool list is a single function, `tools.Registered` in `internal/tools/registry.go`: `sync82` (serve) passes it to `internal/server`, and the release builder starts an in-memory server from the same list to write the tools into the `.mcpb` manifest — so the bundle never lists a tool the binary doesn't have.

---

## Storage model

Everything lives in a single embedded SQLite database file — one file is the whole vault. Default location `~/.sync82/knowledge.db`, overridable via the `SYNC82_DB_PATH` environment variable, `sync82 config set-vault`, a workspace's `.sync82.json`, or a tool call's `path` argument. Every connection sets `PRAGMA journal_mode=WAL` and `PRAGMA foreign_keys=ON`.

For the full schema (tables, indexes, and why they're shaped the way they are), see [Storage](../architecture/storage.md).

---

## Transport

sync82 supports **stdio only** — no HTTP/network transport, no flags to enable one. Running the binary with no arguments starts the MCP server over stdio; that's also the only mode every `sync82 install` target configures. This keeps startup instantaneous and avoids ever needing authentication or a listening port.

---

## Release pipeline

Releases are built by one Go program, `scripts/release`, and two GitHub Actions workflows:

```
go run ./scripts/release vX.Y.Z   → dist/
   ├─ sync82_{linux,darwin,windows}_{amd64,arm64}[.exe]   6 binaries (CGO_ENABLED=0, -trimpath)
   ├─ sync82.mcpb       Claude Desktop bundle (manifest 0.3): universal macOS binary,
   │                    Linux launcher picking amd64/arm64, Windows amd64 binary, icons,
   │                    optional user_config.db_path → SYNC82_DB_PATH
   ├─ checksums.txt     SHA-256 of the 6 binaries and sync82.mcpb
   └─ server.json       MCP Registry entry io.github.oito2/mcp-sync82 (points at the .mcpb)

ci.yml (push/PR to main)
   test (Linux, macOS, Windows) · release-contract: scripts/release -allow-prerelease v0.0.0-ci,
   self-update contract test, mcp-publisher validate

release.yml (tag vX.Y.Z)
   validate → checks → release → publish-registry
                         │          └─ mcp-publisher login github-oidc + publish
                         └─ scripts/release, mcp-publisher validate,
                            cosign sign-blob → checksums.txt.sigstore.json,
                            build provenance attestations (binaries + .mcpb),
                            gh release create
```

Asset names carry no version, so `releases/latest/download/<name>` always points at the newest release — the URLs the install instructions use. See [Contributing — Releasing](../../../CONTRIBUTING.md#releasing).

---

## See also

- [How sync82 works](./how-sync82-works.md) — the tool-call pipeline
- [Storage](../architecture/storage.md) — SQLite schema deep-dive
- [Context Resolution](../architecture/context-resolution.md) — the 4-tier resolution model
- [Installer](../architecture/installer.md) — how the 8 client targets are configured and removed
- [Configuration Reference](../reference/configuration.md) — every configuration source

---

[🏠 Back to Index](../index.md)
