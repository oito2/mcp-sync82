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
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/installer"
)

// fakeTargets returns a small, self-contained target list rooted in home,
// instead of the real installer.Targets, which depend on the CLIs and
// config directories of the machine running the test and could invoke a
// real "mcp add" or write a real client config file.
func fakeTargets(home string) []installer.Target {
	return []installer.Target{
		{Kind: installer.KindCLI, Name: "ok-cli", DetectCmd: "true", Command: "true"},
		{Kind: installer.KindCLI, Name: "ghost-cli", DetectCmd: "definitely-not-a-real-command-sync82", Command: "definitely-not-a-real-command-sync82"},
		{
			Kind:        installer.KindFile,
			Name:        "ghost-file",
			DetectDirs:  func(env installer.Env) []string { return []string{filepath.Join(env.HomeDir, "ghost")} },
			ConfigPaths: func(env installer.Env) []string { return []string{filepath.Join(env.HomeDir, "ghost", "config.json")} },
		},
	}
}

// fakeTargetsWithFailure is like fakeTargets but adds a target whose
// command is detected and exits non-zero, so installing into it fails.
func fakeTargetsWithFailure(home string) []installer.Target {
	targets := fakeTargets(home)
	return append(targets, installer.Target{Kind: installer.KindCLI, Name: "fail-cli", DetectCmd: "false", Command: "false"})
}

// TestRunInstall_SpecificTarget_Failure checks that a failing target exits
// with code 1 and reports the failure.
func TestRunInstall_SpecificTarget_Failure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), []string{"fail-cli"}, strings.NewReader(""), &stdout, &stderr, t.TempDir(), "/opt/sync82", fakeTargetsWithFailure(t.TempDir()))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 when the target genuinely fails", code)
	}
	if !strings.Contains(stdout.String(), "failed") {
		t.Errorf("stdout = %q, want it to report the failure", stdout.String())
	}
}

// TestRunInstall_NoTarget_FailureSurfacesAsExitCode checks that installing
// into all detected targets exits with code 1 when one fails and counts it in
// the summary.
func TestRunInstall_NoTarget_FailureSurfacesAsExitCode(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), nil, strings.NewReader("y\n"), &stdout, &stderr, home, "/opt/sync82", fakeTargetsWithFailure(home))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 when at least one target fails", code)
	}
	if !strings.Contains(stdout.String(), "Done. 1 installed, 0 skipped, 1 failed.") {
		t.Errorf("stdout = %q, want the summary counting 1 installed/0 skipped/1 failed", stdout.String())
	}
}

// TestRunInstall_UnknownTarget checks that an unknown target exits with code 1
// and is named in the error.
func TestRunInstall_UnknownTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), []string{"nonexistent-client"}, strings.NewReader(""), &stdout, &stderr, t.TempDir(), "/opt/sync82", fakeTargets(t.TempDir()))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Unknown target") {
		t.Errorf("stderr = %q, want it to mention the unknown target", stderr.String())
	}
}

// TestRunInstall_SpecificTarget_Success checks that installing a named file
// target configures it and exits with code 0.
func TestRunInstall_SpecificTarget_Success(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), []string{"ok-cli"}, strings.NewReader(""), &stdout, &stderr, t.TempDir(), "/opt/sync82", fakeTargets(t.TempDir()))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Registering /opt/sync82") {
		t.Errorf("stdout = %q, want it to name the binary path being registered", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ok-cli — configured.") {
		t.Errorf("stdout = %q, want it to report the install as configured", stdout.String())
	}
}

// TestRunInstall_NoTarget_AbortsWithoutConfirmation checks that declining the
// confirmation prompt installs nothing and exits with code 0.
func TestRunInstall_NoTarget_AbortsWithoutConfirmation(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), nil, strings.NewReader("n\n"), &stdout, &stderr, home, "/opt/sync82", fakeTargets(home))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Aborted.") {
		t.Errorf("stdout = %q, want \"Aborted.\"", stdout.String())
	}
	for _, want := range []string{"Detected the following MCP clients:\n  - ok-cli\n\n", "Install sync82 into all 1 detected client(s)? [y/N] "} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "ghost") {
		t.Errorf("stdout = %q, should list only detected targets", stdout.String())
	}
	if strings.Contains(stdout.String(), "Installing into") {
		t.Errorf("stdout = %q, should not have installed anything after aborting", stdout.String())
	}
}

// TestRunInstall_NoTarget_ProceedsAndSummarizes checks that confirming
// installs into every detected target and prints the summary.
func TestRunInstall_NoTarget_ProceedsAndSummarizes(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), nil, strings.NewReader("y\n"), &stdout, &stderr, home, "/opt/sync82", fakeTargets(home))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// fakeTargets: only ok-cli ("true") is detected; the undetected ones
	// are neither listed nor counted.
	if !strings.Contains(stdout.String(), "Done. 1 installed, 0 skipped, 0 failed.") {
		t.Errorf("stdout = %q, want the summary counting 1 installed/0 skipped/0 failed", stdout.String())
	}
}

// TestRunInstall_NoTarget_ClosedStdinIsAnError checks that a closed stdin
// at the confirmation prompt exits with code 1 and an error pointing to
// "sync82 install <target>", and installs nothing.
func TestRunInstall_NoTarget_ClosedStdinIsAnError(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), nil, strings.NewReader(""), &stdout, &stderr, home, "/opt/sync82", fakeTargets(home))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "stdin is closed") || !strings.Contains(stderr.String(), `sync82 install <target>`) {
		t.Errorf("stderr = %q, want the closed-stdin error and the per-target hint", stderr.String())
	}
	if strings.Contains(stdout.String(), "Installing into") || strings.Contains(stdout.String(), "configured.") {
		t.Errorf("stdout = %q, should not have installed anything", stdout.String())
	}
}

// TestRunInstall_NoTarget_NoneDetected checks that with no client detected
// nothing is asked or installed.
func TestRunInstall_NoTarget_NoneDetected(t *testing.T) {
	home := t.TempDir()
	targets := fakeTargets(home)[1:]
	var stdout, stderr bytes.Buffer
	code := RunInstall(context.Background(), nil, strings.NewReader("y\n"), &stdout, &stderr, home, "/opt/sync82", targets)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if want := "No supported MCP clients detected. Supported targets: ghost-cli, ghost-file\n"; !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if strings.Contains(stdout.String(), "[y/N]") || strings.Contains(stdout.String(), "Installing into") {
		t.Errorf("stdout = %q, should neither ask nor install", stdout.String())
	}
}

// TestRunInstall_SpecificTarget_NotDetected checks that naming a target whose
// client is not detected skips it with exit code 0.
func TestRunInstall_SpecificTarget_NotDetected(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	for _, name := range []string{"ghost-cli", "ghost-file"} {
		stdout.Reset()
		code := RunInstall(context.Background(), []string{name}, strings.NewReader(""), &stdout, &stderr, home, "/opt/sync82", fakeTargets(home))
		if code != 0 || !strings.Contains(stdout.String(), "Skipped: "+name+" not detected.") {
			t.Errorf("install %s = %d, stdout %q; want the not-detected skip", name, code, stdout.String())
		}
	}
}
