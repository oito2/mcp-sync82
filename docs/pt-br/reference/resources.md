🌐 [English](../../en/reference/resources.md) | **Português** | 🏠 [Índice](../index.md)

---

# Referência de Resources

Além das [tools](./tools.md), o sync82 expõe a memória dos projetos como **resources** MCP somente leitura: documentos Markdown que um cliente pode anexar a uma conversa ou ler sem o agente chamar uma tool. Resources nunca gravam no vault, nunca o criam e nunca mudam o "último projeto usado" que as tools usam como fallback.

## Qual vault

Resources sempre leem o **vault padrão** — o que uma chamada de tool usa quando não informa `path` e nenhum `.sync82.json` se aplica: o vault definido com `sync82 config set-vault`, senão a variável de ambiente `SYNC82_DB_PATH`, senão `~/.sync82/knowledge.db` (veja [Configuração](./configuration.md)). O `.sync82.json` de um workspace não é consultado, porque um pedido de resource não traz workspace. Projetos guardados em outro vault não ficam disponíveis como resources; use as tools com `path` ou `workspace_root` para eles.

## Templates de URI

| Template de URI | Conteúdo |
|---|---|
| `sync82://projects/{project}/context` | A memória do projeto num único bloco, exatamente o que o [`load_project_context`](./tools.md#load_project_context) devolve sem argumentos: arquivos de estado atual por inteiro, as 10 entradas mais recentes de cada log, cortado em 40 KB, com o rodapé de histórico omitido. |
| `sync82://projects/{project}/files/{file}` | Um arquivo de memória (`memory`, `architecture`, `stack`, `decisions`, `progress`, `next_steps` ou um customizado), exatamente o que o [`read_memory`](./tools.md#read_memory) devolve. |
| `sync82://projects/{project}/subprojects/{subproject}/context` | O mesmo que o contexto do projeto, para um subprojeto. |
| `sync82://projects/{project}/subprojects/{subproject}/files/{file}` | O mesmo que um arquivo do projeto, para um subprojeto. |

Todo resource tem o tipo MIME `text/markdown`. Os nomes seguem as mesmas regras dos argumentos `project`, `subproject` e `filename` das tools (letras, dígitos, `-` e `_`, começando com letra ou dígito) e não diferenciam maiúsculas de minúsculas: `sync82://projects/Acme/files/Memory` lê o `memory` de `acme`. Qualquer outra coisa — um nome desconhecido, um segmento de caminho a mais ou vazio, `..`, percent-encoding, uma query ou um fragmento — recebe o erro MCP *Resource not found* (`-32602`), a mesma resposta dada a um projeto ou arquivo que não existe.

## Listagem

O `resources/list` devolve o resource `context` de cada projeto e subprojeto do vault padrão, lido no momento do pedido, então um projeto criado durante a sessão aparece da próxima vez que o cliente listar os resources. Os arquivos não são listados um a um; leia-os pelo template `files/{file}`. Um vault vazio ou inexistente não lista nada.

Quando uma chamada de tool cria, renomeia ou apaga um projeto (`create_project`, `rename_project`, `delete_project`, `init_project_memory`, `import_memory`), o sync82 envia `notifications/resources/list_changed`, para que um cliente que respeite esse aviso liste os resources de novo; quando ele faz isso depende do cliente — alguns mantêm a lista que já têm até mais tarde. O sync82 não envia `notifications/resources/updated`: o cliente vê o conteúdo novo quando lê o resource de novo.

## Autocompletar

As variáveis `{project}`, `{subproject}` e `{file}` dos templates aceitam o autocompletar do MCP (`completion/complete`): um cliente que pede recebe, para o que já foi digitado (prefixo, sem diferenciar maiúsculas de minúsculas), os projetos do vault padrão, os subprojetos do projeto escolhido ou os arquivos desse projeto mais os seis padrão — até 100 valores, com o total. O mesmo autocompletar serve os argumentos `project` e `subproject` dos [prompts MCP](./mcp-prompts.md).

## Veja também

- [Referência de Prompts MCP](./mcp-prompts.md) — os prompts que o sync82 expõe.
- [Referência de Tools](./tools.md) — as tools por trás de cada resource.
