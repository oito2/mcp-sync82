🌐 [Português](../../../pt-br/guides/workflows/examples.md) | **English** | 🏠 [Index](../../index.md)

---

# Usage Examples

Real-world use cases with ready-to-use prompts, step sequences, and expected outcomes. Use these examples as a starting point for your own sessions.

---

## 1. Full Workflow: A Project's First Week

This example shows a project's memory taking shape over three separate sessions.

**Session 1 — Bootstrap:**

```
Initialize memory for this project. Analyze the codebase automatically.
```

`init_project_memory` runs with `auto_detect: true` and `workspace_root` set — it reads `README.md`/`go.mod`/etc., writes `memory`, `architecture`, and `stack`, and saves `.sync82.json` so future sessions in this directory need no `project` argument.

**Session 2 — Recording a decision:**

```
We're switching from REST to gRPC for the internal service calls —
lower latency and we already generate protobufs for the client SDK.
Save that decision.
```

`update_project_memory` appends a dated `decisions` entry. Nothing else is touched.

**Session 3 — Wrapping up before a break:**

```
Load the context first. ... [work happens] ... Save what we did: migrated
the auth service to gRPC. Next steps are the billing service, then load
testing.
```

`load_project_context` restores everything from sessions 1–2; `update_project_memory` appends `progress` and overwrites `next_steps` with the new list.

**What the server does at each step:**
- Session 1: `init_project_memory` — writes 3 overwrite-style files, saves `.sync82.json`
- Session 2: `update_project_memory` — appends one dated `decisions` entry
- Session 3: `load_project_context` (read) then `update_project_memory` (write `progress` + `next_steps`)

---

## 2. Multi-Component Project (Monorepo / Plugin Ecosystem)

**Scenario:** You maintain a Moodle installation and are developing two plugins for it. `project`/`subproject` was built for exactly this — one parent project for shared context, independently-tracked components underneath.

**Step 1 — Set up the parent:**

```
Initialize memory for this project, it's called "moodle".
```

**Step 2 — Add the first plugin as a subproject:**

```
This is a plugin for the moodle project. Set up memory for it as a
subproject called mod_quiz.
```

`init_project_memory` runs with `project: "moodle"`, `subproject: "mod_quiz"` — creates the subproject if it doesn't exist yet.

**Step 3 — Add a second plugin:**

```
Same thing for the block_coursestats plugin — subproject of moodle.
```

**Step 4 — Search across all of them:**

```
Search everywhere in the vault for how we've handled capability checks
before — I don't remember if it was in this plugin or another one.
```

`search_memory` with no `project` given deliberately searches the whole vault, not just the plugin you're currently in.

**Step 5 — Check the whole tree:**

```
What plugins do we have tracked for Moodle?
```

`list_projects` returns the full project/subproject tree.

---

## 3. Debugging With Historical Context

**Scenario:** A bug report comes in for behavior you're pretty sure was discussed before.

```
Load the architecture and decisions for this project. Then help me
figure out why the cache invalidation isn't triggering on a config
reload — I have a feeling we made a deliberate choice about this.
```

The AI calls `read_memory filename="architecture"` and `read_memory filename="decisions"` (or `load_project_context` for everything at once), then reasons about the bug with that history in hand instead of guessing from the code alone.

---

## 4. Session Handoff Between AI Clients

**Scenario:** You used Claude Code this morning, and want to continue with Codex this afternoon — same vault, same project, no context lost.

Nothing special required — as long as both clients are configured against the same vault (the default `~/.sync82/knowledge.db`, or the same `path` override), the memory is already shared. In the new client:

```
Load the project context.
```

If it doesn't resolve automatically (e.g. Codex is running from a different working directory than Claude Code was), give it the project explicitly:

```
Load the project context for "moodle", subproject "mod_quiz".
```

---

## 5. Periodic Maintenance

**Scenario:** A long-running project's `progress` log has gotten long enough that it's eating context budget.

```
Is this project's memory healthy? Archive any progress entries older
than 6 months.
```

The AI calls `check_project_health` first, then `archive_memory filename="progress" keep_days=180`. Entries without a date header are never archived, by design.

```
Export this project's memory to a folder I can commit to git as a backup.
```

`export_memory` writes one plain `.md` file per kind into the directory you specify, plus a small `.sync82-kinds.json` manifest that lets an import restore custom logs as logs — useful for diffing memory changes in a PR, or as a portable backup outside the SQLite vault.

---

## 💡 General Tips

**Be specific about project vs. subproject** when it's ambiguous — words like "plugin", "module", "component", "service" are hints `init_project_memory`'s instructions look for, but naming it directly avoids guesswork.

**`update_project_memory` is the workhorse** for ordinary end-of-session saves — reach for `write_memory`/`append_memory` directly only when doing something more surgical (rewriting a whole file, or appending outside the normal session-save flow).

**A vague "remember this" works fine** — tool descriptions are written to guide the agent's judgment about which file a piece of information belongs in, not to require you to know the schema.

---

## ➡️ Next Steps

- [Tools Reference](../../reference/tools.md) — complete parameters for all tools
- [Example Prompts](../../prompts.md) — more prompts, organized by category
- [Common Issues](../../troubleshooting/common-issues.md) — when something doesn't work as expected
- [Back to Index](../../index.md)
