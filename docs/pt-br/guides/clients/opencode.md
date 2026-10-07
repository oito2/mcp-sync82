🌐 [English](../../../en/guides/clients/opencode.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com OpenCode

O **OpenCode** é um agente de IA open-source para programação com interface TUI, configurado via um `~/.config/opencode/opencode.jsonc` (ou `opencode.json`) global.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

```bash
sync82 install opencode
```

O OpenCode é detectado quando o comando `opencode` está no `PATH`; caso contrário, o target imprime `Skipped: opencode not detected.` O sync82 mescla sua entrada, com o caminho absoluto do binário `sync82` que você executou, no `$XDG_CONFIG_HOME/opencode/opencode.json` global (padrão `~/.config/opencode/opencode.json`) — ou no `opencode.jsonc` quando ele é o único dos dois que existe — mantendo todas as outras chaves. Rodar de novo (ex. depois de mover o binário) substitui a entrada:

```json
{
  "mcp": {
    "sync82": {
      "type": "local",
      "command": ["/usr/local/bin/sync82"],
      "enabled": true
    }
  }
}
```

Um arquivo com comentários ou vírgulas sobrando (comum no `opencode.jsonc`) nunca é reescrito: o target falha e imprime a entrada para adicionar à mão.

Como alternativa, registre-o com a própria CLI do OpenCode, usando o caminho absoluto impresso por `which sync82`:

```bash
opencode mcp add sync82 -- /usr/local/bin/sync82
```

Note o `--` — `opencode mcp add <nome>` sozinho é interativo e rejeita ser chamado sem um comando. Isso também escreve no `~/.config/opencode/opencode.jsonc` (ou `opencode.json`) global, não num arquivo do seu projeto (verificado com o OpenCode 1.17.13).

### 2. Verifique e inicialize

Inicie (ou reinicie) o `opencode`, confirme que o servidor está conectado, depois peça:

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

---

## 💡 Fluxos de Trabalho Recomendados

As mesmas 19 tools, os mesmos prompts que qualquer outro cliente — veja [Exemplos de Uso](../workflows/examples.md) para cenários completos de ponta a ponta.

Como o registro fica na config global do OpenCode, o sync82 fica disponível em todo projeto; o escopo por projeto vem de `init_project_memory workspace_root=<raiz do projeto>` escrevendo `.sync82.json` na raiz do projeto, de modo que toda sessão naquele diretório resolve automaticamente sem repetir `project`.

---

## 🗑️ Removendo

```bash
sync82 uninstall opencode
```

O sync82 apaga a entrada `sync82` de `opencode.json`, `opencode.jsonc` e do legado `config.json` em `~/.config/opencode` (ou `$XDG_CONFIG_HOME/opencode`), mesmo quando o comando `opencode` não está mais no `PATH`. Um arquivo com comentários ou vírgulas sobrando não é alterado e a entrada precisa ser removida à mão. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

### `command not found`

Se o registro usa só o comando `sync82` e ele não estiver no `PATH` que o OpenCode herda (ou o binário foi movido), rode `sync82 install opencode` de novo, ou edite o `~/.config/opencode/opencode.jsonc` (ou `opencode.json`) com um caminho absoluto:

```json
{
  "mcp": {
    "sync82": {
      "type": "local",
      "command": ["/usr/local/bin/sync82"]
    }
  }
}
```

### O servidor não aparece depois de editar a config

Reinicie o `opencode` — mudanças de config exigem uma sessão nova.

### Projeto errado resolvido

```
Mostre a configuração atual do vault.
```

Chama `get_vault_config` — verifique se o diretório do projeto em que o OpenCode está trabalhando tem o `.sync82.json` que você espera que ele use. Veja [Resolução de Contexto](../../architecture/context-resolution.md).

---

## ➡️ Próximos Passos

- [Claude Code](./claude-code.md)
- [Antigravity](./antigravity.md)
- [OpenAI Codex](./codex.md)
- [Exemplos de Uso](../workflows/examples.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
