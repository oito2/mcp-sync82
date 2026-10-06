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

package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestCLIError_Message checks that cliError names the command with at most
// its first two arguments and uses the trimmed command output, or the run
// error when the command printed nothing.
func TestCLIError_Message(t *testing.T) {
	runErr := errors.New("exit status 1")
	cases := []struct {
		args []string
		out  string
		want string
	}{
		{[]string{"mcp", "add", "sync82", "--", "/opt/sync82"}, "  already exists\n", "codex mcp add failed: already exists"},
		{[]string{"mcp", "add"}, "", "codex mcp add failed: exit status 1"},
		{[]string{"mcp"}, " \n", "codex mcp failed: exit status 1"},
	}
	for _, c := range cases {
		if got := cliError("codex", c.args, []byte(c.out), runErr).Error(); got != c.want {
			t.Errorf("cliError(%q, %q) = %q, want %q", c.args, c.out, got, c.want)
		}
	}
}

// TestRunCLI_MissingCommandFails checks that runCLI reports a command that
// cannot be started as an error naming the command.
func TestRunCLI_MissingCommandFails(t *testing.T) {
	err := runCLI(context.Background(), "definitely-not-a-real-command-sync82", []string{"mcp", "add"})
	if err == nil || !strings.Contains(err.Error(), "definitely-not-a-real-command-sync82 mcp add failed") {
		t.Fatalf("runCLI error = %v, want a failure naming the command", err)
	}
}

// claudeGetUser, claudeGetLocal and claudeGetProject are sample outputs of
// "claude mcp get sync82" for a registration in each scope.
const (
	claudeGetUser = `sync82:
  Scope: User config (available in all your projects)
  Status: ✘ Failed to connect
  Type: stdio
  Command: /usr/local/bin/sync82
  Args:
  Environment:

To remove this server, run: claude mcp remove sync82 -s user
`
	claudeGetLocal = `sync82:
  Scope: Local config (private to you in this project)
  Status: ✔ Connected
  Type: stdio
  Command: /usr/local/bin/sync82

To remove this server, run: claude mcp remove sync82 -s local
`
	claudeGetProject = `sync82:
  Scope: Project config (shared via .mcp.json)
  Status: ⏸ Pending approval (run ` + "`claude`" + ` to approve)
  Type: stdio
  Command: /usr/local/bin/sync82

To remove this server, run: claude mcp remove sync82 -s project
`
)

// TestClaudeScope_ParsesGetOutput verifies that claudeScope extracts the scope from
// "claude mcp get" output, including the fallback to the removal hint, and
// returns "" when none is named.
func TestClaudeScope_ParsesGetOutput(t *testing.T) {
	cases := map[string]string{
		claudeGetUser:    scopeUser,
		claudeGetLocal:   scopeLocal,
		claudeGetProject: scopeProject,
		strings.ReplaceAll(claudeGetProject, "\n", "\r\n"):                          scopeProject,
		"sync82:\n  scope: LOCAL config\n":                                          scopeLocal,
		"sync82:\n\nTo remove this server, run: claude mcp remove sync82 -s user\n": scopeUser,
		"run `claude mcp remove --scope=project sync82`":                            scopeProject,
		"run claude mcp remove --scope 'local' sync82":                              scopeLocal,
		"sync82:\n  Scope: Enterprise config\n":                                     "",
		"":                                                                          "",
	}
	for out, want := range cases {
		if got := claudeScope([]byte(out)); got != want {
			t.Errorf("claudeScope(%q) = %q, want %q", out, got, want)
		}
	}
}

// fakeClaudeScript models Claude Code's "mcp get/add/remove" over the
// scopes stored as the files scope-local, scope-project and scope-user
// next to the script. "get" reports only the registration with the
// highest precedence (local, then project, then user) and fails when there
// is none; "add" refuses a scope already holding sync82; "remove" refuses
// a scope without it. A file named "unknown" makes "get" report a scope
// with no known name; a file named "sticky" makes "remove" succeed
// without removing anything. Every argument list is appended to calls.log.
const fakeClaudeScript = `#!/bin/sh
d="$(dirname "$0")"
echo "$*" >> "$d/calls.log"
case "$2" in
get)
  if [ -f "$d/unknown" ]; then printf 'sync82:\n  Scope: Enterprise config\n'; exit 0; fi
  for s in local project user; do
    if [ -f "$d/scope-$s" ]; then
      case $s in
      local) l="Local config (private to you in this project)" ;;
      project) l="Project config (shared via .mcp.json)" ;;
      user) l="User config (available in all your projects)" ;;
      esac
      printf 'sync82:\n  Scope: %s\n  Type: stdio\n\nTo remove this server, run: claude mcp remove sync82 -s %s\n' "$l" "$s"
      exit 0
    fi
  done
  echo 'No MCP server named "sync82". Run claude mcp add to add one.'
  exit 1 ;;
add)
  if [ -f "$d/scope-$4" ]; then echo "MCP server sync82 already exists in $4 config"; exit 1; fi
  touch "$d/scope-$4"
  echo "Added stdio MCP server sync82" ;;
remove)
  if [ ! -f "$d/scope-$4" ]; then echo "No MCP server named \"sync82\" in $4 scope"; exit 1; fi
  [ -f "$d/sticky" ] || rm -f "$d/scope-$4"
  echo "Removed MCP server sync82" ;;
*) exit 1 ;;
esac
`

// fakeClaude writes fakeClaudeScript with sync82 registered in scopes and
// with any marker files, and returns the claude target pointed at it and
// the script's directory.
func fakeClaude(t *testing.T, scopes []string, markers ...string) (Target, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake CLI")
	}
	dir := t.TempDir()
	command := filepath.Join(dir, "claude")
	if err := os.WriteFile(command, []byte(fakeClaudeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range scopes {
		writeFile(t, filepath.Join(dir, "scope-"+s), "", 0o644)
	}
	for _, m := range markers {
		writeFile(t, filepath.Join(dir, m), "", 0o644)
	}
	target := mustFind(t, "claude")
	target.Command = command
	target.DetectCmd = command
	return target, dir
}

// claudeScopesLeft returns the scopes still holding sync82 in a fakeClaude
// directory.
func claudeScopesLeft(t *testing.T, dir string) []string {
	t.Helper()
	var left []string
	for _, s := range []string{scopeLocal, scopeProject, scopeUser} {
		if fileExists(filepath.Join(dir, "scope-"+s)) {
			left = append(left, s)
		}
	}
	return left
}

// countCalls returns how many calls in dir's log equal call.
func countCalls(t *testing.T, dir, call string) int {
	t.Helper()
	n := 0
	for _, c := range calls(t, dir) {
		if c == call {
			n++
		}
	}
	return n
}

// assertWarningAfterStatus checks that out has a project-scope warning
// printed after the status line holding status.
func assertWarningAfterStatus(t *testing.T, out, status string) {
	t.Helper()
	si, wi := strings.Index(out, status), strings.Index(out, "Warning:")
	if si < 0 || wi < si {
		t.Errorf("stdout = %q, want %q followed by a warning", out, status)
	}
	if !strings.Contains(out, "mcp remove --scope project sync82") || !strings.Contains(out, ".mcp.json") {
		t.Errorf("stdout = %q, want the project-scope warning with the manual command", out)
	}
}

// removeUser, removeLocal, removeProject and addUser are the argument lists
// the fake claude logs for the matching removals and for the user-scope add.
const (
	removeUser    = "mcp remove --scope user sync82"
	removeLocal   = "mcp remove --scope local sync82"
	removeProject = "mcp remove --scope project sync82"
	addUser       = "mcp add --scope user sync82 -- /opt/sync82"
)

// TestClaude_InstallConfiguredThenUpdated verifies that a first install reports
// "configured." and a second one replaces the registration and reports
// "updated.".
func TestClaude_InstallConfiguredThenUpdated(t *testing.T) {
	target, dir := fakeClaude(t, nil)
	env := testEnv("linux", t.TempDir(), nil)
	if got, out, errOut := install(t, target, env); got != ResultOK || !strings.Contains(out, "claude — configured.") {
		t.Fatalf("first install = %q, stdout %q, stderr %q; want configured", got, out, errOut)
	}
	if got, out, errOut := install(t, target, env); got != ResultOK || !strings.Contains(out, "claude — updated.") {
		t.Fatalf("second install = %q, stdout %q, stderr %q; want updated", got, out, errOut)
	}
	want := []string{"mcp get sync82", addUser, "mcp get sync82", removeUser, "mcp get sync82", addUser}
	if got := calls(t, dir); !slices.Equal(got, want) {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if left := claudeScopesLeft(t, dir); !slices.Equal(left, []string{scopeUser}) {
		t.Errorf("scopes left = %v, want [user]", left)
	}
}

// TestClaude_InstallReplacesLocalAndUser verifies that install removes both the
// local and user registrations before adding one in user scope.
func TestClaude_InstallReplacesLocalAndUser(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeLocal, scopeUser})
	got, out, errOut := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultOK || !strings.Contains(out, "claude — updated.") || strings.Contains(out, "Warning:") {
		t.Fatalf("install = %q, stdout %q, stderr %q; want updated without warnings", got, out, errOut)
	}
	for _, c := range []string{removeLocal, removeUser} {
		if countCalls(t, dir, c) != 1 {
			t.Errorf("calls = %q, want one %q", calls(t, dir), c)
		}
	}
	if left := claudeScopesLeft(t, dir); !slices.Equal(left, []string{scopeUser}) {
		t.Errorf("scopes left = %v, want [user]", left)
	}
}

// TestClaude_InstallKeepsProjectAndWarns verifies that a project-scope registration
// is left in place with a warning while the user scope is replaced.
func TestClaude_InstallKeepsProjectAndWarns(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeProject, scopeUser})
	got, out, errOut := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultOK {
		t.Fatalf("install = %q, stdout %q, stderr %q", got, out, errOut)
	}
	assertWarningAfterStatus(t, out, "claude — updated.")
	if countCalls(t, dir, removeProject) != 0 {
		t.Error("the project-scope registration must never be removed")
	}
	if left := claudeScopesLeft(t, dir); !slices.Equal(left, []string{scopeProject, scopeUser}) {
		t.Errorf("scopes left = %v, want [project user]", left)
	}
}

// TestClaude_InstallWithOnlyProjectAddsUser verifies that with only a project-scope
// registration, install warns and still adds the user-scope one.
func TestClaude_InstallWithOnlyProjectAddsUser(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeProject})
	got, out, errOut := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultOK {
		t.Fatalf("install = %q, stdout %q, stderr %q", got, out, errOut)
	}
	assertWarningAfterStatus(t, out, "claude — configured.")
	if left := claudeScopesLeft(t, dir); !slices.Equal(left, []string{scopeProject, scopeUser}) {
		t.Errorf("scopes left = %v, want [project user]", left)
	}
}

// TestClaude_InstallUnknownScopeFails verifies that install fails, without adding
// anything, when the registration's scope cannot be determined.
func TestClaude_InstallUnknownScopeFails(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeUser}, "unknown")
	got, out, errOut := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultFail || !strings.Contains(out, "claude — failed") {
		t.Fatalf("install = %q, stdout %q; want a failure", got, out)
	}
	if !strings.Contains(errOut, "could not determine the scope") || !strings.Contains(errOut, "mcp remove --scope <scope> sync82") {
		t.Errorf("stderr = %q, want the manual removal instructions", errOut)
	}
	if c := calls(t, dir); !slices.Equal(c, []string{"mcp get sync82"}) {
		t.Errorf("calls = %q, want only the registration check", c)
	}
}

// TestClaude_InstallRepeatedScopeFails verifies that install fails when a scope is
// still reported after its removal.
func TestClaude_InstallRepeatedScopeFails(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeUser}, "sticky")
	got, out, errOut := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultFail || !strings.Contains(errOut, "still registered in the user scope") {
		t.Fatalf("install = %q, stdout %q, stderr %q; want a repeated-scope failure", got, out, errOut)
	}
	if countCalls(t, dir, addUser) != 0 {
		t.Error("add must not run after a failed removal")
	}
}

// TestClaude_UninstallUser verifies that uninstall removes a user-scope
// registration.
func TestClaude_UninstallUser(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeUser})
	got, out, errOut := uninstall(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultOK || !strings.Contains(out, "claude — removed.") || strings.Contains(out, "Warning:") {
		t.Fatalf("uninstall = %q, stdout %q, stderr %q; want removed", got, out, errOut)
	}
	want := []string{"mcp get sync82", removeUser, "mcp get sync82"}
	if c := calls(t, dir); !slices.Equal(c, want) {
		t.Errorf("calls = %q, want %q", c, want)
	}
}

// TestClaude_UninstallLocalAndUser verifies that uninstall removes both local and
// user registrations.
func TestClaude_UninstallLocalAndUser(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeLocal, scopeUser})
	if got, out, errOut := uninstall(t, target, testEnv("linux", t.TempDir(), nil)); got != ResultOK || !strings.Contains(out, "claude — removed.") {
		t.Fatalf("uninstall = %q, stdout %q, stderr %q; want removed", got, out, errOut)
	}
	if left := claudeScopesLeft(t, dir); len(left) != 0 {
		t.Errorf("scopes left = %v, want none", left)
	}
}

// TestClaude_UninstallProjectAndUser verifies that uninstall removes the user scope
// behind a project-scope registration and warns about the latter.
func TestClaude_UninstallProjectAndUser(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeLocal, scopeProject, scopeUser})
	got, out, errOut := uninstall(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultOK {
		t.Fatalf("uninstall = %q, stdout %q, stderr %q", got, out, errOut)
	}
	assertWarningAfterStatus(t, out, "claude — removed.")
	if left := claudeScopesLeft(t, dir); !slices.Equal(left, []string{scopeProject}) {
		t.Errorf("scopes left = %v, want [project]", left)
	}
	if countCalls(t, dir, removeProject) != 0 {
		t.Error("the project-scope registration must never be removed")
	}
}

// TestClaude_UninstallProjectOnlyWarns verifies that uninstall with only a
// project-scope registration removes nothing and warns.
func TestClaude_UninstallProjectOnlyWarns(t *testing.T) {
	target, dir := fakeClaude(t, []string{scopeProject})
	got, out, errOut := uninstall(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultSkip {
		t.Fatalf("uninstall = %q, stdout %q, stderr %q; want a skip", got, out, errOut)
	}
	assertWarningAfterStatus(t, out, "claude — nothing removed")
	want := []string{"mcp get sync82", removeUser}
	if c := calls(t, dir); !slices.Equal(c, want) {
		t.Errorf("calls = %q, want %q", c, want)
	}
}

// TestClaude_UninstallNotConfigured verifies that uninstall without any registration
// is reported as not configured.
func TestClaude_UninstallNotConfigured(t *testing.T) {
	target, _ := fakeClaude(t, nil)
	if got, out, _ := uninstall(t, target, testEnv("linux", t.TempDir(), nil)); got != ResultSkip || !strings.Contains(out, "claude — not configured, skipping") {
		t.Fatalf("uninstall = %q, stdout %q; want not configured", got, out)
	}
}

// TestClaude_UninstallUnknownScopeFails verifies that uninstall fails when the
// registration's scope cannot be determined.
func TestClaude_UninstallUnknownScopeFails(t *testing.T) {
	target, _ := fakeClaude(t, []string{scopeUser}, "unknown")
	got, out, errOut := uninstall(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultFail || !strings.Contains(errOut, "remove it manually") {
		t.Fatalf("uninstall = %q, stdout %q, stderr %q; want a failure with instructions", got, out, errOut)
	}
}

// TestClaude_UninstallRepeatedScopeFails verifies that uninstall fails when a scope
// is still reported after its removal.
func TestClaude_UninstallRepeatedScopeFails(t *testing.T) {
	target, _ := fakeClaude(t, []string{scopeLocal}, "sticky")
	got, _, errOut := uninstall(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultFail || !strings.Contains(errOut, "still registered in the local scope") {
		t.Fatalf("uninstall = %q, stderr %q; want a repeated-scope failure", got, errOut)
	}
}
