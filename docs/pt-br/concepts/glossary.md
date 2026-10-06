🌐 [English](../../en/concepts/glossary.md) | **Português** | 🏠 [Índice](../index.md)

---

# Glossário

Termos técnicos usados na documentação do `sync82`, organizados alfabeticamente.

---

## `~/.sync82/config.json`

Arquivo de configuração global. Armazena um override opcional de caminho de vault customizado (definido via `sync82 config set-vault`) e o último projeto/subprojeto que uma chamada de tool nomeou e que existe no seu vault, junto com o caminho desse vault — camada 3 da [resolução de contexto](../architecture/context-resolution.md).

→ Veja: [Resolução de Contexto](../architecture/context-resolution.md)

---

## `.sync82.json`

Arquivo de configuração local, por workspace. Escrito pelo `init_project_memory` quando chamado com um `workspace_root`, para que futuras chamadas de tool feitas a partir daquele mesmo diretório resolvam automaticamente o projeto certo sem repetir `project`/`subproject` toda vez. Por padrão só `workspace_root` em si é checado; passe `search_parent_dirs: true` para também descobrir um em um diretório ancestral (ex.: a raiz de um monorepo).

→ Veja: [Resolução de Contexto](../architecture/context-resolution.md)

---

## Kind append-only

Um kind de memória armazenado na tabela `entries`, onde cada escrita adiciona uma nova linha datada em vez de substituir o conteúdo existente. Os dois kinds append-only padrão são `progress` e `decisions`; um kind customizado também pode ser append-only dependendo de qual tool (`append_memory` vs. `write_memory`) o criou primeiro.

→ Veja: [Storage](../architecture/storage.md) · [Referência de Tools](../reference/tools.md)

---

## Resolução de contexto

O processo de 4 camadas que o sync82 usa para determinar a qual projeto (e subprojeto) uma chamada de tool se refere quando `project` não é passado diretamente: argumento explícito → `.sync82.json` local → "último projeto usado" global → pergunta ao agente. `search_memory` é a única exceção deliberada — ela nunca recorre à camada global sozinha.

→ Veja: [Resolução de Contexto](../architecture/context-resolution.md)

---

## Kind customizado

Qualquer nome de arquivo de memória além dos seis padrão (`memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps`). Criado na primeira vez que `write_memory` ou `append_memory` é chamado com um novo `filename` — qual tool o cria determina se ele é do tipo sobrescrita ou append. Diferente dos seis kinds padrão, kinds customizados podem ser removidos com `delete_memory`.

---

## Kind

Termo do sync82 para um arquivo de memória nomeado dentro de um projeto — `memory`, `progress`, `architecture`, ou qualquer nome customizado. Toda tool que lê ou escreve memória recebe um argumento `filename` identificando o kind.

---

## MCP (Model Context Protocol)

Padrão aberto criado pela Anthropic que define como assistentes de IA se comunicam com ferramentas e fontes de dados externas. Permite que um único servidor seja usado por qualquer cliente compatível — Claude Code, OpenAI Codex, OpenCode, Antigravity, e outros.

O `sync82` é um servidor MCP especializado em memória persistente de projeto, escrito em Go e distribuído como um único binário estático.

→ Veja: [O que é MCP?](./what-is-mcp.md)

---

## Kind de sobrescrita

Um kind de memória armazenado na tabela `documents`, onde cada escrita substitui todo o conteúdo no lugar — nenhum histórico de revisão é mantido. Os quatro kinds de sobrescrita padrão são `memory`, `architecture`, `stack` e `next_steps`.

→ Veja: [Storage](../architecture/storage.md)

---

## Projeto / Subprojeto

Um **projeto** é uma entrada de nível superior no vault. Um **subprojeto** pertence a exatamente um projeto pai — apenas um nível de aninhamento é suportado. Isso modela um monorepo ou ecossistema de plugins: um projeto pai para contexto compartilhado, um subprojeto por componente rastreado independentemente.

→ Veja: [Storage](../architecture/storage.md)

---

## stdio

O único modo de transporte MCP que o sync82 suporta. O servidor roda como um subprocesso do cliente de IA, comunicando via entrada e saída padrão (stdin/stdout). Não abre portas de rede — rodar `sync82` sem flags inicia esse modo.

→ Veja: [O que é MCP?](./what-is-mcp.md#transportes-do-mcp)

---

## Tool (MCP)

Uma função executável exposta pelo servidor MCP que o assistente de IA pode chamar explicitamente. O sync82 expõe 18 tools, agrupadas por etapa do fluxo de trabalho:

| Grupo | Tools |
| --- | --- |
| Gerenciamento de projeto | `list_projects`, `create_project`, `delete_project`, `rename_project`, `get_vault_config` |
| Leitura/escrita de memória | `list_files`, `read_memory`, `write_memory`, `append_memory`, `delete_memory`, `archive_memory`, `search_memory` |
| Fluxo de sessão | `load_project_context`, `check_project_health`, `init_project_memory`, `update_project_memory` |
| Exportação e importação | `export_memory`, `import_memory` |

→ Veja: [Referência de Tools](../reference/tools.md)

---

## Vault

O único arquivo de banco de dados SQLite que armazena tudo — todo projeto, subprojeto e arquivo de memória. Localização padrão `~/.sync82/knowledge.db`. Um vault pode conter qualquer número de projetos não relacionados; não há exigência de usar um vault por projeto.

---

[🏠 Voltar ao Índice](../index.md)
