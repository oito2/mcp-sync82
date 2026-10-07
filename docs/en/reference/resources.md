🌐 [Português](../../pt-br/reference/resources.md) | **English** | 🏠 [Index](../index.md)

---

# Resources Reference

Besides its [tools](./tools.md), sync82 exposes project memory as read-only MCP **resources**: Markdown documents a client can attach to a conversation or read without the agent calling a tool. Resources never write to the vault, never create it, and never change the "last used project" that tools fall back to.

## Which vault

Resources always read the **default vault** — the one a tool call uses when it gives no `path` and no `.sync82.json` applies: the vault set with `sync82 config set-vault`, else the `SYNC82_DB_PATH` environment variable, else `~/.sync82/knowledge.db` (see [Configuration](./configuration.md)). A workspace's `.sync82.json` is not consulted, since a resource request carries no workspace. Projects kept in another vault are not available as resources; use the tools with `path` or `workspace_root` for them.

## URI templates

| URI template | Content |
|---|---|
| `sync82://projects/{project}/context` | The project's memory in one block, exactly what [`load_project_context`](./tools.md#load_project_context) returns with no arguments: current-state files in full, the 10 most recent entries of each log, cut at 40 KB, with the omitted-history footer. |
| `sync82://projects/{project}/files/{file}` | One memory file (`memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps` or a custom one), exactly what [`read_memory`](./tools.md#read_memory) returns. |
| `sync82://projects/{project}/subprojects/{subproject}/context` | The same as the project context, for a subproject. |
| `sync82://projects/{project}/subprojects/{subproject}/files/{file}` | The same as a project file, for a subproject. |

Every resource has the MIME type `text/markdown`. Names follow the same rules as the tools' `project`, `subproject` and `filename` arguments (letters, digits, `-` and `_`, starting with a letter or digit) and are case-insensitive: `sync82://projects/Acme/files/Memory` reads `acme`'s `memory`. Anything else — an unknown name, an extra or empty path segment, `..`, percent-encoding, a query or a fragment — is answered with the MCP *Resource not found* error (`-32602`), the same answer as for a project or file that doesn't exist.

## Listing

`resources/list` returns the `context` resource of every project and subproject in the default vault, read when the list is requested, so a project created during the session shows up the next time the client lists resources. Files are not listed one by one; read them through the `files/{file}` template. An empty or missing vault lists nothing.

When a tool call creates, renames or deletes a project (`create_project`, `rename_project`, `delete_project`, `init_project_memory`, `import_memory`), sync82 sends `notifications/resources/list_changed`, so a client that honors it can list the resources again; when it does is up to the client — some keep the list they already have until later. sync82 does not send `notifications/resources/updated`: a client sees new content when it reads a resource again.

## Completion

The `{project}`, `{subproject}` and `{file}` variables of the templates support MCP completion (`completion/complete`): a client that asks gets, for what was typed so far (case-insensitive prefix), the projects of the default vault, the subprojects of the chosen project, or that project's files plus the six standard ones — at most 100 values, with the total. The same completion serves the `project` and `subproject` arguments of the [MCP prompts](./mcp-prompts.md).

## See also

- [MCP Prompts Reference](./mcp-prompts.md) — the prompts sync82 exposes.
- [Tools Reference](./tools.md) — the tools behind each resource.
