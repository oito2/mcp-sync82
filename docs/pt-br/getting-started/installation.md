🌐 [English](../../en/getting-started/installation.md) | **Português** | 🏠 [Índice](../index.md)

---

# Guia de Instalação

**sync82** é distribuído como um único binário estático — sem runtime, sem gerenciador de dependências, sem `node_modules`. Baixe uma release, ou compile a partir do código-fonte se tiver o Go instalado.

---

## 📋 Pré-requisitos

| Componente | Versão mínima | Notas |
| :--- | :--- | :--- |
| Go (só para build a partir do código) | a mesma de [`go.mod`](../../../go.mod) | Não necessário se você baixar um binário de release |
| Sistema operacional | Linux, macOS, Windows | Binários pré-compilados para amd64/arm64 |

Você também vai precisar de um **cliente MCP compatível** para interagir com o servidor. O `sync82 install` configura automaticamente qualquer um destes que detectar:

- [Claude Code](../guides/clients/claude-code.md)
- [Claude Desktop](../guides/clients/claude-desktop.md) — também instalável como extensão de um clique, veja [abaixo](#-claude-desktop-o-bundle-mcpb)
- [Antigravity](../guides/clients/antigravity.md)
- [OpenAI Codex](../guides/clients/codex.md)
- [OpenCode](../guides/clients/opencode.md)
- [Cursor](../guides/clients/cursor.md)
- [Zed](../guides/clients/zed.md)
- [Cline](../guides/clients/cline.md)

---

## 🚀 Instalando o sync82

Duas formas de obter o binário em qualquer sistema — as duas funcionam, mas não misture os mecanismos de atualização (veja [Referência da CLI — self-update](../reference/cli.md#self-update)).

### 🐧 Linux

**Opção A — baixar um binário de release (recomendado, sem precisar de toolchain Go):**

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/sync82_linux_amd64   # ou sync82_linux_arm64
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing   # deve aprovar sync82_linux_amd64 ("OK" ou "SUCESSO")
chmod +x sync82_linux_amd64
sudo mv sync82_linux_amd64 /usr/local/bin/sync82
```

**Opção B — compilar a partir do código-fonte:**

```bash
# precisa de Go 1.26+; o pacote golang-go do Debian/Ubuntu costuma ser mais antigo,
# então instale uma toolchain atual em https://go.dev/dl/ se o `go version` for menor que 1.26

go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

### 🍎 macOS

**Opção A — baixar um binário de release (recomendado, sem precisar de toolchain Go):**

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/sync82_darwin_arm64   # Intel: sync82_darwin_amd64
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt
grep ' sync82_darwin_arm64$' checksums.txt | shasum -a 256 -c   # deve imprimir "sync82_darwin_arm64: OK"
chmod +x sync82_darwin_arm64
sudo mv sync82_darwin_arm64 /usr/local/bin/sync82
```

**Opção B — compilar a partir do código-fonte:**

```bash
# Homebrew, se ainda não tiver uma toolchain Go compatível com o go.mod:
brew install go

go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

### 🪟 Windows

**Opção A — baixar um binário de release (recomendado, sem precisar de toolchain Go):**

```powershell
$base = "https://github.com/oito2/mcp-sync82/releases/latest/download"
Invoke-WebRequest -Uri "$base/sync82_windows_amd64.exe" -OutFile sync82_windows_amd64.exe
Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile checksums.txt
$expected = ((Select-String -Path checksums.txt -SimpleMatch "sync82_windows_amd64.exe").Line -split '\s+')[0]
if ((Get-FileHash sync82_windows_amd64.exe -Algorithm SHA256).Hash -ne $expected) { throw "checksum não confere" }
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\sync82" | Out-Null
Move-Item sync82_windows_amd64.exe "$env:LOCALAPPDATA\sync82\sync82.exe"
# adicione $env:LOCALAPPDATA\sync82 ao PATH: Propriedades do Sistema > Variáveis de Ambiente
```

**Opção B — compilar a partir do código-fonte:**

```powershell
# ainda não tem uma toolchain Go compatível com o go.mod? baixe o instalador em https://go.dev/dl/

go install github.com/oito2/mcp-sync82/cmd/sync82@latest
```

---

Compilar a partir do código-fonte (Opção B em qualquer SO) instala o `sync82` em `$(go env GOPATH)/bin` (ou `$GOBIN`/`%GOBIN%`) — garanta que esse diretório esteja no seu `PATH`.

> Não existe um "passo de build" separado depois — `go build`/`go install` já produz o binário final, pronto para rodar. `CGO_ENABLED=0` funciona sem problemas, já que o driver SQLite (`modernc.org/sqlite`) é puro Go.

### 🔒 Verificando um Binário Baixado

Toda release inclui um `checksums.txt` com o SHA-256 de cada binário. Os passos acima já o verificam antes de mover o binário para o lugar: `sha256sum -c` (Linux), `shasum -a 256 -c` (macOS) ou `Get-FileHash` (Windows) precisa dar certo antes de continuar.

#### Verificando a Assinatura (Opcional)

O próprio `checksums.txt` é assinado pelo workflow de release com uma assinatura keyless do [Sigstore](https://www.sigstore.dev/), publicada como `checksums.txt.sigstore.json`. Com o [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/) v3 instalado:

```bash
curl -LO https://github.com/oito2/mcp-sync82/releases/latest/download/checksums.txt.sigstore.json
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/oito2/mcp-sync82/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

`Verified OK` significa que o `checksums.txt` foi gerado pelo workflow de release deste repositório a partir de uma tag de versão.

#### Verificando a Proveniência do Build (Opcional)

Todo binário e o bundle `sync82.mcpb` também têm uma [atestação de artefato do GitHub](https://docs.github.com/pt/actions/security-for-github-actions/using-artifact-attestations) (proveniência de build SLSA) registrando a execução do workflow que os gerou. Com a [GitHub CLI](https://cli.github.com/) instalada e autenticada:

```bash
gh attestation verify sync82_linux_amd64 --repo oito2/mcp-sync82
```

Passe o arquivo que você baixou (`sync82_darwin_arm64`, `sync82_windows_amd64.exe`, `sync82.mcpb`, ...). Uma verificação bem-sucedida confirma que o arquivo foi gerado pelo GitHub Actions deste repositório.

> O `sync82 self-update` verifica cada download contra o `checksums.txt` (SHA-256) e, quando o `cosign` v3 ou mais novo está instalado, também verifica esta assinatura para a tag exata da release; `--require-signature` faz ele recusar atualizar sem o cosign. Ele não confere a atestação.

### Confirme que está acessível

```bash
which sync82   # ou `where sync82` no Windows
```

Se não retornar nada, o binário ainda não está no `PATH` — adicione o diretório dele ao `PATH` para poder rodar `sync82` no seu próprio shell. As configurações dos clientes MCP não dependem disso: o `sync82 install` registra o caminho absoluto do binário.

---

## 🔌 Registrando o Servidor MCP

O sync82 pode configurar seu cliente MCP para você:

```bash
sync82 install [target]
```

Rode sem target para listar os clientes suportados detectados na máquina (`Detected the following MCP clients:`) e, depois de `Install sync82 into all N detected client(s)? [y/N]`, configurar cada um deles — um cliente é detectado quando o comando dele está no `PATH` ou o diretório de config dele existe (veja [Detecção](../architecture/installer.md#detecção)); sem nenhum detectado, imprime `No supported MCP clients detected. Supported targets: ...`. Ou passe um target explicitamente: `claude`, `claude-desktop`, `antigravity`, `codex`, `opencode`, `cursor`, `zed`, `cline`. Um target explícito cujo cliente não é detectado é pulado (`Skipped: <target> not detected.`), não tratado como erro. Cada target configurado imprime `configured.` para um registro novo ou `updated.` para um existente. Todo target é registrado com o caminho absoluto do binário `sync82` que você executou — rode `sync82 install` de novo depois de mover o binário. Veja [Instalador](../architecture/installer.md#targets) para o arquivo exato que cada target escreve, e os guias de cliente acima se preferir configurar um cliente manualmente.

Para desfazer, rode `sync82 uninstall` — veja [Desinstalação](./uninstallation.md).

### 📦 Claude Desktop: o bundle `.mcpb`

Quem usa o Claude Desktop pode pular o binário: toda release inclui o `sync82.mcpb`, um bundle de extensão do Claude Desktop com os binários do sync82 para macOS, Windows e Linux dentro.

1. Baixe <https://github.com/oito2/mcp-sync82/releases/latest/download/sync82.mcpb>.
2. No Claude Desktop, abra **Settings → Extensions → Advanced settings → Extension Developer**, clique em **Install Extension…** e selecione o arquivo.
3. Opcionalmente, defina **Vault database path** nas configurações da extensão (padrão `~/.sync82/knowledge.db`).

O bundle é atualizado instalando um `sync82.mcpb` mais novo, não com `sync82 self-update`. Ele também está publicado no [MCP Registry](https://registry.modelcontextprotocol.io/) como `io.github.oito2/mcp-sync82`. Veja o [guia do Claude Desktop](../guides/clients/claude-desktop.md).

---

## ⚙️ Escolhendo a Localização do Vault (Opcional)

Por padrão, o sync82 armazena tudo em um único arquivo SQLite em `~/.sync82/knowledge.db`. Para usar um caminho diferente em toda sessão sem passar `path` em cada chamada de tool, configure uma vez:

```bash
sync82 config set-vault /caminho/para/seu/vault.db
```

Veja [Resolução de Contexto](../architecture/context-resolution.md) para a ordem de prioridade completa entre isso, a variável de ambiente `SYNC82_DB_PATH`, o `.sync82.json` de um workspace, e um argumento `path` explícito.

---

## 🔍 Verificando a Instalação

Depois de instalar e registrar o cliente MCP, verifique se o servidor funciona perguntando ao seu assistente de IA:

```
Liste todos os projetos no meu vault do sync82.
```

A IA vai chamar a tool `list_projects`. Uma lista vazia é esperada em um vault novo — isso confirma que a conexão e o caminho do vault estão funcionando.

---

## ⬆️ Mantendo o sync82 Atualizado

```bash
sync82 self-update --check   # reporta se existe uma release mais nova, sem instalar
sync82 self-update           # baixa, verifica e instala a última release
```

Os downloads são verificados por checksum contra o `checksums.txt` da release, cuja assinatura também é verificada quando o `cosign` v3 ou mais novo está instalado, antes de o binário em execução ser substituído; a versão anterior é guardada como `<binário>.bak`, e `sync82 self-update --rollback` a restaura. O `self-update` funciona em um binário de release ou em um compilado com `go install .../sync82@vX.Y.Z`; só um build local a partir de um checkout do código-fonte ou um `go install` de um commit sem tag (versão `dev`) recusa rodar — nesse caso, rode `go install .../sync82@latest` novamente.

> **Escolha um mecanismo de atualização e mantenha-o** — os dois caminhos não têm consciência um do outro. Veja [Referência da CLI — self-update](../reference/cli.md#self-update) para detalhes.

---

## ➡️ Próximos passos

Com o servidor instalado, configure seu cliente MCP:

- [Configurar Claude Code](../guides/clients/claude-code.md)
- [Configurar Claude Desktop](../guides/clients/claude-desktop.md)
- [Configurar Antigravity](../guides/clients/antigravity.md)
- [Configurar OpenAI Codex](../guides/clients/codex.md)
- [Configurar OpenCode](../guides/clients/opencode.md)
- [Configurar Cursor](../guides/clients/cursor.md)
- [Configurar Zed](../guides/clients/zed.md)
- [Configurar Cline](../guides/clients/cline.md)

Ou vá direto para o uso:

- [Início Rápido](./quickstart.md)
- [Voltar ao Índice](../index.md)
