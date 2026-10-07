🌐 [English](../../en/reference/cli.md) | **Português** | 🏠 [Índice](../index.md)

---

# Referência da CLI

`sync82` é ao mesmo tempo o binário do servidor MCP e sua própria CLI. Todos os comandos abaixo são subcomandos do mesmo binário `sync82` — não há nada mais pra instalar.

**Leitura de argumentos:** flags podem ser dadas como `--path valor` ou `--path=valor`, em qualquer posição da linha. Uma opção desconhecida (ex. `--force`), um `--path` vazio ou repetido, um `--path` seguido de outra flag (ex. `--path --all`; escreva `--path=<valor>` para um valor que começa com `-`), e argumentos sobrando (ex. `sync82 version x`, `sync82 install claude codex`) são **erros de uso**: o comando não faz nada, imprime `Error: <motivo>` seguido de `Run 'sync82 --help' for usage.` no stderr e sai com código `2`. Um subcomando desconhecido (ex. `sync82 instal`) sai com código `1` e imprime a ajuda, e um primeiro argumento com cara de flag (ex. `sync82 --bogus`) é erro de uso. Um prompt de confirmação que fica sem resposta porque o stdin está fechado (ex. `sync82 install </dev/null`) é um erro (código `1`), nunca lido como "não".

## `sync82` (serve)

Rodar o binário sem argumentos inicia o servidor MCP via stdio. É isso que seu cliente MCP de fato executa — normalmente você não roda isso à mão.

```bash
sync82
sync82 serve   # o mesmo, por extenso
```

- Registra todas as [19 tools](./tools.md), os [resources](./resources.md) e os [prompts MCP](./mcp-prompts.md).
- Resolve o caminho padrão do vault a partir da variável de ambiente `SYNC82_DB_PATH`, ou `~/.sync82/knowledge.db` se não definida.
- Registra logs só em stderr — stdout é reservado pro protocolo JSON-RPC.
- Para de forma limpa com `SIGINT`/`SIGTERM` (assim como os outros subcomandos): o servidor fecha os vaults abertos e sai com código `0`.

## `help` / `--help` / `-h`

Imprime a lista de subcomandos e sai — nenhum vault ou cliente MCP é tocado.

```bash
sync82 help
sync82 --help
sync82 -h
```

Um subcomando não reconhecido imprime a mesma lista de uso (em stderr) antes de sair com código `1`.

## `version` / `--version` / `-v`

Imprime a versão instalada e sai.

```bash
sync82 version
sync82 --version
sync82 -v
```

Builds de release imprimem a versão injetada em tempo de compilação em `github.com/oito2/mcp-sync82/internal/version.Current`. Um binário compilado com `go install github.com/oito2/mcp-sync82/cmd/sync82@vX.Y.Z` informa `vX.Y.Z` (lido das informações de build do módulo). Qualquer outro build imprime `dev`: um build local a partir de um checkout do código-fonte (mesmo um que o Go marca com dados do VCS, como `v1.0.0+dirty`) e um `go install` de um commit sem tag (uma pseudo-versão como `v1.0.1-0.20261007120000-abcdef123456`) — veja [`self-update`](#self-update) abaixo pra entender por que isso importa.

## `install`

Conecta o sync82 a um ou todos os clientes MCP suportados.

```bash
sync82 install            # lista os clientes detectados, pede confirmação, configura cada um deles
sync82 install <alvo>     # configura só um alvo específico, sem prompt de confirmação
```

O `install` aceita no máximo um alvo.

Todo alvo é registrado com o **caminho absoluto** do binário `sync82` em execução (symlinks resolvidos), não com o comando puro `sync82`, então o binário não precisa estar no `PATH` do cliente MCP. O `install` imprime primeiro `Registering <path> — run "sync82 install" again after moving the binary.`: se você mover o binário depois, rode `sync82 install` de novo. Rodar de novo é seguro — substitui o registro existente.

**Alvos:** `claude`, `codex` (cada um via seu próprio subcomando `mcp add`, depois de remover qualquer registro existente), `claude-desktop`, `antigravity`, `opencode`, `cursor`, `zed`, `cline` (cada um via leitura, merge e escrita de um arquivo de config JSON). Veja [Instalação](../getting-started/installation.md) ou [Internals do Instalador](../architecture/installer.md#targets) pra saber o arquivo exato que cada alvo mexe em cada SO.

Um cliente é **detectado** quando o comando dele está no `PATH` ou o diretório de config dele existe (por alvo — veja [Detecção](../architecture/installer.md#detecção)). Rodar `sync82 install` sem alvo age só sobre os clientes detectados: imprime `Detected the following MCP clients:` com uma linha `  - <alvo>` para cada um, e então pergunta `Install sync82 into all N detected client(s)? [y/N]` via stdin antes de mexer em qualquer coisa (qualquer resposta diferente de `y`/`yes` imprime `Aborted.`; um stdin fechado é um erro que sugere `sync82 install <alvo>`). Quando nada é detectado, imprime `No supported MCP clients detected. Supported targets: ...` e sai com `0`. Um alvo explícito cujo cliente não é detectado é **pulado**, não tratado como erro: `Skipped: <alvo> not detected.` Num SO que não seja macOS, Windows ou Linux, o `claude-desktop` imprime `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).` Cada alvo que dá certo imprime `✓  <alvo> — configured.` para um registro novo ou `✓  <alvo> — updated.` quando o sync82 já estava registrado (no `claude` e no `codex`: um registro anterior foi removido antes); sem alvo, a execução termina com `Done. N installed, N skipped, N failed.` Um arquivo de config com comentários ou vírgulas sobrando nunca é reescrito: aquele alvo falha e imprime a entrada para adicionar à mão.

**Códigos de saída:**

| Código | Significado |
|---|---|
| `0` | Todo alvo foi instalado ou pulado sem problema (ou nenhum cliente foi detectado, ou o usuário recusou o prompt de confirmação) |
| `1` | Um nome de alvo não reconhecido, um stdin fechado no prompt de confirmação, ou pelo menos um alvo falhou |
| `2` | Erro de uso: uma flag ou mais de um alvo foi dado |

## `uninstall`

Remove o registro do sync82 de um ou de todos os clientes MCP suportados e, opcionalmente, apaga os arquivos próprios do sync82.

```bash
sync82 uninstall                    # lista os clientes detectados, pede confirmação, remove o sync82 de cada um
sync82 uninstall <alvo>             # um alvo, sem prompt de confirmação
sync82 uninstall [alvo] --purge     # depois também apaga os arquivos de ~/.sync82, após uma confirmação separada
sync82 uninstall --help             # imprime o uso
```

O `uninstall` aceita no máximo um alvo — os mesmos alvos do [`install`](#install) — e a flag `--purge`. Sem alvo, ele age só sobre os clientes detectados, como o `install`: imprime `Detected the following MCP clients:` e pergunta `Remove sync82 from all N detected client(s)? [y/N]`; a execução termina com `Done. N removed, N skipped, N failed.` Quando nada é detectado, imprime `No supported MCP clients detected. Supported targets: ...`, e o `--purge` roda mesmo assim. Cada alvo imprime `✓  <alvo> — removed.`, `⚠  <alvo> — not configured, skipping` (nada registrado), `⚠  <alvo> — nothing removed` (`claude` só com um registro em escopo de projeto), `Skipped: <alvo> not detected.` (um alvo `claude` ou `codex` explícito cujo comando não está no `PATH`), `Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux).`, ou `✗  <alvo> — failed`.

- O `claude` repete `claude mcp get sync82` e roda `claude mcp remove --scope <escopo> sync82` para cada escopo que ele informa (`local`, `user`). Um registro em escopo de projeto (`.mcp.json`) não é alterado e gera um `Warning: sync82 is also registered in project scope (.mcp.json, shared with the project) for <dir>; it was left unchanged and, inside that project, it takes precedence over any user-scope registration. To remove it, run from that directory: claude mcp remove --scope project sync82`, e o escopo de usuário é então removido diretamente. O `install` faz o mesmo antes do `claude mcp add`.
- O `codex` roda `codex mcp get sync82` e, quando registrado, `codex mcp remove sync82`.
- Alvos de arquivo (incluindo o `opencode`) têm a chave `sync82` apagada de todo arquivo de config que a tenha, mantendo todas as outras chaves — mesmo quando o cliente de um alvo de arquivo nomeado explicitamente não é mais detectado. Um arquivo com comentários ou vírgulas sobrando não é alterado e o alvo falha com uma mensagem para remover a entrada à mão.

**`--purge`** roda depois dos alvos. Ele avisa quais vaults estão configurados fora de `~/.sync82` (`SYNC82_DB_PATH`, `vaultPath`/`lastVaultPath` no `config.json`, o `path` de um `.sync82.json` no diretório atual ou num pai) — esses **nunca são apagados** — e então lista os arquivos existentes entre `knowledge.db`, `knowledge.db-wal`, `knowledge.db-shm`, `config.json`, `config.lock` e `config.json.corrupt-*` em `~/.sync82`, e pergunta `Delete these files? [y/N]`. Imprime `Purge complete.`, `Purge cancelled.`, ou `Purge incomplete.` quando um arquivo não pôde ser apagado. O próprio `~/.sync82` só é removido quando fica vazio. Feche antes todo cliente MCP que usa o sync82.

**Códigos de saída:**

| Código | Significado |
|---|---|
| `0` | Todo alvo foi removido ou pulado sem problema, e o purge (se houver) terminou ou foi recusado — também quando o usuário recusou o primeiro prompt de confirmação |
| `1` | Um alvo desconhecido, um stdin fechado num prompt de confirmação, pelo menos um alvo falhou, ou um purge incompleto |
| `2` | Erro de uso: uma flag desconhecida ou mais de um alvo |

Veja [Desinstalação](../getting-started/uninstallation.md) para uma remoção completa, binário incluído.

## `config`

Gerencia o override global do caminho do vault, registrado em `~/.sync82/config.json` — o mesmo arquivo que rastreia o último projeto usado pra [resolução de contexto](../architecture/context-resolution.md).

```bash
sync82 config set-vault <caminho>    # usa um banco de vault não-padrão em toda sessão
sync82 config get-vault              # mostra o caminho configurado atualmente, se houver
sync82 config unset-vault            # volta ao padrão (~/.sync82/knowledge.db)
```

`<caminho>` aceita a mesma expansão `HOME`/`$HOME`/`~` que o argumento opcional `path` de toda tool, e é gravado como caminho absoluto (um relativo é resolvido a partir do diretório atual). Ele vale para toda chamada de tool e para os comandos `export`/`import`; um argumento `path` explícito (ou `--path`), ou o `.sync82.json` de um workspace, ainda tem prioridade sobre ele, e ele tem prioridade sobre `SYNC82_DB_PATH` e o padrão. Um projeto vindo da última sessão abre no vault em que foi lembrado (`lastVaultPath`), e não neste.

Atualizações do `config.json` feitas por vários processos do sync82 ao mesmo tempo (ex. dois clientes MCP) são serializadas com um arquivo de lock, `~/.sync82/config.lock`, e uma atualização que não muda nada não regrava o arquivo. Um `config.json` vazio ou com JSON inválido é movido para `config.json.corrupt-<unix-timestamp>` na próxima atualização (`set-vault`/`unset-vault`, ou uma chamada de tool que registra o último projeto usado), e uma config nova é iniciada — veja [Solução de Problemas](../troubleshooting/common-issues.md#um-arquivo-configjsoncorrupt--apareceu-em-sync82).

**Códigos de saída:** `0` em sucesso, `1` numa falha de leitura/escrita da config, `2` (erro de uso) num subcomando de `config` desconhecido ou ausente, num caminho ausente no `set-vault` ou em argumentos sobrando.

## `self-update`

Verifica o GitHub Releases por uma versão mais nova e substitui o binário em disco no lugar. As versões são comparadas pela precedência semver completa, incluindo identificadores de pré-release: `v1.2.3` é mais nova que `v1.2.3-rc.1`, `rc.10` mais nova que `rc.2`, e metadados de build (`+...`) são ignorados.

```bash
sync82 self-update            # verifica, mostra atual → mais recente, confirma, depois atualiza
sync82 self-update --check    # só verifica, não baixa nem instala nada
sync82 self-update --yes      # pula o prompt de confirmação (também aceita -y)
sync82 self-update --rollback # volta pra versão anterior guardada pela última atualização
```

Qualquer outra opção é erro de uso (código de saída `2`). Sem `--yes`, um stdin fechado no prompt `Update now? [y/N]` é um erro (código de saída `1`) que sugere `--yes`.

`self-update` só funciona num binário que conhece sua versão — um build de release, ou um build via `go install github.com/oito2/mcp-sync82/cmd/sync82@vX.Y.Z`. Um build local a partir de um checkout do código-fonte, ou um `go install` de um commit sem tag, informa `dev` e se recusa a rodar. A atualização:

1. Busca a release mais recente na API do GitHub (User-Agent `sync82/<versão>`, timeout de 30 s; uma resposta de limite de requisições do GitHub é informada como tal).
2. Baixa o binário da plataforma correspondente (`sync82_<so>_<arquitetura>`, `.exe` no Windows, timeout de 5 min) e o `checksums.txt` — só URLs HTTPS em `github.com` ou `*.githubusercontent.com` são aceitas, tanto a URL inicial quanto cada redirecionamento seguido (no máximo 10). O binário é preparado num diretório temporário `.sync82-update-*` **ao lado do binário em execução**, pra que o rename final nunca atravesse sistemas de arquivos (ex. um `/tmp` em tmpfs).
3. Verifica o checksum SHA-256 antes de mexer em qualquer coisa. O checksum detecta um download corrompido ou incompleto; o `checksums.txt` vem da mesma release, então a assinatura Sigstore dele não é conferida aqui — pra confirmar que uma release foi gerada por este repositório, verifique à mão como mostrado em [Instalação — Verificando a assinatura](../getting-started/installation.md#verificando-a-assinatura-opcional).
4. Roda o novo binário com `--version` (timeout de 10 s); ele precisa imprimir exatamente a tag da release, senão nada é alterado.
5. Renomeia o binário em execução pra `<binário>.bak` e depois renomeia o novo pro lugar dele — se esse segundo passo falhar, o original é restaurado. Funciona igual no Windows, onde um `.exe` em execução pode ser renomeado.

Em caso de sucesso, imprime onde a versão anterior foi guardada e como desfazer a atualização com `sync82 self-update --rollback`. Como o download é preparado ao lado do binário, o `self-update` precisa de permissão de escrita no diretório do binário; um erro de permissão sugere rodar de novo com privilégios elevados ou reinstalar via `go install`.

`sync82 self-update --rollback` troca o binário pelo `<binário>.bak`, depois de verificar que o backup roda (`--version`). Rodar de novo troca de volta. Falha com código de saída `1` se não houver backup. Voltar para antes de uma release que atualizou o schema do vault (como a 1.1.0) deixa o binário restaurado sem conseguir abrir um vault que o mais novo já abriu — veja [Solução de Problemas](../troubleshooting/common-issues.md#a-versão-do-schema-do-vault-é-mais-nova-do-que-este-sync82-suporta).

A atualização entra em vigor na **próxima vez** que algo iniciar o `sync82` do zero (ex. o próximo reinício do seu cliente MCP) — o processo do servidor rodando no momento não é afetado.

**Códigos de saída:**

| Código | Significado |
|---|---|
| `0` | Já está atualizado, atualização disponível (com `--check`), atualização concluída, rollback concluído, ou o usuário recusou o prompt de confirmação |
| `1` | Erro (falha de rede, checksum não bate, build de desenvolvimento, permissão negada, stdin fechado no prompt, ...) |
| `2` | Erro de uso: uma opção desconhecida |

> **Escolha um mecanismo de atualização e mantenha-o.** Se você instalou via `go install` e depois roda `self-update`, o binário deixa de ser "gerenciado" pelo `go install` — rodar `go install .../sync82@latest` de novo depois vai sobrescrevê-lo de volta silenciosamente. Os dois caminhos não têm consciência um do outro.

## `export`

Extrai a memória de um projeto pra arquivos Markdown simples em disco — o equivalente em CLI da tool [`export_memory`](./tools.md#export_memory), pra scripts ou backups pontuais sem passar por um cliente MCP.

```bash
sync82 export <projeto> [subprojeto] <pasta-de-saída>
sync82 export <projeto> [subprojeto] <pasta-de-saída> --path <vault>
sync82 export --all <pasta-de-saída>                    # todo projeto/subprojeto do vault
sync82 export --all <pasta-de-saída> --path <vault>
```

Escreve um arquivo `.md` por kind (`memory.md`, `progress.md`, etc.) em `<pasta-de-saída>`, no mesmo formato de arquivos simples que o projeto usava antes de migrar pro SQLite. Arquivos existentes lá são sobrescritos, mas um destino que seja symlink ou arquivo especial é recusado. Um kind com entradas arquivadas também ganha um arquivo `<kind>.archived.md` com elas. O vault precisa já existir — um caminho de vault onde não existe nenhum é um erro, não uma exportação vazia. `--all` dispensa nomear um projeto por completo — percorre todo projeto de nível superior e subprojeto no vault, escrevendo cada um na sua própria subpasta `<pasta-de-saída>/<projeto>[/<subprojeto>]`, pra um backup completo do vault em um único comando.

**Códigos de saída:** `0` em sucesso (inclusive "nada pra exportar"), `2` (erro de uso) num argumento ausente ou sobrando ou numa opção desconhecida, `1` num vault inexistente, um projeto desconhecido, um nome de projeto que não é um nome de pasta válido, ou um erro de sistema de arquivos.

## `import`

Restaura a memória de um projeto a partir de arquivos Markdown simples — o inverso de `export`, e o equivalente em CLI da tool [`import_memory`](./tools.md#import_memory).

```bash
sync82 import <projeto> [subprojeto] <pasta-de-entrada>
sync82 import <projeto> [subprojeto] <pasta-de-entrada> --path <vault>
sync82 import <projeto> [subprojeto] <pasta-de-entrada> --dry-run   # reporta o que aconteceria, sem escrever nada (nem um vault novo)
```

Lê todo arquivo `"<kind>.md"` em `<pasta-de-entrada>` e o escreve em `<projeto>` (criando-o primeiro se ainda não existir). `<projeto>` e `[subprojeto]` precisam começar com letra ou dígito e conter só letras, dígitos, hífens e underscores. Symlinks, arquivos especiais e arquivos acima de 10 MB são pulados. Um arquivo `"<kind>.archived.md"` restaura as entradas arquivadas daquele kind. Arquivos que não são nomes de kind válidos, ou estão vazios, são pulados e listados na saída. **Destrutivo por kind**: o conteúdo de um kind já existente é sobrescrito, não mesclado; entradas arquivadas que já estão no vault são mantidas, a menos que a pasta tenha um `<kind>.archived.md` daquele kind. A importação é tudo ou nada: todo arquivo é lido e verificado primeiro, depois tudo é escrito numa única transação, então uma falha deixa o vault inalterado. A saída lista `New:` (arquivos que criam um kind), `Overwritten:` (arquivos que substituem um kind existente) e `Skipped:`; com `--dry-run` ela mostra o mesmo relatório sem escrever nada. Dois arquivos que viram o mesmo kind depois de convertidos para minúsculas (ex. `Memory.md` e `memory.md`) fazem a importação falhar.

```bash
# Ciclo completo: faça backup de um projeto, depois restaure-o num vault novo
sync82 export acme ./backup
sync82 import acme ./backup --path ./vault-restaurado.db
```

**Códigos de saída:** `0` em sucesso, `2` (erro de uso) num argumento ausente ou sobrando, numa opção desconhecida ou num nome de projeto/subprojeto inválido, `1` numa `<pasta-de-entrada>` ausente ou num erro de sistema de arquivos.

---

[← Voltar ao Índice](../index.md)
