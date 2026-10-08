🌐 [English](../../../en/guides/clients/cline.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com Cline

O **Cline** é um agente de programação com IA disponível como extensão do VS Code e como CLI. Ambos guardam seus servidores MCP num arquivo `cline_mcp_settings.json`. O sync82 o configura como um **target de arquivo**: o `sync82 install` mescla uma entrada no arquivo de settings de cada variante do Cline instalada.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

```bash
sync82 install cline
```

Ele grava em toda variante que encontrar:

| Variante | Detectada por | Arquivo |
|---|---|---|
| Extensão do VS Code | o diretório de armazenamento `saoudrizwan.claude-dev` da extensão | Linux: `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` (respeita `$XDG_CONFIG_HOME`); macOS: `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`; Windows: `%APPDATA%\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json` |
| Cline CLI (e o diretório de dados compartilhado do Cline) | `~/.cline` (ou um `$CLINE_DIR` absoluto, ou `$CLINE_DATA_DIR`, ou a pasta de `$CLINE_MCP_SETTINGS_PATH`) | `$CLINE_MCP_SETTINGS_PATH` quando definida com um caminho absoluto; senão `~/.cline/data/settings/cline_mcp_settings.json`, ou `$CLINE_DATA_DIR/settings/cline_mcp_settings.json` quando `CLINE_DATA_DIR` está definida |

O Cline documenta o `~/.cline` como compartilhado pelas extensões de IDE, pela CLI e pelo SDK, e o `~/.cline/data/settings/cline_mcp_settings.json` como o arquivo de configuração MCP da CLI; o arquivo em `globalStorage` do VS Code é onde a extensão tem guardado seus servidores. O sync82 grava em todo arquivo cujo diretório existe. Só o diretório de armazenamento do VS Code estável é verificado.

### 2. Ou configure manualmente

No VS Code, clique no ícone **MCP Servers** da barra de ferramentas do painel do Cline, abra a aba **Configure** e clique em **Configure MCP Servers**, o que abre o arquivo de configuração que a extensão usa; na CLI, edite o arquivo acima. Adicione, usando o caminho absoluto impresso por `which sync82` (`where sync82` no Windows):

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

### 3. Verifique e inicialize

Confira se o `sync82` aparece na visão **MCP Servers** do Cline. Depois peça:

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

---

## 🗑️ Removendo

```bash
sync82 uninstall cline
```

Apaga a entrada `sync82` dos dois arquivos de settings, onde estiver presente. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

- **`Skipped: cline not detected.`** — nem o diretório de armazenamento da extensão, nem o `~/.cline` (ou um `$CLINE_DIR` absoluto, ou `$CLINE_DATA_DIR`), nem a pasta de `$CLINE_MCP_SETTINGS_PATH` existem; abra o Cline uma vez para ele criar o armazenamento, ou configure manualmente.
- **`command` não encontrado** — use um caminho absoluto; rode `sync82 install cline` de novo depois de mover o binário.
- **Projeto errado resolvido** — pergunte _"Mostre a configuração atual do vault"_ (`get_vault_config`) e veja [Resolução de Contexto](../../architecture/context-resolution.md).

---

## ➡️ Próximos Passos

- [Exemplos de Uso](../workflows/examples.md)
- [Prompts de Exemplo](../../prompts.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
