🌐 [English](../../../en/guides/clients/zed.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com Zed

O **Zed** é um editor de código com um agente embutido que suporta servidores MCP, que o Zed chama de *context servers*. O sync82 o configura como um **target de arquivo**: o `sync82 install` mescla uma entrada no objeto `context_servers` do `settings.json` de usuário do Zed.

---

## 🛠️ Configuração Inicial

### 1. Adicione o servidor

```bash
sync82 install zed
```

Ele grava no arquivo de configurações de usuário do Zed — o target é detectado pela presença do diretório dele:

| SO | Arquivo |
|---|---|
| Linux | `$XDG_CONFIG_HOME/zed/settings.json` (padrão `~/.config/zed/settings.json`) |
| macOS | `~/.config/zed/settings.json` |
| Windows | `%APPDATA%\Zed\settings.json` |

> **Comentários no `settings.json`.** O arquivo de configurações do Zed costuma ter comentários. O sync82 nunca reescreve um arquivo com comentários ou vírgulas sobrando: o target falha, o arquivo fica exatamente como estava e a entrada é impressa para você colar à mão (passo 2).

### 2. Ou configure manualmente

Abra o arquivo de configurações (paleta de comandos: **zed: open settings file**, ou edite o arquivo acima) e adicione, usando o caminho absoluto impresso por `which sync82` (`where sync82` no Windows):

```json
{
  "context_servers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

Se `context_servers` já existir, adicione só a entrada `"sync82"` dentro dele. O Zed também pode adicioná-la em **Settings → AI → MCP Servers → Add Server → Add Local Server**.

### 3. Verifique e inicialize

Abra **Settings → AI → MCP Servers** e confira se o indicador ao lado do `sync82` mostra que ele está rodando — o Zed aplica mudanças no `settings.json` sem reiniciar. Depois peça ao agente:

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

---

## 🗑️ Removendo

```bash
sync82 uninstall zed
```

Apaga a chave `sync82` de `context_servers`. Com comentários no arquivo, o target falha e pede para você remover a entrada à mão. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

- **`zed — failed` com "contains comments or trailing commas"** — esperado num arquivo de configurações com comentários; adicione (ou remova) a entrada impressa à mão.
- **`command` não encontrado** — use um caminho absoluto; rode `sync82 install zed` de novo depois de mover o binário.
- **Projeto errado resolvido** — pergunte _"Mostre a configuração atual do vault"_ (`get_vault_config`) e veja [Resolução de Contexto](../../architecture/context-resolution.md).

---

## ➡️ Próximos Passos

- [Exemplos de Uso](../workflows/examples.md)
- [Prompts de Exemplo](../../prompts.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
