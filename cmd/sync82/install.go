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
)

// RunInstall implements "sync82 install [target]". With no target given, it
// lists the detected targets and asks for confirmation before installing
// into all of them. It returns the process exit code: 0 on success or when
// the user declines, 1 on an unknown target, a closed stdin at the prompt or
// any failed installation, 2 on a usage error (a flag or more than one
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
	fmt.Fprintf(stdout, "Registering %s — run \"sync82 install\" again after moving the binary.\n", binaryPath)

	if len(args) > 1 {
		return usageError(stderr, fmt.Errorf("install takes one target at a time (got %d arguments)", len(args)))
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "-") {
		return usageError(stderr, fmt.Errorf("unknown install flag %q", args[0]))
	}
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
		if installer.InstallTarget(ctx, target, env, binaryPath, stdout, stderr) == installer.ResultFail {
			return 1
		}
		return 0
	}

	detected := installer.DetectedIn(targets, env)
	confirmed, err := confirmDetected(bufio.NewReader(stdin), stdout, targets, detected, "Install sync82 into all %d detected client(s)? [y/N] ")
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v; run \"sync82 install <target>\" to install without asking.\n", err)
		return 1
	}
	if !confirmed {
		return 0
	}

	var ok, skipped, failed int
	for _, target := range detected {
		switch installer.InstallTarget(ctx, target, env, binaryPath, stdout, stderr) {
		case installer.ResultOK:
			ok++
		case installer.ResultSkip:
			skipped++
		default:
			failed++
		}
	}
	fmt.Fprintf(stdout, "\nDone. %d installed, %d skipped, %d failed.\n", ok, skipped, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// confirmDetected lists the detected targets and asks prompt, formatted
// with their count, reporting whether the answer read from in was yes. A
// refusal prints "Aborted.". With no target detected it prints the names of
// all targets instead and returns false without asking.
func confirmDetected(in *bufio.Reader, stdout io.Writer, targets, detected []installer.Target, prompt string) (bool, error) {
	if len(detected) == 0 {
		fmt.Fprintf(stdout, "No supported MCP clients detected. Supported targets: %s\n", strings.Join(installer.TargetNamesIn(targets), ", "))
		return false, nil
	}
	fmt.Fprintln(stdout, "Detected the following MCP clients:")
	for _, name := range installer.TargetNamesIn(detected) {
		fmt.Fprintf(stdout, "  - %s\n", name)
	}
	fmt.Fprintln(stdout)
	yes, err := askYes(in, stdout, fmt.Sprintf(prompt, len(detected)))
	if err != nil {
		return false, err
	}
	if !yes {
		fmt.Fprintln(stdout, "Aborted.")
		return false, nil
	}
	return true, nil
}

// errNoAnswer is returned by askYes when stdin ends before any answer is
// read, as with a closed or redirected-from-/dev/null stdin.
var errNoAnswer = errors.New("no answer to the confirmation prompt: stdin is closed")

// askYes prints prompt and reports whether the next line read from in is
// "y" or "yes", ignoring case and surrounding spaces. It returns
// errNoAnswer when in ends before a non-empty answer, and the read error
// for any other read failure.
func askYes(in *bufio.Reader, stdout io.Writer, prompt string) (bool, error) {
	fmt.Fprint(stdout, prompt)
	line, err := in.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if err != nil && answer == "" {
		fmt.Fprintln(stdout)
		if errors.Is(err, io.EOF) {
			return false, errNoAnswer
		}
		return false, err
	}
	return answer == "y" || answer == "yes", nil
}
