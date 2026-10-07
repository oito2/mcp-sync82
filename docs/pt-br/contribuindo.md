# Contribuindo com o sync82

🌐 **Idioma:** [English](../../CONTRIBUTING.md) · Português

Obrigado por considerar contribuir. Este documento cobre os passos práticos: como compilar e testar o projeto, o que se espera de um pull request, e onde tirar dúvidas.

Ao participar deste projeto, você concorda em seguir o [Código de Conduta](codigo-de-conduta.md).

## Antes de começar

Para qualquer coisa além de uma correção pequena (novas tools, novos comandos de CLI, mudanças de schema, adição de dependências), abra uma issue primeiro para discutir a abordagem. Este projeto tem uma filosofia de **dependências mínimas e justificadas** — veja [Conceitos — Arquitetura](concepts/architecture.md) e o bloco `require` em [`go.mod`](../../go.mod) para a base atual (`go-sdk`, `modernc.org/sqlite` e `golang.org/x/text`, para a normalização Unicode da busca de texto completo, todas puro Go, sem cgo). Um PR que adiciona uma nova dependência sem discussão prévia provavelmente será solicitado a removê-la.

## Configurando o ambiente

Requisito: um toolchain Go compatível com a versão em [`go.mod`](../../go.mod).

```bash
git clone https://github.com/oito2/mcp-sync82
cd mcp-sync82
go build ./...
```

Rode os mesmos checks que o CI roda antes de abrir um PR:

```bash
gofmt -l .        # não deve imprimir nada
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...   # precisa informar "0 issues."
go build ./...
go test ./... -race
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Os seis precisam passar — [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) os roda a cada push e pull request contra `main` e bloqueia o merge caso contrário. Os testes rodam em Linux, macOS e Windows (formatação, golangci-lint e `govulncheck` só no Linux). O golangci-lint usa o `.golangci.yml` do repositório e a versão fixada em `GOLANGCI_LINT_VERSION` nos workflows — atualize lá e no comando acima juntos; alguns testes compilam o binário real e conversam com ele via stdio — `go test -short ./...` pula esses. A CI também gera um conjunto completo de artefatos de release com `scripts/release` (o programa que o workflow de release usa), confere que o `self-update` consegue encontrar, verificar e executar os binários, e valida o `server.json` com o `mcp-publisher` — veja [Publicando uma release](#publicando-uma-release).

## Fazendo uma mudança

1. Faça um fork do repositório e crie um branch a partir de `main`.
2. Mantenha a mudança focada — uma mudança lógica por PR. Limpezas não relacionadas tornam a revisão mais lenta, não mais rápida.
3. Siga o estilo de código e a organização de pacotes existente (veja [Conceitos — Arquitetura](concepts/architecture.md) para como os pacotes são organizados: uma tool por arquivo em `internal/tools`, injeção de dependência para tudo que toca rede/filesystem/`os.Executable()` fora do store, etc.).
4. Adicione ou atualize testes para o comportamento que você mudou. Este projeto depende de `go test ./... -race` como rede de segurança — comportamento sem teste é considerado quebrado.
5. Atualize a documentação relevante em [`docs/en/`](../en/) e sua contraparte em [`docs/pt-br/`](.) (veja [Documentação](#documentação) abaixo) se você mudou os parâmetros de uma tool, um comando de CLI, ou a arquitetura.
6. Rode os checks de [Configurando o ambiente](#configurando-o-ambiente) localmente.

## Mensagens de commit

Escreva uma linha de resumo concisa explicando *por que* a mudança foi feita, não apenas o que mudou — o diff já mostra o que mudou. Mantenha mudanças relacionadas em um único commit em vez de uma sequência de commits "fix".

## Documentação

Documentos de nível raiz neste repositório (README, CONTRIBUTING, CODE_OF_CONDUCT) são publicados em inglês (canônico, na raiz) e português (mesmo nome em minúsculo e traduzido, dentro de [`docs/pt-br/`](.) — `leiame.md`, `contribuindo.md`, `codigo-de-conduta.md` —, mantido sincronizado); o site de referência em `docs/` é dividido em árvores paralelas [`docs/en/`](../en/) e [`docs/pt-br/`](.) do mesmo jeito. Se sua mudança afeta comportamento descrito na [Referência de Tools](reference/tools.md), [Referência da CLI](reference/cli.md), [Guia de Instalação](getting-started/installation.md), [Arquitetura](concepts/architecture.md), ou no README, atualize as duas versões de idioma no mesmo PR — um PR que atualiza só uma será solicitado a adicionar a outra.

Adicione uma entrada em `[Unreleased]` no [`CHANGELOG.md`](../../CHANGELOG.md) para toda mudança visível ao usuário (refatorações internas e mudanças só de testes não precisam). O changelog segue o [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/) e é mantido só em inglês.

## Pull requests

- Descreva o que mudou e por quê na descrição do PR; vincule a issue relacionada, se existir.
- Mantenha o PR restrito à mudança discutida — refatorações grandes e não solicitadas provavelmente serão recusadas mesmo que o código em si esteja correto, seguindo a preferência deste projeto por mudanças mínimas e precisas.
- Um mantenedor vai revisar, pedir ajustes se necessário, e fazer o merge quando o CI estiver verde e a discussão estiver resolvida.

## Publicando uma release

As releases são geradas pelo `scripts/release` e publicadas pelo [`.github/workflows/release.yml`](../../.github/workflows/release.yml). Só para mantenedores.

**Teste local** — gera todos os artefatos em `dist/` sem publicar nada:

```bash
go run ./scripts/release v1.2.3
```

O `dist/` passa a ter os 6 binários (`sync82_{linux,darwin,windows}_{amd64,arm64}`, `.exe` no Windows), o `sync82.mcpb` (o bundle do Claude Desktop), o `checksums.txt` e o `server.json` (a entrada do MCP Registry). A versão precisa ser `vMAJOR.MINOR.PATCH`; `-allow-prerelease` aceita também um sufixo como `v0.0.0-ci` (é assim que a CI o roda).

**Publicando** — primeiro, no `CHANGELOG.md`, renomeie `[Unreleased]` para `[X.Y.Z] - AAAA-MM-DD`, adicione um `[Unreleased]` vazio acima dele, atualize os links de comparação no fim do arquivo e faça o commit. Depois faça push de uma tag `vMAJOR.MINOR.PATCH` para disparar o workflow:

```bash
git tag v1.2.3
git push origin v1.2.3
```

| Job | O que faz |
|---|---|
| `validate` | Rejeita uma tag que não seja exatamente `vMAJOR.MINOR.PATCH`. |
| `checks` | Roda as checagens da CI (gofmt, vet, golangci-lint, build, `go test -race`, govulncheck) no commit da tag. |
| `release` | Roda `go run ./scripts/release <tag>`, valida o `server.json` com `mcp-publisher validate`, assina o `checksums.txt` com `cosign sign-blob` (keyless) em `checksums.txt.sigstore.json`, registra atestações de proveniência de build para `dist/sync82_*` e `dist/sync82.mcpb`, e cria a release do GitHub com todos os arquivos de `dist/`. |
| `publish-registry` | Baixa o `server.json` da release, faz login no MCP Registry com OIDC do GitHub e o publica. |

O `publish-registry` é um job separado para poder ser executado de novo sozinho (ex. depois de uma indisponibilidade do registry) sem recriar a release.

**Atualizando o `mcp-publisher`** — a versão é fixada por `MCP_PUBLISHER_VERSION` e o SHA-256 do `mcp-publisher_linux_amd64.tar.gz` dela por `MCP_PUBLISHER_SHA256`, tanto no `release.yml` quanto no `ci.yml`. Atualize os dois juntos, nos dois arquivos; um hash que não confere faz a checagem do download falhar.

## Reportando bugs e sugerindo funcionalidades

Abra uma [issue no GitHub](https://github.com/oito2/mcp-sync82/issues) com:

- Para bugs: o que você rodou (subcomando do `sync82` ou chamada de tool MCP), o que esperava, o que aconteceu de fato, e a saída de `sync82 version`.
- Para funcionalidades: o problema que você está tentando resolver, não só a solução que você tem em mente — veja [Antes de começar](#antes-de-começar).

## Licença

Ao contribuir, você concorda que suas contribuições serão licenciadas sob a [GNU General Public License v3.0](../../LICENSE), a mesma licença que cobre o restante do projeto.
