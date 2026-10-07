🌐 [English](../../en/troubleshooting/common-issues.md) | **Português** | 🏠 [Índice](../index.md)

---

# Solução de Problemas

Tendo problemas com o `sync82`? Esta página cobre os erros mais comuns com soluções diretas.

---

## 🔍 Diagnóstico Inicial

Antes de investigar qualquer problema específico, faça este passo primeiro — ele resolve a maioria dos casos:

**Verifique se o servidor está conectado:**

No chat do seu cliente de IA, rode o comando de status MCP dele (ex. `/mcp` no Claude Code). Se `sync82` aparecer conectado com 19 tools, o servidor está funcionando — o problema está na resolução de vault/projeto, não na conexão.

Se ele não aparecer, o problema está na configuração do servidor — veja [Erros de Conexão e PATH](#-erros-de-conexão-e-path) abaixo.

Se estiver conectado mas resolvendo o projeto errado (ou nenhum), pergunte:

```
Mostre a configuração atual do vault.
```

Isso chama `get_vault_config`, que reporta o caminho ativo do vault, a config global e (com `workspace_root`) o `.sync82.json` local — a maioria dos problemas de configuração fica óbvia nessa saída. Veja [Resolução de Contexto](../architecture/context-resolution.md) para entender como cada campo é derivado.

---

## 🚫 Erros de Conexão e PATH

### O cliente de IA não encontra o `sync82`

**Sintoma:** o cliente de IA reporta que não consegue se conectar ao servidor, ou o servidor aparece como "Connecting..." e nunca completa.

**Causa:** o sync82 é um binário único — o cliente só precisa do caminho absoluto quando ele não está no `PATH` que o cliente herda (IDEs e algumas CLIs nem sempre herdam o `PATH` do seu shell interativo). O `sync82 install` já registra o caminho absoluto, mas esse caminho fica desatualizado se o binário for movido depois.

**Solução:** encontre o caminho absoluto e use-o explicitamente na configuração do servidor.

```bash
which sync82
# → /usr/local/bin/sync82
```

**Claude Code (`~/.claude.json`):**
```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82"
    }
  }
}
```

**Antigravity (`~/.gemini/config/mcp_config.json`) — e o mesmo formato `mcpServers` para Claude Desktop, Cline e Cursor (o Cursor acrescenta `"type": "stdio"`):**
```json
{
  "mcpServers": {
    "sync82": {
      "command": "/usr/local/bin/sync82",
      "args": []
    }
  }
}
```

**OpenAI Codex (`~/.codex/config.toml`):**
```toml
[mcp_servers.sync82]
command = "/usr/local/bin/sync82"
```

**OpenCode (`~/.config/opencode/opencode.jsonc` ou `opencode.json` global):**
```json
{
  "mcp": {
    "sync82": {
      "type": "local",
      "command": ["/usr/local/bin/sync82"]
    }
  }
}
```

**Zed (`settings.json`):** a mesma entrada sob `"context_servers"` em vez de `"mcpServers"`.

O arquivo exato de cada cliente e SO está listado em [Instalador — Targets](../architecture/installer.md#targets).

> A correção mais fácil de todas: rode `sync82 install [target]` de novo — ele escreve o caminho absoluto sozinho.

---

### Servidor travado em "Connecting..."

**Sintoma:** O servidor aparece mas nunca sai do estado "Connecting...". Nenhuma tool é listada.

**Solução:** Reinicie a sessão do cliente — mudanças na config MCP exigem uma sessão nova em todo cliente, não só a escrita do arquivo de config.

---

### O `sync82 install` pula um cliente que você tem instalado

**Sintoma:** O `sync82 install` imprime `No supported MCP clients detected. Supported targets: ...`, deixa um cliente fora de `Detected the following MCP clients:`, ou o `sync82 install <target>` imprime `Skipped: <target> not detected.`

**Solução:** Um cliente só é detectado quando o comando dele está no `PATH` (`claude`, `codex`, `opencode`, `agy`) ou o diretório de config dele existe — veja [Detecção](../architecture/installer.md#detecção). Coloque o comando no `PATH`, ou configure o cliente à mão seguindo o [guia do cliente](../guides/clients/claude-code.md).

---

### `claude`: could not determine the scope of the sync82 registration

**Sintoma:** O `sync82 install claude` ou o `sync82 uninstall claude` falha com ``[claude] could not determine the scope of the sync82 registration from `claude mcp get sync82`; remove it manually with `claude mcp remove --scope <scope> sync82` `` ou ``[claude] sync82 is still registered in the <scope> scope after `claude mcp remove --scope <scope> sync82` ``.

**Solução:** O sync82 não conseguiu ler o escopo do `claude mcp get sync82`, ou a remoção não teve efeito. Rode `claude mcp get sync82` para ver onde o sync82 está registrado, remova com `claude mcp remove --scope <escopo> sync82` (`local`, `user` ou, a partir do diretório do projeto, `project`), depois rode `sync82 install claude` de novo.

---

## 🛠️ Erros de Build e Runtime

### `command not found: sync82` depois de compilar do código-fonte

**Sintoma:** `go build -o sync82 ./cmd/sync82` funciona, mas rodar `sync82` falha.

**Causa:** o binário foi escrito no diretório atual, que geralmente não está no `PATH`.

**Solução:**
```bash
sudo mv sync82 /usr/local/bin/
# ou, se você usou `go install`:
export PATH="$(go env GOPATH)/bin:$PATH"
```

---

### Permissão negada ao abrir o vault

**Sintoma:** Uma chamada de tool falha com um erro genérico "internal error: could not open the vault database" — o caminho de arquivo real é deliberadamente nunca incluído na mensagem devolvida ao agente (só registrado no log do servidor, em stderr).

**Causa:** O usuário rodando `sync82` não tem permissão de escrita no diretório pai do vault.

**Solução:**
```bash
mkdir -p ~/.sync82
chmod u+rwx ~/.sync82
```

Ou aponte para um local com permissão de escrita:
```bash
sync82 config set-vault /um/caminho/gravavel/vault.db
```

### A versão do schema do vault é mais nova do que este sync82 suporta

**Sintoma:** toda chamada de tool nesse vault falha. O sync82 1.0.0 responde `internal error: could not open the vault database`; as versões seguintes respondem `could not open the vault database: the vault's schema is newer than this sync82 supports; upgrade sync82`. O log do servidor (stderr, que aparece nos logs MCP do cliente) e comandos da CLI como o `sync82 export` mostram as versões envolvidas: `vault schema version 3 is newer than this sync82 supports (2)`.

**Causa:** o vault foi aberto por um sync82 mais novo, que atualizou o schema dele — a versão 3, por exemplo, cria os índices da busca de texto completo — e agora está sendo aberto por um binário mais antigo. Vários clientes que compartilham um vault podem rodar binários diferentes do sync82. O binário mais antigo recusa o vault em vez de gravar nele, porque as gravações dele deixariam desatualizadas as partes mais novas do schema.

**Solução:** atualize o binário do sync82 que cada cliente usa (`sync82 self-update`, ou `go install github.com/oito2/mcp-sync82/cmd/sync82@latest`) e reinicie os clientes. Confira o binário de cada cliente com `sync82 version` ou pelo caminho que a configuração MCP dele aponta. Uma atualização de schema não pode ser desfeita. Para voltar a um sync82 mais antigo, exporte os projetos com o mais novo (`sync82 export --all <dir>`) e importe com o mais antigo num vault novo (`sync82 import`, com `--path` apontando para o novo arquivo de vault).

---

## 🔄 Confusão de Projeto e Vault

### A IA resolve o projeto errado, ou pergunta por um que já deveria saber

**Sintoma:** Você esperava que a auto-descoberta do `.sync82.json` entrasse em ação, mas o agente pergunta por `project` mesmo assim.

**Causa:** ou o `init_project_memory` nunca rodou com um `workspace_root` neste diretório (então o `.sync82.json` nunca foi escrito), ou você está num diretório diferente daquele em que foi escrito — por padrão, a resolução só checa `workspace_root` em si, nunca um diretório pai ou irmão. Se seu `.sync82.json` fica na raiz de um monorepo acima do diretório atual, passe `search_parent_dirs: true` para habilitar a busca lá.

**Solução:**
```
Inicialize a memória deste projeto, usando este diretório como workspace root.
```

Ou verifique o que está de fato configurado:
```
Mostre a configuração atual do vault para este workspace.
```

---

### Dois clientes de IA parecem ter memórias diferentes para o mesmo projeto

**Sintoma:** Algo salvo no Claude Code não aparece quando você pergunta ao Codex sobre isso.

**Causa:** os dois clientes estão apontando para vaults diferentes — mais provavelmente um tem um override de `SYNC82_DB_PATH`/`config set-vault` que o outro não tem.

**Solução:** peça a cada cliente para reportar `get_vault_config` e compare o `path` reportado pelos dois. Padronize em um só (geralmente via `sync82 config set-vault`, já que isso é compartilhado entre todo cliente na máquina, não por cliente).

---

### `.sync82.json` ou `~/.sync82/` aparecendo no `git status`

**Sintoma:** `git status` mostra um novo arquivo `.sync82.json` não rastreado no seu projeto.

**Solução:** Adicione ao `.gitignore` se a config de auto-descoberta for específica da máquina (geralmente é — ela é pensada como conveniência local, não algo para compartilhar via git):

```gitignore
.sync82.json
```

`~/.sync82/` vive fora de qualquer diretório de projeto, então nunca aparece no `git status` por conta própria.

---

### "No vault exists at …"

**Sintoma:** uma chamada de tool responde `No vault exists at <caminho>. Check the "path" argument, the workspace's .sync82.json or "sync82 config set-vault"; a vault is created by create_project, init_project_memory or import_memory.`

**Causa:** o arquivo de vault a que a chamada chegou não existe. Tools de leitura nunca criam um vault, então um erro de digitação no `path`, um `.sync82.json` ou um `sync82 config set-vault` apontando para um lugar novo, ou uma instalação nova, terminam aqui.

**Solução:** confira qual vault a chamada usou — o `get_vault_config` informa — e corrija o `path`, o `.sync82.json` ou o valor do `set-vault` se estiver errado. Se estiver certo e o vault só for novo, crie-o com `create_project`, `init_project_memory` ou `import_memory`.

---

### Um arquivo `config.json.corrupt-*` apareceu em `~/.sync82/`

**Sintoma:** `~/.sync82/` contém um arquivo chamado `config.json.corrupt-<unix-timestamp>`, e o override do vault (`sync82 config get-vault`) ou o último projeto usado sumiu.

**Causa:** o `~/.sync82/config.json` estava vazio ou não era um JSON válido (ex. uma edição manual com erro de digitação, ou um disco que encheu). Na próxima atualização — `sync82 config set-vault`/`unset-vault`, ou uma chamada de tool que registra o último projeto usado — o sync82 moveu o arquivo quebrado para esse nome e começou uma config nova.

**Solução:** abra o arquivo `.corrupt-*`, recupere os valores de que precisa e defina-os de novo (ex. `sync82 config set-vault <caminho>`). Apague o arquivo `.corrupt-*` quando não precisar mais dele — o sync82 nunca o lê.

---

## ❓ Perguntas Comuns

### A IA diz que não conhece o sync82 ou suas tools

**Sintoma:** A IA responde que não tem acesso ao servidor ou não sabe o que é o sync82.

**Solução:** Seja mais explícito na sua instrução:

```
Use a tool load_project_context para carregar a memória deste projeto.
```

```
Use a tool search_memory para encontrar menções a "autenticação".
```

Nomear a tool explicitamente garante que a IA a use em vez de responder com conhecimento genérico.

### O contexto carregado não traz progresso ou decisões antigas

**Sintoma:** depois do `load_project_context`, a IA só conhece as últimas entradas de `progress`/`decisions`, e a resposta termina com `[older history omitted — ...]`.

**Causa:** por padrão o `load_project_context` roda no modo `summary`: as 10 entradas datadas mais recentes de cada log, dentro de 40 KB, para a resposta caber nos limites de saída de tools dos clientes MCP. O Claude Code, por exemplo, salva em arquivo um resultado com mais de 50.000 caracteres em vez de mostrá-lo.

**Solução:** peça o que precisa — uma data ("carregue as decisões desde 2026-09-01" → `since`), um número de entradas (`max_entries`) ou tudo (`mode: "full"`). Para achar uma entrada antiga específica, o `search_memory` sai mais barato que carregar o histórico inteiro.

---

## 📝 Reportando um Novo Bug

Se seu problema não está listado aqui:

1. Peça ao seu assistente para rodar `get_vault_config` e copie a saída completa.
2. Anote qual cliente de IA você está usando e seu SO/plataforma.
3. Abra uma **Issue** no GitHub: [github.com/oito2/mcp-sync82/issues](https://github.com/oito2/mcp-sync82/issues)
4. Descreva os passos para reproduzir o erro.

---

> **Dica:** Reiniciar a IDE ou o processo do cliente de IA resolve a maioria dos travamentos de servidor MCP — especialmente depois de editar arquivos de configuração como `~/.claude.json`, `~/.gemini/config/mcp_config.json`, `~/.codex/config.toml`, ou `opencode.json`.

---

[🏠 Voltar ao Índice](../index.md)
