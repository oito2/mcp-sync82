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
const maxFrameBytes = 64 << 20

// usageExitCode is the exit status of a usage error: an unknown flag, a
// flag without its value, or an unexpected argument.
const usageExitCode = 2

// usageError writes err to stderr as "Error: ..." followed by a pointer to
// --help, and returns usageExitCode for the caller to exit with.
func usageError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "Error: %v\nRun 'sync82 --help' for usage.\n", err)
	return usageExitCode
}

// main runs the command line and exits with its status code. SIGINT and
// SIGTERM cancel the context passed to run.
func main() {
	// SIGINT/SIGTERM cancel ctx, so every subcommand stops cleanly and the
	// deferred vault closes still run (a server killed by its MCP client
	// otherwise leaves its SQLite WAL un-checkpointed).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
	logger := logging.New()

	subcommand := "serve"
	if len(args) > 0 {
		subcommand = args[0]
		args = args[1:]
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
  (none)        Start the MCP server over stdio
  install       Wire sync82 into a supported MCP client
  uninstall     Remove sync82 from MCP clients (--purge also deletes ~/.sync82 files)
  config        Manage the global vault path override
  self-update   Check for and install an update
  export        Dump a project's memory to plain .md files (--all for the whole vault)
  import        Restore a project's memory from plain .md files (the inverse of export)
  version       Print the installed version
  help          Show this message`)
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

	s := server.New(serverName, version.Get(), logger, tools.Registered(resolver, stores), &tools.Resources{Resolver: resolver, Stores: stores})
	if err := s.Run(ctx, &mcp.StdioTransport{MaxLineLength: maxFrameBytes}); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
