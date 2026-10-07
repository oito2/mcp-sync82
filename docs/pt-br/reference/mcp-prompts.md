🌐 [English](../../en/reference/mcp-prompts.md) | **Português** | 🏠 [Índice](../index.md)

---

# Referência de Prompts MCP

O sync82 expõe dois **prompts** MCP — instruções prontas que um cliente oferece ao usuário, em geral como comando de barra ou item de menu. Um prompt não lê nem grava nada por conta própria: ele produz uma mensagem de usuário que diz ao agente quais tools do sync82 chamar. Não confunda com os [Prompts de Exemplo](../prompts.md), que listam pedidos que você pode digitar para o seu agente.

Os dois prompts recebem os mesmos argumentos opcionais:

| Argumento | Obrigatório | Descrição |
|---|---|---|
| `project` | ❌ | Nome do projeto, em minúsculas. Sem ele, a mensagem diz ao agente para passar a pasta do workspace atual como `workspace_root`, e o projeto vem do `.sync82.json` do workspace. |
| `subproject` | ❌ | Nome do subprojeto. Exige `project`. |

Um nome inválido, ou `subproject` sem `project`, recebe um erro *invalid params* (`-32602`). Clientes que suportam o autocompletar do MCP podem sugerir os nomes dos projetos e subprojetos do vault padrão nos dois argumentos — veja [Resources — Autocompletar](./resources.md#autocompletar).

## `start_session`

**Começar uma sessão com a memória do projeto.** A mensagem pede ao agente para chamar o [`load_project_context`](./tools.md#load_project_context) do projeto e depois resumir o que é o projeto, em que ponto está, as decisões e o progresso mais recentes e os próximos passos — sem carregar o histórico mais antigo, a menos que o usuário peça.

## `end_session`

**Salvar esta sessão na memória do projeto.** A mensagem pede ao agente uma única chamada ao [`update_project_memory`](./tools.md#update_project_memory) do projeto com:

- `progress` — o que a sessão fez, sob um cabeçalho `## YYYY-MM-DD` com a data de hoje;
- `decisions` — cada decisão e o motivo dela, se houver;
- `next_steps` — a lista completa e atualizada do que está pendente;
- `memory`, `architecture` ou `stack` — só se mudaram.

Também pede ao agente para corrigir com o [`edit_entry`](./tools.md#edit_entry) uma entrada que esteja errada, em vez de acrescentar outra que a contradiga, e para contar o que salvou.

## Veja também

- [Referência de Resources](./resources.md) — os resources que o sync82 expõe.
- [Prompts de Exemplo](../prompts.md) — pedidos em linguagem natural que usam o sync82.
