🌐 [Português](../../pt-br/reference/mcp-prompts.md) | **English** | 🏠 [Index](../index.md)

---

# MCP Prompts Reference

sync82 exposes two MCP **prompts** — ready-made instructions a client offers the user, usually as a slash command or a menu entry. A prompt doesn't read or write anything itself: it produces one user message that tells the agent which sync82 tools to call. Not to be confused with [Example Prompts](../prompts.md), which lists things you can type to your agent.

Both prompts take the same optional arguments:

| Argument | Required | Description |
|---|---|---|
| `project` | ❌ | Project name, lower-cased. Without it, the message tells the agent to pass the current workspace folder as `workspace_root`, so the project comes from the workspace's `.sync82.json`. |
| `subproject` | ❌ | Subproject name. Requires `project`. |

An invalid name, or `subproject` without `project`, is answered with an *invalid params* error (`-32602`). Clients that support MCP completion can suggest the names of the default vault's projects and subprojects for both arguments — see [Resources — Completion](./resources.md#completion).

## `start_session`

**Start a session with the project memory.** The message asks the agent to call [`load_project_context`](./tools.md#load_project_context) for the project and then summarize what the project is, where it stands, the most recent decisions and progress, and the next steps — without loading older history unless the user asks.

## `end_session`

**Save this session to the project memory.** The message asks the agent to make one [`update_project_memory`](./tools.md#update_project_memory) call for the project with:

- `progress` — what the session did, under a `## YYYY-MM-DD` header with today's date;
- `decisions` — each decision and its reasoning, if any;
- `next_steps` — the full, updated list of what is pending;
- `memory`, `architecture` or `stack` — only if they changed.

It also asks the agent to fix an entry that is now wrong with [`edit_entry`](./tools.md#edit_entry) instead of adding a contradicting one, and to report what it saved.

## See also

- [Resources Reference](./resources.md) — the resources sync82 exposes.
- [Example Prompts](../prompts.md) — natural-language requests that use sync82.
