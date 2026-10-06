🌐 [English](../../en/getting-started/quickstart.md) | **Português** | 🏠 [Índice](../index.md)

---

# Início Rápido

Este guia vai te ajudar a conectar o `sync82` ao seu assistente de IA e salvar a memória da sua primeira sessão em menos de 5 minutos.

---

## 1. Conecte Seu Assistente

O jeito mais rápido: deixe o sync82 configurar o cliente para você.

```bash
sync82 install
```

Ele detecta automaticamente todo cliente suportado instalado na máquina (Claude Code, Claude Desktop, Antigravity, OpenAI Codex, OpenCode, Cursor, Zed, Cline) e escreve a configuração sozinho. Passe um target explicitamente (ex. `sync82 install claude`) para configurar só um.

### Configuração manual

Se preferir configurar um cliente manualmente — ou quiser ver exatamente o que o `install` escreve — aqui está o equivalente para o Claude Code:

```bash
claude mcp add --scope user sync82 -- /usr/local/bin/sync82   # use o caminho impresso por `which sync82`
```

Verifique se o servidor foi registrado:

```bash
claude mcp list
# → sync82: /usr/local/bin/sync82  - ✔ Connected
```

> Para os demais clientes — incluindo os caminhos exatos de arquivo de config e trechos JSON — veja os guias completos:
> [Claude Code](../guides/clients/claude-code.md) · [Claude Desktop](../guides/clients/claude-desktop.md) · [Antigravity](../guides/clients/antigravity.md) · [OpenAI Codex](../guides/clients/codex.md) · [OpenCode](../guides/clients/opencode.md) · [Cursor](../guides/clients/cursor.md) · [Zed](../guides/clients/zed.md) · [Cline](../guides/clients/cline.md)

---

## 2. Inicialize um Projeto

Com o servidor conectado, abra um chat com a IA e peça para configurar a memória do projeto em que você está.

**Digite no chat:**

```
Inicialize a memória deste projeto. Analise o código automaticamente.
```

A IA chama `init_project_memory` com `auto_detect: true` — ela lê seu `README.md`, `go.mod`/`package.json`/`Cargo.toml`/`composer.json`/etc., infere descrição, linguagens, frameworks e infraestrutura, e escreve os seis arquivos de memória padrão. Ela também pergunta antes de sobrescrever qualquer conteúdo já existente.

> Prefere responder às perguntas você mesmo em vez de auto-detecção? Basta dizer: _"Configure a memória do sync82 para este projeto — eu mesmo respondo as perguntas."_

---

## 3. Salve Sua Primeira Sessão

No final de uma sessão de trabalho, peça para a IA registrar o que aconteceu.

**Experimente este prompt:**

```
Salve o que fizemos hoje: implementamos o fluxo de login e corrigimos
o bug de timeout de sessão. Decidimos usar JWT em vez de sessões no
servidor porque precisamos de autenticação stateless para o cliente mobile.
```

A IA chama `update_project_memory` — ela adiciona uma entrada datada em `progress` e uma entrada datada em `decisions` numa única chamada, sem tocar em mais nada. Essa é a tool que você vai usar no final de quase toda sessão.

---

## 4. Retome em uma Nova Sessão

Na sua próxima sessão — um chat novo, sem memória da anterior — peça para a IA carregar o que ela já sabe.

**Experimente este prompt:**

```
Carregue o contexto do projeto antes de começarmos.
```

A IA chama `load_project_context`, que concatena todo arquivo de memória não vazio (visão geral, arquitetura, stack, decisões, progresso, próximos passos) em um único bloco — todo o histórico que você construiu, colado direto na conversa.

---

## 🎯 Próximos Passos

Agora que você está conectado, explore todo o potencial do servidor:

- **Projetos multi-componente:** Veja [Exemplos de Uso](../guides/workflows/examples.md) para o padrão `project`/`subproject` (ex. um monorepo ou um ecossistema de plugins).
- **Verifique a saúde da memória:** Peça à IA para _"Verificar se a memória deste projeto está saudável"_ — dispara `check_project_health`.
- **Explore os prompts de exemplo:** [Prompts de Exemplo](../prompts.md) — um pedido pronto para cada recurso
- [Voltar ao Índice](../index.md)

---

> 💡 **Dica:** Se a IA disser que não conhece a tool, seja explícito: _"Use a tool MCP `search_memory` para encontrar..."_.
