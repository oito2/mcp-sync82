🌐 [English](../../en/reference/configuration.md) | **Português** | 🏠 [Índice](../index.md)

---

# Referência de Configuração

Toda fonte de configuração que o sync82 lê. O sync82 **não tem flags de linha de comando** para o servidor e tem **uma variável de ambiente**; todo o resto fica em dois pequenos arquivos JSON. Para ver como essas fontes se combinam numa chamada de tool, veja [Resolução de Contexto](../architecture/context-resolution.md).

---

## Visão geral

| Fonte | Escopo | Escrita por | O que define |
|---|---|---|---|
| `SYNC82_DB_PATH` | processo (por entrada de cliente MCP) | você, ou o bundle do Claude Desktop | caminho padrão do vault |
| `~/.sync82/config.json` | usuário (todo cliente, todo projeto) | `sync82 config set-vault`/`unset-vault`, e chamadas de tool | caminho global do vault, último projeto usado |
| `.sync82.json` | um workspace | `init_project_memory` (com `workspace_root`), ou você | projeto, subprojeto e caminho do vault daquele workspace |
| argumento `path` da tool / flag `--path` | uma chamada | o agente, ou você na CLI | caminho do vault daquela chamada |
| `user_config.db_path` do MCPB | extensão do Claude Desktop | configurações da extensão no Claude Desktop | repassado ao servidor como `SYNC82_DB_PATH` |

---

## Variáveis de ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `SYNC82_DB_PATH` | `~/.sync82/knowledge.db` | Arquivo de vault usado quando nenhum argumento `path`, `path` do `.sync82.json` ou `vaultPath` global se aplica. Um `~`, `~/`, `HOME`, `HOME/`, `$HOME` ou `$HOME/` no início é expandido para o diretório home. Lida uma vez, quando o servidor (ou `export`/`import`) inicia. |

Para defini-la em um cliente, adicione-a à entrada do servidor naquele cliente, ex. numa config no estilo `mcpServers`:

```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": [],
      "env": { "SYNC82_DB_PATH": "~/vaults/work.db" }
    }
  }
}
```

O `sync82 uninstall --purge` também lê `SYNC82_DB_PATH`, só para listar esse vault como um que ele não toca.

---

## `~/.sync82/config.json` (config global)

Criado sob demanda em `~/.sync82/` (diretório com modo `0700`, arquivo com modo `0600`). Um arquivo ausente equivale a uma config vazia.

| Campo | Tipo | Escrito por | Descrição |
|---|---|---|---|
| `vaultPath` | string | `sync82 config set-vault <caminho>` / removido por `unset-vault` | Override global do caminho do vault, guardado como caminho absoluto. |
| `lastProject` | string | chamadas de tool | Último projeto nomeado por uma chamada de tool que existe no vault dele — camada 3 da resolução de contexto. |
| `lastSubproject` | string | chamadas de tool | Subprojeto de `lastProject`, se houver. |
| `lastVaultPath` | string | chamadas de tool | Caminho absoluto do vault onde `lastProject` está. Um projeto resolvido pela config global é aberto nesse vault, a menos que a chamada passe `path`. |

Todo campo é omitido quando vazio. Exemplo:

```json
{
  "lastProject": "acme",
  "lastSubproject": "api",
  "lastVaultPath": "/home/user/.sync82/knowledge.db",
  "vaultPath": "/home/user/vaults/work.db"
}
```

**`config.lock`** — toda atualização do `config.json` segura um lock consultivo exclusivo em `~/.sync82/config.lock` (criado com modo `0600`), então vários processos do sync82 (um por cliente MCP, mais a CLI) nunca sobrescrevem a atualização um do outro. Uma atualização que não muda nada não reescreve o arquivo.

**Recuperação de arquivo corrompido** — quando uma atualização encontra um `config.json` vazio, só com espaços ou que não é JSON válido, ela o renomeia para `config.json.corrupt-<timestamp-unix>` (mantendo as permissões) e continua a partir de uma config vazia. Até a próxima atualização, as tools que só leem o arquivo registram o erro de parse no log e pulam as camadas globais. Veja [Solução de Problemas](../troubleshooting/common-issues.md#um-arquivo-configjsoncorrupt--apareceu-em-sync82).

---

## `.sync82.json` (config do workspace)

Um arquivo marcador na raiz de um workspace, lido quando uma chamada de tool passa `workspace_root` (e, com `search_parent_dirs: true`, procurado em até 64 diretórios pais). O `init_project_memory` o escreve (modo `0644`) quando chamado com `workspace_root`; você também pode escrevê-lo à mão.

| Campo | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project` | string | sim | Projeto ao qual este workspace corresponde. Um arquivo sem ele é ignorado. |
| `subproject` | string | não | Subprojeto de `project`. Um argumento `subproject` explícito na chamada prevalece sobre ele. |
| `path` | string | não | Caminho do vault deste workspace. Prefixos `~`/`HOME`/`$HOME` são expandidos; um caminho **relativo** é resolvido a partir do diretório que contém o `.sync82.json`, não do diretório de trabalho do servidor. |

```json
{
  "project": "moodle",
  "subproject": "mod_quiz",
  "path": "../vaults/moodle.db"
}
```

Um `.sync82.json` que existe mas não pode ser lido faz a chamada pedir `project` e explicar o motivo — nunca cai para o último projeto usado. `init_project_memory` e `search_memory` devolvem um resultado de erro em vez disso, e `init_project_memory` nunca sobrescreve esse arquivo.

---

## Precedência

**Projeto:** argumento `project` explícito → `.sync82.json` em `workspace_root` → `lastProject` do `config.json` (só quando nem `project` nem `workspace_root` são passados) → a tool pede um.

**Caminho do vault:**

1. Argumento `path` na chamada de tool (`--path` no `export`/`import`).
2. `path` do `.sync82.json` resolvido.
3. `vaultPath` do `config.json` (`sync82 config set-vault`).
4. `SYNC82_DB_PATH`.
5. `~/.sync82/knowledge.db`.

Quando o próprio projeto veio de `lastProject`, o `lastVaultPath` dele é usado no lugar dos passos 2–5. Regras completas e diagramas: [Resolução de Contexto](../architecture/context-resolution.md).

---

## Transporte

O sync82 serve MCP **somente via stdio**: rodar `sync82` sem argumentos inicia o servidor. Não há transporte HTTP, SSE ou Streamable HTTP, nem flag ou variável para ativar um.

---

## Bundle do Claude Desktop (`sync82.mcpb`)

O bundle `sync82.mcpb` declara uma configuração de usuário opcional, exibida pelo Claude Desktop nas configurações da extensão:

| Chave | Título | Tipo | Padrão | Efeito |
|---|---|---|---|---|
| `db_path` | Vault database path | string | `~/.sync82/knowledge.db` | Repassado ao servidor como `SYNC82_DB_PATH`. Um `~` no início é expandido; o arquivo é criado se não existir. |

Como ela chega como `SYNC82_DB_PATH`, um `vaultPath` definido com `sync82 config set-vault` e o `.sync82.json` de um workspace continuam tendo precedência sobre ela.

---

## Veja também

- [Referência da CLI — config](./cli.md#config) — `set-vault`, `get-vault`, `unset-vault`
- [Resolução de Contexto](../architecture/context-resolution.md) — como essas fontes se combinam
- [Referência de Tools](./tools.md) — os argumentos `path`, `workspace_root` e `search_parent_dirs`
