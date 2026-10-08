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
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Claude Code scopes, as accepted by "claude mcp remove --scope".
const (
	scopeLocal   = "local"
	scopeProject = "project"
	scopeUser    = "user"
)

// claudeScope reads the scope from "claude mcp get sync82" output. The
// command reports only the registration that takes precedence (local over
// project over user) in a line such as
// "  Scope: User config (available in all your projects)", and ends with a
// hint such as "To remove this server, run: claude mcp remove sync82 -s user",
// whose -s/--scope/--scope= token is used when no Scope line names a
// known scope. It returns "" when the output names none.
func claudeScope(out []byte) string {
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < len("scope:") || !strings.EqualFold(line[:len("scope:")], "scope:") {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(line[len("scope:"):]))
		for _, s := range []string{scopeLocal, scopeProject, scopeUser} {
			if strings.HasPrefix(value, s) {
				return s
			}
		}
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		for i, f := range fields {
			var value string
			switch {
			case (f == "-s" || f == "--scope") && i+1 < len(fields):
				value = fields[i+1]
			case strings.HasPrefix(f, "--scope="):
				value = strings.TrimPrefix(f, "--scope=")
			default:
				continue
			}
			value = strings.ToLower(strings.Trim(value, "`'\".,"))
			if value == scopeLocal || value == scopeProject || value == scopeUser {
				return value
			}
		}
	}
	return ""
}

// runCombined runs command with args under InstallTimeout and returns its
// combined stdout and stderr.
func runCombined(ctx context.Context, command string, args []string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()
	return exec.CommandContext(cctx, command, args...).CombinedOutput()
}

// runCLI runs command with args, turning a failure into an error that
// carries the command's output.
func runCLI(ctx context.Context, command string, args []string) error {
	if out, err := runCombined(ctx, command, args); err != nil {
		return cliError(command, args, out, err)
	}
	return nil
}

// cliError describes a failed command by its first two arguments and its
// trimmed output, or the run error when there was no output.
func cliError(command string, args []string, out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("%s %s failed: %s", command, strings.Join(args[:min(2, len(args))], " "), msg)
}

// notRegisteredOutput reports whether out, the output of a failed "mcp get"
// or "mcp remove", says that no server of that name is registered ("No MCP
// server named ..."), the answer of both the claude and the codex CLI.
func notRegisteredOutput(out []byte) bool {
	return strings.Contains(string(out), "No MCP server named")
}

// removeCLIRegistration removes every sync82 registration a CLI target
// reports, so that a following MCP-add never collides with an existing
// one. removed reports whether anything was removed; warnings describe
// registrations deliberately left in place. A target without GetArgs or
// RemoveArgs has nothing to remove.
//
// A target without Scope is queried once with GetArgs and, when sync82 is
// registered, cleaned with RemoveArgs(""); a failed query counts as "not
// registered" only when its output says so (notRegisteredOutput), and is
// an error otherwise. A target with Scope is queried
// again after each removal, since its GetArgs reports only the
// registration that takes precedence: the reported scope is removed until
// GetArgs fails. A scope that cannot be determined, or that is reported
// again after its removal, is an error. A project-scope registration is
// never removed: it adds a warning, and the user scope (hidden behind it)
// is then removed directly, treating "No MCP server named" output as
// nothing to remove.
func removeCLIRegistration(ctx context.Context, t Target) (removed bool, warnings []string, err error) {
	if len(t.GetArgs) == 0 || t.RemoveArgs == nil {
		return false, nil, nil
	}
	if t.Scope == nil {
		if out, err := runCombined(ctx, t.Command, t.GetArgs); err != nil {
			if notRegisteredOutput(out) {
				return false, nil, nil
			}
			return false, nil, cliError(t.Command, t.GetArgs, out, err)
		}
		if err := runCLI(ctx, t.Command, t.RemoveArgs("")); err != nil {
			return false, nil, err
		}
		return true, nil, nil
	}
	seen := map[string]bool{}
	for {
		out, err := runCombined(ctx, t.Command, t.GetArgs)
		if err != nil {
			return removed, warnings, nil
		}
		scope := t.Scope(out)
		switch {
		case scope == "":
			return removed, warnings, fmt.Errorf("could not determine the scope of the %s registration from `%s %s`; "+
				"remove it manually with `%s mcp remove --scope <scope> %s`",
				serverName, t.Command, strings.Join(t.GetArgs, " "), t.Command, serverName)
		case scope == scopeProject:
			warnings = append(warnings, projectScopeWarning(t.Command))
			args := t.RemoveArgs(scopeUser)
			out, err := runCombined(ctx, t.Command, args)
			if err == nil {
				return true, warnings, nil
			}
			if notRegisteredOutput(out) {
				return removed, warnings, nil
			}
			return removed, warnings, cliError(t.Command, args, out, err)
		case seen[scope]:
			return removed, warnings, fmt.Errorf("%s is still registered in the %s scope after `%s %s`",
				serverName, scope, t.Command, strings.Join(t.RemoveArgs(scope), " "))
		}
		seen[scope] = true
		if err := runCLI(ctx, t.Command, t.RemoveArgs(scope)); err != nil {
			return removed, warnings, err
		}
		removed = true
	}
}

// projectScopeWarning describes a project-scope registration left
// unchanged: it belongs to the current directory's project, takes
// precedence there over the user-scope one, and is removed with the
// command given.
func projectScopeWarning(command string) string {
	dir, err := os.Getwd()
	if err != nil {
		dir = "the current directory"
	}
	return fmt.Sprintf("%s is also registered in project scope (.mcp.json, shared with the project) for %s; "+
		"it was left unchanged and, inside that project, it takes precedence over any user-scope registration. "+
		"To remove it, run from that directory: %s mcp remove --scope project %s", serverName, dir, command, serverName)
}

// printWarnings prints each warning on its own line.
func printWarnings(stdout io.Writer, warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(stdout, "  Warning: %s\n", w)
	}
}
