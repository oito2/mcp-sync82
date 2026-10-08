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
documents_fts      — índice de texto completo sobre documents.content
entries_fts        — índice de texto completo sobre entries.body
```

**`projects`** — um projeto é uma linha com `parent_id NULL`; um subprojeto é uma linha cujo `parent_id` aponta pro seu pai. Só um nível de aninhamento é suportado (um subprojeto não pode ter subprojetos próprios). Dois índices únicos parciais garantem que nomes de projetos de nível superior são únicos entre si, e nomes de subprojetos são únicos dentro do seu pai — `NULL != NULL` em SQL significa que uma única constraint ingênua `UNIQUE(parent_id, name)` não impediria de fato dois projetos de nível superior compartilhando um nome.

**`documents`** — uma linha por `(project_id, kind)` pros quatro kinds de sobrescrita (`memory`, `architecture`, `stack`, `next_steps`) mais qualquer kind customizado escrito com `write_memory` em modo de sobrescrita. Uma escrita substitui `content` e `updated_at` no lugar; nenhum histórico de revisão é mantido — uma versão anterior do schema arquivava cada versão prévia de um documento, mas nada jamais lia de volta, então foi removido.

**`entries`** — uma linha por entrada datada (ou não-datada) pros dois kinds só-anexa (`progress`, `decisions`) mais qualquer kind customizado de anexação. `entry_date` é `NULL` pra conteúdo não-datado — essas entradas ordenam por último e nunca são pegas por `archive_memory`. `position` dá uma ordenação estável entre entradas que compartilham a mesma data. O `id` da linha é o id de entrada que `read_memory` (`with_ids: true`, com `archived: true` para as linhas arquivadas), `search_memory` (`entry_id` no JSON) e `edit_entry` usam: único dentro do vault e inalterado enquanto a linha existir, mas não estável depois de reescrever o kind inteiro (`write_memory`, `update_project_memory`) ou de um `import_memory`, que apagam as linhas e inserem novas. Ids nunca são reaproveitados: `entries` e `projects` têm chaves `AUTOINCREMENT`, então o id de uma linha apagada nunca é dado a uma nova. O `edit_entry` só altera uma linha cujo `project_id` e `kind` batem com a chamada, então um id de outro projeto ou arquivo é reportado como não encontrado.

**`schema_migrations`** — toda versão de schema aplicada a esse vault, cada uma registrada uma vez. A versão 1 cria as tabelas acima; a versão 2 converte para minúsculas os nomes de projeto, subprojeto e kind já armazenados (veja [Nomes](#nomes) abaixo); a versão 3 cria os índices de texto completo e indexa as linhas que já existem (veja [Busca de texto completo](#busca-de-texto-completo) abaixo); a versão 4 recria `entries` e `projects` com chaves `AUTOINCREMENT`, mantendo todos os ids, para que ids nunca sejam reaproveitados. O framework de migração permite que uma mudança de schema seja aplicada com segurança a vaults que já existem, sem re-executar declarações não-idempotentes contra eles.

**`documents_fts`, `entries_fts`** — índices de texto completo FTS5 com conteúdo externo: guardam só o índice, chaveado pelo `id` da linha de `documents`/`entries`, e leem o texto dessa linha. Os dois usam o tokenizer `unicode61 remove_diacritics 2`, então maiúsculas/minúsculas e os acentos de letras latinas são ignorados. Veja [Busca de texto completo](#busca-de-texto-completo).

## Busca de texto completo

O `search_memory` nos modos `words` e `phrase` consulta `documents_fts` e `entries_fts` numa única instrução SQL, ordenada pela relevância do `bm25()` e depois por projeto, kind e ordem de leitura, então a ordem é a mesma em toda chamada. Entradas arquivadas continuam indexadas e são filtradas pela consulta. O modo `exact` encontra as linhas com um `LIKE` sem diferenciar maiúsculas/minúsculas, como antes da versão 3, mas antes as restringe pelos índices quando a consulta tem uma palavra que precisa aparecer inteira em toda linha encontrada: toda palavra menos a primeira (que o texto pode estender à esquerda, como `o_bar` em `foo_barbaz`), a última como prefixo. Só entram palavras em ASCII ou nos blocos latino, grego e cirílico, onde o índice comprovadamente divide e normaliza o texto como o sync82. Uma consulta sem uma palavra assim — `100%`, uma palavra só, ou texto em outra escrita — lê todas as linhas.

Gatilhos mantêm os índices em sincronia com toda mudança na tabela deles: `AFTER INSERT`, `AFTER DELETE` (inclusive as exclusões em cascata de um projeto apagado) e `AFTER UPDATE OF content`/`body`. Arquivar uma entrada muda só `archived`, então não mexe no índice.

A consulta digitada pelo usuário nunca é passada ao FTS5 como está. O sync82 a divide em palavras do jeito que o tokenizer divide o texto (letras, números e os 25 acentos combinantes que o `unicode61` remove; todo o resto — inclusive outras marcas combinantes, como sinais vocálicos do devanágari, pontos do hebraico ou harakat do árabe — separa palavras), põe cada palavra entre aspas e só mantém um `*` no final como marcador de prefixo, então operadores do FTS5 não podem ser injetados. As linhas encontradas são calculadas em Go com uma função de normalização (decomposição Unicode do `golang.org/x/text`, em cache por caractere) que produz os mesmos tokens que o tokenizer; um teste compara os dois, caractere por caractere, nos blocos latino, grego e cirílico. Fora deles, ainda podem divergir em caracteres para os quais as tabelas Unicode do SQLite são mais antigas que as do Go (letras e símbolos recentes, como `₽`): `words` e `phrase` seguem o índice; quando ele não acha nada para uma consulta com um caractere assim, o `SearchText` seleciona as linhas de novo com cada palavra como substring do texto normalizado pela mesma função em Go (a função SQL `sync82_fold`), mantém as linhas cujos tokens casam como o índice casaria e reporta os resultados na ordem de leitura (`SearchInfo.Substring`). Essa varredura lê todas as linhas do escopo.

A migração 3 cria os índices quando um vault existente é aberto pela primeira vez por um sync82 que os tem. Um sync82 mais antigo se recusa a abrir um vault cuja versão de schema é mais nova do que ele suporta, em vez de gravar nele sem manter os índices em sincronia — veja [Solução de Problemas](../troubleshooting/common-issues.md#a-versão-do-schema-do-vault-é-mais-nova-do-que-este-sync82-suporta).

## Nomes

Nomes de projeto, subprojeto e kind (`filename`) não diferenciam maiúsculas de minúsculas: toda tool remove espaços das pontas e converte o nome para minúsculas antes de usá-lo, então `Acme` e `acme` são o mesmo projeto, `Memory` e `memory` o mesmo arquivo, e os nomes sempre aparecem em minúsculas. A versão 2 do schema converte para minúsculas os nomes de um vault existente na primeira abertura; um nome cuja forma em minúsculas já existe no mesmo escopo (outro projeto de nível superior, um subprojeto irmão, outro kind do mesmo projeto) fica como está. Entradas de um kind só-anexa sempre passam para o kind em minúsculas, então o log datado dele continua num lugar só.

## Kinds padrão vs. customizados

Todo projeto ganha seis kinds padrão no `init_project_memory`: `memory`, `architecture`, `stack`, `next_steps` (estilo sobrescrita, armazenados em `documents`) e `decisions`, `progress` (só-anexa, armazenados em `entries`). Além desses seis, qualquer tool que recebe um argumento `filename` aceita um nome customizado arbitrário — `write_memory`/`append_memory` decidem se um kind customizado *novo* é de sobrescrita ou de anexação com base em qual tool o criou. `delete_memory` se recusa a apagar qualquer um dos seis kinds padrão; só os customizados podem ser removidos.

## Concorrência

Toda requisição MCP recebida roda em sua própria goroutine (modelo de transporte do SDK subjacente), então múltiplas chamadas de tool podem executar concorrentemente contra o mesmo vault. Cada `*Store` mantém uma única conexão com o banco, então as chamadas dele esperam pela conexão dentro do processo em vez de disputar o lock de escrita do SQLite; o modo `WAL` mais o `busy_timeout` cobrem a disputa que resta entre processos (vários clientes usando o mesmo vault). `store.Manager` mantém em cache um `*Store` por vault path resolvido, então chamadas concorrentes contra vaults diferentes também não se bloqueiam entre si.

Toda escrita busca o seu projeto dentro da mesma transação que escreve, então um projeto apagado ou recriado por outra chamada (ou outro processo) entre a busca e a escrita não recebe uma escrita obsoleta: a escrita falha com o erro de projeto "não encontrado" em vez de um erro de restrição do banco.

Leituras não usam transação: uma leitura busca o projeto e depois lê as linhas dele em comandos separados. Um projeto apagado nesse meio-tempo dá um resultado vazio ou "não encontrado", nunca as linhas de outro projeto, já que ids de projeto e de entrada nunca são reutilizados (`AUTOINCREMENT`, schema versão 4).

---

## Veja também

- [Resolução de Contexto](./context-resolution.md) — como uma chamada de tool resolve para um projeto e um vault path
- [Instalador](./installer.md) — como cada cliente MCP é apontado para um vault do sync82
- [Conceitos — Arquitetura](../concepts/architecture.md) — o panorama geral

---

[🏠 Voltar ao Índice](../index.md)
