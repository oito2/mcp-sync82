🌐 [English](../../../en/guides/clients/cursor.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com Cursor

O **Cursor** é um editor de código com IA e suporte a MCP. O sync82 o configura como um **target de arquivo**: o `sync82 install` mescla uma entrada no `~/.cursor/mcp.json` global do Cursor.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

```bash
sync82 install cursor
```

O target é detectado pela presença do diretório `~/.cursor`. Ele grava o caminho absoluto do binário `sync82` que você executou — rode de novo depois de mover o binário.

### 2. Ou configure manualmente

Edite o `~/.cursor/mcp.json` (global, todo projeto), usando o caminho absoluto impresso por `which sync82` (`where sync82` no Windows):

```json
{
  "mcpServers": {
    "sync82": {
      "type": "stdio",
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

O Cursor também lê um `.cursor/mcp.json` no nível do projeto, com o mesmo formato; o `sync82 install` nunca o escreve.

### 3. Verifique e inicialize

Abra **Customize** na barra lateral do Cursor e confira se o `sync82` está listado entre os servidores MCP e ativado (reinicie o Cursor se ele já estava aberto quando o arquivo mudou). Depois peça ao agente:

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

O Cursor também suporta resources e prompts MCP, então os [resources](../../reference/resources.md) (a memória de cada projeto) e os [prompts MCP](../../reference/mcp-prompts.md) (`start_session`, `end_session`) do sync82 ficam disponíveis junto com as tools.

---

## 🗑️ Removendo

```bash
sync82 uninstall cursor
```

Remove a entrada `sync82` do `~/.cursor/mcp.json`, mantendo todos os outros servidores. Remova à mão uma entrada que você adicionou ao `.cursor/mcp.json` de um projeto. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

- **Servidor não aparece na lista** — reinicie o Cursor e confira se o `~/.cursor/mcp.json` é JSON válido. Um arquivo com comentários ou vírgulas sobrando não é alterado pelo `sync82 install`, que imprime a entrada para adicionar à mão.
- **`command` não encontrado** — use um caminho absoluto; rode `sync82 install cursor` de novo depois de mover o binário.
- **Projeto errado resolvido** — pergunte _"Mostre a configuração atual do vault"_ (`get_vault_config`) e veja [Resolução de Contexto](../../architecture/context-resolution.md).

---

## ➡️ Próximos Passos

- [Exemplos de Uso](../workflows/examples.md)
- [Prompts de Exemplo](../../prompts.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
