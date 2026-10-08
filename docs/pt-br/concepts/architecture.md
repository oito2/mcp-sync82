🌐 [English](../../en/concepts/architecture.md) | **Português** | 🏠 [Índice](../index.md)

---

# Arquitetura

Visão geral dos componentes do `sync82`, o fluxo de dados e o modelo de armazenamento.

---

## Estrutura do projeto

```
mcp-sync82/
├── go.mod, go.sum        ← definição do módulo e dependências
├── CONTRIBUTING.md        ← setup de dev, checks de CI, processo de PR
├── docs/                 ← documentação completa (en/ e pt-br/), imagens e ícones (img/)
├── .github/workflows/    ← ci.yml (checks + contrato de release) e release.yml (tag → release → MCP Registry)
├── scripts/release/      ← o gerador de release: `go run ./scripts/release vX.Y.Z` escreve dist/
├── cmd/sync82/          ← main.go — dispatch da CLI: serve (padrão), install, uninstall, config, self-update, export, import, search, context, health
└── internal/
    ├── server/           ← construção do servidor MCP, registro de tools, resources e prompts, ícone do servidor, tratamento de erro JSON-RPC
    ├── tools/            ← um arquivo por tool — as 19 tools documentadas em reference/tools.md; registry.go as lista (tools.Registered)
    ├── store/            ← persistência SQLite: projects, documents, entries, schema_migrations, índices de texto completo
    ├── config/           ← config global (~/.sync82/config.json) e descoberta de config local (.sync82.json)
    ├── analyzer/         ← auto-detecção de stack/descrição, usada pelo auto_detect do init_project_memory
    ├── installer/        ← o instalador e desinstalador de 8 targets de cliente MCP (incl. --purge)
    ├── selfupdate/       ← self-update: checagem de GitHub Releases, verificação de checksum, substituição de binário
    ├── cli/               ← o subcomando `config set-vault|get-vault|unset-vault`
    ├── prompt/            ← as perguntas sim/não de confirmação do install, uninstall e self-update (um Ctrl-C as encerra na hora)
    ├── fsutil/            ← helper compartilhado de escrita atômica de arquivo
    ├── binpath/           ← caminho absoluto, com symlinks resolvidos, do binário em execução (install, self-update)
    ├── version/           ← a string de versão (injetada no build de release, ou lida das informações de build)
    ├── logging/           ← logger somente-stderr
    └── */                 ← uma suíte `_test.go` por pacote (go test -race ./...)
```

---

## Fluxo de dados

```
1. Cliente de IA chama uma tool (ex. update_project_memory)
   └─ internal/tools/context.go resolve project/subproject/path
      através do modelo de 4 camadas (argumento explícito >
      .sync82.json > config global > pergunta)

2. Tool chama internal/store
   └─ store.Manager mantém em cache um *Store por vault path resolvido
   └─ kinds do tipo sobrescrita (memory, architecture, stack,
      next_steps) vão para a tabela documents
   └─ kinds append-only (progress, decisions) vão para a
      tabela entries, uma linha por entrada datada

3. internal/server envolve o resultado
   └─ uma falha de Validate() vira isError:true (o agente vê uma
      mensagem útil, a conexão permanece saudável)
   └─ uma falha de Execute() (ou um pânico) também vira isError:true;
      só uma falha ao abrir o vault tem os detalhes de baixo nível
      (caminhos, erros do SQLite) trocados por uma mensagem genérica —
      o erro completo vai para o log do servidor

4. Resultado retornado ao cliente de IA via stdio
   └─ tools de listagem chamadas com format: "json" também
      devolvem os mesmos dados como structuredContent
```

A lista de tools do servidor é uma única função, `tools.Registered` em `internal/tools/registry.go`: o `sync82` (serve) a passa para o `internal/server`, e o gerador de release sobe um servidor em memória com a mesma lista para escrever as tools no manifesto do `.mcpb` — assim o bundle nunca lista uma tool que o binário não tem. Os [resources](../reference/resources.md) (`tools.ResourceTemplates`, lidos por `tools.Resources`) e os [prompts MCP](../reference/mcp-prompts.md) (`tools.Prompts`) também são definidos em `internal/tools`, sem depender do SDK MCP; o `server.New` recebe as tools, os prompts, os templates e os resources num `server.Options` e só os adapta ao SDK. Os resources leem o vault padrão diretamente, sem a resolução de projeto das tools, e nunca gravam. As anotações MCP das tools (somente leitura, destrutiva, idempotente) vêm de uma única tabela, `internal/tools/hints.go`, conferida por um teste contra as tools registradas.

---

## Modelo de armazenamento

Tudo vive em um único arquivo de banco SQLite embutido — um arquivo é o vault inteiro. Localização padrão `~/.sync82/knowledge.db`, sobrescrevível via a variável de ambiente `SYNC82_DB_PATH`, `sync82 config set-vault`, o `.sync82.json` de um workspace, ou o argumento `path` de uma chamada de tool. Toda conexão define `PRAGMA journal_mode=WAL` e `PRAGMA foreign_keys=ON`.

Para o schema completo (tabelas, índices, e por que têm esse formato), veja [Storage](../architecture/storage.md).

---

## Transporte

O sync82 suporta **apenas stdio** — sem transporte HTTP/rede, sem flags para habilitar um. Rodar o binário sem argumentos inicia o servidor MCP via stdio; esse é também o único modo que todo target do `sync82 install` configura. Isso mantém a inicialização instantânea e evita a necessidade de autenticação ou de uma porta escutando.

---

## Pipeline de release

As releases são geradas por um programa Go, `scripts/release`, e dois workflows do GitHub Actions:

```
go run ./scripts/release vX.Y.Z   → dist/
   ├─ sync82_{linux,darwin,windows}_{amd64,arm64}[.exe]   6 binários (CGO_ENABLED=0, -trimpath)
   ├─ sync82.mcpb       bundle do Claude Desktop (manifesto 0.3): binário universal de macOS,
   │                    launcher Linux que escolhe amd64/arm64, binário Windows amd64, ícones,
   │                    user_config.db_path opcional → SYNC82_DB_PATH
   ├─ checksums.txt     SHA-256 dos 6 binários e do sync82.mcpb
   └─ server.json       entrada do MCP Registry io.github.oito2/mcp-sync82 (aponta para o .mcpb)

ci.yml (push/PR para main)
   test (Linux, macOS, Windows) · release-contract: scripts/release -allow-prerelease v0.0.0-ci,
   teste de contrato do self-update, mcp-publisher validate

release.yml (tag vX.Y.Z)
   validate → checks (Linux, macOS, Windows) → build → sign-publish → publish-registry
                                               │       │              └─ mcp-publisher login github-oidc + publish
                                               │       └─ escrita + OIDC, sem código do repositório:
                                               │          baixa o dist/, sha256sum --check,
                                               │          cosign sign-blob → checksums.txt.sigstore.json,
                                               │          cosign verify-blob (como o self-update faz),
                                               │          atestações de proveniência de build (binários + .mcpb),
                                               │          gh release create
                                               └─ somente leitura: scripts/release, --version == tag,
                                                  teste de contrato do self-update, mcp-publisher validate,
                                                  envia o dist/
```

Os nomes dos assets não levam versão, então `releases/latest/download/<nome>` sempre aponta para a release mais nova — as URLs que as instruções de instalação usam. Veja [Contribuindo — Publicando uma release](../contribuindo.md#publicando-uma-release).

---

## Veja também

- [Como o sync82 funciona](./how-sync82-works.md) — o pipeline de chamada de tool
- [Storage](../architecture/storage.md) — aprofundamento no schema SQLite
- [Resolução de Contexto](../architecture/context-resolution.md) — o modelo de resolução em 4 camadas
- [Instalador](../architecture/installer.md) — como os 8 targets de cliente são configurados e removidos
- [Referência de Configuração](../reference/configuration.md) — toda fonte de configuração

---

[🏠 Voltar ao Índice](../index.md)
