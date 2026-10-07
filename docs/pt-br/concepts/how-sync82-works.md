🌐 [English](../../en/concepts/how-sync82-works.md) | **Português** | 🏠 [Índice](../index.md)

---

# Como o sync82 funciona

O `sync82` dá ao seu assistente de IA uma memória persistente e estruturada para um projeto — sem que você precise tocar em um banco de dados diretamente.

---

## O pipeline

O fluxo tem três estágios: a **resolução de contexto** determina a qual projeto (e qual vault) uma chamada de tool se refere, o **Store** lê/escreve no SQLite, e o **servidor MCP** expõe tudo isso ao cliente de IA como 19 tools via stdio.

```
Cliente de IA chama uma tool (ex. update_project_memory)
         │
         ▼  Resolução de contexto
    project/subproject/path resolvidos em 4 camadas:
    argumento explícito > .sync82.json > config global > pergunta
         │
         ▼  Store
    lê/escreve as linhas do projeto resolvido no SQLite
    (tabela documents para kinds do tipo overwrite,
     tabela entries para kinds append-only)
         │
         ▼  Servidor MCP
    retorna um CallToolResult via stdio ao cliente de IA
```

---

## Tools, por etapa do fluxo de trabalho

| Etapa | Tools |
| --- | --- |
| **Gerenciamento de projeto** | `list_projects`, `create_project`, `delete_project`, `rename_project`, `get_vault_config` |
| **Leitura e escrita de memória** | `list_files`, `read_memory`, `write_memory`, `append_memory`, `delete_memory`, `edit_entry`, `archive_memory`, `search_memory` |
| **Fluxo de sessão** | `load_project_context`, `check_project_health`, `init_project_memory`, `update_project_memory` |
| **Exportação e importação** | `export_memory`, `import_memory` |

`update_project_memory` e `load_project_context` são as duas tools que a maioria das sessões realmente usa — salvar no final, carregar no início. O resto existe para configuração, busca e manutenção.

Para as tabelas completas de parâmetros, veja a [Referência de Tools](../reference/tools.md).

---

## Arquivos de memória padrão

Todo projeto ganha seis kinds no `init_project_memory`:

| Kind | Estilo | O que vai nele |
| --- | --- | --- |
| `memory` | sobrescrita | Nome do projeto, descrição, status geral |
| `architecture` | sobrescrita | Componentes e como se relacionam |
| `stack` | sobrescrita | Linguagens, frameworks, infraestrutura |
| `decisions` | append, datado | Uma entrada por decisão, com o raciocínio |
| `progress` | append, datado | Uma entrada por sessão de trabalho concluída |
| `next_steps` | sobrescrita | A lista de tarefas atual |

Além desses seis, qualquer tool que aceite um argumento `filename` aceita um nome customizado arbitrário — útil para arquivos específicos do projeto que um template genérico não consegue antecipar.

---

## Resolução de contexto

A maioria das tools recebe argumentos opcionais `project`/`subproject`/`workspace_root` em vez de exigir `project` em toda chamada. O sync82 resolve o projeto real em quatro camadas — argumento explícito, `.sync82.json` local, "último projeto usado" global, ou perguntando ao agente — veja [Resolução de Contexto](../architecture/context-resolution.md) para o modelo completo, incluindo a única exceção deliberada (`search_memory` nunca cai silenciosamente no "último projeto usado").

---

## Veja também

- [Por que sync82?](./why-sync82.md)
- [Arquitetura](./architecture.md)
- [Referência de Tools](../reference/tools.md)
- [Resolução de Contexto](../architecture/context-resolution.md)

---

[🏠 Voltar ao Índice](../index.md)
