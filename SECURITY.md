# Security Policy

🌐 **Language:** English · [Português](docs/pt-br/seguranca.md)

## Supported Versions

Security fixes are released only for the latest minor version. Update with `sync82 self-update` (or `go install github.com/oito2/mcp-sync82/cmd/sync82@latest`) to receive them.

| Version | Supported |
|---|---|
| 1.3.x | ✅ |
| < 1.3 | ❌ |

## Reporting a Vulnerability

**Please don't report security vulnerabilities in public issues, discussions or pull requests.**

Report them privately through GitHub: open the repository's [**Security** tab](https://github.com/oito2/mcp-sync82/security) and click **Report a vulnerability**. Only the maintainers can see the report.

Relevant areas include:

- **`sync82 self-update`** — download, checksum and signature verification, and the binary swap.
- **`sync82 install` / `uninstall`** — the MCP client configuration files they rewrite.
- **The MCP tools and the CLI** — access to the vault and the files read or written by `export_memory` / `import_memory`.
- **The release pipeline** — signing, attestations and publication to the MCP Registry.

A vulnerability in a dependency is in scope when sync82 is affected by it; for one in an MCP client, report it to that client's maintainers.

## What to Include

- The sync82 version (`sync82 version`) and your operating system.
- The affected subcommand, MCP tool or file.
- Steps to reproduce, and what an attacker gains.
- A proof of concept, if you have one.

## What to Expect

1. The report is acknowledged in the private advisory, where the discussion continues.
2. Once the issue is confirmed, a fix is prepared in that advisory and released as a new patch version of the supported line.
3. The advisory is then published, with credit to you unless you prefer to stay anonymous.

To check that a release binary is genuine, see [Verifying a Downloaded Binary](docs/en/getting-started/installation.md#-verifying-a-downloaded-binary).
