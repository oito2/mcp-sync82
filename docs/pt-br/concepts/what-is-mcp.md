🌐 [English](../../en/concepts/what-is-mcp.md) | **Português** | 🏠 [Índice](../index.md)

---

# O que é MCP?

**Model Context Protocol (MCP)** é um padrão aberto criado pela Anthropic que define como assistentes de IA se comunicam com ferramentas e fontes de dados externas. Pense nele como um adaptador universal: qualquer cliente de IA compatível com MCP pode se conectar a qualquer servidor MCP usando o mesmo protocolo.

---

## O problema que o MCP resolve

Sem o MCP, cada integração de IA é construída de forma customizada:

- O Cursor tem seu próprio sistema de plugins
- O VS Code tem sua própria API de extensões
- O Claude Desktop tem seu próprio formato de tool

Com o MCP, você constrói **um único servidor** e qualquer cliente compatível pode usá-lo.

---

## Como o MCP funciona

```
Cliente de IA (Claude Code, Codex, OpenCode...)
       │
       │  JSON-RPC 2.0 via stdio
       │
  Servidor MCP (sync82)
       │
       │  lê/escreve SQLite
       │
  Seu vault de memória (~/.sync82/knowledge.db)
```

Um servidor MCP pode expor três tipos de capacidades — **Tools**, **Resources** e **Prompts**. O sync82 expõe os três: 19 **Tools** que a IA chama explicitamente (`init_project_memory`, `search_memory`, `update_project_memory`, ...), **Resources** somente leitura com a memória de cada projeto e dois **Prompts** que começam e encerram uma sessão.

| Capacidade | Descrição | Usado pelo sync82? |
| --- | --- | --- |
| **Tools** | Funções que a IA pode chamar | ✅ Sim — todas as 19 ([referência](../reference/tools.md)) |
| **Resources** | URIs que expõem dados, lidos passivamente | ✅ Sim — `sync82://projects/...` ([referência](../reference/resources.md)) |
| **Prompts** | Templates de prompt prontos | ✅ Sim — `start_session`, `end_session` ([referência](../reference/mcp-prompts.md)) |

---

## Transportes do MCP

O MCP suporta dois mecanismos de transporte: **stdio** (o servidor roda como um subprocesso do cliente de IA) e **Streamable HTTP** (o servidor roda como um serviço de rede independente).

O sync82 suporta **apenas stdio** — rodar o binário sem argumentos inicia ele via stdio, que é também o que todo target do `sync82 install` configura. Não existe uma flag `--http`: o objetivo de design é um servidor que inicia instantaneamente com zero chamadas de rede, e vaults SQLite por projeto são pensados para viver na mesma máquina do agente que os usa, não para serem compartilhados pela rede.

---

## Leitura adicional

- [Especificação do Model Context Protocol](https://modelcontextprotocol.io)
- [Por que sync82?](./why-sync82.md)
- [Como o sync82 funciona](./how-sync82-works.md)
- [Glossário](./glossary.md)

---

[🏠 Voltar ao Índice](../index.md)
