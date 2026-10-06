🌐 [English](../../en/concepts/why-sync82.md) | **Português** | 🏠 [Índice](../index.md)

---

# Por que usar o sync82?

Assistentes de IA são poderosos — mas não lembram de nada entre sessões. Este documento explica o problema que o `sync82` resolve e quando faz sentido usá-lo.

---

## O problema: IA sem memória persistente

Todo novo chat com um assistente de IA de programação começa do zero. Ele não sabe o que você decidiu na semana passada, por que escolheu SQLite em vez de um armazenamento em arquivo plano, ou o que falta no roadmap. Na prática, isso significa:

**❌ Reexplicar o contexto a cada sessão**
Você recola o mesmo resumo de arquitetura, relista as mesmas convenções e redescreve o mesmo trabalho em andamento, repetidamente.

**❌ Decisões e seus motivos perdidos**
"Tentamos X, não funcionou por causa de Y" é exatamente o tipo de coisa que é esquecida e retestada numa sessão futura — geralmente tentando X de novo.

**❌ Nenhuma continuidade entre sessões**
Se uma sessão termina no meio de uma tarefa, a próxima não tem ideia de onde parou, a menos que você mesmo tenha anotado em algum lugar.

**❌ Nenhuma memória compartilhada entre múltiplos clientes de IA**
Se você usa Claude Code para uma tarefa e Codex para outra, eles não compartilham nada — cada um começa do zero.

---

## A solução: um vault persistente, estruturado, por projeto

O `sync82` resolve isso dando à IA seis arquivos de memória padrão por projeto — visão geral, arquitetura, stack, decisões, progresso, próximos passos — armazenados em um vault SQLite local e expostos via tools MCP que qualquer cliente compatível pode chamar.

**✅ A IA retoma exatamente de onde parou**
`load_project_context` concatena todo arquivo de memória não vazio em um único bloco — o agente começa a sessão já conhecendo o projeto.

**✅ Decisões mantêm seu raciocínio**
`decisions` é append-only e datado — cada entrada é um registro permanente do que foi decidido e por quê, nunca sobrescrito silenciosamente.

**✅ Um vault, todo cliente**
O vault é só um arquivo SQLite em disco. Claude Code, Codex, OpenCode e Antigravity podem todos apontar para o mesmo — memória construída em um cliente fica visível em outro.

**✅ Projetos multi-componente são um caso de primeira classe**
`project`/`subproject` modela um projeto pai com componentes rastreados independentemente (um monorepo, um ecossistema de plugins) sem duplicar contexto compartilhado.

**✅ Zero configuração para o uso do dia a dia**
Depois que um workspace é inicializado com `init_project_memory workspace_root=...`, toda chamada futura de tool naquele diretório resolve automaticamente o projeto certo — sem precisar do argumento `project`.

**✅ Um único binário estático, nada para instalar junto**
Sem runtime, sem `node_modules`, sem gerenciador de versão — baixe, ou faça `go install`, e ele já roda. Ele até se autoatualiza (`sync82 self-update`), tudo isso sem nunca fazer uma chamada de API de LLM ele mesmo.

---

## Comparação direta

| Situação | Sem o sync82 | Com o sync82 |
| --- | --- | --- |
| Nova sessão | Você reexplica o projeto do zero | `load_project_context` restaura tudo |
| Uma decisão passada | Você tenta lembrar, ou vasculha chats antigos | `search_memory` encontra, com o raciocínio original |
| Trocar de cliente de IA | O contexto não é transferido | Mesmo vault, qualquer cliente MCP |
| Monorepo / projeto com plugins | Você descreve cada componente toda vez | `project`/`subproject` rastreia cada um independentemente |
| Fim de sessão | Você espera lembrar de anotar | `update_project_memory` salva numa única chamada |
| Dependência de runtime | — | Nenhuma — binário estático único, sem chave de API necessária |

---

## Quando usar

`sync82` é ideal para:

| Cenário | Por que o servidor ajuda |
| --- | --- |
| **Projetos de longa duração com muitas sessões** | Arquitetura, stack e status persistem automaticamente |
| **Times ou devs solo alternando entre ferramentas de IA** | Um vault, compartilhado entre todo cliente MCP |
| **Monorepos e ecossistemas de plugins** | `project`/`subproject` evita redescrever contexto compartilhado |
| **Histórico de decisões auditável** | `decisions` nunca perde uma entrada — append-only, datado |
| **Onboarding em um código já existente** | `init_project_memory auto_detect:true` faz o bootstrap a partir do próprio código |

---

## Quando não usar

`sync82` **não é a ferramenta certa** para:

- **Tarefas curtas, de sessão única** — o overhead de inicializar memória de projeto não compensa para algo que você nunca vai revisitar.
- **Armazenar segredos ou credenciais** — é um arquivo SQLite simples, sem criptografia em repouso; trate o caminho do vault como qualquer outro arquivo de config local.
- **Substituir controle de versão** — `progress`/`decisions` registram *por que* as coisas aconteceram, não o código em si; isso é papel do git.

---

## Leitura adicional

- [O que é MCP?](./what-is-mcp.md)
- [Como o sync82 funciona](./how-sync82-works.md)
- [Glossário](./glossary.md)

---

[🏠 Voltar ao Índice](../index.md)
