🌐 [English](../../en/architecture/installer.md) | **Português** | 🏠 [Índice](../index.md)

---

# Instalador

Como o `sync82 install` e o `sync82 uninstall` configuram cada um dos 8 targets de cliente MCP suportados. A lista de targets fica em `internal/installer/targets.go`; os comandos são ligados em `cmd/sync82/install.go` e `cmd/sync82/uninstall.go`.

---

## Dois tipos de target

- **Targets de CLI** (`claude`, `codex`) são instalados removendo qualquer registro existente e depois rodando o próprio subcomando `mcp add` do cliente, que herda o stdin/stdout/stderr do seu terminal — se esse comando pedir alguma coisa, você vê diretamente.
- **Targets de arquivo** (`claude-desktop`, `antigravity`, `opencode`, `cursor`, `zed`, `cline`) são instalados lendo a config JSON do cliente (um arquivo ausente ou em branco conta como `{}`), mesclando uma entrada `sync82` e gravando de volta de forma atômica, mantendo todas as outras chaves, as permissões do arquivo, um symlink naquele caminho e os números exatamente como estavam escritos.

Quais targets contam como instalados na sua máquina é decidido por target — veja [Detecção](#detecção).

`<caminho-absoluto>` abaixo é o caminho absoluto do binário `sync82` em execução, com symlinks resolvidos — não o comando `sync82` puro — então o binário não precisa estar no `PATH` do cliente. O `install` imprime `Registering <caminho> — run "sync82 install" again after moving the binary.` antes de tocar qualquer target.

---

## Targets

| Target | Tipo | Instalação | Arquivos de config |
|---|---|---|---|
| `claude` | CLI | [Remove os registros existentes](#removendo-registros-de-cli-existentes), depois `claude mcp add --scope user sync82 -- <caminho-absoluto>` | `~/.claude.json`, escrito pelo Claude Code |
| `claude-desktop` | arquivo | `mcpServers.sync82 = {command, args}` | macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`; Windows: `%APPDATA%\Claude\claude_desktop_config.json` e, para o pacote MSIX (o download do claude.ai e a Microsoft Store), `%LOCALAPPDATA%\Packages\Claude_<id>\LocalCache\Roaming\Claude\claude_desktop_config.json` — todo aquele cujo diretório existe; Linux: `$XDG_CONFIG_HOME/Claude/claude_desktop_config.json` (padrão `~/.config/Claude/claude_desktop_config.json`); qualquer outro SO é pulado com `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).` |
| `antigravity` | arquivo | `mcpServers.sync82 = {command, args}` | O install escreve só no `~/.gemini/config/mcp_config.json`; o uninstall também limpa os legados `~/.gemini/antigravity/mcp_config.json` e `~/.gemini/antigravity-ide/mcp_config.json` |
| `codex` | CLI | `codex mcp get sync82`, depois `codex mcp remove sync82` quando ele está registrado, depois `codex mcp add sync82 -- <caminho-absoluto>` | `~/.codex/config.toml`, escrito pelo Codex |
| `opencode` | arquivo | `mcp.sync82 = {"type": "local", "command": ["<caminho-absoluto>"], "enabled": true}` | O install escreve `$XDG_CONFIG_HOME/opencode/opencode.json` (padrão `~/.config/opencode/opencode.json`), ou `opencode.jsonc` quando ele é o único dos dois que existe; o uninstall limpa `opencode.json`, `opencode.jsonc` e o legado `config.json` nesse diretório |
| `cursor` | arquivo | `mcpServers.sync82 = {type: "stdio", command, args}` | `~/.cursor/mcp.json` |
| `zed` | arquivo | `context_servers.sync82 = {command, args}` | Linux: `$XDG_CONFIG_HOME/zed/settings.json` (padrão `~/.config/zed/settings.json`); macOS: `~/.config/zed/settings.json`; Windows: `%APPDATA%\Zed\settings.json` |
| `cline` | arquivo | `mcpServers.sync82 = {command, args}` | Extensão do VS Code: `<dir. de usuário do VS Code>/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`, onde o diretório de usuário é `~/.config/Code/User` (Linux, respeitando `$XDG_CONFIG_HOME`), `~/Library/Application Support/Code/User` (macOS) ou `%APPDATA%\Code\User` (Windows); Cline CLI: `$CLINE_MCP_SETTINGS_PATH` quando definida com um caminho absoluto, senão `~/.cline/data/settings/cline_mcp_settings.json`, ou `$CLINE_DATA_DIR/settings/cline_mcp_settings.json` quando essa variável está definida; com a variável definida, o uninstall limpa também o arquivo do diretório de dados |

`%APPDATA%` cai para `<home>\AppData\Roaming` quando não está definida, e `$XDG_CONFIG_HOME` só é usada quando é um caminho absoluto.

O `cline` escreve só o arquivo de settings da variante instalada — os dois, quando ambas estão.

A entrada `{command, args}` é `{"command": "<caminho-absoluto>", "args": []}`.

### Detecção

Um target é **detectado** quando o comando dele está no `PATH` ou um dos diretórios de config dele existe (`DetectCmd`/`DetectDirs` em `targets.go`); um target não suportado no SO atual nunca é detectado.

| Target | Detectado quando |
|---|---|
| `claude` | `claude` está no `PATH` |
| `claude-desktop` | o diretório de config do Claude Desktop existe (`~/Library/Application Support/Claude`, `%APPDATA%\Claude`, `%LOCALAPPDATA%\Packages\Claude_<id>\LocalCache\Roaming\Claude` ou `$XDG_CONFIG_HOME/Claude`) |
| `antigravity` | `agy` está no `PATH`, ou `~/.gemini/config` ou `~/.gemini/antigravity` existe (um `~/.gemini` sozinho não basta) |
| `codex` | `codex` está no `PATH` |
| `opencode` | `opencode` está no `PATH` |
| `cursor` | `~/.cursor` existe |
| `zed` | o diretório de config do Zed existe (o diretório do `settings.json` acima) |
| `cline` | o diretório `saoudrizwan.claude-dev` da extensão, o `~/.cline` da CLI (ou `$CLINE_DATA_DIR`) ou a pasta de `$CLINE_MCP_SETTINGS_PATH` existe |

---

## Fluxo de instalação

1. Sem target, o `install` age só sobre os clientes detectados: imprime `Detected the following MCP clients:` com uma linha `  - <target>` para cada um e pergunta `Install sync82 into all N detected client(s)? [y/N]` (qualquer resposta diferente de `y`/`yes` imprime `Aborted.`). Quando nada é detectado, imprime `No supported MCP clients detected. Supported targets: <targets>` e sai com `0` sem perguntar. Com um target, roda só aquele, sem perguntar. Mais de um target é erro de uso (código `2`); um target desconhecido sai com `1`.
2. Um SO não suportado imprime `Skipped: <target> (<motivo>).` — só o `claude-desktop` tem um: `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).`; um target explícito cujo cliente não é detectado imprime `Skipped: <target> not detected.` Nenhum dos dois é erro.
3. Caso contrário, instala e imprime `✓  <target> — configured.` para um registro novo, ou `✓  <target> — updated.` quando o sync82 já estava registrado — no `claude` e no `codex`, quando um registro anterior foi [removido](#removendo-registros-de-cli-existentes) antes; nos targets de arquivo, quando um arquivo de config já tinha uma entrada `sync82`. Avisos são impressos depois da linha de status.
4. Um target de arquivo cujo arquivo de config tem comentários ou vírgulas sobrando o deixa como está e imprime `!  <target> — manual step needed`, com a entrada para adicionar à mão no stderr — a menos que o arquivo já tenha essa entrada (chaves que você adicionou a ela, como `env`, são permitidas), o que conta como `✓  <target> — updated.`, então rodar o `install` de novo depois de adicionar a entrada à mão dá certo.
5. Sem target, termina com `Done. N installed, N skipped, N need a manual step, N failed.` e sai com `1` se algum target falhou ou precisa de um passo manual.

Rodar de novo é seguro: `claude` e `codex` removem todo registro existente antes do `mcp add`, já que o `mcp add` deles se recusa a substituir um; targets de arquivo sobrescrevem a entrada `sync82`. Um arquivo de config que começa com o byte order mark do UTF-8 (gravado por alguns editores do Windows) é lido normalmente e regravado sem a marca. Um arquivo JSON regravado mantém cada chave na sua ordem e cada valor como foi escrito (números e escapes de string incluídos); só a indentação muda, para dois espaços. Uma entrada `sync82` nova é adicionada no fim do objeto de servidores, e uma existente é substituída onde está. Uma chave escrita duas vezes fica na primeira posição com o último valor, que é o que os parsers de JSON leem.

Um target de arquivo é uma leitura-mescla-escrita de um arquivo que o cliente também controla. Um cliente rodando enquanto o `install` grava pode salvar a própria cópia da config depois e descartar a entrada `sync82`, então feche o cliente antes de instalar nele. O caminho do binário registrado é aquele para o qual o `sync82` em execução resolve, seguindo symlinks (`internal/binpath`), então os clientes executam o arquivo real, que também é o que o `self-update` substitui. No uninstall, um arquivo de config que não pode ser lido é reportado como `failed` com o motivo, nunca tomado por um sem o sync82.

### Removendo registros de CLI existentes

Tanto o `install` quanto o `uninstall` começam um target de CLI removendo os registros existentes do sync82 (`internal/installer/cliregistration.go`):

- **`codex`** roda `codex mcp get sync82` uma vez e, quando ele sai com `0`, `codex mcp remove sync82`. Um `get` que falha só significa "não registrado" quando a saída diz `No MCP server named`; qualquer outra falha (um `config.toml` quebrado, um timeout) faz o target falhar com a saída do CLI, para que o `install` não adicione por cima de um registro que não conseguiu ver e o `uninstall` não diga `not configured` enquanto um registro pode continuar lá.
- **`claude`** repete `claude mcp get sync82`, que informa só o registro que tem precedência (local, depois projeto, depois usuário). Ele lê o escopo da saída e roda `claude mcp remove --scope <escopo> sync82` (`local`, `user`), repetindo até o `claude mcp get` falhar.
  - Um registro em **escopo de projeto** (`.mcp.json`) nunca é removido. Ele imprime `Warning: sync82 is also registered in project scope (.mcp.json, shared with the project) for <diretório atual>; it was left unchanged and, inside that project, it takes precedence over any user-scope registration. To remove it, run from that directory: claude mcp remove --scope project sync82`, e o escopo de usuário escondido atrás dele é então removido diretamente com `claude mcp remove --scope user sync82` (uma saída `No MCP server named` significa que não havia nada a remover).
  - Quando o escopo não pode ser lido da saída, o target falha com ``could not determine the scope of the sync82 registration from `claude mcp get sync82`; remove it manually with `claude mcp remove --scope <scope> sync82` ``; quando um escopo é informado de novo depois de removido, com ``sync82 is still registered in the <scope> scope after `claude mcp remove --scope <scope> sync82` ``. Os dois são impressos no stderr com o prefixo `[claude]`, e o `mcp add` não roda.

**JSON com comentários.** Um arquivo de config que só é lido depois de remover comentários e vírgulas sobrando (JSONC — comum no `settings.json` do Zed) nunca é reescrito: o target falha, nenhum backup é feito e a entrada para adicionar à mão é impressa no stderr. Um arquivo que não pode ser lido de jeito nenhum falha do mesmo jeito.

---

## Fluxo de desinstalação

O `sync82 uninstall [target] [--purge]` espelha o install: sem target, imprime `Detected the following MCP clients:` e pergunta `Remove sync82 from all N detected client(s)? [y/N]` (recusar encerra a execução, `--purge` incluído, e com `--purge` imprime `--purge skipped too: nothing was removed or deleted.`); com um target, roda sem perguntar. A lista tem os targets detectados mais todo target de arquivo suportado cujo cliente não é mais detectado mas cujo arquivo de config ainda tem uma entrada `sync82` (`UninstallCandidatesIn`); um arquivo de config que não pode ser lido conta como tendo uma. Quando nenhum target é listado, imprime `No supported MCP clients detected. Supported targets: <targets>` e o `--purge` roda mesmo assim.

- **Targets de CLI** (`claude`, `codex`) — um target explícito cujo comando não está no `PATH` imprime `Skipped: <target> not detected.`; caso contrário, os registros dele são [removidos](#removendo-registros-de-cli-existentes). Imprime `✓  <target> — removed.` quando algo foi removido, `⚠  <target> — nothing removed` quando só foi encontrado um registro do `claude` em escopo de projeto (seguido do aviso dele), e `⚠  <target> — not configured, skipping` quando nada estava registrado.
- **Targets de arquivo** apagam a chave `sync82` de todo arquivo de config que a tenha — no `antigravity`, os três `mcp_config.json`; no `opencode`, `opencode.json`, `opencode.jsonc` e `config.json`; no `cline`, os arquivos de settings da extensão e da CLI — mantendo todas as outras chaves. Um target de arquivo nomeado explicitamente é limpo mesmo quando o cliente dele não é mais detectado. Um arquivo JSONC não é alterado e o target falha com uma mensagem para remover a entrada à mão.

Cada target imprime `✓  <target> — removed.`, `⚠  <target> — not configured, skipping`, `⚠  <target> — nothing removed`, uma linha `Skipped: ...`, `!  <target> — manual step needed` (um arquivo de config com comentários, mantido como está, com a entrada para remover à mão no stderr), ou `✗  <target> — failed`; sem target, a execução termina com `Done. N removed, N skipped, N need a manual step, N failed.`

### `--purge`

Depois dos targets, o `--purge` (implementado em `internal/installer/purge.go`):

1. Lista os vaults configurados fora do padrão — `SYNC82_DB_PATH`, `vaultPath` e `lastVaultPath` do `~/.sync82/config.json`, e o `path` de um `.sync82.json` encontrado no diretório atual ou num pai — em `Note: these configured vaults are outside the purge and are left untouched:`. Eles nunca são apagados.
2. Lista os arquivos existentes entre `~/.sync82/knowledge.db`, `knowledge.db-wal`, `knowledge.db-shm`, `config.json`, `config.lock` e todo `config.json.corrupt-*` (ou imprime `Nothing to purge in <home>/.sync82.`).
3. Pergunta `Delete these files? [y/N]`. Não imprime `Purge cancelled.`; sim apaga os arquivos, remove `~/.sync82` se ele ficar vazio e imprime `Purge complete.` — ou `Purge incomplete.` mais um erro por arquivo que não pôde ser removido.

**Códigos de saída:** `0` quando nada falhou (inclusive uma pergunta respondida com não), `1` para um target desconhecido, um stdin fechado numa pergunta, um target que falhou ou um purge incompleto, `2` (erro de uso) para uma flag desconhecida ou mais de um target.

---

## Clientes fora desta lista

Qualquer outro cliente MCP pode ser conectado manualmente: a config dele só precisa de uma entrada de servidor nestas linhas (ajuste a estrutura JSON ao redor para o formato do seu cliente):

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

Sem wrapper estilo `npx`, sem argumentos — só o caminho absoluto do binário (encontre-o com `which sync82`, ou `where sync82` no Windows), então funciona mesmo quando o cliente não herda o `PATH` do seu shell.

---

## Segurança nos testes

`RunInstall` e `RunUninstall` recebem `homeDir` e `targets []installer.Target` como parâmetros explícitos em vez de ler `os.UserHomeDir()`/`installer.Targets` diretamente, e todo target resolve seus caminhos e sua presença por um `installer.Env` (SO, diretório home, consulta a variáveis de ambiente, consulta ao `PATH`). Isso é deliberado: os testes automatizados rodam na mesma máquina onde `claude`/`codex`/`opencode` estão de fato no `PATH`, então se os comandos usassem os valores reais diretamente, a suíte de testes rodaria comandos `mcp add`/`mcp remove` de verdade e editaria as configs reais dos seus clientes. Com a injeção, os testes usam uma lista de targets e um diretório home fake e nunca tocam configs reais. Só a wiring de produção em `cmd/sync82/main.go` passa os valores reais.

---

## Veja também

- [Guia de Instalação](../getting-started/installation.md) — o passo a passo voltado ao usuário
- [Desinstalação](../getting-started/uninstallation.md) — remoção completa, passo a passo
- [Referência da CLI — install](../reference/cli.md#install) e [uninstall](../reference/cli.md#uninstall) — códigos de saída e flags
- [Guias de cliente](../guides/clients/claude-code.md) — configuração e solução de problemas por cliente

---

[🏠 Voltar ao Índice](../index.md)
