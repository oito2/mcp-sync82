🌐 [English](../../en/getting-started/uninstallation.md) | **Português** | 🏠 [Índice](../index.md)

---

# Desinstalação

Como remover o sync82 por completo: o registro dele em cada cliente MCP, os arquivos próprios em `~/.sync82/`, o binário e os arquivos que ele deixa nos seus workspaces. Siga os passos nesta ordem — o `sync82 uninstall` precisa do binário.

---

## 1. Remova o sync82 dos seus clientes MCP

```bash
sync82 uninstall            # lista os clientes detectados, pergunta "Remove sync82 from all N detected client(s)? [y/N]", remove o sync82 de cada um
sync82 uninstall cursor     # só um target, sem pergunta de confirmação
```

Targets: `claude`, `claude-desktop`, `antigravity`, `codex`, `opencode`, `cursor`, `zed`, `cline` — o mesmo conjunto que o `sync82 install` escreve. Sem target, só os clientes detectados na sua máquina são processados (veja [Detecção](../architecture/installer.md#detecção)); quando nenhum é detectado, imprime `No supported MCP clients detected. Supported targets: ...`. Nomeie um target de arquivo explicitamente para limpar os arquivos de config dele mesmo depois de o cliente ter sido desinstalado. Cada target imprime uma linha:

| Linha | Significado |
|---|---|
| `✓  <target> — removed.` | O registro foi removido. |
| `⚠  <target> — not configured, skipping` | Nenhuma entrada do sync82 foi encontrada. |
| `⚠  <target> — nothing removed` | O `claude` só tinha um registro em escopo de projeto, que não é alterado (segue uma linha `Warning: ...`). |
| `Skipped: <target> not detected.` | Um target `claude` ou `codex` explícito cujo comando não está no `PATH`. |
| `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).` | `claude-desktop` em qualquer outro SO. |
| `✗  <target> — failed` | A entrada não pôde ser removida; o motivo é impresso no stderr. |

Sem target, a execução termina com `Done. N removed, N skipped, N failed.` O comando sai com `1` se algum target falhou (ou com um target desconhecido, ou quando o stdin está fechado e a pergunta fica sem resposta), `2` com uma flag desconhecida, `0` caso contrário — inclusive quando você responde não à pergunta. Veja [Referência da CLI — uninstall](../reference/cli.md#uninstall).

O que cada target limpa:

| Target | Como |
|---|---|
| `claude` | Repete `claude mcp get sync82` e roda `claude mcp remove --scope <escopo> sync82` para cada escopo que ele informa (`local`, `user`); um registro em escopo de projeto não é alterado e gera um aviso, e o escopo de usuário é então removido diretamente — veja [Instalador](../architecture/installer.md#removendo-registros-de-cli-existentes). |
| `codex` | Roda `codex mcp get sync82` e, quando registrado, `codex mcp remove sync82`. |
| `opencode` | O sync82 apaga a chave `sync82` de `opencode.json`, `opencode.jsonc` e do legado `config.json` em `$XDG_CONFIG_HOME/opencode` (padrão `~/.config/opencode`). |
| `claude-desktop`, `antigravity`, `cursor`, `zed`, `cline` | O sync82 apaga a chave `sync82` de cada arquivo de config listado em [Instalador](../architecture/installer.md#targets), mantendo todas as outras chaves. |

### O que o `sync82 uninstall` deixa para você

- **Escopo de projeto do Claude Code** — uma entrada `sync82` no `.mcp.json` de um projeto é compartilhada com o projeto e nunca é removida automaticamente. Dentro de cada projeto assim, rode:

  ```bash
  claude mcp remove --scope project sync82
  ```

- **Arquivos de config com comentários ou vírgulas sobrando** — um arquivo JSONC (comum no `settings.json` do Zed e no `opencode.jsonc` do OpenCode) não é alterado e o target falha com uma mensagem indicando o arquivo; apague a entrada `sync82` dele à mão.
- **Extensão do Claude Desktop** — se você instalou o bundle `sync82.mcpb`, remova-o em **Settings → Extensions** no Claude Desktop; o `sync82 uninstall claude-desktop` só edita o `claude_desktop_config.json`.
- **Configs de projeto** — o sync82 só escreve configs globais. Uma entrada que você mesmo adicionou num arquivo de projeto (por exemplo o `.cursor/mcp.json` do Cursor ou o `.agents/mcp_config.json` do Antigravity) precisa ser removida à mão.

Reinicie cada cliente depois, para ele parar de iniciar o sync82.

---

## 2. Apague os dados do sync82 (`--purge`)

> **Isto apaga o seu vault de memória.** Exporte antes o que quiser manter, ex. `sync82 export --all ~/sync82-backup`.

```bash
sync82 uninstall --purge          # todos os targets detectados, depois o purge
sync82 uninstall codex --purge    # um target, depois o purge
```

Depois da etapa dos clientes, o `--purge` lista os arquivos que vai apagar de `~/.sync82/` e faz uma pergunta separada, `Delete these files? [y/N]`:

- `knowledge.db`, `knowledge.db-wal`, `knowledge.db-shm` — o vault padrão
- `config.json`, `config.lock` — a config global
- `config.json.corrupt-*` — arquivos de config movidos de lado após um erro de parse

Só os arquivos que existem são listados; nada mais no diretório é tocado, e o próprio `~/.sync82/` só é removido quando fica vazio. Feche antes todo cliente MCP que usa o sync82. Responder não imprime `Purge cancelled.`; um arquivo que não pode ser apagado imprime `Purge incomplete.` e o comando sai com `1`.

Vaults configurados em outro lugar — `SYNC82_DB_PATH`, `vaultPath`/`lastVaultPath` no `config.json` e o `path` de um `.sync82.json` no diretório atual ou num pai — são listados em `Note: these configured vaults are outside the purge and are left untouched:`. Apague-os à mão se quiser, junto com os arquivos `-wal` e `-shm` deles.

---

## 3. Remova o binário

Apague o binário de onde você o instalou, mais o `<binário>.bak` que o `sync82 self-update` mantém ao lado dele:

```bash
# Linux / macOS, binário de release
sudo rm -f /usr/local/bin/sync82 /usr/local/bin/sync82.bak

# go install
rm -f "$(go env GOPATH)/bin/sync82" "$(go env GOPATH)/bin/sync82.bak"
```

```powershell
# Windows, binário de release
Remove-Item "$env:LOCALAPPDATA\sync82\sync82.exe", "$env:LOCALAPPDATA\sync82\sync82.exe.bak" -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\sync82"
```

No Windows, remova também `%LOCALAPPDATA%\sync82` do seu `PATH` se você o adicionou. Um arquivo `.bak` só existe depois de um `self-update`.

---

## 4. Remova os arquivos dos workspaces

O `init_project_memory` escreve um `.sync82.json` na raiz de cada workspace que inicializa. Encontre e apague-os:

```bash
find ~ -name .sync82.json -type f 2>/dev/null
```

Remova qualquer instrução sobre o sync82 que você tenha adicionado ao `CLAUDE.md` (ou arquivo de instruções de agente parecido) de um projeto, e qualquer pasta criada com `export_memory`/`sync82 export` que você não precise mais.

---

## Veja também

- [Instalação](./installation.md)
- [Referência da CLI — uninstall](../reference/cli.md#uninstall)
- [Referência de Configuração](../reference/configuration.md) — todo arquivo que o sync82 lê e escreve
