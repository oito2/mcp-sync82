🌐 [English](../../en/reference/tools.md) | **Português** | 🏠 [Índice](../index.md)

---

# Referência de Tools

O sync82 expõe 19 tools via MCP. Toda tool que opera sobre um projeto específico aceita os mesmos argumentos de contexto — `project`, `subproject`, `workspace_root`, `search_parent_dirs` — resolvidos pelo mesmo modelo de [resolução de contexto em 4 camadas](../architecture/context-resolution.md), então eles são documentados uma vez aqui em vez de repetidos em cada tabela abaixo.

**Argumentos de contexto comuns** (todos opcionais, presentes em toda tool com escopo de projeto):

| Argumento | Tipo | Descrição |
|---|---|---|
| `project` | string | Nome do projeto. Se omitido, é auto-descoberto a partir de `workspace_root` ou do último projeto usado. |
| `subproject` | string | Nome do subprojeto, para um componente de um projeto existente. |
| `workspace_root` | string | Caminho da pasta do seu projeto, usado para auto-descobrir o projeto via `.sync82.json`. |
| `search_parent_dirs` | boolean | Se `true`, também procura `.sync82.json` em diretórios acima de `workspace_root` (útil em monorepos, onde o arquivo de marcador fica na raiz do repositório). O padrão é `false` — só `workspace_root` é checado, já que um `.sync82.json` encontrado em um ancestral que você não controla poderia redirecionar silenciosamente para onde a memória é armazenada. |
| `path` | string | Caminho base onde a memória é armazenada. Se deixado em branco, usa o caminho padrão do vault. Um `~`, `HOME` ou `$HOME` no início (ex. `"~/vaults/trabalho.db"`, `"HOME/vault-customizado"` e, no Windows, também `"~\\vaults\\trabalho.db"`) é expandido para o diretório do usuário. |

Se nenhum de `project`, `subproject`, `workspace_root` resolver para um projeto e não houver um último projeto usado registrado, a tool retorna uma mensagem pedindo ao agente chamador um nome de projeto ou `workspace_root`, em vez de falhar.

Nomes de projeto e subprojeto — passados, lidos do `.sync82.json` ou lembrados — precisam começar com letra ou dígito, conter só letras, dígitos, hífens e underscores e ter no máximo 128 caracteres — a mesma regra dos nomes de kind (`filename`); qualquer outro nome é recusado com um resultado de erro. Nomes não diferenciam maiúsculas de minúsculas: nomes de projeto, subprojeto e kind (`filename`) têm os espaços das pontas removidos e são convertidos para minúsculas em todo lugar, então `Acme` e `acme` são o mesmo projeto e `Memory` e `memory` o mesmo arquivo, e eles sempre aparecem em minúsculas (vaults existentes são migrados — veja [Armazenamento — Nomes](../architecture/storage.md#nomes)). Só `create_project`, `init_project_memory` e `import_memory` (exceto num dry run) criam um arquivo de vault; toda outra tool reporta um `path` (ou caminho do `.sync82.json`/`set-vault`) onde não existe vault, em vez de criar um vault vazio lá.

Toda tool recusa argumentos que ela não define — ex. `keepDays` em vez de `keep_days` — com um resultado de erro, e o schema de entrada dela define `additionalProperties: false`.

O servidor se identifica aos clientes como `sync82`, com a versão e um ícone PNG de 64×64 (embutido como URI `data:` no `serverInfo`), para que clientes que mostram ícones de servidor possam exibi-lo.

Sempre que um projeto é auto-descoberto em vez de passado explicitamente, o texto da resposta da tool inclui um sufixo `[project: ..., from ..., vault: ...]` reportando o caminho do vault resolvido — para que um `path` customizado aplicado no caminho (ex.: vindo de `.sync82.json`) nunca fique silenciosamente invisível.

---

## Gerenciamento de projetos

### `list_projects`

Lista todo projeto e subprojeto atualmente no vault.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `path` | string | não | Override do caminho do vault. |
| `format` | string (`text` \| `json`) | não | `text` (padrão) para texto legível; `json` para um documento JSON com a mesma informação, devolvido como texto e como conteúdo estruturado — veja [Saída JSON](#saída-json). |

---

### `create_project`

Cria um novo projeto (ou subprojeto de um projeto existente) no vault. Seguro chamar de novo — um projeto já existente é reportado, não duplicado.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project` | string | ✅ | Nome do projeto (letras, dígitos, hífens, underscores — deve começar com letra ou dígito). |
| `subproject` | string | ❌ | Nome do subprojeto. Quando fornecido, cria-o sob `project` em vez da raiz do vault. Mesma regra de formato de `project`. |
| `path` | string | ❌ | Override do caminho do vault. |

---

### `delete_project`

Apaga permanentemente um projeto ou subprojeto do vault. **Exige `confirm: true`** — o agente chamador é instruído a perguntar ao usuário antes de definir isso.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project` | string | ✅ | Nome do projeto a apagar. |
| `subproject` | string | ❌ | Nome do subprojeto. Quando fornecido, só aquele subprojeto é apagado. |
| `confirm` | boolean | ✅ | Deve ser `true` para confirmar a exclusão permanente. |
| `subproject_action` | string | condicional | Um de `cancel`, `promote`, `delete_all`. Obrigatório só ao apagar um projeto de nível superior que tem subprojetos. |
| `path` | string | ❌ | Override do caminho do vault. |

Se um projeto de nível superior tem subprojetos e `subproject_action` não é dado, a tool não apaga nada — retorna a lista de subprojetos e pergunta qual ação tomar: `cancel` (abortar), `promote` (mover cada subprojeto pra raiz do vault como projeto próprio), ou `delete_all` (apagar tudo).

`subproject_action` também pode ser passado na primeira chamada, junto com `confirm: true`: `delete_all` então apaga o projeto e todos os subprojetos sem listá-los antes. Pergunte ao usuário antes de passá-lo.

---

### `rename_project`

Renomeia um projeto ou subprojeto no lugar.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project` | string | ✅ | O projeto a renomear. Quando `subproject` também é dado, renomeia esse subprojeto em vez disso. |
| `subproject` | string | ❌ | O subprojeto a renomear, se estiver renomeando um subprojeto em vez do projeto de nível superior. |
| `new_name` | string | ✅ | O novo nome (letras, dígitos, hífens, underscores — deve começar com letra ou dígito, no máximo 128 caracteres). O nome atual é recusado. |
| `path` | string | ❌ | Override do caminho do vault. |

Não mexe em nenhum `.sync82.json` em outro lugar do disco que já aponte pro nome antigo — eles continuam se referindo a ele até serem reinicializados (`init_project_memory`) ou editados manualmente.

---

### `get_vault_config`

Reporta a configuração efetiva atual do vault: caminho ativo do vault, config global, e (se `workspace_root` for dado) a config local `.sync82.json` daquele workspace.

O relatório JSON também traz `last_vault_path`, o vault em que o último projeto usado foi lembrado (onde uma chamada que depende da última sessão o abre), e, em `local_config`, `vault`, o vault para o qual aquele workspace resolve.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `workspace_root` | string | ❌ | Se fornecido, o relatório também inclui a `.sync82.json` local daquele workspace, se existir. |
| `search_parent_dirs` | boolean | ❌ | Veja [Argumentos de contexto comuns](#referência-de-tools) acima — afeta se um `.sync82.json` ancestral entra no relatório. |
| `path` | string | ❌ | Override do caminho do vault. |

---

## Lendo e escrevendo memória

### `list_files`

Lista todo arquivo de memória (documento ou kind de entradas) registrado para um projeto.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `metadata` | boolean | ❌ | Quando `true`, inclui tamanho, tokens estimados, e data de última modificação por arquivo. |
| `path` | string | ❌ | Override do caminho do vault. |
| `format` | string (`text` \| `json`) | ❌ | `text` (padrão) para texto legível; `json` para um documento JSON com a mesma informação, devolvido como texto e como conteúdo estruturado — veja [Saída JSON](#saída-json). |

---

### `read_memory`

Lê o conteúdo de um arquivo de memória. Para um kind só-anexa (`progress`, `decisions`, ou um kind customizado de anexação), retorna toda entrada não-arquivada concatenada em ordem de data.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `filename` | string | ✅ | O arquivo/kind a ler (ex. `"memory"`, `"progress"`, ou um nome customizado). |
| `with_ids` | boolean | ❌ | Coloca uma linha `<!-- entry:N -->` antes de cada entrada de um kind só-anexa, com o id que o [`edit_entry`](#edit_entry) recebe. Não muda nada em arquivos de sobrescrita. |
| `max_bytes` | integer | ❌ | Limite de tamanho da resposta em bytes (1024 a 52428800, padrão 1048576 — 1 MB). Um conteúdo maior é cortado numa quebra de linha e termina com `[cut: N of M bytes shown; …]`; o [`load_project_context`](#load_project_context) com `since` ou `max_entries` lê parte de um log longo. |
| `path` | string | ❌ | Override do caminho do vault. |

As linhas `<!-- entry:N -->` nunca são gravadas: `write_memory`, `append_memory`, `update_project_memory`, `edit_entry` e `import_memory` removem linhas inteiras nesse formato do conteúdo que recebem, então um conteúdo lido com ids pode ser gravado de volta como está.

---

### `write_memory`

Sobrescreve o conteúdo inteiro de um arquivo de memória. **Destrutivo**: para kinds só-anexa (`progress`, `decisions`, ou um kind customizado criado com `append_memory`), isso substitui toda entrada não-arquivada, não só a última — use `append_memory` para adicionar sem perder entradas anteriores. Entradas arquivadas são mantidas. O projeto precisa vir de `project` ou `workspace_root`: um projeto tirado só da sessão anterior é recusado. Um kind mantém o armazenamento que já tem: um kind customizado criado com `append_memory` continua sendo um log de entradas datadas. Para um kind assim, o conteúdo é dividido em entradas nos cabeçalhos de data (veja [Cabeçalhos de data](#date-headers) abaixo).

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `filename` | string | ✅ | O arquivo/kind a sobrescrever. |
| `content` | string | ✅ | O novo conteúdo completo. |
| `path` | string | ❌ | Override do caminho do vault. |

---

### `append_memory`

Anexa uma nova entrada datada a um arquivo de memória só-anexa (`progress`, `decisions`, ou um kind customizado). `progress` e `decisions` **exigem** um cabeçalho de data `## YYYY-MM-DD` em algum lugar de `content`. Arquivos de sobrescrita (`memory`, `architecture`, `stack`, `next_steps`, ou um kind customizado criado com `write_memory`) são recusados com um resultado de erro — use `write_memory` para eles.

<a id="date-headers"></a>**Cabeçalhos de data:** um cabeçalho de data é `##`, espaços ou tabs, e a data na **mesma linha** (`## 2026-10-06`). Cabeçalhos dentro de blocos de código cercados (```` ``` ```` ou `~~~`) são ignorados. Se o primeiro cabeçalho tem uma data inválida (ex. `## 2026-13-01`), vale o primeiro cabeçalho válido depois dele. Um bloco aberto com ```` ``` ```` só fecha no próximo ```` ``` ````, e um aberto com `~~~` só no próximo `~~~`. As datas que o próprio sync82 grava ou compara — o corte e o cabeçalho de resumo do `archive_memory`, a marca de superação do `edit_entry`, as idades do `check_project_health` — usam o dia do calendário em UTC, que à noite (a oeste de UTC) já pode ser amanhã.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `filename` | string | ✅ | O arquivo/kind a anexar. |
| `content` | string | ✅ | O conteúdo a anexar. Para `progress`/`decisions`, deve conter um cabeçalho `## YYYY-MM-DD`. |
| `path` | string | ❌ | Override do caminho do vault. |

---

### `delete_memory`

Apaga permanentemente um arquivo de memória customizado. Os seis arquivos padrão (`memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps`) **não podem** ser apagados assim — use `write_memory` pra limpar o conteúdo deles em vez disso. O projeto precisa vir de `project` ou `workspace_root`: um projeto tirado só da sessão anterior é recusado.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `filename` | string | ✅ | O arquivo/kind customizado a apagar. |
| `confirm` | boolean | ✅ | Precisa ser `true` para confirmar a exclusão permanente. Pergunte ao usuário antes de definir. |
| `path` | string | ❌ | Override do caminho do vault. |

---

### `edit_entry`

Altera uma entrada de um arquivo de memória só-anexa (`progress`, `decisions`, ou um kind customizado de anexação) sem reescrever o resto. A entrada é identificada pelo id: leia com [`read_memory`](#read_memory) e `with_ids: true`, ou pegue o `entry_id` da saída JSON do [`search_memory`](#search_memory). O projeto precisa vir de `project` ou `workspace_root`: um projeto tirado só da sessão anterior é recusado.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `filename` | string | ✅ | O arquivo/kind só-anexa da entrada. Arquivos de sobrescrita são recusados — use `write_memory` para eles. |
| `entry_id` | integer | ✅ | O id da entrada (≥ 1). Precisa pertencer a este projeto, subprojeto e arquivo; qualquer outro id é reportado como não encontrado. |
| `action` | string (`replace` \| `supersede` \| `delete`) | ✅ | O que fazer com a entrada — veja abaixo. |
| `content` | string | para `replace` e `supersede` | O novo texto da entrada. Para `progress`/`decisions`, precisa ter um cabeçalho `## YYYY-MM-DD`, como no `append_memory`. Não é aceito com `delete`. |
| `confirm` | boolean | para `delete` | Precisa ser `true` para apagar. Pergunte ao usuário antes de definir. |
| `path` | string | ❌ | Override do caminho do vault. |

| Ação | Efeito |
|---|---|
| `replace` | Reescreve a entrada no lugar, mantendo a posição dela entre as entradas da mesma data. Para `progress`/`decisions`, a data da entrada passa a ser a do novo cabeçalho, então corrigir uma data errada move a entrada; para um kind customizado a data é mantida. |
| `supersede` | Anexa `content` como uma nova entrada e acrescenta uma linha `> Superseded by entry N on YYYY-MM-DD.` à antiga, numa única transação. O texto antigo continua no histórico, marcado. Prefira quando uma decisão mudou, e não quando foi registrada errada. |
| `delete` | Remove a entrada permanentemente. |

Os ids de entrada são únicos dentro de um vault e não mudam enquanto a entrada existir. Reescrever um arquivo inteiro (`write_memory`, `update_project_memory`) ou importá-lo (`import_memory`) cria entradas novas, com ids novos, então leia os ids de novo depois disso. Um id nunca é dado a outra entrada, nem depois que a entrada dele é apagada, então um id antigo é reportado como não encontrado em vez de alterar outra entrada. Entradas arquivadas também podem ser editadas, mas o `read_memory` não as mostra. Substituir (`supersede`) uma entrada arquivada a marca no arquivo morto e acrescenta a substituta como entrada ativa.

---

### `archive_memory`

Arquiva entradas datadas antigas de `progress` ou `decisions`, mantendo ativos só os últimos N dias. Entradas sem cabeçalho de data **nunca** são arquivadas. O projeto precisa vir de `project` ou `workspace_root`: um projeto tirado só da sessão anterior é recusado.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `filename` | string (`progress` \| `decisions`) | ✅ | Qual arquivo só-anexa arquivar. |
| `keep_days` | integer | ❌ | Entradas mais antigas que esse número de dias são arquivadas (padrão 90, de 1 a 36500). |
| `summary` | string | ❌ | Um resumo das entradas que estão sendo arquivadas, escrito pelo agente. É adicionado como uma nova entrada ativa na mesma transação que as arquiva. |
| `dry_run` | boolean | ❌ | Quando `true`, lista as entradas que seriam arquivadas — data, id da entrada e primeira linha, até 200 — sem mudar nada. |
| `path` | string | ❌ | Override do caminho do vault. |

Entradas arquivadas saem do `read_memory` e do `load_project_context`, então o que elas diziam deixa de estar no contexto do agente. O sync82 nunca escreve um resumo por conta própria (não faz chamadas a LLM), mas pode guardar um escrito pelo agente:

1. Chame com `dry_run: true` para ver quais entradas sairiam.
2. Leia essas entradas (`read_memory`) e escreva um resumo.
3. Chame de novo com o mesmo `keep_days` e o `summary`.

O resumo é gravado como veio quando tem um cabeçalho `## YYYY-MM-DD` próprio, que não pode ser anterior ao corte (hoje menos `keep_days`) — senão o próximo arquivamento o arquivaria também. Sem cabeçalho, ele ganha um datado de hoje que diz o que cobre: `## 2026-10-07 — Summary of 42 archived entries (from 2026-01-02 to 2026-07-08)`. Por estar datado de hoje, fica ativo por mais `keep_days` dias e aparece entre as entradas recentes. Quando nada é antigo o bastante para arquivar, o resumo não é gravado e o resultado avisa.

---

### `search_memory`

Busca nos arquivos de memória. Por padrão encontra os documentos e entradas que têm todas as palavras da consulta, em qualquer ordem, ignorando maiúsculas/minúsculas e acentos (`decisao` encontra `Decisão`), com os melhores resultados primeiro. Sem `project` busca o vault inteiro; `project` sozinho busca aquele projeto e todos os subprojetos; `project`+`subproject` busca só aquele subprojeto.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `query` | string | ✅ | O que buscar: palavras, uma frase ou uma substring literal, conforme `match`. Precisa ser uma única linha. |
| `match` | string (`words` \| `phrase` \| `exact`) | ❌ | Como a consulta casa — veja a tabela abaixo (padrão `words`). |
| `kinds` | array de strings | ❌ | Busca só nesses arquivos/kinds (ex. `["decisions"]`). |
| `since` | string (`YYYY-MM-DD`) | ❌ | Busca só entradas datadas nessa data ou depois. Documentos e entradas sem data ficam de fora. |
| `until` | string (`YYYY-MM-DD`) | ❌ | Busca só entradas datadas nessa data ou antes. Documentos e entradas sem data ficam de fora. |
| `project` | string | ❌ | Limita a busca a esse projeto (e seus subprojetos, a menos que `subproject` também seja dado). |
| `subproject` | string | ❌ | Limita a busca a esse subprojeto específico. Exige `project`, ou um `workspace_root` cujo `.sync82.json` informe o projeto — senão a chamada é um resultado de erro, e não uma busca no vault inteiro. |
| `workspace_root` | string | ❌ | Usado para auto-descobrir o projeto **só se** `project` não for dado diretamente — veja a nota abaixo. |
| `search_parent_dirs` | boolean | ❌ | Veja [Argumentos de contexto comuns](#referência-de-tools) acima. |
| `limit` | integer | ❌ | Número máximo de resultados a retornar (1–1000, padrão 100). |
| `offset` | integer | ❌ | Número de resultados a pular, para paginação (padrão 0). |
| `context_lines` | integer | ❌ | Número de linhas ao redor a incluir por resultado (0–20, padrão 0). |
| `path` | string | ❌ | Override do caminho do vault. |
| `format` | string (`text` \| `json`) | ❌ | `text` (padrão) para texto legível; `json` para um documento JSON com a mesma informação, devolvido como texto e como conteúdo estruturado — veja [Saída JSON](#saída-json). |

| `match` | Encontra | Ordem |
|---|---|---|
| `words` (padrão) | Documentos e entradas que têm **todas** as palavras da consulta, em qualquer lugar e em qualquer ordem. Maiúsculas/minúsculas e os acentos de letras latinas são ignorados (`sessao` encontra `Sessão`). Uma palavra terminada em `*` casa como prefixo (`instal*` encontra `instalador`). Pontuação só separa palavras: `edit_entry` busca `edit` e `entry`, e `"`, `NEAR`, `OR`, `:` ou `-` não têm significado especial. As palavras seguem o tokenizer `unicode61` do SQLite, então uma palavra com um símbolo recente (`100₽`) pode precisar de `exact`. | Relevância (melhores primeiro) |
| `phrase` | Documentos e entradas que têm as palavras **nessa ordem**, com as mesmas regras do `words` (`sobre o instal*` funciona). | Relevância (melhores primeiro) |
| `exact` | Linhas que têm a consulta como substring literal, sem diferenciar maiúsculas/minúsculas mas diferenciando acentos (`decisão` encontra `DECISÃO`, não `decisao`). Use para caminhos, identificadores ou pontuação (`100%`, `foo_bar`, `v1.0`). | Ordem do arquivo |

No modo `words`, cada linha que tem uma das palavras é reportada, então um documento com as palavras em linhas diferentes mostra cada uma dessas linhas. Uma frase que continua na linha seguinte é reportada pelas linhas que têm as palavras dela. Uma consulta sem letras nem números é recusada nos modos `words` e `phrase` — use `exact` para ela.

Cada resultado é rotulado `project/file:line`, ou `project/file[YYYY-MM-DD]:line` para uma entrada de um log datado (a linha é contada dentro daquela entrada). Os resultados vêm numa ordem estável — por relevância nos modos `words`/`phrase`, com empates resolvidos por projeto, arquivo e ordem de leitura; por projeto, arquivo e ordem de leitura no modo `exact` —, então as páginas de `offset` são consistentes. No formato texto, uma busca sem resultado responde `No results for "<consulta>"`, e um `offset` depois do último resultado `No more results for "<consulta>" at offset N`. Cada linha encontrada ou de contexto é cortada em 4 KB, terminando em `…`. A resposta é limitada a cerca de 1 MB como enviada, somando o texto e o conteúdo estruturado (com o escape do JSON); passando disso, ela termina com uma nota dizendo para continuar com `offset`. Num vault muito grande, a varredura para depois de 5000 linhas (por tabela no modo `exact`) ou 64 MB de conteúdo, e o resultado avisa.

> `search_memory` deliberadamente nunca cai sozinha no "último projeto usado" como as outras tools fazem — uma busca sem escopo deve buscar o vault inteiro, não adivinhar um projeto silenciosamente. Com `workspace_root` e sem `project`, um workspace sem `.sync82.json` também busca o vault inteiro, enquanto um `.sync82.json` que não pode ser lido ou que nomeia um projeto inválido dá um resultado de erro em vez de uma busca.

---

## Fluxo de sessão

### `load_project_context`

Carrega a memória de um projeto (todo arquivo não-vazio) concatenada num único bloco de contexto, pronto pra colar numa nova sessão. Por padrão carrega o estado atual por completo e só o histórico recente, num tamanho que cabe nos limites de saída de tools dos clientes MCP.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `files` | array de strings | ❌ | Carrega só esses arquivos/kinds específicos, em vez de tudo. Um nome sem arquivo aparece como `[no file named: …]`. |
| `mode` | string (`summary` \| `full`) | ❌ | `summary` (padrão): as 10 entradas datadas mais recentes por kind append-only, resposta cortada em 40 KB. `full`: todas as entradas, resposta cortada em 200 KB. `since`, `max_entries` e `max_bytes` substituem esses padrões. |
| `since` | string (`YYYY-MM-DD`) | ❌ | Inclui só entradas datadas (`progress`, `decisions`, ou um kind customizado de anexação) nessa data ou depois. Entradas sem data são sempre incluídas. Arquivos de sobrescrita não são afetados. No modo `summary`, informar `since` remove o limite padrão de 10 entradas. |
| `max_entries` | integer | ❌ | Inclui só as N entradas datadas mais recentes por kind append-only (padrão 10 no modo `summary`). Entradas sem data (como um título antes da primeira entrada datada) são sempre incluídas e não contam para N. Arquivos de sobrescrita não são afetados. |
| `max_bytes` | integer | ❌ | Tamanho máximo da resposta, em bytes (padrão 40960 = 40 KB no modo `summary`, 204800 = 200 KB no modo `full`; de 1024 a 52428800). |
| `path` | string | ❌ | Override do caminho do vault. |

Para um projeto de longa duração, `progress`/`decisions` crescem sem limite, então o modo `summary` — o padrão — carrega só o histórico recente: as 10 entradas datadas mais recentes de cada log, dentro de 40 KB. Isso fica abaixo do limite de 50.000 caracteres a partir do qual o Claude Code salva o resultado de uma tool em arquivo em vez de mostrá-lo. `since`/`max_entries` escolhem outro recorte do histórico, e `mode: "full"` carrega todas as entradas. `memory`, `architecture`, `stack` e `next_steps` sempre carregam por completo: eles representam estado atual, não histórico, então não há nada datado pra filtrar. Eles vêm primeiro, seguidos dos outros arquivos em ordem alfabética.

Quando entradas datadas ficam de fora — pelo padrão do `summary`, por `since` ou por `max_entries` — a resposta termina com um rodapé por kind, por exemplo:

```text
[older history omitted — progress: 10 of 142 dated entries shown (oldest shown 2026-09-15); load more with "max_entries" or "since", or set "mode" to "full"]
```

Uma resposta nunca passa de `max_bytes`, contando as notas. Quando passaria, o conteúdo é cortado — numa quebra de linha quando há uma perto o bastante, senão no meio da linha, sem partir um caractere — deixando espaço para o rodapé, que nunca é cortado (um rodapé que ocuparia mais de um quarto do `max_bytes` só conta os arquivos). Depois do corte vem `[context truncated at N of M bytes; cut short: <kinds>; left out: <kinds> — narrow it with "since", "max_entries" or "files", or raise "max_bytes"]`, com os arquivos que foram cortados ou que não couberam.

---

### `check_project_health`

Reporta quais dos seis arquivos de memória padrão existem para um projeto, além de avisos sobre memória que pode estar desatualizada (veja abaixo). O projeto está **não-saudável** quando falta um arquivo de estado atual (`memory`, `architecture`, `stack`, `next_steps`); `progress` e `decisions` ainda sem entrada aparecem como `EMPTY (no entry yet)` e só geram o aviso `empty_log`, então um projeto que o `init_project_memory` acabou de criar está saudável. Retorna um resultado de erro (`isError: true`) quando o projeto está não-saudável — um sinal deliberado pro agente chamador agir, não uma falha — e recomenda o `init_project_memory`, que só grava os arquivos que faltam. Isso vale também para `format: "json"`.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `all_projects` | boolean | ❌ | Verifica todo projeto e subprojeto do vault (o indicado por `path`, ou o vault padrão) em vez de um projeto só. Não pode ser combinado com `project`, `subproject` ou `workspace_root`, e nunca usa o último projeto usado. |
| `path` | string | ❌ | Override do caminho do vault. |
| `format` | string (`text` \| `json`) | ❌ | `text` (padrão) para texto legível; `json` para um documento JSON com a mesma informação, devolvido como texto e como conteúdo estruturado — veja [Saída JSON](#saída-json). |
| `stale_days` | integer | ❌ | Dias depois dos quais um arquivo de estado atual mais antigo que a entrada mais nova de `progress`/`decisions` é reportado como possivelmente desatualizado (≥ 1, padrão 30). |

Também reporta **avisos** — memória que pode estar desatualizada. Avisos nunca deixam o projeto não-saudável e nunca definem `isError`:

| Verificação (`check` no JSON) | Reportada quando |
|---|---|
| `stale` | Um arquivo de estado atual (`memory`, `architecture`, `stack`, `next_steps`) foi atualizado pela última vez há mais de `stale_days` dias **e** `progress` ou `decisions` tem uma entrada datada depois desse dia — o histórico andou e o arquivo pode não bater mais com ele. |
| `template` | Um arquivo de estado atual está vazio, ou ainda igual ao template em branco que o `init_project_memory` grava quando nenhuma resposta é dada (a data da linha `Last updated` não conta). |
| `undated_entries` | `progress` ou `decisions` tem entradas sem data — um título antes da primeira entrada datada não conta. O `archive_memory` nunca as arquiva; o `edit_entry` pode dar a elas um cabeçalho `## YYYY-MM-DD`. |
| `large_history` | `progress` ou `decisions` tem mais de 200 entradas datadas ativas — o `archive_memory` mantém pequeno o contexto carregado. |
| `empty_log` | `progress` ou `decisions` ainda não tem nenhuma entrada — registre o trabalho com `update_project_memory` ou `append_memory`. |

No relatório em texto, os avisos vêm depois da lista de arquivos, sob `Warnings:`, uma linha `- <arquivo>: <mensagem>` cada.

Com `all_projects: true`, o relatório em texto tem uma linha por projeto e subprojeto — `HEALTHY ✅`, `UNHEALTHY ❌ (N missing)` ou `WARNINGS ⚠️ (N)` —, depois uma contagem de cada, depois os arquivos ausentes e avisos de todo projeto que não está plenamente saudável. O resultado é de erro quando algum projeto está não-saudável. Um vault vazio é saudável (`No projects in the vault.`).

---

### `init_project_memory`

Inicialização guiada da memória de um projeto. A descrição dessa tool funciona como um roteiro pro agente — ele deve determinar se o alvo é um projeto ou subprojeto (perguntando ao usuário se não estiver claro), e então auto-detectar os dados do projeto a partir do código ou perguntar ao usuário um conjunto fixo de perguntas. Só arquivos vazios ou que ainda contêm o template em branco são escritos — rodar de novo num projeto já inicializado não sobrescreve conteúdo existente. Com `workspace_root`, ela grava o `.sync82.json` lá — a menos que já exista um apontando para outro projeto, ou para outro vault que não o `path` dado na chamada; esse arquivo é mantido sem alteração e reportado, para que a memória do workspace não mude de lugar. Um `path` dado como `~/…`, `HOME/…` ou caminho absoluto é registrado no `.sync82.json` como foi dado. Quando `workspace_root` é dado, o último projeto usado nunca é usado como alternativa. Sem `project` e `workspace_root`, ela pode inicializar o último projeto usado — o resultado então o identifica —, mas se recusa a gravar nele qualquer resposta (ou `auto_detect`).

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. `workspace_root` também habilita auto-descoberta via `.sync82.json` em sessões futuras, e é obrigatório quando `auto_detect` é verdadeiro. |
| `auto_detect` | boolean | ❌ | Quando `true`, analisa os arquivos em `workspace_root` pra inferir descrição, linguagens, frameworks e infraestrutura automaticamente. |
| `description` | string | ❌ | O que o projeto faz. |
| `goal` | string | ❌ | O objetivo principal. |
| `phase` | string | ❌ | Fase atual: `planning` / `mvp` / `active` / `maintenance`. |
| `architecture_overview` | string | ❌ | Descrição breve da arquitetura. |
| `components` | string | ❌ | Principais componentes, separados por vírgula. |
| `languages` | string | ❌ | Linguagens usadas. |
| `frameworks` | string | ❌ | Frameworks e bibliotecas usadas. |
| `infrastructure` | string | ❌ | Infraestrutura e hospedagem. |
| `next_steps` | string | ❌ | Próximas tarefas imediatas, separadas por vírgula ou quebra de linha. |
| `path` | string | ❌ | Override do caminho do vault. |

Os campos de resposta, somados, podem ter no máximo 10 MB.

`auto_detect` inspeciona `README.md`, `package.json`, `composer.json`, `Cargo.toml`, arquivos de projeto Python, `go.mod`, `pom.xml`/`build.gradle`/`build.gradle.kts` (Java/Kotlin), `Gemfile` (Ruby), `*.csproj`/`*.fsproj`/`*.vbproj` (.NET), e marcadores comuns de infraestrutura (Docker, configs de CI etc.) — incluindo projetos PHP/Composer (plugins de Moodle e afins), detectando frameworks como Laravel, Symfony e Slim a partir de `require`/`require-dev`. As dependências são reconhecidas pelo nome exato nas listas de dependências de cada manifesto (tabelas de dependências do Cargo, linhas `gem` do `Gemfile`, coordenadas do `pom.xml`/Gradle, nomes de requisitos do `requirements.txt`/`pyproject.toml`/`setup.py`, linhas `require` do `go.mod`, `PackageReference`/`FrameworkReference` do `.csproj`), então comentários e pacotes com nomes parecidos não geram detecções falsas, e os resultados vêm numa ordem estável. Ele também reconhece `compose.yaml`/`compose.yml`, ignora `vendor`, `target`, `venv`, `bin`, `obj` e `__pycache__` ao listar componentes, e lê títulos de README no estilo setext ignorando blocos de código. Ele só lê arquivos regulares de até 5 MiB dentro de `workspace_root`: symlinks apontando para fora dele, FIFOs e dispositivos são ignorados, e as descrições detectadas viram uma única linha.

---

### `update_project_memory`

Salva o trabalho de uma sessão no vault do projeto numa única chamada — a tool que a maioria dos agentes deve usar ao final de uma sessão de trabalho. Analisa o que mudou e escreve só os campos que de fato mudaram. Quando algum campo sobrescreve (`next_steps`, `memory`, `architecture`, `stack`, ou um item de `custom` com `mode: "write"`), o projeto precisa vir de `project` ou `workspace_root`: um projeto tirado só da sessão anterior é recusado, e nada é gravado. Uma chamada que só anexa pode usá-lo.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `progress` | string | ❌ | Anexado a `progress`. Deve conter um cabeçalho de data `## YYYY-MM-DD`. |
| `decisions` | string | ❌ | Anexado a `decisions`. Deve conter um cabeçalho de data `## YYYY-MM-DD`. |
| `next_steps` | string | ❌ | Sobrescreve `next_steps` com o conteúdo atualizado completo. |
| `memory` | string | ❌ | Sobrescreve `memory` com o conteúdo atualizado completo. |
| `architecture` | string | ❌ | Sobrescreve `architecture` com o conteúdo atualizado completo. |
| `stack` | string | ❌ | Sobrescreve `stack` com o conteúdo atualizado completo. |
| `custom` | array de `{filename, content, mode?}` | ❌ | Arquivos customizados fora dos seis padrão (nomear um arquivo padrão é recusado — use o campo próprio dele). `mode` é `"append"` (padrão) ou `"write"`. |
| `path` | string | ❌ | Override do caminho do vault. |

`custom` aceita no máximo 50 itens, e o conteúdo total de uma chamada pode ter no máximo 10 MB.

A chamada é tudo ou nada: todos os campos são conferidos antes — `progress`/`decisions` precisam de um cabeçalho `## YYYY-MM-DD`, nenhum campo pode estar vazio — e todos os problemas são informados juntos. Depois, todos os campos são gravados numa única transação. Se algum campo for recusado (inclusive anexar a um arquivo customizado guardado como documento de sobrescrita), nada é gravado e o resultado lista os campos a corrigir.

---

## Exportação e importação

### `export_memory`

Exporta a memória de um projeto para arquivos Markdown simples (um por kind, ex. `memory.md`, `progress.md`) no sistema de arquivos local, pra navegação ou diff via git fora de um cliente MCP.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `output_dir` | string | ✅ | Pasta onde escrever os arquivos `.md` exportados. Precisa ser absoluta (ou começar com `~/` ou `HOME/`); fora isso, qualquer pasta em que o processo do servidor possa escrever — não é confinada ao vault ou workspace. Criada (privada ao usuário) se não existir. |
| `overwrite` | boolean | ❌ | Substitui arquivos `.md` que já existem em `output_dir`. Sem ele, a exportação é recusada — e nada é escrito — quando algum arquivo de destino existe. Um destino que seja symlink ou arquivo especial é sempre recusado. Com `overwrite`, um projeto que veio só da última sessão é recusado: passe `project` ou `workspace_root`. |
| `path` | string | ❌ | Override do caminho do vault. |

Um kind com entradas arquivadas também ganha um arquivo `<kind>.archived.md` com elas, então a exportação mantém o histórico inteiro. Um kind cujo nome não pode ser nome de arquivo (gravado por uma versão anterior, por exemplo com mais de 128 caracteres) fica de fora e aparece como pulado.

Quando grava algum arquivo, a exportação grava também `.sync82-kinds.json`, um manifesto que diz se cada kind é um `log` (append-only, guardado como entradas) ou um `document`, para que uma importação restaure um log personalizado como log. Ele não conta como arquivo exportado. Depois de gravar, o resultado lista os arquivos `.md` que já estavam em `output_dir`, que esta exportação não gravou e que são nomes de kind válidos — deixados por uma exportação anterior de um arquivo que o projeto não tem mais. Eles não são apagados: uma importação da pasta os traria de volta, então apague-os se esses arquivos foram removidos de propósito.

Também existe um comando de CLI `sync82 export` que faz a mesma coisa sem passar por um cliente MCP, mais um modo `--all` que exporta todo projeto/subprojeto do vault de uma vez — veja [Referência da CLI — export](./cli.md#export).

---

### `import_memory`

Importa a memória de um projeto a partir de arquivos Markdown simples previamente produzidos por `export_memory` — a operação inversa. Cria o projeto primeiro se ele ainda não existir.

| Argumento | Tipo | Obrigatório | Descrição |
|---|---|---|---|
| `project`, `subproject`, `workspace_root`, `search_parent_dirs` | — | ❌ | Argumentos de contexto padrão. |
| `input_dir` | string | ✅ | Pasta de onde ler os arquivos `.md` exportados. Precisa ser absoluta (ou começar com `~/` ou `HOME/`); fora isso, qualquer pasta que o processo do servidor possa ler — não é confinada ao vault ou workspace. Todo arquivo `"<kind>.md"` presente é importado, e um arquivo `"<kind>.archived.md"` restaura as entradas arquivadas daquele kind; arquivos que não são nomes de kind válidos, estão vazios, passam de 10 MB, ou são symlinks ou arquivos especiais são pulados e reportados. No máximo 256 arquivos `.md` são lidos por importação. Um manifesto `.sync82-kinds.json` gravado pela exportação, quando presente, diz quais kinds novos são logs; ele precisa ser um arquivo regular de no máximo 1 MB com versão `1`, ou a importação falha. |
| `dry_run` | boolean | ❌ | Quando `true`, reporta o que a importação criaria e sobrescreveria, sem escrever nada — nem um arquivo de vault ausente é criado. |
| `path` | string | ❌ | Override do caminho do vault. |

A importação é tudo ou nada: todo arquivo é lido e verificado primeiro, depois tudo é escrito numa única transação, então uma falha deixa o vault inalterado. O resultado lista `New:` (arquivos que criam um kind), `Overwritten:` (arquivos que substituem um kind existente) e `Skipped:`. Dois arquivos que viram o mesmo kind depois de convertidos para minúsculas (ex. `Memory.md` e `memory.md`) fazem a importação falhar.

**Destrutivo por kind**: o conteúdo de um kind já existente é sobrescrito, não mesclado, o mesmo comportamento de `write_memory` (entradas arquivadas que já estão no vault são mantidas, a menos que a pasta tenha um `<kind>.archived.md` daquele kind). O projeto precisa vir de `project` ou `workspace_root`: um projeto tirado só da última sessão é recusado, então o import nunca sobrescreve nem recria um projeto lembrado. Use pra restaurar um backup, migrar um projeto entre vaults, ou popular um vault novo a partir de uma pasta de Markdown escrita à mão. Também existe um comando de CLI `sync82 import` — veja [Referência da CLI — import](./cli.md#import).

---

## Anotações das tools

Toda tool declara anotações MCP — dicas que os clientes usam para decidir quando pedir confirmação antes de rodar uma tool e como rotulá-la. Todas as tools definem `openWorldHint: false` (só mexem no vault local e, no export/import, em arquivos locais), e `destructiveHint` é sempre definido explicitamente, porque o protocolo assume `true` quando ele falta.

| Tool | `title` | `readOnlyHint` | `destructiveHint` | `idempotentHint` |
|---|---|---|---|---|
| `list_projects` | List projects | ✅ | — | ✅ |
| `create_project` | Create a project | — | — | ✅ |
| `delete_project` | Delete a project | — | ✅ | ✅ |
| `rename_project` | Rename a project | — | — | — |
| `get_vault_config` | Show the vault configuration | ✅ | — | ✅ |
| `list_files` | List memory files | ✅ | — | ✅ |
| `read_memory` | Read a memory file | ✅ | — | ✅ |
| `write_memory` | Overwrite a memory file | — | ✅ | ✅ |
| `append_memory` | Append a memory entry | — | — | — |
| `delete_memory` | Delete a memory file | — | ✅ | ✅ |
| `edit_entry` | Edit a memory entry | — | ✅ | — |
| `archive_memory` | Archive old entries | — | — | — |
| `search_memory` | Search memory | ✅ | — | ✅ |
| `load_project_context` | Load the project context | ✅ | — | ✅ |
| `check_project_health` | Check the project memory | ✅ | — | ✅ |
| `init_project_memory` | Initialize project memory | — | — | ✅ |
| `update_project_memory` | Save the session to memory | — | ✅ | — |
| `export_memory` | Export memory to Markdown files | — | ✅ | ✅ |
| `import_memory` | Import memory from Markdown files | — | ✅ | ✅ |

As tools "somente leitura" ainda registram o último projeto usado em `~/.sync82/config.json`; isso é controle interno, não uma mudança na memória. `destructiveHint` marca as tools que podem sobrescrever ou apagar memória existente (ou, no `export_memory` com `overwrite`, arquivos existentes); as demais só acrescentam. O arquivamento mantém as entradas arquivadas, então o `archive_memory` não é destrutivo.

---

## Saída JSON

`list_projects`, `list_files`, `check_project_health` e `search_memory` aceitam `format: "json"`. O texto do resultado passa a ser um documento JSON indentado, e o mesmo objeto é devolvido como o `structuredContent` do resultado MCP. Qualquer outro valor de `format` é rejeitado com um resultado de erro. Notas de contexto (`[project: ..., from ..., vault: ...]`) não são adicionadas no modo JSON — o campo `vault` traz o caminho resolvido. Um resultado vazio é um array vazio (`[]`), nunca uma frase de "nada encontrado". Mensagens que não são resultados — um pedido de `project`, um vault ausente (exceto no `list_projects`), um argumento inválido — continuam em texto simples.

Essas quatro tools declaram um **output schema** (`outputSchema` no `tools/list`) que descreve os objetos abaixo, e todo resultado de sucesso traz o objeto correspondente como `structuredContent`, **qualquer que seja o `format`**: no formato `text`, o texto continua legível e o conteúdo estruturado traz a mesma informação. O cliente escolhe qual dos dois o agente vê: o Claude Code (2.1) entrega ao agente o conteúdo estruturado quando o resultado tem um, então nele essas tools respondem em JSON qualquer que seja o `format`. Um resultado que não tem esse objeto — um pedido de `project` ou `workspace_root`, um vault ausente — é devolvido com `isError: true`, já que os clientes rejeitam um resultado de sucesso sem conteúdo estruturado de uma tool que declara output schema.

**`list_projects`**

| Campo | Tipo | Descrição |
|---|---|---|
| `vault` | string | Caminho do vault listado. Um vault que ainda não existe dá um array `projects` vazio. |
| `projects` | array | Um objeto por projeto de nível superior. |
| `projects[].name` | string | Nome do projeto. |
| `projects[].subprojects` | array de strings | Nomes dos subprojetos (`[]` quando não há). |

**`list_files`**

| Campo | Tipo | Descrição |
|---|---|---|
| `project` | string | `projeto` ou `projeto/subprojeto`. |
| `vault` | string | Caminho do vault resolvido. |
| `files` | array | Um objeto por arquivo (kind). |
| `files[].name` | string | Nome do arquivo (kind). |
| `files[].size_bytes` | integer | Tamanho em bytes — só com `metadata: true`. |
| `files[].estimated_tokens` | integer | Contagem estimada de tokens — só com `metadata: true`. |
| `files[].last_modified` | string | Data da última modificação — só com `metadata: true`. |

**`check_project_health`**

| Campo | Tipo | Descrição |
|---|---|---|
| `project` | string | `projeto` ou `projeto/subprojeto`. |
| `vault` | string | Caminho do vault resolvido. |
| `healthy` | boolean | `true` quando os seis arquivos padrão existem (caso contrário o resultado é marcado como `isError`). |
| `files` | object | Um boolean por arquivo padrão: `memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps`. |
| `warnings` | array | Um objeto por aviso, com `file`, `check` (`stale`, `template`, `undated_entries`, `large_history`) e `message` — omitido quando não há nenhum. |

Com `all_projects: true`:

| Campo | Tipo | Descrição |
|---|---|---|
| `vault` | string | Caminho do vault verificado. |
| `healthy` | boolean | `true` quando todo projeto está saudável, ou o vault está vazio. |
| `projects` | array | Um objeto por projeto e subprojeto, cada projeto seguido dos subprojetos dele, em ordem de nome (`[]` num vault vazio). |
| `projects[].project` | string | Nome do projeto de nível superior. |
| `projects[].subproject` | string | Nome do subprojeto — omitido para um projeto de nível superior. |
| `projects[].healthy` | boolean | `true` quando os seis arquivos padrão existem. |
| `projects[].missing` | array of strings | Os arquivos padrão que não existem (`[]` quando nenhum). |
| `projects[].warnings` | array | Os avisos dele, no formato acima (`[]` quando nenhum). |

**`search_memory`**

| Campo | Tipo | Descrição |
|---|---|---|
| `query` | string | O texto buscado. |
| `results` | array | Os resultados, na mesma ordem estável da saída em texto. |
| `results[].project` | string | Projeto do resultado. |
| `results[].subproject` | string | Subprojeto do resultado — omitido para um projeto de nível superior. |
| `results[].file` | string | Arquivo (kind) do resultado. |
| `results[].entry_id` | integer | Id da entrada que contém o resultado, para o [`edit_entry`](#edit_entry) — omitido para um resultado num arquivo de sobrescrita. |
| `results[].entry_date` | string | `AAAA-MM-DD` da entrada datada que contém o resultado — omitido para conteúdo sem data. |
| `results[].line` | integer | Número da linha (dentro da entrada, para uma entrada datada). |
| `results[].text` | string | A linha encontrada. |
| `results[].context_before`, `results[].context_after` | array de strings | Linhas ao redor — só com `context_lines` > 0, omitidas quando vazias. |
| `next_offset` | integer | `offset` a passar para a próxima página — omitido quando não há mais resultados. |
| `scan_truncated` | boolean | `true` quando a varredura parou cedo num vault muito grande — omitido caso contrário. |

---

## Os seis arquivos padrão

Todo projeto ganha esses seis kinds. `decisions` e `progress` são **só-anexa** (cada escrita adiciona uma nova entrada datada, nunca substitui o histórico); os outros quatro são **de sobrescrita** (cada escrita substitui o arquivo inteiro).

| Kind | Estilo | O que vai nele |
|---|---|---|
| `memory` | sobrescrita | Nome do projeto, descrição, status de alto nível |
| `architecture` | sobrescrita | Componentes e como se relacionam |
| `stack` | sobrescrita | Linguagens, frameworks, infraestrutura |
| `decisions` | anexação, datado | Uma entrada por decisão, com o raciocínio |
| `progress` | anexação, datado | Uma entrada por sessão de trabalho concluída |
| `next_steps` | sobrescrita | A lista de tarefas atual do projeto |

`delete_memory` se recusa a apagar qualquer um desses seis — só kinds customizados podem ser apagados. `write_memory` sobrescreve qualquer um deles, inclusive os só-anexa (ele re-analisa o novo conteúdo em seções datadas), pra flexibilidade total quando você precisar.

---

[← Voltar ao Índice](../index.md)
