🌐 [English](../../en/architecture/context-resolution.md) | **Português** | 🏠 [Índice](../index.md)

---

# Resolução de Contexto

O modelo de 4 camadas que o sync82 usa para determinar a qual projeto uma chamada de tool se refere, e contra qual vault ela opera.

---

## Resolução de projeto

A maioria das tools recebe argumentos opcionais `project`/`subproject`/`workspace_root` em vez de exigir `project` em toda chamada. Quando `project` não é dado diretamente, ele é resolvido em quatro camadas, em ordem:

1. **Argumento explícito** — `project` (e `subproject`) passados diretamente na chamada da tool.
2. **Config local** — `.sync82.json` no próprio `workspace_root`, se dado. Escrito pelo `init_project_memory` quando roda pela primeira vez com um `workspace_root`, então chamadas futuras naquele workspace resolvem automaticamente. Por padrão, essa camada só checa `workspace_root` exatamente — ela **não** sobe para diretórios ancestrais, porque um `.sync82.json` encontrado em um ancestral que o chamador não controla poderia redirecionar silenciosamente para onde a memória é armazenada (seu campo `path` é confiado sem confirmação). Passe `search_parent_dirs: true` para habilitar a busca ascendente (útil em monorepos, onde o arquivo de marcador fica na raiz do repositório) — até 64 diretórios acima, ou a raiz do filesystem, o que vier primeiro. **Quando `workspace_root` é dado, a resolução para nesta camada:** um `.sync82.json` ausente, vazio (sem `project`) ou ilegível nunca cai na camada 3 — a tool pede `project` em vez disso, e explica o motivo quando o arquivo não pôde ser lido. O último projeto de outra sessão nunca é usado num workspace que se identificou.
3. **Config global** — só quando nem `project` nem `workspace_root` são dados: o último projeto/subprojeto usado, e o vault em que ele está, registrados em `~/.sync82/config.json`. Ele é registrado depois de uma chamada que nomeou um projeto que **existe** no seu vault (um erro de digitação nunca vira o padrão), e usado nesse mesmo vault, a menos que `path` seja dado. O registro é melhor esforço — uma falha ao persistir isso nunca faz a chamada da tool falhar. `rename_project` o atualiza quando renomeia o projeto lembrado, e `import_memory`, `delete_memory` e `archive_memory` se recusam a agir sobre ele, já que sobrescrevem ou removem dados.
4. **Perguntar** — se nada acima resolver, a tool retorna uma mensagem (não um erro) pedindo ao agente chamador que forneça `project` ou `workspace_root`.

Um argumento `subproject` explícito é mantido em todas as camadas: `workspace_root` resolvendo para `acme` mais `subproject: "api"` aponta para `acme/api`.

```
project foi dado diretamente?
   │
   ├─ sim ──────────────────────────────► usa ele
   │
   não
   │
   ▼
workspace_root foi dado?
   │
   ├─ sim ─► .sync82.json com um project encontrado lá
   │         (ou, com search_parent_dirs: true, em um diretório ancestral)?
   │            ├─ sim ─────────────────► usa ele
   │            └─ não ─────────────────► pergunta (nunca a camada 3)
   │
   não
   │
   ▼
~/.sync82/config.json tem um último projeto usado?
   │
   ├─ sim ──────────────────────────────► usa ele
   │
   não
   │
   ▼
retorna uma mensagem pedindo ao agente `project` ou `workspace_root`
```

Sempre que um projeto é auto-descoberto pela camada 2 ou 3 (não passado explicitamente), o sufixo `ContextNote` da resposta da tool reporta tanto o projeto quanto o caminho do vault resolvido — ex.: `[project: acme, from .sync82.json, vault: /home/user/.sync82/knowledge.db]` — para que um `path` customizado aplicado no caminho nunca fique silenciosamente invisível.

### A exceção do `search_memory`

`search_memory` é a única exceção deliberada: ela nunca cai na camada 3 sozinha. Uma busca sem escopo deve buscar o vault inteiro, não adivinhar silenciosamente "o último projeto em que você estava trabalhando" — então ela só usa a auto-descoberta de config local (camada 2) quando `workspace_root` é dado explicitamente, e nunca cai de volta pro último projeto usado globalmente por conta própria. Um workspace sem `.sync82.json` busca o vault inteiro; um cujo `.sync82.json` não pode ser lido ou nomeia um projeto inválido dá um resultado de erro.

---

## Resolução do caminho do vault

O **caminho** do vault em si resolve independentemente do projeto, nesta ordem de prioridade:

1. Um argumento `path` explícito na chamada da tool.
2. Um caminho customizado registrado no `.sync82.json` resolvido.
3. Um caminho customizado registrado na config global (`sync82 config set-vault`).
4. A variável de ambiente `SYNC82_DB_PATH`, ou o padrão fixo no código (`~/.sync82/knowledge.db`).

Essa é uma resolução completamente separada da resolução de projeto acima — você pode sobrescrever o caminho do vault sem tocar em qual projeto é resolvido, e vice-versa. A única ligação entre as duas é a camada 3: um projeto resolvido pela config global é aberto no vault em que foi lembrado (ainda sobrescrevível por `path`).

Um `path` relativo no `.sync82.json` é resolvido a partir do diretório que contém esse `.sync82.json`, não do diretório de trabalho do servidor. `sync82 config set-vault` grava o caminho como absoluto.

---

## Veja também

- [Referência de Configuração](../reference/configuration.md) — todo campo do `config.json` e do `.sync82.json`
- [Storage](./storage.md) — o que de fato vive no vault resolvido
- [Instalador](./installer.md) — como cada cliente é apontado para o sync82 em primeiro lugar
- [Referência de Tools](../reference/tools.md) — os argumentos `project`/`subproject`/`workspace_root`/`search_parent_dirs`/`path` em toda tool

---

[🏠 Voltar ao Índice](../index.md)
