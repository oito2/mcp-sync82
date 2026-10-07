🌐 [English](../../../en/guides/clients/codex.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com OpenAI Codex

O **OpenAI Codex** é o agente de CLI da OpenAI, configurado via um arquivo TOML.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

```bash
sync82 install codex
```

Ele registra o caminho absoluto do binário `sync82` que você executou; rodar de novo (ex. depois de mover o binário) substitui a entrada: ele roda `codex mcp get sync82` e, quando o sync82 está registrado, `codex mcp remove sync82` (imprimindo `updated.` em vez de `configured.`), depois `codex mcp add sync82 -- <caminho-absoluto>`. O comando `codex` precisa estar no `PATH` (caso contrário, imprime `Skipped: codex not detected.`).

Ou manualmente, usando o caminho absoluto impresso por `which sync82`:

```bash
codex mcp add sync82 -- /usr/local/bin/sync82
```

Note o `--` — o Codex exige ele para separar suas próprias flags do comando que deve rodar. Isso escreve em `~/.codex/config.toml`:

```toml
[mcp_servers.sync82]
command = "/usr/local/bin/sync82"
```

### 2. Verifique a conexão

```bash
codex mcp list
```

`sync82` deve aparecer na lista. Inicie uma sessão e confirme que as tools estão disponíveis.

### 3. Inicialize seu primeiro projeto

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

---

## 💡 Fluxos de Trabalho Recomendados

As mesmas 19 tools, os mesmos prompts que qualquer outro cliente — veja [Exemplos de Uso](../workflows/examples.md) para cenários completos de ponta a ponta.

### Retomando uma sessão

O Codex suporta retomar sua conversa mais recente:

```bash
codex resume --last
```

Combine isso com a própria memória do sync82: o resume de sessão do Codex devolve a conversa bruta, `load_project_context` dá à IA o resumo estruturado e curado — peça os dois ao retomar uma tarefa.

A documentação de MCP do Codex cobre só tools, não resources nem prompts MCP, então os [resources](../../reference/resources.md) e os [prompts MCP](../../reference/mcp-prompts.md) do sync82 não ficam disponíveis nele; peça as mesmas coisas em linguagem natural ("carregue a memória do projeto", "salve esta sessão") e o Codex chama as tools.

---

## 🗑️ Removendo

```bash
sync82 uninstall codex
```

Roda `codex mcp get sync82` e, quando o sync82 está registrado, `codex mcp remove sync82`. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

### `command not found: sync82`

O Codex pode não herdar o `PATH` do seu shell, então um registro que usa só o comando `sync82` (ou um caminho de onde o binário foi movido) pode falhar. Rode `sync82 install codex` de novo, ou encontre o caminho absoluto e adicione de novo com ele:

```bash
which sync82
# → /usr/local/bin/sync82

codex mcp remove sync82
codex mcp add sync82 -- /usr/local/bin/sync82
```

Ou edite `~/.codex/config.toml` diretamente:

```toml
[mcp_servers.sync82]
command = "/usr/local/bin/sync82"
```

### Servidor configurado mas nenhuma tool responde

Peça ao agente para verificar o vault:

```
Mostre a configuração atual do vault.
```

Isso chama `get_vault_config` — se o caminho reportado não for o esperado, veja [Resolução de Contexto](../../architecture/context-resolution.md).

---

## ➡️ Próximos Passos

- [Claude Code](./claude-code.md)
- [Antigravity](./antigravity.md)
- [OpenCode](./opencode.md)
- [Exemplos de Uso](../workflows/examples.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
