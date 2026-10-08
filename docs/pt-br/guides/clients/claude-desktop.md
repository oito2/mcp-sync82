🌐 [English](../../../en/guides/clients/claude-desktop.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Usando com Claude Desktop

O **Claude Desktop** é o app de desktop da Anthropic para macOS, Windows e Linux (no Linux, uma beta para distribuições baseadas em Debian, como Ubuntu e Debian, em x86_64 e arm64). O sync82 pode ser adicionado a ele de duas formas: como uma **extensão** de um clique (o bundle `sync82.mcpb`), ou como uma entrada no `claude_desktop_config.json` apontando para um binário do sync82 que você mesmo instalou. Use uma ou outra, não as duas.

---

## 🛠️ Opção A — Instalar a extensão (`sync82.mcpb`)

Toda release inclui o `sync82.mcpb`, um [MCP Bundle](https://github.com/modelcontextprotocol/mcpb) com os binários do sync82 para macOS (universal), Windows (amd64) e Linux (amd64/arm64) dentro — nada mais para instalar.

1. Baixe-o: <https://github.com/oito2/mcp-sync82/releases/latest/download/sync82.mcpb>
2. Opcionalmente, verifique-o — ele está listado no `checksums.txt` da release e coberto por uma atestação de proveniência de build (veja [Instalação — Verificando um Binário Baixado](../../getting-started/installation.md#-verificando-um-binário-baixado)).
3. No Claude Desktop, abra **Settings → Extensions → Advanced settings → Extension Developer**, clique em **Install Extension…**, selecione o `sync82.mcpb` e confirme a instalação.

A extensão tem uma configuração opcional, **Vault database path** (padrão `~/.sync82/knowledge.db`), repassada ao servidor como `SYNC82_DB_PATH` — veja a [Referência de Configuração](../../reference/configuration.md#bundle-do-claude-desktop-sync82mcpb).

O binário do bundle é gerenciado pelo Claude Desktop: atualize-o instalando um `sync82.mcpb` mais novo, não com `sync82 self-update`, e remova-o em **Settings → Extensions**.

---

## 🛠️ Opção B — Configurar um binário que você instalou

Com o binário `sync82` [instalado](../../getting-started/installation.md):

```bash
sync82 install claude-desktop
```

Ele mescla uma entrada `sync82` com o caminho absoluto do binário em:

| SO | Arquivo |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json`; no pacote MSIX (o download do claude.ai e a Microsoft Store), o app lê `%LOCALAPPDATA%\Packages\Claude_<id>\LocalCache\Roaming\Claude\claude_desktop_config.json` no lugar (o botão "Edit Config" dele pode abrir o outro arquivo). O `sync82 install` grava em cada um deles cujo diretório existe. |
| Linux (beta) | `$XDG_CONFIG_HOME/Claude/claude_desktop_config.json` (padrão `~/.config/Claude/claude_desktop_config.json`) — o local que a beta para Linux usa; a documentação da Anthropic ainda não lista um caminho para Linux |

O cliente é detectado quando esse diretório `Claude` existe; caso contrário, o target imprime `Skipped: claude-desktop not detected.` Em qualquer outro SO, imprime `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).`

Ou edite o arquivo à mão — **Settings → Developer → Edit Config** o abre — usando o caminho absoluto impresso por `which sync82` (`where sync82` no Windows):

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

No Windows, escape as barras invertidas: `"command": "C:\\Users\\voce\\AppData\\Local\\sync82\\sync82.exe"`.

---

## 🔄 Reinicie e verifique

**Feche o Claude Desktop por completo e abra-o de novo** — fechar a janela não basta; ele só lê a config MCP ao iniciar. Depois pergunte:

```
Liste todos os projetos do meu vault do sync82.
```

Uma lista vazia num vault novo significa que a conexão funciona. Então inicialize seu primeiro projeto:

```
Inicialize a memória do projeto em /home/eu/code/acme. Analise o código automaticamente.
```

O Claude Desktop não tem a noção de "diretório atual", então informe explicitamente ao agente o caminho do projeto (ou o nome dele).

O sync82 também expõe [resources](../../reference/resources.md) (a memória de cada projeto, legível sem chamar uma tool) e dois [prompts MCP](../../reference/mcp-prompts.md) (`start_session`, `end_session`). O Claude Desktop oferece resources e prompts MCP pelo menu da caixa de mensagem; os rótulos exatos mudam entre versões, então procure o servidor sync82 ali. A lista de projetos vem do seu vault padrão; informe o `project` nos prompts, já que o Claude Desktop não tem um workspace de onde tirá-lo.

---

## 🗑️ Removendo

```bash
sync82 uninstall claude-desktop
```

Isso remove a entrada do `claude_desktop_config.json` (Opção B). Uma extensão instalada pelo `sync82.mcpb` (Opção A) é removida em **Settings → Extensions**. Veja [Desinstalação](../../getting-started/uninstallation.md).

---

## ⚠️ Solução de Problemas

- **O sync82 não aparece** — confira se você fechou e reabriu o Claude Desktop por completo, e se o arquivo é JSON válido. Se o `sync82 install claude-desktop` reportou `manual step needed` porque o arquivo tem comentários ou vírgulas sobrando, adicione à mão a entrada impressa.
- **`command` não encontrado** — o caminho em `command` precisa ser absoluto e apontar para um binário que existe; rode `sync82 install claude-desktop` de novo depois de mover o binário.
- **Vault errado** — pergunte _"Mostre a configuração atual do vault"_ (`get_vault_config`) e veja [Resolução de Contexto](../../architecture/context-resolution.md).

---

## ➡️ Próximos Passos

- [Exemplos de Uso](../workflows/examples.md)
- [Prompts de Exemplo](../../prompts.md)
- [Referência de Tools](../../reference/tools.md)
- [Voltar ao Índice](../../index.md)
