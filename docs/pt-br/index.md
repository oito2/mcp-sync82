# Documentação — sync82 🧠

🌐 [English](../en/index.md) | **Português** | 🏠 [Voltar ao README](./leiame.md)

---

**sync82** é um servidor MCP (Model Context Protocol) que dá a agentes de IA de programação memória persistente por projeto — decisões, progresso, arquitetura, stack e próximos passos — armazenada em um vault SQLite embutido. Distribuído como um único binário estático.

Use este índice para navegar por toda a documentação.

---

## 🚀 Primeiros Passos

Configure seu ambiente e coloque o servidor no ar em minutos.

- [Instalação](./getting-started/installation.md) — Obtendo o binário (release ou `go install`) e conectando-o ao seu cliente MCP.
- [Início Rápido](./getting-started/quickstart.md) — Conecte seu assistente e salve a memória da sua primeira sessão.
- [Desinstalação](./getting-started/uninstallation.md) — Removendo o sync82 de todo cliente, apagando os dados dele e removendo o binário.

---

## 🧠 Conceitos

Entenda o que é MCP, por que este servidor existe e como ele funciona internamente.

- [O que é MCP?](./concepts/what-is-mcp.md) — Introdução ao Model Context Protocol.
- [Por que sync82?](./concepts/why-sync82.md) — O problema que o servidor resolve e quando usá-lo.
- [Como o sync82 funciona](./concepts/how-sync82-works.md) — O fluxo entre seu agente de IA, as tools e o vault.
- [Arquitetura](./concepts/architecture.md) — Modelo de armazenamento, resolução de contexto e organização dos pacotes.
- [Glossário](./concepts/glossary.md) — Termos e conceitos usados nesta documentação.

---

## 📖 Guias

### Clientes MCP

Configure o servidor no seu assistente de IA preferido.

- [Claude Code](./guides/clients/claude-code.md) — A CLI da Anthropic, registrada via `claude mcp add`.
- [Claude Desktop](./guides/clients/claude-desktop.md) — A extensão `sync82.mcpb`, ou o `claude_desktop_config.json`.
- [Antigravity](./guides/clients/antigravity.md) — IDE e CLI do Google, compartilhando `~/.gemini/config/mcp_config.json`.
- [OpenAI Codex](./guides/clients/codex.md) — A CLI da OpenAI, configurada em `~/.codex/config.toml`.
- [OpenCode](./guides/clients/opencode.md) — Agente open-source com TUI, `opencode.json(c)` global.
- [Cursor](./guides/clients/cursor.md) — Editor com IA, `~/.cursor/mcp.json` global.
- [Zed](./guides/clients/zed.md) — Editor com context servers no `settings.json`.
- [Cline](./guides/clients/cline.md) — Extensão do VS Code e CLI, `cline_mcp_settings.json`.

### Fluxos de trabalho

- [Exemplos de uso](./guides/workflows/examples.md) — Sessões reais de ponta a ponta, da configuração do projeto até o handoff.

---

## 💬 Prompts de Exemplo

- [Prompts de Exemplo](./prompts.md) — Pedidos em linguagem natural para cada recurso: configuração, sessões, busca, saída JSON, validação, manutenção e backups.

---

## 🛠️ Referência Técnica

Consulte as tools e comandos disponíveis no servidor.

- [Tools](./reference/tools.md) — As 19 tools MCP que a IA pode chamar, referência completa de parâmetros.
- [Resources](./reference/resources.md) — A memória dos projetos como resources MCP somente leitura (`sync82://projects/...`).
- [Prompts MCP](./reference/mcp-prompts.md) — Os prompts `start_session` e `end_session`.
- [CLI](./reference/cli.md) — Todos os subcomandos da CLI `sync82`: `install`, `uninstall`, `config`, `self-update`, `export`, `import`.
- [Configuração](./reference/configuration.md) — `SYNC82_DB_PATH`, `~/.sync82/config.json`, `.sync82.json`, precedência, transporte e as configurações do bundle.

---

## 🔬 Arquitetura Interna

Para quem quer entender ou contribuir com o código do servidor.

- [Storage](./architecture/storage.md) — O schema SQLite, tabelas e os tipos de memória padrão vs. customizados.
- [Resolução de Contexto](./architecture/context-resolution.md) — O modelo de resolução de projeto/vault path em 4 camadas.
- [Instalador](./architecture/installer.md) — Como `sync82 install` e `sync82 uninstall` tratam cada um dos 8 clientes MCP suportados.

---

## 🆘 Solução de Problemas

- [Problemas Comuns](./troubleshooting/common-issues.md) — Erros de conexão, PATH e confusão de vault.

---

> 💡 **Dica:** Se você está usando Claude Code ou outro assistente compatível com MCP, tente perguntar diretamente: _"Quais tools o sync82 oferece?"_ — a IA vai consultar o servidor em tempo real e responder com a lista atualizada.
