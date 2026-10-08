🌐 [English](../en/prompts.md) | **Português** | 🏠 [Índice](./index.md)

---

# Prompts de Exemplo

Pedidos que você pode digitar para o seu agente de IA, em linguagem natural, para fazê-lo usar o sync82 — depois que o sync82 estiver [instalado](./getting-started/installation.md) e registrado no seu cliente. Você nunca precisa nomear uma tool nem escrever JSON; o agente escolhe a tool e os argumentos. Toda tool e argumento citados abaixo estão detalhados na [Referência de Tools](./reference/tools.md).

**Sobre "este projeto":** dentro de um workspace, o agente normalmente passa a pasta em que está trabalhando como `workspace_root`, então o sync82 encontra o projeto pelo `.sync82.json` daquela pasta (escrito na primeira vez que você inicializa a memória ali). Num cliente sem pasta de trabalho (ex. Claude Desktop), diga o nome do projeto ou o caminho dele.

---

## 🚀 Prompts de Configuração

### Inicializar a memória automaticamente

- **Parâmetros esperados:** um workspace aberto (a pasta do projeto).
- **Exemplo:**
  > "Inicialize a memória deste projeto. Analise o código automaticamente."
- **Resultado esperado:** o agente chama `init_project_memory` com `workspace_root` e `auto_detect: true`. O sync82 infere descrição, linguagens, frameworks e infraestrutura a partir de arquivos como `README.md`, `go.mod` ou `package.json`, escreve os seis arquivos padrão (só os que ainda estão vazios ou em branco) e cria o `.sync82.json`, para que as próximas sessões encontrem o projeto sozinhas.

### Inicializar a memória respondendo perguntas

- **Parâmetros esperados:** um workspace aberto; suas respostas (descrição, objetivo, fase, arquitetura, componentes, linguagens, frameworks, infraestrutura, próximos passos).
- **Exemplo:**
  > "Configure a memória do sync82 para este projeto. Eu mesmo respondo as perguntas."
- **Resultado esperado:** o agente faz as perguntas uma de cada vez e depois chama `init_project_memory` com suas respostas e `workspace_root`.

### Acompanhar um componente como subprojeto

- **Parâmetros esperados:** o nome do projeto pai e o nome do subprojeto.
- **Exemplo:**
  > "Este é um plugin do projeto Moodle que eu já acompanho. Configure a memória dele como um subprojeto chamado mod_quiz dentro de moodle."
- **Resultado esperado:** `init_project_memory` (ou `create_project`) com `project: "moodle"`, `subproject: "mod_quiz"`; o pai é criado antes, se ainda não existir.

### Criar um projeto vazio

- **Parâmetros esperados:** o nome do projeto.
- **Exemplo:**
  > "Crie um projeto no sync82 chamado billing-api, eu preencho depois."
- **Resultado esperado:** `create_project` com `project: "billing-api"`. Pedir de novo para um projeto existente o reporta em vez de duplicá-lo.

### Manter um projeto num vault separado

- **Parâmetros esperados:** o caminho do arquivo do vault.
- **Exemplo:**
  > "Inicialize a memória deste projeto, mas guarde em ~/vaults/cliente-x.db em vez do vault padrão."
- **Resultado esperado:** `init_project_memory` com `path: "~/vaults/cliente-x.db"`; o caminho é gravado no `.sync82.json`, então as próximas chamadas neste workspace também usam esse vault.

### Usar um arquivo marcador da raiz do repositório (monorepo)

- **Parâmetros esperados:** uma subpasta de um repositório cuja raiz tem um `.sync82.json`.
- **Exemplo:**
  > "Estou em packages/web do nosso monorepo. Carregue a memória do projeto — o marcador do sync82 está na raiz do repo, procure nas pastas acima."
- **Resultado esperado:** o agente passa `search_parent_dirs: true`, então o sync82 procura o `.sync82.json` em até 64 diretórios pais e carrega a memória daquele projeto.

---

## 🔁 Prompts do Fluxo de Sessão

### Carregar o projeto no início de uma sessão

- **Parâmetros esperados:** nenhum (o workspace atual), ou o nome do projeto.
- **Exemplo:**
  > "Carregue o contexto do projeto antes de começarmos."
- **Resultado esperado:** `load_project_context` devolve todo arquivo de memória não vazio num único bloco: `memory`, `architecture`, `stack` e `next_steps` primeiro e por inteiro, depois os outros arquivos em ordem alfabética. Logs como `progress` e `decisions` trazem só as 10 entradas datadas mais recentes; um rodapé diz quantas ficaram de fora.

### Começar ou encerrar uma sessão com um atalho

- **Parâmetros esperados:** opcionalmente o nome do projeto (e do subprojeto).
- **Exemplo:** no Claude Code, digite `/mcp__sync82__start_session acme` no início e `/mcp__sync82__end_session acme` no fim; ou anexe `@sync82:sync82://projects/acme/context` a qualquer mensagem.
- **Resultado esperado:** os [prompts MCP](./reference/mcp-prompts.md) mandam ao agente o mesmo pedido que "carregue o contexto do projeto" / "salve esta sessão", então ele chama `load_project_context` ou `update_project_memory`. O [resource](./reference/resources.md) anexa a memória do projeto à mensagem sem nenhuma chamada de tool.

### Carregar o histórico inteiro

- **Parâmetros esperados:** nenhum (o workspace atual), ou o nome do projeto.
- **Exemplo:**
  > "Carregue a memória completa deste projeto, com todo o histórico de progresso e decisões."
- **Resultado esperado:** `load_project_context` com `mode: "full"`: todas as entradas de todos os logs, cortadas em 200 KB com um aviso se passar disso.

### Carregar só o histórico recente

- **Parâmetros esperados:** uma data, ou um número de entradas.
- **Exemplo:**
  > "Carregue a memória deste projeto, mas só o progresso e as decisões das últimas duas semanas."
- **Resultado esperado:** `load_project_context` com `since` na data de duas semanas atrás (ou `max_entries` para "as últimas 5 entradas"). Visão geral, arquitetura, stack e próximos passos continuam carregando por inteiro; entradas sem data sempre entram.

### Carregar só alguns arquivos

- **Parâmetros esperados:** os arquivos que você quer.
- **Exemplo:**
  > "Carregue só a arquitetura e os próximos passos deste projeto."
- **Resultado esperado:** `load_project_context` com `files: ["architecture", "next_steps"]`.

### Salvar o trabalho da sessão

- **Parâmetros esperados:** o que você fez (o agente pode resumir a conversa).
- **Exemplo:**
  > "Salve o que fizemos hoje: implementamos a tool archive_memory e escrevemos os testes dela."
- **Resultado esperado:** `update_project_memory` com uma entrada de `progress` sob o cabeçalho `## AAAA-MM-DD` de hoje; as entradas anteriores ficam intactas.

### Registrar uma decisão

- **Parâmetros esperados:** a decisão e o motivo.
- **Exemplo:**
  > "Decidimos usar SQLite em vez de um armazenamento em arquivos planos porque precisávamos de transações de verdade. Salve isso."
- **Resultado esperado:** `update_project_memory` com uma entrada datada em `decisions`.

### Atualizar várias coisas de uma vez

- **Parâmetros esperados:** o que mudou.
- **Exemplo:**
  > "Atualize a memória — terminamos a migração e agora os próximos passos são escrever a documentação e testar em staging."
- **Resultado esperado:** uma chamada de `update_project_memory` com `progress` e `next_steps` (e qualquer outro campo que mudou). Cada campo é aplicado de forma independente; o resultado diz quais falharam, se algum falhar.

### Manter um log ou nota customizada

- **Parâmetros esperados:** o nome do arquivo customizado e o conteúdo.
- **Exemplo:**
  > "Adicione o deploy de hoje a um log 'deploys' na memória do projeto: v1.4.0 em produção, sem incidentes."
- **Resultado esperado:** `append_memory` (ou `update_project_memory` com um item `custom` no modo `append`) adiciona uma nova entrada (agentes costumam dar a ela um cabeçalho `## AAAA-MM-DD`) ao arquivo customizado `deploys`, criando-o como log no primeiro uso.

### Ler um arquivo

- **Parâmetros esperados:** qual arquivo.
- **Exemplo:**
  > "Qual é a arquitetura atual deste projeto?"
- **Resultado esperado:** `read_memory` com `filename: "architecture"`.

### Reescrever um arquivo por inteiro

- **Parâmetros esperados:** o novo conteúdo (ou o que mudar).
- **Exemplo:**
  > "O arquivo de stack está desatualizado — reescreva: Go 1.26, SQLite via modernc.org/sqlite, GitHub Actions para CI."
- **Resultado esperado:** `write_memory` com `filename: "stack"` e o novo conteúdo completo, substituindo o antigo.

### Corrigir uma entrada errada

- **Parâmetros esperados:** qual entrada (a data ou o que ela diz) e a correção.
- **Exemplo:**
  > "A entrada de progresso de ontem diz que migramos para o Postgres, mas continuamos no SQLite. Corrija essa entrada."
- **Resultado esperado:** `read_memory` com `filename: "progress"` e `with_ids: true` para achar o id da entrada, depois `edit_entry` com `action: "replace"` e o texto corrigido. Só essa entrada muda; o resto do log fica como estava.

### Registrar que uma decisão mudou

- **Parâmetros esperados:** a decisão antiga e a nova.
- **Exemplo:**
  > "Decidimos abandonar a API REST em favor de gRPC. Marque a decisão antiga sobre REST como superada."
- **Resultado esperado:** `edit_entry` com `action: "supersede"` no id da decisão antiga e a nova decisão em `content`. A nova decisão entra como uma entrada, e a antiga ganha uma linha `> Superseded by entry N on YYYY-MM-DD.`, então as duas ficam no histórico.

### Remover uma entrada duplicada

- **Parâmetros esperados:** qual entrada é a duplicada.
- **Exemplo:**
  > "As duas últimas entradas de progresso são iguais — apague a duplicada."
- **Resultado esperado:** depois da sua confirmação, `edit_entry` com `action: "delete"` e `confirm: true` num dos dois ids.

### Corrigir ou remover uma entrada arquivada

- **Parâmetros esperados:** o log (`progress`, `decisions` ou um log personalizado) e o que mudar numa entrada que foi arquivada.
- **Exemplo:**
  > "Uma das entradas de progresso que arquivamos no ano passado diz que fomos para produção em março, mas foi em abril. Corrija no arquivo morto."
- **Resultado esperado:** `read_memory` com `filename: "progress"`, `archived: true` e `with_ids: true` lista as entradas arquivadas com seus ids, depois `edit_entry` com `action: "replace"` (ou `"delete"` com `confirm: true`, depois da sua confirmação) nesse id. A entrada continua arquivada; `supersede` é recusado em entradas arquivadas.

---

## 🔍 Prompts de Análise e Busca

### Buscar no projeto atual

- **Parâmetros esperados:** o que procurar.
- **Exemplo:**
  > "Mostre todas as decisões que tomamos sobre autenticação."
- **Resultado esperado:** `search_memory` com `kinds: ["decisions"]` e uma busca como `"autentica*"` (um prefixo, então encontra também "autenticação" e "autenticar"), restrita ao projeto atual e seus subprojetos. Os melhores resultados vêm primeiro; cada resultado vem rotulado `projeto/arquivo:linha` (`projeto/arquivo[AAAA-MM-DD]:linha` para uma entrada datada).

### Buscar no vault inteiro

- **Parâmetros esperados:** o que procurar.
- **Exemplo:**
  > "Procure no vault inteiro como já lidamos com migrações de banco antes — não lembro em qual projeto foi."
- **Resultado esperado:** `search_memory` sem `project`, que deliberadamente cobre todos os projetos do vault.

### Buscar sem lembrar as palavras exatas

- **Parâmetros esperados:** algumas palavras do que você lembra, em qualquer ordem, com ou sem acento.
- **Exemplo:**
  > "Ache onde escrevemos sobre a decisao do caminho de configuracao do instalador."
- **Resultado esperado:** `search_memory` com palavras como `"decisao configuracao instalador"` (o modo padrão `words`): todo documento ou entrada que tenha todas elas, em qualquer ordem, com `decisão`/`Decisão` encontradas por `decisao`, melhores resultados primeiro.

### Buscar uma frase exata ou um texto literal

- **Parâmetros esperados:** a frase, ou o texto literal (um caminho, um identificador, uma versão).
- **Exemplo:**
  > "Ache a frase exata 'uma conexão por vault'." / "Procure o texto literal `SetMaxOpenConns(1)`."
- **Resultado esperado:** `search_memory` com `match: "phrase"` para a frase (as palavras nessa ordem), ou `match: "exact"` para um texto literal com pontuação.

### Buscar num período do histórico

- **Parâmetros esperados:** o que procurar; o período.
- **Exemplo:**
  > "O que decidimos sobre o workflow de release em setembro de 2026?"
- **Resultado esperado:** `search_memory` com `kinds: ["decisions"]`, `since: "2026-09-01"` e `until: "2026-09-30"`. Só entradas datadas nesse intervalo são buscadas.

### Buscar com linhas ao redor, página por página

- **Parâmetros esperados:** a busca; quantas linhas de contexto; tamanho da página.
- **Exemplo:**
  > "Procure 'retry' neste projeto com 3 linhas de contexto em volta de cada resultado, 20 resultados por vez."
- **Resultado esperado:** `search_memory` com `context_lines: 3` e `limit: 20`; pedir "a próxima página" repete a busca com `offset: 20`.

### Listar projetos e subprojetos

- **Parâmetros esperados:** nenhum.
- **Exemplo:**
  > "Quais projetos eu tenho no sync82? Inclua os subprojetos."
- **Resultado esperado:** `list_projects` devolve a árvore de projetos do vault.

### Listar os arquivos de um projeto com tamanhos

- **Parâmetros esperados:** nenhum (o projeto atual).
- **Exemplo:**
  > "Liste todos os arquivos de memória deste projeto, com os tamanhos."
- **Resultado esperado:** `list_files` com `metadata: true`: cada arquivo com tamanho em bytes, tokens estimados e data da última modificação.

### Descobrir qual vault está em uso

- **Parâmetros esperados:** nenhum.
- **Exemplo:**
  > "Onde meu vault está guardado agora, de fato?"
- **Resultado esperado:** `get_vault_config` reporta o caminho do vault ativo, a config global e, com o workspace atual, o `.sync82.json` dele.

---

## 🧾 Prompts de Saída Estruturada (JSON)

`list_projects`, `list_files`, `check_project_health` e `search_memory` aceitam `format: "json"`, que devolve um documento JSON (também enviado como conteúdo estruturado do MCP) em vez de texto legível — útil quando o agente vai processar o resultado.

### Árvore de projetos em JSON

- **Parâmetros esperados:** nenhum.
- **Exemplo:**
  > "Me dê a lista de projetos do sync82 em JSON."
- **Resultado esperado:** `list_projects` com `format: "json"`: `{"vault": ..., "projects": [{"name": ..., "subprojects": [...]}]}`.

### Inventário de arquivos em JSON

- **Parâmetros esperados:** nenhum (o projeto atual).
- **Exemplo:**
  > "Devolva os arquivos de memória deste projeto com os tamanhos em JSON, para montarmos uma tabela."
- **Resultado esperado:** `list_files` com `metadata: true` e `format: "json"`.

### Relatório de saúde em JSON

- **Parâmetros esperados:** nenhum (o projeto atual).
- **Exemplo:**
  > "Verifique a saúde da memória deste projeto e me dê o resultado em JSON."
- **Resultado esperado:** `check_project_health` com `format: "json"`: `{"project", "vault", "healthy", "files": {"memory": true, ...}}`.

### Resultados de busca em JSON

- **Parâmetros esperados:** a busca.
- **Exemplo:**
  > "Procure 'rate limit' no vault e devolva os resultados em JSON."
- **Resultado esperado:** `search_memory` com `format: "json"`: cada resultado com `project`, `file`, `line`, `text` e, quando houver, `entry_date` e linhas de contexto; `next_offset` quando existirem mais resultados.

---

## ✅ Prompts de Teste e Validação

### Conferir se os arquivos padrão existem

- **Parâmetros esperados:** nenhum (o projeto atual).
- **Exemplo:**
  > "A memória deste projeto está saudável? Todos os arquivos padrão estão lá?"
- **Resultado esperado:** `check_project_health` lista os seis arquivos padrão e se cada um existe. Um projeto não saudável volta como resultado de erro — um sinal deliberado para o agente preencher os arquivos que faltam, não uma falha.

### Conferir se a memória está atualizada

- **Parâmetros esperados:** nenhum (o projeto atual); opcionalmente quantos dias contam como antigo.
- **Exemplo:**
  > "A memória deste projeto está em dia? Tem algo que parece desatualizado ou que nunca foi preenchido?"
- **Resultado esperado:** `check_project_health` (com `stale_days` se você deu um número de dias). A lista `Warnings:` aponta arquivos de estado atual mais antigos que as entradas mais novas de progresso/decisões, arquivos ainda vazios ou com o template em branco, entradas de log sem data e logs grandes o bastante para arquivar. O agente pode então oferecer atualizar cada arquivo, por exemplo reescrevendo `architecture` a partir do que as decisões recentes dizem.

### Verificar todos os projetos de uma vez

- **Parâmetros esperados:** nenhum; opcionalmente outro vault.
- **Exemplo:**
  > "Verifique a saúde da memória de todos os projetos do meu vault. Quais precisam de atenção?"
- **Resultado esperado:** `check_project_health` com `all_projects: true`: uma linha `HEALTHY`, `UNHEALTHY` ou `WARNINGS` por projeto e subprojeto, uma contagem de cada, depois os arquivos ausentes e os avisos dos que precisam de atenção. É um resultado de erro quando algum projeto está não-saudável.

### Confirmar a conexão depois de instalar

- **Parâmetros esperados:** nenhum.
- **Exemplo:**
  > "Liste todos os projetos do meu vault do sync82."
- **Resultado esperado:** `list_projects`; uma lista vazia num vault novo confirma que o servidor e o caminho do vault funcionam.

### Pré-visualizar uma importação antes de rodá-la

- **Parâmetros esperados:** a pasta de onde importar.
- **Exemplo:**
  > "Antes de restaurar meu backup de ~/backups/acme, me mostre o que seria sobrescrito."
- **Resultado esperado:** `import_memory` com `input_dir: "~/backups/acme"` e `dry_run: true`: nada é gravado; o resultado lista os arquivos `New:`, `Overwritten:` e `Skipped:`. Peça de novo sem a pré-visualização para importar.

---

## 🧹 Prompts de Manutenção

### Arquivar entradas antigas

- **Parâmetros esperados:** qual log (`progress` ou `decisions`) e quantos dias manter.
- **Exemplo:**
  > "Arquive as entradas de progresso com mais de 6 meses, não precisamos mais delas poluindo o contexto."
- **Resultado esperado:** `archive_memory` com `filename: "progress"` e `keep_days: 180`. Entradas sem data nunca são arquivadas. O projeto precisa ser nomeado ou vir do workspace, não só da última sessão.

### Arquivar entradas antigas e guardar um resumo

- **Parâmetros esperados:** qual log e quantos dias manter (ou uma data).
- **Exemplo:**
  > "Arquive as decisões com mais de 90 dias, mas guarde um resumo curto delas para não perdermos o contexto."
- **Resultado esperado:** `archive_memory` com `dry_run: true` para listar as entradas que sairiam, `read_memory` para lê-las e então `archive_memory` de novo com `summary` contendo o resumo que o agente escreveu. As entradas antigas são arquivadas e o resumo entra como uma entrada datada de hoje, com um cabeçalho dizendo quantas entradas cobre e o período — tudo de uma vez.

### Apagar um arquivo customizado

- **Parâmetros esperados:** o nome do arquivo customizado; sua confirmação.
- **Exemplo:**
  > "Apague o arquivo antigo 'testing-notes', não usamos mais."
- **Resultado esperado:** o agente confirma com você e então chama `delete_memory` com `confirm: true`. Os seis arquivos padrão não podem ser apagados assim.

### Apagar um projeto ou subprojeto

- **Parâmetros esperados:** o projeto (e o subprojeto); sua confirmação; para um projeto com subprojetos, o que fazer com eles.
- **Exemplo:**
  > "Terminamos o projeto legacy-portal. Apague-o, e promova os subprojetos dele a projetos independentes."
- **Resultado esperado:** depois da sua confirmação, `delete_project` com `confirm: true` e `subproject_action: "promote"` (ou `cancel`). Sem `subproject_action`, o sync82 lista os subprojetos e pergunta. Para apagar os subprojetos também, o agente mostra a lista deles antes e passa `subproject_action: "delete_all"` com `expected_subprojects`, a contagem deles; se até lá o projeto tiver outro número de subprojetos, nada é apagado e você recebe a lista nova.

### Renomear um projeto

- **Parâmetros esperados:** o nome atual e o novo.
- **Exemplo:**
  > "Este projeto foi renomeado de borg-82 para sync82, atualize aqui também."
- **Resultado esperado:** `rename_project` com `new_name: "sync82"`. Um `.sync82.json` em outro lugar que ainda cite o nome antigo precisa ser reinicializado ou editado.

---

## 💾 Prompts de Backup e Migração

### Exportar um projeto para Markdown

- **Parâmetros esperados:** a pasta de destino; se arquivos existentes podem ser substituídos.
- **Exemplo:**
  > "Exporte a memória deste projeto para ~/notes/acme-memory para eu versionar no git. Sobrescreva o que estiver lá."
- **Resultado esperado:** `export_memory` com `output_dir` e `overwrite: true`: um arquivo `.md` por kind, mais `<kind>.archived.md` para as entradas arquivadas e um manifesto `.sync82-kinds.json`; arquivos `.md` deixados por uma exportação anterior são listados.

### Restaurar um projeto a partir de Markdown

- **Parâmetros esperados:** a pasta de onde importar.
- **Exemplo:**
  > "Restaure a memória deste projeto a partir de ~/backups/acme."
- **Resultado esperado:** `import_memory` com `input_dir`: cria o projeto se preciso e sobrescreve cada kind encontrado na pasta, tudo numa única transação.

### Mover um projeto para outro vault

- **Parâmetros esperados:** uma pasta temporária; o caminho do vault de destino.
- **Exemplo:**
  > "Copie o projeto acme para o vault em ~/vaults/work.db: exporte para /tmp/acme primeiro e depois importe lá."
- **Resultado esperado:** `export_memory` a partir do vault atual, depois `import_memory` com `path: "~/vaults/work.db"`.

Para um backup do vault **inteiro** num passo só, use a CLI: `sync82 export --all <pasta-de-saída>` — veja [Referência da CLI — export](./reference/cli.md#export).

---

## ➡️ Próximos Passos

- [Exemplos de Uso](./guides/workflows/examples.md) — sessões completas de ponta a ponta
- [Referência de Tools](./reference/tools.md) — toda tool e argumento
- [Voltar ao Índice](./index.md)
