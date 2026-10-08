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

// Package cli implements the "config set-vault|get-vault|unset-vault"
// subcommand. RunConfig returns an exit code and writes to the given
// stdout/stderr writers instead of calling os.Exit or using the process's
// streams directly, so it stays a plain, easily testable function.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/mcp-sync82/internal/config"
)

// RunConfig dispatches "sync82 config <subcommand> [args...]", where args
// holds the arguments after "config" and the subcommand is one of set-vault,
// get-vault or unset-vault. Normal output goes to stdout and diagnostics to
// stderr. It returns the process exit code: 0 on success, 1 for a failing
// subcommand, and 2 (usage error) for an unknown or missing subcommand, a
// missing set-vault path or unexpected arguments.
func RunConfig(args []string, stdout, stderr io.Writer) int {
	var subcommand string
	var rest []string
	if len(args) > 0 {
		subcommand, rest = args[0], args[1:]
	}

	switch subcommand {
	case "set-vault":
		if len(rest) != 1 || rest[0] == "" {
			return usageError(stderr, "usage: sync82 config set-vault <path>")
		}
		return runSetVault(rest, stdout, stderr)
	case "get-vault", "unset-vault":
		if len(rest) > 0 {
			return usageError(stderr, fmt.Sprintf("config %s takes no arguments, got %q", subcommand, rest))
		}
		if subcommand == "get-vault" {
			return runGetVault(stdout, stderr)
		}
		return runUnsetVault(stdout, stderr)
	default:
		return usageError(stderr, fmt.Sprintf("unknown config subcommand %q (available: set-vault, get-vault, unset-vault)", subcommand))
	}
}

// usageExitCode is the exit status of a usage error.
const usageExitCode = 2

// usageError writes msg to stderr as "Error: ..." followed by a pointer to
// --help, and returns usageExitCode.
func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "Error: %s\nRun 'sync82 config --help' for usage.\n", msg)
	return usageExitCode
}

// runSetVault implements "config set-vault <path>": it resolves args[0] to an
// absolute path and stores it as the global vault path. args holds exactly
// one non-empty path, which RunConfig checks. It prints the stored path to
// stdout and returns 0, or prints the failure to stderr and returns 1 when
// the path cannot be resolved or saved.
func runSetVault(args []string, stdout, stderr io.Writer) int {
	// Stored absolute: every MCP client starts the server from its own
	// working directory, so a relative path would point each one at a
	// different file.
	vaultPath, err := filepath.Abs(config.ResolvePath(args[0]))
	if err != nil {
		fmt.Fprintf(stderr, "Failed to resolve vault path: %v\n", err)
		return 1
	}
	err = config.UpdateGlobalConfig(func(cfg *config.GlobalConfig) {
		cfg.VaultPath = vaultPath
	})
	if err != nil {
		fmt.Fprintf(stderr, "Failed to set vault path: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Global vault path set to: %s\n", vaultPath)
	return 0
}

// runGetVault implements "config get-vault": it prints the configured global
// vault path or, when none is set, the vault used instead — the
// SYNC82_DB_PATH variable's or the default path — to stdout and returns 0.
// It prints to stderr and returns 1 when the configuration cannot be read.
func runGetVault(stdout, stderr io.Writer) int {
	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		fmt.Fprintf(stderr, "Failed to read vault config: %v\n", err)
		return 1
	}
	if cfg.VaultPath != "" {
		fmt.Fprintf(stdout, "Global vault path: %s\n", cfg.VaultPath)
	} else {
		source := "the default"
		if os.Getenv(config.DBPathEnvVar) != "" {
			source = config.DBPathEnvVar
		}
		fmt.Fprintf(stdout, "No global vault configured. Using %s: %s\n", source, config.DefaultVaultPath())
	}
	return 0
}

// runUnsetVault implements "config unset-vault": it clears the global vault
// path and prints a confirmation to stdout, returning 0. It prints to stderr
// and returns 1 when the configuration cannot be updated.
func runUnsetVault(stdout, stderr io.Writer) int {
	// VaultPath has the json ",omitempty" tag, so clearing it here and
	// rewriting the file drops the key entirely.
	err := config.UpdateGlobalConfig(func(cfg *config.GlobalConfig) {
		cfg.VaultPath = ""
	})
	if err != nil {
		fmt.Fprintf(stderr, "Failed to unset vault path: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Global vault path removed. Using default.")
	return 0
}
