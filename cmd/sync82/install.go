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
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/oito2/mcp-sync82/internal/installer"
	"github.com/oito2/mcp-sync82/internal/prompt"
)

// RunInstall implements "sync82 install [target]". With no target given, it
// lists the detected targets and asks for confirmation before installing
// into all of them. It returns the process exit code: 0 on success or when
// the user declines, 1 on an unknown target, a closed stdin at the prompt,
// any failed installation or one left for a manual edit, 2 on a usage error (a flag or more than one
// target).
//
// binaryPath is the absolute path every client is configured to launch.
// targets and homeDir are passed in explicitly so tests can run the flow
// against a fake target list and home directory; the targets resolve
// their paths against the running system (installer.HostEnv).
func RunInstall(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, homeDir, binaryPath string, targets []installer.Target) int {
	return runInstall(ctx, args, stdin, stdout, stderr, installer.HostEnv(homeDir), binaryPath, targets)
}

// runInstall is RunInstall with the host environment env passed in, so tests
// can substitute a fake one.
func runInstall(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, env installer.Env, binaryPath string, targets []installer.Target) int {
	if len(args) > 1 {
		return commandUsageError(stderr, "install", fmt.Errorf("install takes one target at a time (got %d arguments)", len(args)))
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "-") {
		return commandUsageError(stderr, "install", fmt.Errorf("unknown install flag %q", args[0]))
	}
	fmt.Fprintf(stdout, "Registering %s — run \"sync82 install\" again after moving the binary.\n", binaryPath)
	targetName := ""
	if len(args) > 0 {
		targetName = strings.ToLower(strings.TrimSpace(args[0]))
	}

	if targetName != "" {
		target, ok := installer.FindIn(targets, targetName)
		if !ok {
			fmt.Fprintf(stderr, "Unknown target: %q\nAvailable: %s\n", targetName, strings.Join(installer.TargetNamesIn(targets), ", "))
			return 1
		}
		switch installer.InstallTarget(ctx, target, env, binaryPath, stdout, stderr) {
		case installer.ResultFail, installer.ResultManual:
			return 1
		}
		return 0
	}

	detected := installer.DetectedIn(targets, env)
	confirmed, err := confirmDetected(ctx, bufio.NewReader(stdin), stdout, targets, detected, "Install sync82 into all %d detected client(s)? [y/N] ")
	if err != nil {
		reportPromptError(stderr, err, "run \"sync82 install <target>\" to install without asking")
		return 1
	}
	if !confirmed {
		return 0
	}

	var ok, skipped, manual, failed int
	for _, target := range detected {
		switch installer.InstallTarget(ctx, target, env, binaryPath, stdout, stderr) {
		case installer.ResultOK:
			ok++
		case installer.ResultSkip:
			skipped++
		case installer.ResultManual:
			manual++
		default:
			failed++
		}
	}
	fmt.Fprintf(stdout, "\nDone. %d installed, %d skipped, %d need a manual step, %d failed.\n", ok, skipped, manual, failed)
	if failed > 0 || manual > 0 {
		return 1
	}
	return 0
}

// confirmDetected lists the detected targets and asks prompt, formatted
// with their count, reporting whether the answer read from in was yes. A
// refusal prints "Aborted.". With no target detected it prints the names of
// all targets instead and returns false without asking.
func confirmDetected(ctx context.Context, in *bufio.Reader, stdout io.Writer, targets, detected []installer.Target, question string) (bool, error) {
	if len(detected) == 0 {
		fmt.Fprintf(stdout, "No supported MCP clients detected. Supported targets: %s\n", strings.Join(installer.TargetNamesIn(targets), ", "))
		return false, nil
	}
	fmt.Fprintln(stdout, "Detected the following MCP clients:")
	for _, name := range installer.TargetNamesIn(detected) {
		fmt.Fprintf(stdout, "  - %s\n", name)
	}
	fmt.Fprintln(stdout)
	yes, err := prompt.AskYes(ctx, in, stdout, fmt.Sprintf(question, len(detected)))
	if err != nil {
		return false, err
	}
	if !yes {
		fmt.Fprintln(stdout, "Aborted.")
		return false, nil
	}
	return true, nil
}

// reportPromptError prints the error of a confirmation prompt to stderr:
// "Interrupted; nothing was changed." when the command was interrupted
// (Ctrl-C) while it waited, otherwise the error followed by hint.
func reportPromptError(stderr io.Writer, err error, hint string) {
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(stderr, "Interrupted; nothing was changed.")
		return
	}
	fmt.Fprintf(stderr, "Error: %v; %s.\n", err, hint)
}
