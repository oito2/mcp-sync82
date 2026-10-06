🌐 [English](../../../en/guides/workflows/examples.md) | **Português** | 🏠 [Índice](../../index.md)

---

# Exemplos de Uso

Casos de uso reais com prompts prontos para usar, sequências de passos e resultados esperados. Use estes exemplos como ponto de partida para suas próprias sessões.

---

## 1. Fluxo Completo: A Primeira Semana de um Projeto

Este exemplo mostra a memória de um projeto tomando forma ao longo de três sessões separadas.

**Sessão 1 — Bootstrap:**

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

`init_project_memory` roda com `auto_detect: true` e `workspace_root` definido — ele lê `README.md`/`go.mod`/etc., escreve `memory`, `architecture` e `stack`, e salva `.sync82.json` para que sessões futuras nesse diretório não precisem do argumento `project`.

**Sessão 2 — Registrando uma decisão:**

```
Estamos trocando REST por gRPC nas chamadas internas de serviço —
menor latência e já geramos os protobufs para o SDK do cliente.
Salve essa decisão.
```

`update_project_memory` adiciona uma entrada datada em `decisions`. Nada mais é tocado.

**Sessão 3 — Fechando antes de uma pausa:**

```
Carregue o contexto primeiro. ... [trabalho acontece] ... Salve o que
fizemos: migramos o serviço de auth para gRPC. Os próximos passos são
o serviço de billing, depois teste de carga.
```

`load_project_context` restaura tudo das sessões 1–2; `update_project_memory` adiciona `progress` e sobrescreve `next_steps` com a nova lista.

**O que o servidor faz em cada passo:**
- Sessão 1: `init_project_memory` — escreve 3 arquivos do tipo sobrescrita, salva `.sync82.json`
- Sessão 2: `update_project_memory` — adiciona uma entrada datada em `decisions`
- Sessão 3: `load_project_context` (leitura) depois `update_project_memory` (escrita de `progress` + `next_steps`)

---

## 2. Projeto Multi-Componente (Monorepo / Ecossistema de Plugins)

**Cenário:** Você mantém uma instalação Moodle e está desenvolvendo dois plugins para ela. `project`/`subproject` foi construído exatamente para isso — um projeto pai para contexto compartilhado, componentes rastreados independentemente por baixo.

**Passo 1 — Configure o pai:**

```
Inicialize a memória deste projeto, ele se chama "moodle".
```

**Passo 2 — Adicione o primeiro plugin como subprojeto:**

```
Este é um plugin para o projeto moodle. Configure a memória dele
como um subprojeto chamado mod_quiz.
```

`init_project_memory` roda com `project: "moodle"`, `subproject: "mod_quiz"` — cria o subprojeto se ele ainda não existir.

**Passo 3 — Adicione um segundo plugin:**

```
Mesma coisa para o plugin block_coursestats — subprojeto de moodle.
```

**Passo 4 — Busque em todos eles:**

```
Busque em todo o vault como já tratamos verificações de capability
antes — não lembro se foi neste plugin ou em outro.
```

`search_memory` sem `project` deliberadamente busca o vault inteiro, não só o plugin em que você está agora.

**Passo 5 — Verifique a árvore inteira:**

```
Quais plugins temos rastreados para o Moodle?
```

`list_projects` retorna a árvore completa de projeto/subprojeto.

---

## 3. Depuração Com Contexto Histórico

**Cenário:** Chega um relato de bug para um comportamento que você tem quase certeza que já foi discutido antes.

```
Carregue a arquitetura e as decisões deste projeto. Depois me ajude a
descobrir por que a invalidação de cache não está disparando num reload
de config — tenho a impressão que fizemos uma escolha deliberada sobre isso.
```

A IA chama `read_memory filename="architecture"` e `read_memory filename="decisions"` (ou `load_project_context` para tudo de uma vez), e então raciocina sobre o bug com esse histórico em mãos em vez de adivinhar só a partir do código.

---

## 4. Handoff de Sessão Entre Clientes de IA

**Cenário:** Você usou o Claude Code de manhã e quer continuar com o Codex à tarde — mesmo vault, mesmo projeto, sem perder contexto.

Nada de especial é necessário — desde que os dois clientes estejam configurados contra o mesmo vault (o padrão `~/.sync82/knowledge.db`, ou o mesmo override de `path`), a memória já é compartilhada. No novo cliente:

```
Carregue o contexto do projeto.
```

Se não resolver automaticamente (ex. o Codex está rodando de um diretório de trabalho diferente do que o Claude Code usava), informe o projeto explicitamente:

```
Carregue o contexto do projeto "moodle", subprojeto "mod_quiz".
```

---

## 5. Manutenção Periódica

**Cenário:** O log de `progress` de um projeto de longa duração ficou grande o suficiente para consumir orçamento de contexto.

```
A memória deste projeto está saudável? Arquive entradas de progress
com mais de 6 meses.
```

A IA chama `check_project_health` primeiro, depois `archive_memory filename="progress" keep_days=180`. Entradas sem cabeçalho de data nunca são arquivadas, por design.

```
Exporte a memória deste projeto para uma pasta que eu possa commitar
no git como backup.
```

`export_memory` escreve um arquivo `.md` simples por kind no diretório que você especificar — útil para diffar mudanças de memória em um PR, ou como backup portável fora do vault SQLite.

---

## 💡 Dicas Gerais

**Seja específico sobre project vs. subproject** quando for ambíguo — palavras como "plugin", "módulo", "componente", "serviço" são pistas que as instruções do `init_project_memory` procuram, mas nomear diretamente evita qualquer suposição.

**`update_project_memory` é a ferramenta principal** para salvamentos comuns de fim de sessão — use `write_memory`/`append_memory` diretamente só quando fizer algo mais cirúrgico (reescrever um arquivo inteiro, ou adicionar fora do fluxo normal de salvar sessão).

**Um "lembre disso" vago funciona bem** — as descrições das tools são escritas para guiar o julgamento do agente sobre a qual arquivo uma informação pertence, não para exigir que você conheça o schema.

---

## ➡️ Próximos Passos

- [Referência de Tools](../../reference/tools.md) — parâmetros completos de todas as tools
- [Prompts de Exemplo](../../prompts.md) — mais prompts, organizados por categoria
- [Problemas Comuns](../../troubleshooting/common-issues.md) — quando algo não funciona como esperado
- [Voltar ao Índice](../../index.md)
