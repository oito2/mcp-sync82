🌐 [English](../../../en/guides/clients/antigravity.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com Antigravity

O **Antigravity** é a plataforma de desenvolvimento agêntico do Google, disponível como IDE e como CLI (`agy`). Ambos leem o mesmo arquivo de config MCP. O sync82 o configura como um **target de arquivo** — o `sync82 install` mescla uma entrada diretamente nesse config JSON, em vez de invocar um subcomando `mcp add`.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

```bash
sync82 install antigravity
```

O target é detectado pelo comando `agy` no `PATH` ou pela presença de `~/.gemini/config` ou `~/.gemini/antigravity` — um `~/.gemini` sozinho não basta; caso contrário, imprime `Skipped: antigravity not detected.` Ele grava só no `~/.gemini/config/mcp_config.json`, a config global compartilhada pela IDE e pela CLI.

Ou apenas rode `sync82 install` (sem target) para configurar todo cliente detectado de uma vez, incluindo o Antigravity.

### 2. Ou configure manualmente

Edite o `~/.gemini/config/mcp_config.json`, com o caminho absoluto do binário (`which sync82` / `where sync82` o imprime):

```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

Você também pode adicionar servidores pelas configurações de MCP da IDE ou pelo gerenciador interativo `/mcp` da CLI. O Antigravity também lê um `.agents/mcp_config.json` no nível do workspace, que o `sync82 install` nunca escreve; no momento em que isto foi escrito, a Antigravity CLI descobre servidores do nível do workspace mas não os inicia, então quem usa a CLI deve usar o arquivo global.

O comando `install` grava a mesma entrada acima, com o caminho absoluto do binário `sync82` que você executou — rode de novo depois de mover o binário. Ele lê o arquivo existente (um arquivo ausente ou em branco conta como `{}`), mescla essa entrada sob `mcpServers` e grava de volta de forma atômica, mantendo as permissões do arquivo e os números exatamente como estavam escritos — seus outros servidores configurados são preservados. Um arquivo que não é JSON puro (ex. JSON com comentários, ou um arquivo quebrado) não é alterado: o target falha e imprime a entrada para você adicionar à mão.

### 3. Verifique e inicialize

Reinicie o Antigravity (ou comece uma nova sessão da CLI) e confira o servidor nas configurações de MCP ou com `/mcp`, depois peça:

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

---

## 💡 Fluxos Recomendados

O mesmo conjunto de tools, os mesmos prompts que qualquer outro cliente — veja [Exemplos de Uso](../workflows/examples.md) para cenários completos de ponta a ponta. Algumas notas específicas do Antigravity:

- **Salvar/retomar sessão:** a Antigravity CLI suporta `/chat save <nome>` e `/chat resume <nome>` — combine isso com `update_project_memory` no final de uma sessão para duas camadas de continuidade (histórico de chat bruto mais memória estruturada).
- Reinicie a sessão depois de editar o arquivo de config — o Antigravity não recarrega a config MCP a quente.

---

## 🗑️ Removendo

```bash
sync82 uninstall antigravity
```

Apaga a entrada `sync82` do `~/.gemini/config/mcp_config.json` e dos legados `~/.gemini/antigravity/mcp_config.json` e `~/.gemini/antigravity-ide/mcp_config.json` (lidos por builds mais antigos), onde estiver presente. Remova à mão uma entrada que você adicionou ao `.agents/mcp_config.json` de um workspace. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

### O servidor não aparece depois de instalar

Reinicie o Antigravity — mudanças na config MCP exigem uma sessão nova, não só a escrita do arquivo.

### `command not found`

Se a entrada usa só o comando `sync82` e ele não estiver no `PATH` que o Antigravity herda (ou o binário foi movido), rode `sync82 install antigravity` de novo, ou edite o arquivo de config diretamente e use um caminho absoluto, como no exemplo acima.

### Servidor configurado num arquivo de workspace não inicia (CLI)

Mova a entrada para o `~/.gemini/config/mcp_config.json` global — veja a nota no passo 2.

---

## ➡️ Próximos Passos

- [Claude Code](./claude-code.md)
- [OpenAI Codex](./codex.md)
- [OpenCode](./opencode.md)
- [Exemplos de Uso](../workflows/examples.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
