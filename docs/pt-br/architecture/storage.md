🌐 [English](../../en/architecture/storage.md) | **Português** | 🏠 [Índice](../index.md)

---

# Storage

Como o vault é de fato armazenado em disco. Útil se você está depurando algum comportamento, contribuindo, ou só curioso.

---

## Modelo de armazenamento

Tudo vive num único arquivo de banco SQLite embutido — um arquivo é o vault inteiro. Local padrão `~/.sync82/knowledge.db`, sobrescrevível via a variável de ambiente `SYNC82_DB_PATH`, `sync82 config set-vault`, o `.sync82.json` de um workspace, ou o argumento `path` de uma chamada de tool (nessa ordem crescente de prioridade — veja [Resolução de Contexto](./context-resolution.md)). Um arquivo de vault novo, seus companheiros `-wal`/`-shm` e um diretório de vault recém-criado só podem ser lidos pelo usuário dono (`0600`/`0700`), assim como o `~/.sync82/config.json`.

Toda conexão define `PRAGMA journal_mode=WAL` e `PRAGMA foreign_keys=ON`.

## Tabelas

```
projects           — a árvore de projeto/subprojeto
documents          — arquivos de memória de sobrescrita (memory, architecture, stack, next_steps, customizados)
entries            — entradas de memória só-anexa (progress, decisions, customizadas)
schema_migrations  — rastreia quais migrations versionadas de schema já rodaram
```

**`projects`** — um projeto é uma linha com `parent_id NULL`; um subprojeto é uma linha cujo `parent_id` aponta pro seu pai. Só um nível de aninhamento é suportado (um subprojeto não pode ter subprojetos próprios). Dois índices únicos parciais garantem que nomes de projetos de nível superior são únicos entre si, e nomes de subprojetos são únicos dentro do seu pai — `NULL != NULL` em SQL significa que uma única constraint ingênua `UNIQUE(parent_id, name)` não impediria de fato dois projetos de nível superior compartilhando um nome.

**`documents`** — uma linha por `(project_id, kind)` pros quatro kinds de sobrescrita (`memory`, `architecture`, `stack`, `next_steps`) mais qualquer kind customizado escrito com `write_memory` em modo de sobrescrita. Uma escrita substitui `content` e `updated_at` no lugar; nenhum histórico de revisão é mantido — uma versão anterior do schema arquivava cada versão prévia de um documento, mas nada jamais lia de volta, então foi removido.

**`entries`** — uma linha por entrada datada (ou não-datada) pros dois kinds só-anexa (`progress`, `decisions`) mais qualquer kind customizado de anexação. `entry_date` é `NULL` pra conteúdo não-datado — essas entradas ordenam por último e nunca são pegas por `archive_memory`. `position` dá uma ordenação estável entre entradas que compartilham a mesma data.

**`schema_migrations`** — toda versão de schema aplicada a esse vault, cada uma registrada uma vez. A versão 1 cria as tabelas acima; a versão 2 converte para minúsculas os nomes de projeto, subprojeto e kind já armazenados (veja [Nomes](#nomes) abaixo). O framework de migração permite que uma mudança de schema seja aplicada com segurança a vaults que já existem, sem re-executar declarações não-idempotentes contra eles.

## Nomes

Nomes de projeto, subprojeto e kind (`filename`) não diferenciam maiúsculas de minúsculas: toda tool remove espaços das pontas e converte o nome para minúsculas antes de usá-lo, então `Acme` e `acme` são o mesmo projeto, `Memory` e `memory` o mesmo arquivo, e os nomes sempre aparecem em minúsculas. A versão 2 do schema converte para minúsculas os nomes de um vault existente na primeira abertura; um nome cuja forma em minúsculas já existe no mesmo escopo (outro projeto de nível superior, um subprojeto irmão, outro kind do mesmo projeto) fica como está. Entradas de um kind só-anexa sempre passam para o kind em minúsculas, então o log datado dele continua num lugar só.

## Kinds padrão vs. customizados

Todo projeto ganha seis kinds padrão no `init_project_memory`: `memory`, `architecture`, `stack`, `next_steps` (estilo sobrescrita, armazenados em `documents`) e `decisions`, `progress` (só-anexa, armazenados em `entries`). Além desses seis, qualquer tool que recebe um argumento `filename` aceita um nome customizado arbitrário — `write_memory`/`append_memory` decidem se um kind customizado *novo* é de sobrescrita ou de anexação com base em qual tool o criou. `delete_memory` se recusa a apagar qualquer um dos seis kinds padrão; só os customizados podem ser removidos.

## Concorrência

Toda requisição MCP recebida roda em sua própria goroutine (modelo de transporte do SDK subjacente), então múltiplas chamadas de tool podem executar concorrentemente contra o mesmo vault. O modo `WAL` mais o `busy_timeout` em toda conexão é o que torna isso seguro sem nenhum lock adicional no nível da aplicação — `store.Manager` mantém em cache um `*Store` por vault path resolvido, então chamadas concorrentes contra vaults diferentes também não se bloqueiam entre si.

---

## Veja também

- [Resolução de Contexto](./context-resolution.md) — como uma chamada de tool resolve para um projeto e um vault path
- [Instalador](./installer.md) — como cada cliente MCP é apontado para um vault do sync82
- [Conceitos — Arquitetura](../concepts/architecture.md) — o panorama geral

---

[🏠 Voltar ao Índice](../index.md)
