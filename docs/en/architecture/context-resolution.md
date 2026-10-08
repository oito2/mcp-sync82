🌐 [Português](../../pt-br/architecture/context-resolution.md) | **English** | 🏠 [Index](../index.md)

---

# Context Resolution

The 4-tier model sync82 uses to determine which project a tool call refers to, and which vault it operates against.

---

## Project resolution

Most tools take optional `project`/`subproject`/`workspace_root` arguments rather than requiring `project` on every call. When `project` isn't given directly, it's resolved through four tiers, in order:

1. **Explicit argument** — `project` (and `subproject`) passed directly in the tool call.
2. **Local config** — `.sync82.json` at `workspace_root` itself, if given. Written by `init_project_memory` when it's first run with a `workspace_root`, so future calls in that workspace resolve automatically. By default this tier only ever checks `workspace_root` exactly — it does **not** walk up into parent directories, because a `.sync82.json` found in an ancestor the caller doesn't control could silently redirect where memory is stored (its `path` field is trusted without confirmation). Pass `search_parent_dirs: true` to opt into walking up (useful in monorepos where the marker file lives at the repo root) — up to 64 parent directories, or the filesystem root, whichever comes first. **When `workspace_root` is given, resolution stops at this tier:** a missing, empty (no `project`) or unreadable `.sync82.json` never falls back to tier 3 — the tool asks for `project` instead, and says why when the file couldn't be parsed. Another session's last project is never used for a workspace that names itself. A `workspace_root` of only spaces is refused, not read as absent.
3. **Global config** — only when neither `project` nor `workspace_root` is given: the last project/subproject used, and the vault it lives in, recorded in `~/.sync82/config.json`. It's recorded after a call that named a project which **exists** in its vault (a typo never becomes the default), and used in that same vault. A `path` that names another vault doesn't reuse it: the call asks for `project` instead, since the remembered name says nothing about that vault's projects. Recording it is best-effort — a failure to persist it never fails the tool call itself. `rename_project` updates it when it renames the remembered project, and `import_memory`, `delete_memory`, `archive_memory`, `edit_entry` and `write_memory` refuse to act on it, since they overwrite or remove data — as does `update_project_memory` when any of its fields overwrites (appends alone may use it), and `init_project_memory` when the call carries answers or `auto_detect`.
4. **Ask** — if none of the above resolve, the tool returns a message (not an error) asking the calling agent to supply `project` or `workspace_root`.

An explicit `subproject` argument is kept in every tier: `workspace_root` resolving to `acme` plus `subproject: "api"` targets `acme/api`.

```
project given directly?
   │
   ├─ yes ──────────────────────────────► use it
   │
   no
   │
   ▼
workspace_root given?
   │
   ├─ yes ─► .sync82.json with a project found there
   │         (or, with search_parent_dirs: true, in a parent directory)?
   │            ├─ yes ─────────────────► use it
   │            └─ no ──────────────────► ask (never tier 3)
   │
   no
   │
   ▼
~/.sync82/config.json has a last-used project?
   │
   ├─ yes ──────────────────────────────► use it
   │
   no
   │
   ▼
return a message asking the agent for `project` or `workspace_root`
```

Whenever a project is auto-discovered via tier 2 or tier 3 (not passed explicitly), the tool response's `ContextNote` suffix reports both the project and the resolved vault path — e.g. `[project: acme, from .sync82.json, vault: /home/user/.sync82/knowledge.db]` — so a custom `path` applied along the way is never silently invisible.

### The `search_memory` exception

`search_memory` is the one deliberate exception: it never falls through to tier 3 on its own. An unscoped search is meant to search the whole vault, not silently guess "the last project you were working on" — so it only uses local-config auto-discovery (tier 2) when `workspace_root` is explicitly given, and never falls back to the global last-used project by itself. A workspace without `.sync82.json` searches the whole vault; one whose `.sync82.json` can't be read or names an invalid project gives an error result.

---

## Vault path resolution

The vault **path** itself resolves independently of the project, in this priority order:

1. An explicit `path` argument on the tool call.
2. A custom path recorded in the resolved `.sync82.json`.
3. A custom path recorded in the global config (`sync82 config set-vault`).
4. The `SYNC82_DB_PATH` environment variable, or the hardcoded default (`~/.sync82/knowledge.db`).

This is a separate resolution from the project one above — an explicit `project` or a `.sync82.json` resolves the same way whatever vault path applies. The one link between them is tier 3: a project resolved from the global config is opened in the vault it was remembered with, and is only resolved for a `path` naming that same vault.

A relative `path` in `.sync82.json` is resolved against the directory that holds that `.sync82.json`, not the server's working directory. `sync82 config set-vault` stores the path as absolute.

---

## See also

- [Configuration Reference](../reference/configuration.md) — every field of `config.json` and `.sync82.json`
- [Storage](./storage.md) — what actually lives in the resolved vault
- [Installer](./installer.md) — how each client is pointed at sync82 in the first place
- [Tools Reference](../reference/tools.md) — the `project`/`subproject`/`workspace_root`/`search_parent_dirs`/`path` arguments on every tool

---

[🏠 Back to Index](../index.md)
