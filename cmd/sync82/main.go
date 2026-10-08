// Copyright (C) 2026  OITO2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Command sync82 is an MCP server that keeps a persistent memory of a
// software project across AI agent sessions.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/binpath"
	"github.com/oito2/mcp-sync82/internal/cli"
	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/installer"
	"github.com/oito2/mcp-sync82/internal/logging"
	"github.com/oito2/mcp-sync82/internal/selfupdate"
	"github.com/oito2/mcp-sync82/internal/server"
	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
	"github.com/oito2/mcp-sync82/internal/version"
)

// serverName is the name the MCP server reports to clients.
const serverName = "sync82"

// maxFrameBytes is the largest inbound JSON-RPC message the stdio transport
// accepts. A tool call carries at most 10 MB of content, and JSON escaping
// can grow each byte up to six (a control character becomes \u00XX), so
// 64 MiB fits the largest valid call with room for the other arguments.
// A larger frame can't be read, so the SDK can't answer it: it ends the
// session without a JSON-RPC error, and the client sees the server exit.
const maxFrameBytes = 64 << 20

// usageExitCode is the exit status of a usage error: an unknown flag, a
// flag without its value, or an unexpected argument.
const usageExitCode = 2

// usageError writes err to stderr as "Error: ..." followed by a pointer to
// --help, and returns usageExitCode for the caller to exit with.
func usageError(stderr io.Writer, err error) int {
	return commandUsageError(stderr, "", err)
}

// commandUsageError is usageError for a subcommand: the pointer names
// "sync82 <command> --help", or "sync82 --help" when command is "".
func commandUsageError(stderr io.Writer, command string, err error) int {
	help := "sync82 --help"
	if command != "" {
		help = "sync82 " + command + " --help"
	}
	fmt.Fprintf(stderr, "Error: %v\nRun '%s' for usage.\n", err, help)
	return usageExitCode
}

// main runs the command line and exits with its status code. SIGINT and
// SIGTERM cancel the context passed to run.
func main() {
	// SIGINT/SIGTERM cancel ctx, so every subcommand stops cleanly and the
	// deferred vault closes still run (a server killed by its MCP client
	// otherwise leaves its SQLite WAL un-checkpointed).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Once the first signal arrives, the default handling is restored, so a
	// second Ctrl-C ends a shutdown that hangs.
	go func() {
		<-ctx.Done()
		stop()
	}()
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run dispatches args — the command line without the program name — to a
// subcommand and returns the process exit code: 0 on success, 1 on an
// unknown subcommand or any failure, 2 on a usage error (an unknown flag or
// an unexpected argument). With no arguments it serves MCP over the process's
// own stdin/stdout. Subcommand output goes to stdout and diagnostics to
// stderr; stdin supplies answers to confirmation prompts. ctx cancellation
// stops the running subcommand.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	logger := logging.New(stderr)

	subcommand := "serve"
	if len(args) > 0 {
		subcommand = args[0]
		args = args[1:]
	}

	// Help flags count only before "--", after which every argument is
	// positional.
	optionArgs := args
	if i := slices.Index(args, "--"); i >= 0 {
		optionArgs = args[:i]
	}
	if usage := subcommandUsage(subcommand); usage != "" && slices.ContainsFunc(optionArgs, isHelpFlag) {
		fmt.Fprintln(stdout, usage)
		return 0
	}

	switch subcommand {
	case "serve", "-h", "--help", "help", "-v", "--version", "version":
		if len(args) > 0 {
			return usageError(stderr, fmt.Errorf("%s takes no arguments, got %q", subcommand, args))
		}
	}

	switch subcommand {
	case "serve":
		if err := runServe(ctx, logger); err != nil {
			logger.Error("server failed", "error", err)
			return 1
		}
		return 0
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintln(stdout, version.Get())
		return 0
	case "config":
		return cli.RunConfig(args, stdout, stderr)
	case "install":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "resolve home directory: %v\n", err)
			return 1
		}
		binaryPath, err := binpath.Resolve()
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		return RunInstall(ctx, args, stdin, stdout, stderr, homeDir, binaryPath, installer.Targets)
	case "uninstall":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "resolve home directory: %v\n", err)
			return 1
		}
		return RunUninstall(ctx, args, stdin, stdout, stderr, homeDir, installer.Targets)
	case "self-update":
		deps := selfupdate.Deps{
			Version:        version.Get(),
			GOOS:           runtime.GOOS,
			GOARCH:         runtime.GOARCH,
			APIURL:         selfupdate.DefaultAPIURL,
			HTTPClient:     http.DefaultClient,
			ExecutablePath: binpath.Resolve,
		}
		return selfupdate.RunSelfUpdate(ctx, args, deps, stdin, stdout, stderr)
	case "export", "import":
		stores := store.NewManager()
		defer func() {
			if err := stores.Close(); err != nil {
				logger.Error("close store manager", "error", err)
			}
		}()
		deps := memoryCmdDeps{Stdout: stdout, Stderr: stderr, Resolver: tools.NewResolver(config.DefaultVaultPath(), logger), Stores: stores}
		if subcommand == "export" {
			return RunExport(ctx, args, deps)
		}
		return RunImport(ctx, args, deps)
	case "search", "context", "health":
		stores := store.NewManager()
		defer func() {
			if err := stores.Close(); err != nil {
				logger.Error("close store manager", "error", err)
			}
		}()
		// Read-only commands: the last used project is never recorded.
		resolver := tools.NewResolver(config.DefaultVaultPath(), logger)
		resolver.SkipRemember = true
		deps := memoryCmdDeps{Stdout: stdout, Stderr: stderr, Resolver: resolver, Stores: stores}
		switch subcommand {
		case "search":
			return RunSearch(ctx, args, deps)
		case "context":
			return RunContext(ctx, args, deps)
		default:
			return RunHealth(ctx, args, deps)
		}
	default:
		if strings.HasPrefix(subcommand, "-") {
			return usageError(stderr, fmt.Errorf("unknown flag %q", subcommand))
		}
		fmt.Fprintf(stderr, "unknown subcommand %q\n", subcommand)
		printUsage(stderr)
		return 1
	}
}

// printUsage writes the list of every top-level subcommand to w. It is the
// output of "sync82 help", "-h" and "--help", and is appended to the error
// output for an unrecognized subcommand or invalid arguments.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: sync82 [command]

Commands:
  (none), serve Start the MCP server over stdio
  install       Wire sync82 into a supported MCP client
  uninstall     Remove sync82 from MCP clients (--purge also deletes ~/.sync82 files)
  config        Manage the global vault path override
  self-update   Check for and install an update
  export        Dump a project's memory to plain .md files (--all for the whole vault)
  import        Restore a project's memory from plain .md files (the inverse of export)
  search        Search the memory, like the search_memory tool
  context       Print a project's memory, like the load_project_context tool
  health        Check a project, or with --all every project, for missing files
  version       Print the installed version
  help          Show this message

Run 'sync82 <command> --help' for a command's options.`)
}

// isHelpFlag reports whether arg asks for help: -h or --help.
func isHelpFlag(arg string) bool {
	return arg == "-h" || arg == "--help"
}

// subcommandUsage returns the help text "sync82 <subcommand> --help"
// prints, or "" for a subcommand without one (uninstall prints its own,
// and help/version take no options).
func subcommandUsage(subcommand string) string {
	switch subcommand {
	case "serve":
		return `Usage: sync82 [serve]

Starts the MCP server over stdio. MCP clients launch it; it is not meant to
be run by hand. It stops on SIGINT/SIGTERM or when stdin closes.`
	case "install":
		return `Usage: sync82 install [target]

Registers sync82 in the given MCP client or, with no target, in every
detected client after confirmation.

Targets: ` + strings.Join(installer.TargetNamesIn(installer.Targets), ", ")
	case "config":
		return `Usage: sync82 config set-vault <path>
       sync82 config get-vault
       sync82 config unset-vault

Manages the global vault path, used when no path argument or .sync82.json
names one. get-vault shows it, or the vault used instead.`
	case "self-update":
		return `Usage: sync82 self-update [--check] [--yes]
       sync82 self-update --rollback

Updates sync82 to the latest release, after confirmation.

  --check      Only report whether an update is available
  --yes, -y    Update without asking
  --rollback   Restore the version replaced by the last update`
	case "export":
		return `Usage: sync82 export <project> [subproject] <output-dir> [--path <vault>]
       sync82 export --all <output-dir> [--path <vault>]

Writes a project's memory, or with --all every project's, as .md files.`
	case "search":
		return `Usage: sync82 search <query> [--project <project>] [--subproject <subproject>]
                     [--match words|phrase|exact] [--kinds <k1,k2>] [--since <date>]
                     [--until <date>] [--limit <n>] [--offset <n>] [--context-lines <n>]
                     [--json] [--path <vault>]

Searches the vault, or one project, like the search_memory tool, and
prints its text or, with --json, its JSON report. No match is not an error.`
	case "context":
		return `Usage: sync82 context <project> [subproject] [--full] [--since <date>]
                      [--max-entries <n>] [--max-bytes <n>] [--files <f1,f2>]
                      [--path <vault>]

Prints a project's memory as the load_project_context tool returns it: the
recent history by default, everything with --full.`
	case "health":
		return `Usage: sync82 health <project> [subproject] [--json] [--stale-days <n>] [--path <vault>]
       sync82 health --all [--json] [--stale-days <n>] [--path <vault>]

Checks that a project, or with --all every project of the vault, has the
six standard files, and lists warnings. Exits with 1 when one is missing.`
	case "import":
		return `Usage: sync82 import <project> [subproject] <input-dir> [--path <vault>] [--dry-run]

Reads every <kind>.md file in <input-dir> into the project, overwriting the
kinds it already has. --dry-run reports what would change and writes nothing.`
	default:
		return ""
	}
}

// runServe serves MCP over stdio until stdin closes or ctx is cancelled
// (SIGINT/SIGTERM), then closes every open vault. Cancellation is a clean
// shutdown, not an error. It returns the error from the server run for any
// other failure; a failure to close the vaults is only logged to logger.
func runServe(ctx context.Context, logger *slog.Logger) error {
	stores := store.NewManager()
	defer func() {
		if err := stores.Close(); err != nil {
			logger.Error("close store manager", "error", err)
		}
	}()

	resolver := tools.NewResolver(config.DefaultVaultPath(), logger)

	s := server.New(serverName, version.Get(), logger, server.Options{
		Tools:             tools.Registered(resolver, stores),
		Prompts:           tools.Prompts,
		ResourceTemplates: tools.ResourceTemplates,
		Resources:         &tools.Resources{Resolver: resolver, Stores: stores},
	})
	if err := s.Run(ctx, &mcp.StdioTransport{MaxLineLength: maxFrameBytes}); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
