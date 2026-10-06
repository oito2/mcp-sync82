🌐 [Português](../../pt-br/concepts/what-is-mcp.md) | **English** | 🏠 [Index](../index.md)

---

# What is MCP?

**Model Context Protocol (MCP)** is an open standard created by Anthropic that defines how AI assistants communicate with external tools and data sources. Think of it as a universal adapter: any MCP-compatible AI client can connect to any MCP server using the same protocol.

---

## The problem MCP solves

Without MCP, each AI integration is built in a custom way:

- Cursor has its own plugin system
- VS Code has its own extension API
- Claude Desktop has its own tool format

With MCP, you build **a single server** and any compatible client can use it.

---

## How MCP works

```
AI Client (Claude Code, Codex, OpenCode...)
       │
       │  JSON-RPC 2.0 via stdio
       │
  MCP Server (sync82)
       │
       │  reads/writes SQLite
       │
  Your memory vault (~/.sync82/knowledge.db)
```

An MCP server can expose three types of capabilities — **Tools**, **Resources**, and **Prompts**. sync82 only exposes **Tools**: 16 functions the AI explicitly calls (`init_project_memory`, `search_memory`, `update_project_memory`, ...). It doesn't register any MCP Resources or Prompts.

| Capability | Description | Used by sync82? |
| --- | --- | --- |
| **Tools** | Functions the AI can call | ✅ Yes — all 16 |
| **Resources** | URIs that expose data, read passively | ❌ No |
| **Prompts** | Prebuilt prompt templates | ❌ No |

---

## MCP transports

MCP supports two transport mechanisms: **stdio** (the server runs as a subprocess of the AI client) and **Streamable HTTP** (the server runs as an independent network service).

sync82 supports **stdio only** — running the binary with no arguments starts it over stdio, which is also what every `sync82 install` target configures. There's no `--http` flag: the design goal is a server that starts instantly with zero network calls, and per-project SQLite vaults are meant to live on the same machine as the agent using them, not be shared over a network.

---

## Additional reading

- [Model Context Protocol specification](https://modelcontextprotocol.io)
- [Why sync82?](./why-sync82.md)
- [How sync82 works](./how-sync82-works.md)
- [Glossary](./glossary.md)

---

[🏠 Back to Index](../index.md)
