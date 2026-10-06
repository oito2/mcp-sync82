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

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/oito2/mcp-sync82/internal/installer"
)

// uninstallUsage is the help text printed for "sync82 uninstall --help" and for invalid
// arguments.
const uninstallUsage = `Usage: sync82 uninstall [target] [--purge]

Removes sync82's registration from the given target or, with no target,
from every detected target after confirmation.

  --purge   Also delete sync82's own files in ~/.sync82 (default vault,
            config), after a separate confirmation. A vault configured
            elsewhere is never deleted.`

// RunUninstall implements "sync82 uninstall [target] [--purge]". With no
// target given, it lists the detected targets and asks for confirmation
// before removing sync82 from all of them; with none detected it only
// says so. --purge then lists sync82's files under homeDir and asks a
// separate confirmation before deleting them. It returns the process exit
// code: 0 on success, on help, or when the user declines; 1 on an unknown
// target, a closed stdin at a prompt or any failed removal or deletion; 2 on
// a usage error (an unknown flag or more than one target).
//
// targets and homeDir are passed in explicitly so tests can run the flow
// against a fake target list and home directory; the targets resolve
// their paths against the running system (installer.HostEnv). When the
// current directory cannot be read and --purge is given, a warning on
// stderr says that a vault set in a .sync82.json there is not listed.
func RunUninstall(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, homeDir string, targets []installer.Target) int {
	cwd, err := os.Getwd()
	if err != nil && slices.Contains(args, "--purge") {
		fmt.Fprintf(stderr, "Warning: cannot read the current directory (%v); a vault set in a .sync82.json there is not listed below.\n", err)
	}
	return runUninstall(ctx, args, stdin, stdout, stderr, installer.HostEnv(homeDir), cwd, targets)
}

// runUninstall is RunUninstall with the host environment env and the current
// directory cwd (used to find a .sync82.json vault) passed in.
func runUninstall(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, env installer.Env, cwd string, targets []installer.Target) int {
	targetName, purge, help, err := parseUninstallArgs(args)
	if help {
		fmt.Fprintln(stdout, uninstallUsage)
		return 0
	}
	if err != nil {
		return usageError(stderr, err)
	}

	in := bufio.NewReader(stdin)
	failed := 0
	if targetName != "" {
		target, ok := installer.FindIn(targets, targetName)
		if !ok {
			fmt.Fprintf(stderr, "Unknown target: %q\nAvailable: %s\n", targetName, strings.Join(installer.TargetNamesIn(targets), ", "))
			return 1
		}
		if installer.UninstallTarget(ctx, target, env, stdout, stderr) == installer.ResultFail {
			failed++
		}
	} else {
		detected := installer.DetectedIn(targets, env)
		confirmed, err := confirmDetected(in, stdout, targets, detected, "Remove sync82 from all %d detected client(s)? [y/N] ")
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v; run \"sync82 uninstall <target>\" to remove without asking.\n", err)
			return 1
		}
		if !confirmed && len(detected) > 0 {
			return 0
		}
		if confirmed {
			var removed, skipped int
			for _, target := range detected {
				switch installer.UninstallTarget(ctx, target, env, stdout, stderr) {
				case installer.ResultOK:
					removed++
				case installer.ResultSkip:
					skipped++
				default:
					failed++
				}
			}
			fmt.Fprintf(stdout, "\nDone. %d removed, %d skipped, %d failed.\n", removed, skipped, failed)
		}
	}

	if purge && !runPurge(in, stdout, stderr, env, cwd) {
		failed++
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// parseUninstallArgs splits the uninstall arguments into at most one
// target name (lowercased) and the --purge flag. help reports -h/--help. It
// returns an error for an unknown flag or more than one target.
func parseUninstallArgs(args []string) (target string, purge, help bool, err error) {
	var targets []string
	for _, a := range args {
		switch {
		case a == "--purge":
			purge = true
		case a == "-h" || a == "--help":
			help = true
		case strings.HasPrefix(a, "-"):
			return "", false, false, fmt.Errorf("unknown flag: %q", a)
		default:
			targets = append(targets, strings.ToLower(strings.TrimSpace(a)))
		}
	}
	if len(targets) > 1 {
		return "", false, false, fmt.Errorf("one target at a time (got %d)", len(targets))
	}
	if len(targets) == 1 {
		target = targets[0]
	}
	return target, purge, help, nil
}

// runPurge lists sync82's files in its data directory, notes every vault
// configured elsewhere (which is left untouched), and deletes the listed
// files after a "Delete these files? [y/N]" confirmation read from in. It
// returns false when a file could not be deleted.
func runPurge(in *bufio.Reader, stdout, stderr io.Writer, env installer.Env, cwd string) bool {
	files := installer.PurgeCandidates(env.HomeDir)
	vaults := installer.ConfiguredVaults(env, cwd)

	fmt.Fprintln(stdout)
	if len(vaults) > 0 {
		fmt.Fprintln(stdout, "Note: these configured vaults are outside the purge and are left untouched:")
		for _, v := range vaults {
			fmt.Fprintf(stdout, "  - %s\n", v)
		}
	}
	if len(files) == 0 {
		fmt.Fprintf(stdout, "Nothing to purge in %s.\n", installer.DataDir(env.HomeDir))
		return true
	}

	fmt.Fprintln(stdout, "--purge will delete these files (close every MCP client using sync82 first):")
	for _, f := range files {
		fmt.Fprintf(stdout, "  - %s\n", f)
	}
	yes, err := askYes(in, stdout, "Delete these files? [y/N] ")
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v; nothing was deleted.\n", err)
		return false
	}
	if !yes {
		fmt.Fprintln(stdout, "Purge cancelled.")
		return true
	}
	errs := installer.DeleteFiles(env.HomeDir, files)
	for _, err := range errs {
		fmt.Fprintln(stderr, err)
	}
	if len(errs) > 0 {
		fmt.Fprintln(stdout, "Purge incomplete.")
		return false
	}
	fmt.Fprintln(stdout, "Purge complete.")
	return true
}
