# Política de Segurança

🌐 **Idioma:** [English](../../SECURITY.md) · Português

## Versões Suportadas

As correções de segurança saem só para a última versão minor. Atualize com `sync82 self-update` (ou `go install github.com/oito2/mcp-sync82/cmd/sync82@latest`) para recebê-las.

| Versão | Suportada |
|---|---|
| 1.3.x | ✅ |
| < 1.3 | ❌ |

## Reportando uma Vulnerabilidade

**Não reporte vulnerabilidades de segurança em issues, discussions ou pull requests públicos.**

Reporte-as de forma privada pelo GitHub: abra a [aba **Security**](https://github.com/oito2/mcp-sync82/security) do repositório e clique em **Report a vulnerability**. Só os mantenedores veem o relato.

Áreas relevantes incluem:

- **`sync82 self-update`** — download, verificação de checksum e de assinatura, e a troca do binário.
- **`sync82 install` / `uninstall`** — os arquivos de configuração dos clientes MCP que eles reescrevem.
- **As tools MCP e a CLI** — acesso ao vault e aos arquivos lidos ou gravados por `export_memory` / `import_memory`.
- **O pipeline de release** — assinatura, atestações e publicação no MCP Registry.

Uma vulnerabilidade numa dependência entra no escopo quando afeta o sync82; uma num cliente MCP deve ser reportada aos mantenedores desse cliente.

## O que Incluir

- A versão do sync82 (`sync82 version`) e o seu sistema operacional.
- O subcomando, a tool MCP ou o arquivo afetado.
- Os passos para reproduzir e o que um atacante ganha.
- Uma prova de conceito, se tiver.

## O que Esperar

1. O relato é confirmado no advisory privado, onde a conversa continua.
2. Confirmado o problema, a correção é preparada nesse advisory e sai numa nova versão patch da linha suportada.
3. Em seguida o advisory é publicado, com crédito a você, a menos que prefira ficar anônimo.

Para conferir se um binário de release é autêntico, veja [Verificando um Binário Baixado](getting-started/installation.md#-verificando-um-binário-baixado).
