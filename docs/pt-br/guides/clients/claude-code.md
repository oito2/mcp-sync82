🌐 [English](../../../en/guides/clients/claude-code.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com Claude Code

O **Claude Code** é a CLI da Anthropic para desenvolvimento assistido por IA, com suporte nativo ao protocolo MCP via stdio.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

O jeito mais rápido — deixe o sync82 configurar para você:

```bash
sync82 install claude
```

Ele registra o sync82 no **escopo de usuário** (disponível em todos os projetos) com o caminho absoluto do binário — então rodar de novo é seguro, e é o que fazer depois de mover o binário. O comando `claude` precisa estar no `PATH` (caso contrário, imprime `Skipped: claude not detected.`).

Antes do `claude mcp add`, ele remove todo registro `sync82` existente: roda `claude mcp get sync82` e `claude mcp remove --scope <escopo> sync82` para o escopo informado (`local`, `user`), repetindo até não sobrar nenhum, e imprime `updated.` em vez de `configured.` quando removeu algum. Um registro em **escopo de projeto** (`.mcp.json`, compartilhado com o projeto) nunca é removido; ele imprime:

```text
  Warning: sync82 is also registered in project scope (.mcp.json, shared with the project) for <current directory>; it was left unchanged and, inside that project, it takes precedence over any user-scope registration. To remove it, run from that directory: claude mcp remove --scope project sync82
```

e então remove diretamente o registro em escopo de usuário. Se o escopo não puder ser lido do `claude mcp get`, ou um escopo continuar sendo informado depois de removido, o target falha — veja [Solução de Problemas](../../troubleshooting/common-issues.md#claude-could-not-determine-the-scope-of-the-sync82-registration).

Ou adicione manualmente, usando o caminho absoluto impresso por `which sync82`:

```bash
claude mcp add --scope user sync82 -- /usr/local/bin/sync82
```

O Claude vai criar ou atualizar `~/.claude.json`. A partir daí, toda vez que você iniciar `claude`, ele vai se conectar automaticamente ao servidor MCP em segundo plano.

### 2. Verifique a conexão

Dentro de uma sessão do Claude Code, rode:

```
/mcp
```

Você verá `sync82` listado como conectado com as 19 tools disponíveis. Se o servidor não aparecer, veja [Solução de Problemas](#solucao-de-problemas) abaixo.

### 3. Inicialize seu primeiro projeto

Na primeira sessão em um workspace, peça ao Claude:

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

O Claude vai chamar `init_project_memory` com `workspace_root` definido como seu diretório atual — isso também escreve `.sync82.json`, então toda sessão futura nesse workspace resolve automaticamente o projeto certo sem você precisar nomeá-lo de novo.

---

## 📄 Melhorando com CLAUDE.md

O Claude Code lê automaticamente o arquivo `CLAUDE.md` na raiz do projeto ao iniciar cada sessão. Como `load_project_context` ainda precisa ser chamado explicitamente, um lembrete curto no `CLAUDE.md` evita que você tenha que digitá-lo toda vez:

```markdown
# Contexto do Projeto

No início de uma sessão, carregue a memória do sync82 (`load_project_context`)
antes de fazer qualquer outra coisa. No final de uma sessão, salve o que
mudou com `update_project_memory`.
```

---

## 💡 Fluxos de Trabalho Recomendados

### Iniciando uma sessão

```
Carregue o contexto do projeto antes de começarmos.
```

O Claude chama `load_project_context` e ganha a memória registrada do projeto — visão geral, arquitetura, stack, próximos passos e as decisões e o progresso mais recentes — em uma única chamada.

### Registrando uma decisão no meio da sessão

```
Decidimos usar SQLite em vez de um armazenamento em arquivo plano
porque precisávamos de transações de verdade. Salve isso.
```

O Claude chama `update_project_memory` com um campo `decisions` — adicionado com a data de hoje, sem tocar nas decisões existentes.

### Encerrando uma sessão

```
Salve o que fizemos hoje.
```

O Claude resume a sessão e chama `update_project_memory`, geralmente com `progress` e, se algo mudou, `next_steps`.

### Buscando trabalho anterior

```
Já tratamos retries em algum lugar antes? Busque em todo o vault,
não só neste projeto.
```

O Claude chama `search_memory` sem `project` — uma busca sem escopo cobre o vault inteiro deliberadamente.

### Anexando a memória e usando os prompts

Os [resources](../../reference/resources.md) e os [prompts MCP](../../reference/mcp-prompts.md) do sync82 funcionam no Claude Code sem pedir ao agente que chame uma tool:

- Digite `@` e escolha um resource do sync82, ou escreva a URI dele depois do nome do servidor: `@sync82:sync82://projects/acme/context` anexa a memória de acme, `@sync82:sync82://projects/acme/files/decisions` as decisões dele.
- Digite `/` para achar os prompts, listados como `/sync82:start_session (MCP)` e `/sync82:end_session (MCP)`, ou rode-os como `/mcp__sync82__start_session acme`. Os argumentos vêm depois do comando, separados por espaço, nesta ordem: `project` e depois `subproject`. Sem argumentos, o agente acha o projeto pelo workspace atual.

O nome do servidor é o que você registrou (`sync82` com `sync82 install claude`).

### Projetos multi-componente

```
Este é um plugin para o projeto moodle que já estou rastreando.
Configure a memória dele como um subprojeto chamado mod_quiz.
```

O Claude chama `create_project` (ou `init_project_memory`) com `project: "moodle"`, `subproject: "mod_quiz"`. Veja [Exemplos de Uso](../workflows/examples.md) para o padrão completo.

---

## 🗑️ Removendo

```bash
sync82 uninstall claude
```

Remove os registros em escopo local e de usuário do mesmo jeito que o `install`, imprimindo `✓  claude — removed.`, ou `⚠  claude — nothing removed` seguido do aviso acima quando só foi encontrado um registro em escopo de projeto. Uma entrada em escopo de projeto (`.mcp.json`, compartilhada com o projeto) nunca é removida automaticamente — rode `claude mcp remove --scope project sync82` dentro de cada projeto que tiver uma. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

<a id="solucao-de-problemas"></a>

## ⚠️ Solução de Problemas

### Primeiro passo: verifique a conexão

Rode `/mcp` dentro da sessão do Claude. Se `sync82` não aparecer como conectado, o problema está na configuração do servidor, não no seu prompt.

### `sync82: command not found`

O Claude Code nem sempre herda o PATH do seu shell, então um registro que usa só o comando `sync82` pode falhar, assim como um que aponta para um caminho de onde o binário foi movido. Rode `sync82 install claude` de novo, ou substitua o registro manualmente com o caminho absoluto:

```bash
which sync82
# → /usr/local/bin/sync82

claude mcp remove --scope user sync82
claude mcp add --scope user sync82 -- /usr/local/bin/sync82
```

### Erros de permissão

```bash
ls -l $(which sync82)
chmod +x $(which sync82)
```

### Vault errado ou lista de projetos vazia

```
Mostre a configuração atual do vault.
```

O Claude chama `get_vault_config`, que reporta o caminho ativo do vault, a config global e (com `workspace_root`) o `.sync82.json` local — veja [Resolução de Contexto](../../architecture/context-resolution.md) se o caminho resolvido for inesperado.

---

## ➡️ Próximos Passos

- [Antigravity](./antigravity.md) — guia equivalente para a IDE e a CLI do Google
- [OpenAI Codex](./codex.md) — CLI da OpenAI com configuração TOML
- [OpenCode](./opencode.md) — agente open-source
- [Exemplos de Uso](../workflows/examples.md) — fluxos de trabalho reais
- [Referência de Tools](../../reference/tools.md) — parâmetros completos de todas as tools
- [Voltar ao Índice](../../index.md)
