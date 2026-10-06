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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/installer"
)

// uninstallFixture is a fake home holding one file target with a sync82
// entry next to another server, plus sync82's data directory.
type uninstallFixture struct {
	home, config string
	targets      []installer.Target
}

// newUninstallFixture builds a fixture in a temporary home with a CLI target
// that is never detected and a file target whose config holds a sync82 entry
// next to another server.
func newUninstallFixture(t *testing.T) uninstallFixture {
	t.Helper()
	home := t.TempDir()
	config := filepath.Join(home, "app", "config.json")
	writeTestFile(t, config, `{"mcpServers":{"sync82":{"command":"/opt/sync82"},"other":{"command":"x"}}}`)
	targets := []installer.Target{
		{
			Kind:       installer.KindCLI,
			Name:       "ghost-cli",
			DetectCmd:  "definitely-not-a-real-command-sync82",
			Command:    "definitely-not-a-real-command-sync82",
			GetArgs:    []string{"x"},
			RemoveArgs: func(string) []string { return []string{"x"} },
		},
		{
			Kind:        installer.KindFile,
			Name:        "app",
			Shape:       installer.Shape{Key: "mcpServers"},
			DetectDirs:  func(env installer.Env) []string { return []string{filepath.Join(env.HomeDir, "app")} },
			ConfigPaths: func(env installer.Env) []string { return []string{filepath.Join(env.HomeDir, "app", "config.json")} },
		},
	}
	return uninstallFixture{home: home, config: config, targets: targets}
}

// run executes runUninstall with the fixture's targets and a Linux
// environment where no command is found on PATH, feeding input to stdin. It
// returns the exit code, stdout and stderr.
func (f uninstallFixture) run(t *testing.T, args []string, input string) (int, string, string) {
	t.Helper()
	env := installer.Env{
		GOOS:     "linux",
		HomeDir:  f.home,
		Getenv:   func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
	}
	var stdout, stderr bytes.Buffer
	code := runUninstall(context.Background(), args, strings.NewReader(input), &stdout, &stderr, env, "", f.targets)
	return code, stdout.String(), stderr.String()
}

// writeTestFile writes content to path, creating parent directories, and
// fails the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// hasEntry reports whether the fixture's config file still contains a sync82
// entry. It fails the test if the file cannot be read.
func (f uninstallFixture) hasEntry(t *testing.T) bool {
	t.Helper()
	raw, err := os.ReadFile(f.config)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(raw), `"sync82"`)
}

// TestRunUninstall_WarnsWhenCurrentDirIsUnreadable checks that, when the
// current directory has been removed, "--purge" prints a warning on stderr
// that a .sync82.json vault there is not listed. It runs only on Linux,
// where the current directory can be removed while in use.
func TestRunUninstall_WarnsWhenCurrentDirIsUnreadable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("removing the current directory is only possible on Linux")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Getwd(); err == nil {
		t.Skip("os.Getwd still succeeds after removing the current directory")
	}

	var stdout, stderr bytes.Buffer
	RunUninstall(context.Background(), []string{"--purge", "not-a-target"}, strings.NewReader(""), &stdout, &stderr, t.TempDir(), nil)
	if !strings.Contains(stderr.String(), "cannot read the current directory") {
		t.Errorf("stderr = %q, want the unreadable-directory warning", stderr.String())
	}
}

// TestParseUninstallArgs checks target lowercasing, the --purge and --help
// flags, and the errors for several targets or unknown flags.
func TestParseUninstallArgs(t *testing.T) {
	cases := []struct {
		args          []string
		target        string
		purge, help   bool
		wantsAnyError bool
	}{
		{nil, "", false, false, false},
		{[]string{"Claude"}, "claude", false, false, false},
		{[]string{"--purge", "zed"}, "zed", true, false, false},
		{[]string{"zed", "--purge"}, "zed", true, false, false},
		{[]string{"--help"}, "", false, true, false},
		{[]string{"zed", "cursor"}, "", false, false, true},
		{[]string{"--force"}, "", false, false, true},
	}
	for _, c := range cases {
		target, purge, help, err := parseUninstallArgs(c.args)
		if (err != nil) != c.wantsAnyError || target != c.target || purge != c.purge || help != c.help {
			t.Errorf("parseUninstallArgs(%q) = %q, %v, %v, %v", c.args, target, purge, help, err)
		}
	}
}

// TestRunUninstall_InvalidArguments checks that an unknown target exits
// with code 1, that several targets or an unknown flag are usage errors
// (exit code 2), and that the entry stays in place.
func TestRunUninstall_InvalidArguments(t *testing.T) {
	f := newUninstallFixture(t)
	for _, c := range []struct {
		args []string
		code int
	}{{[]string{"nonexistent-client"}, 1}, {[]string{"app", "ghost-cli"}, usageExitCode}, {[]string{"--force"}, usageExitCode}} {
		args := c.args
		code, _, errOut := f.run(t, args, "")
		if code != c.code {
			t.Errorf("uninstall %q = %d, want %d", args, code, c.code)
		}
		if args[0] == "nonexistent-client" && (!strings.Contains(errOut, "Unknown target") || !strings.Contains(errOut, "ghost-cli, app")) {
			t.Errorf("stderr = %q, want the unknown target and the available ones", errOut)
		}
	}
	if !f.hasEntry(t) {
		t.Error("invalid arguments removed the entry")
	}
}

// TestRunUninstall_Help checks that --help prints the usage and exits with
// code 0.
func TestRunUninstall_Help(t *testing.T) {
	code, out, _ := newUninstallFixture(t).run(t, []string{"--help"}, "")
	if code != 0 || !strings.Contains(out, "sync82 uninstall [target] [--purge]") {
		t.Fatalf("uninstall --help = %d, %q", code, out)
	}
}

// TestRunUninstall_SpecificTarget checks that uninstalling a named target
// removes only the sync82 entry and keeps other servers.
func TestRunUninstall_SpecificTarget(t *testing.T) {
	f := newUninstallFixture(t)
	code, out, _ := f.run(t, []string{"app"}, "")
	if code != 0 || !strings.Contains(out, "app — removed.") {
		t.Fatalf("uninstall app = %d, stdout %q", code, out)
	}
	if f.hasEntry(t) {
		t.Error("sync82 entry still present")
	}
	raw, _ := os.ReadFile(f.config)
	if !strings.Contains(string(raw), `"other"`) {
		t.Errorf("config = %s, want the other server kept", raw)
	}
}

// TestRunUninstall_NoTarget_AbortsWithoutConfirmation checks that declining
// the confirmation leaves the entry in place.
func TestRunUninstall_NoTarget_AbortsWithoutConfirmation(t *testing.T) {
	f := newUninstallFixture(t)
	code, out, _ := f.run(t, nil, "n\n")
	if code != 0 || !strings.Contains(out, "Aborted.") || !strings.Contains(out, "Detected the following MCP clients:\n  - app\n") {
		t.Fatalf("uninstall = %d, stdout %q", code, out)
	}
	if !strings.Contains(out, "Remove sync82 from all 1 detected client(s)? [y/N] ") || strings.Contains(out, "ghost-cli") {
		t.Errorf("stdout = %q, want the question over the detected targets only", out)
	}
	if !f.hasEntry(t) {
		t.Error("declined uninstall removed the entry")
	}
}

// TestRunUninstall_NoTarget_ProceedsAndSummarizes checks that confirming
// removes the entry from every detected target and prints the summary.
func TestRunUninstall_NoTarget_ProceedsAndSummarizes(t *testing.T) {
	f := newUninstallFixture(t)
	code, out, _ := f.run(t, nil, "y\n")
	if code != 0 || !strings.Contains(out, "Done. 1 removed, 0 skipped, 0 failed.") {
		t.Fatalf("uninstall = %d, stdout %q", code, out)
	}
	if f.hasEntry(t) {
		t.Error("sync82 entry still present")
	}
}

// TestRunUninstall_NoTarget_NoneDetected checks that with no client
// detected nothing is asked or removed, while --purge still runs.
func TestRunUninstall_NoTarget_NoneDetected(t *testing.T) {
	f := newUninstallFixture(t)
	f.targets[1].DetectDirs = func(env installer.Env) []string { return []string{filepath.Join(env.HomeDir, "missing")} }
	dir := writeDataDir(t, f.home)
	code, out, _ := f.run(t, []string{"--purge"}, "n\n")
	if code != 0 {
		t.Fatalf("uninstall = %d, stdout %q", code, out)
	}
	if want := "No supported MCP clients detected. Supported targets: ghost-cli, app\n"; !strings.Contains(out, want) {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if strings.Contains(out, "detected client(s)?") || !strings.Contains(out, "Purge cancelled.") {
		t.Errorf("stdout = %q, want no target question and the purge question", out)
	}
	if !f.hasEntry(t) {
		t.Error("an undetected target was cleaned")
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge.db")); err != nil {
		t.Error("declined purge deleted files")
	}
}

// TestRunUninstall_SpecificTarget_NotDetected checks that an explicit CLI
// target whose client is missing is skipped, while an explicit file target
// is cleaned even when its client is no longer detected.
func TestRunUninstall_SpecificTarget_NotDetected(t *testing.T) {
	f := newUninstallFixture(t)
	code, out, _ := f.run(t, []string{"ghost-cli"}, "")
	if code != 0 || !strings.Contains(out, "Skipped: ghost-cli not detected.") {
		t.Fatalf("uninstall ghost-cli = %d, stdout %q", code, out)
	}
	f.targets[1].DetectDirs = nil
	code, out, _ = f.run(t, []string{"app"}, "")
	if code != 0 || !strings.Contains(out, "app — removed.") || f.hasEntry(t) {
		t.Fatalf("uninstall app = %d, stdout %q; want the entry removed", code, out)
	}
}

// TestRunUninstall_FailureSurfacesAsExitCode checks that a target whose
// removal fails makes the command exit with code 1.
func TestRunUninstall_FailureSurfacesAsExitCode(t *testing.T) {
	f := newUninstallFixture(t)
	writeTestFile(t, f.config, "{\n// comment\n\"mcpServers\": {\"sync82\": {}}\n}")
	code, out, _ := f.run(t, []string{"app"}, "")
	if code != 1 || !strings.Contains(out, "app — failed") {
		t.Fatalf("uninstall = %d, stdout %q; want a failure", code, out)
	}
}

// writeDataDir fills <home>/.sync82 with sync82's files plus one
// unrelated file, returning the data directory.
func writeDataDir(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".sync82")
	for _, name := range []string{"knowledge.db", "knowledge.db-shm", "config.json", "config.lock", "keep.txt"} {
		writeTestFile(t, filepath.Join(dir, name), "{}")
	}
	return dir
}

// TestRunUninstall_PurgeAsksSeparately checks that --purge asks its own
// question after the target confirmation and that declining it deletes
// nothing.
func TestRunUninstall_PurgeAsksSeparately(t *testing.T) {
	f := newUninstallFixture(t)
	dir := writeDataDir(t, f.home)
	code, out, _ := f.run(t, []string{"--purge"}, "y\nn\n")
	if code != 0 {
		t.Fatalf("uninstall --purge = %d, stdout %q", code, out)
	}
	if !strings.Contains(out, "Remove sync82 from all 1 detected client(s)? [y/N]") || !strings.Contains(out, "Delete these files? [y/N]") || !strings.Contains(out, "Purge cancelled.") {
		t.Errorf("stdout = %q, want both questions and the cancellation", out)
	}
	if f.hasEntry(t) {
		t.Error("target confirmation was not honored")
	}
	for _, name := range []string{"knowledge.db", "config.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("declined purge deleted %s", name)
		}
	}
}

// TestRunUninstall_ClosedStdinIsAnError checks that a closed stdin at the
// target confirmation, or at the --purge confirmation, exits with code 1
// and an error instead of being read as "no", and that nothing is removed
// or deleted without an answer.
func TestRunUninstall_ClosedStdinIsAnError(t *testing.T) {
	f := newUninstallFixture(t)
	code, _, errOut := f.run(t, nil, "")
	if code != 1 || !strings.Contains(errOut, "stdin is closed") {
		t.Fatalf("uninstall with closed stdin = %d, stderr %q; want 1 and the closed-stdin error", code, errOut)
	}
	if !f.hasEntry(t) {
		t.Error("entry removed without a confirmation")
	}

	dir := writeDataDir(t, f.home)
	code, _, errOut = f.run(t, []string{"--purge"}, "y\n")
	if code != 1 || !strings.Contains(errOut, "stdin is closed") || !strings.Contains(errOut, "nothing was deleted") {
		t.Fatalf("purge with closed stdin = %d, stderr %q; want 1 and the closed-stdin error", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge.db")); err != nil {
		t.Errorf("purge deleted knowledge.db without a confirmation: %v", err)
	}
}

// TestRunUninstall_PurgeDeletesOnlySync82Files checks that --purge removes
// sync82's own files from the data directory and keeps unrelated ones.
func TestRunUninstall_PurgeDeletesOnlySync82Files(t *testing.T) {
	f := newUninstallFixture(t)
	dir := writeDataDir(t, f.home)
	writeTestFile(t, filepath.Join(dir, "config.json"), `{"vaultPath":"`+filepath.ToSlash(filepath.Join(f.home, "custom", "vault.db"))+`"}`)
	custom := filepath.Join(f.home, "custom", "vault.db")
	writeTestFile(t, custom, "db")

	code, out, _ := f.run(t, []string{"app", "--purge"}, "y\n")
	if code != 0 || !strings.Contains(out, "Purge complete.") {
		t.Fatalf("uninstall app --purge = %d, stdout %q", code, out)
	}
	if !strings.Contains(out, "left untouched") || !strings.Contains(out, custom) {
		t.Errorf("stdout = %q, want the custom vault listed as untouched", out)
	}
	for _, name := range []string{"knowledge.db", "knowledge.db-shm", "config.json", "config.lock"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s not deleted", name)
		}
	}
	for _, p := range []string{filepath.Join(dir, "keep.txt"), custom} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s deleted: %v", p, err)
		}
	}
}

// TestRunUninstall_PurgeAfterDeclinedTargetsDoesNothing checks that declining
// the target confirmation skips the purge question and deletes nothing.
func TestRunUninstall_PurgeAfterDeclinedTargetsDoesNothing(t *testing.T) {
	f := newUninstallFixture(t)
	dir := writeDataDir(t, f.home)
	code, out, _ := f.run(t, []string{"--purge"}, "n\ny\n")
	if code != 0 || strings.Contains(out, "Delete these files?") {
		t.Fatalf("uninstall = %d, stdout %q; want no purge question after aborting", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "knowledge.db")); err != nil {
		t.Error("aborted uninstall purged files")
	}
}
