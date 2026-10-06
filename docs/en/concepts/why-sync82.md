🌐 [Português](../../pt-br/concepts/why-sync82.md) | **English** | 🏠 [Index](../index.md)

---

# Why use sync82?

AI assistants are powerful — but they don't remember anything between sessions. This document explains the problem `sync82` solves and when it makes sense to use it.

---

## The problem: AI without persistent memory

Every new chat with an AI coding assistant starts from zero. It doesn't know what you decided last week, why you chose SQLite over a flat-file store, or what's left on the roadmap. In practice, this means:

**❌ Re-explaining context every session**
You re-paste the same architecture summary, re-list the same conventions, and re-describe the same in-progress work, over and over.

**❌ Lost decisions and their reasoning**
"We tried X, it didn't work because Y" is exactly the kind of thing that gets forgotten and re-litigated in a future session — usually by re-trying X.

**❌ No continuity across a session boundary**
If a session ends mid-task, the next one has no idea where it left off unless you wrote it down somewhere yourself.

**❌ No shared memory across multiple AI clients**
If you use Claude Code for one task and Codex for another, they don't share anything — each starts cold.

---

## The solution: a persistent, structured, per-project vault

`sync82` solves this by giving the AI six standard memory files per project — overview, architecture, stack, decisions, progress, next steps — stored in a local SQLite vault and exposed via MCP tools any compatible client can call.

**✅ The AI picks up exactly where it left off**
`load_project_context` concatenates every non-blank memory file into one block — the agent starts the session already knowing the project.

**✅ Decisions keep their reasoning**
`decisions` is append-only and dated — each entry is a permanent record of what was decided and why, never silently overwritten.

**✅ One vault, every client**
The vault is just a SQLite file on disk. Claude Code, Codex, OpenCode, and Antigravity can all point at the same one — memory built in one client is visible in another.

**✅ Multi-component projects are a first-class case**
`project`/`subproject` models a parent project with independently-tracked components (a monorepo, a plugin ecosystem) without duplicating shared context.

**✅ Zero configuration for day-to-day use**
Once a workspace is initialized with `init_project_memory workspace_root=...`, every future tool call in that directory auto-resolves the right project — no `project` argument needed.

**✅ A single static binary, nothing to install alongside it**
No runtime, no `node_modules`, no version manager — download it, or `go install` it, and it just runs. It even updates itself (`sync82 self-update`), all without ever making an LLM API call itself.

---

## Direct comparison

| Situation | Without sync82 | With sync82 |
| --- | --- | --- |
| New session | You re-explain the project from scratch | `load_project_context` restores everything |
| A past decision | You try to remember, or search old chat logs | `search_memory` finds it, with the original reasoning |
| Switching AI clients | Context doesn't transfer | Same vault, any MCP client |
| Monorepo / plugin project | You describe each component every time | `project`/`subproject` tracks each independently |
| End of session | You hope you'll remember to write notes | `update_project_memory` saves it in one call |
| Runtime dependency | — | None — single static binary, no API key required |

---

## When to use

`sync82` is ideal for:

| Scenario | Why the server helps |
| --- | --- |
| **Long-running projects across many sessions** | Architecture, stack, and status persist automatically |
| **Teams or solo devs switching between AI tools** | One vault, shared across every MCP client |
| **Monorepos and plugin ecosystems** | `project`/`subproject` avoids re-describing shared context |
| **Auditable decision history** | `decisions` never loses an entry — append-only, dated |
| **Onboarding into an existing codebase** | `init_project_memory auto_detect:true` bootstraps from the code itself |

---

## When not to use

`sync82` **is not the right tool** for:

- **Short-lived, single-session tasks** — the overhead of initializing project memory isn't worth it for something you'll never revisit.
- **Storing secrets or credentials** — it's a plain SQLite file with no encryption at rest; treat the vault path like any other local config file.
- **Replacing version control** — `progress`/`decisions` record *why* things happened, not the code itself; that's what git is for.

---

## Additional reading

- [What is MCP?](./what-is-mcp.md)
- [How sync82 works](./how-sync82-works.md)
- [Glossary](./glossary.md)

---

[🏠 Back to Index](../index.md)
