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
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// seedQueryVault isolates HOME (and SYNC82_DB_PATH) and imports a project
// "acme" with memory and one progress entry into the default vault, through
// the import command. It returns the vault path.
func seedQueryVault(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.DBPathEnvVar, "")
	in := t.TempDir()
	for name, content := range map[string]string{
		"memory.md":   "# Memory\n\ninstaller notes",
		"progress.md": "## 2026-10-01\n- installer fixed",
	} {
		if err := os.WriteFile(filepath.Join(in, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, stderr := runCLI(t, "import", "acme", in); code != 0 {
		t.Fatalf("import: %d %s", code, stderr)
	}
	return config.DefaultVaultPath()
}

// runCLI runs the command line args and returns its exit code, stdout and
// stderr.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// toolText runs tool with args directly, as an MCP call would, and returns
// its text.
func toolText(t *testing.T, tool tools.Tool, args map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(args)
	parsed, err := tool.Validate(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimRight(res.Text, "\n") + "\n"
}

// TestQueryCommands_MatchTheTools checks that search, context and health
// print exactly what the matching tool returns for the same arguments.
func TestQueryCommands_MatchTheTools(t *testing.T) {
	vault := seedQueryVault(t)
	stores := store.NewManager()
	t.Cleanup(func() { stores.Close() })
	resolver := tools.NewResolver(vault, slog.New(slog.DiscardHandler))
	resolver.SkipRemember = true

	cases := []struct {
		cli  []string
		tool tools.Tool
		args map[string]any
	}{
		{[]string{"search", "installer", "--context-lines", "1"}, &tools.SearchMemoryTool{Resolver: resolver, Stores: stores}, map[string]any{"query": "installer", "context_lines": 1}},
		{[]string{"search", "installer", "--project", "acme", "--kinds", "progress, memory", "--json"}, &tools.SearchMemoryTool{Resolver: resolver, Stores: stores}, map[string]any{"query": "installer", "project": "acme", "kinds": []string{"progress", "memory"}, "format": "json"}},
		{[]string{"context", "acme", "--full"}, &tools.LoadProjectContextTool{Resolver: resolver, Stores: stores}, map[string]any{"project": "acme", "mode": "full"}},
		{[]string{"context", "acme", "--files", "memory", "--max-bytes", "4096"}, &tools.LoadProjectContextTool{Resolver: resolver, Stores: stores}, map[string]any{"project": "acme", "files": []string{"memory"}, "max_bytes": 4096}},
		{[]string{"health", "acme", "--json"}, &tools.CheckProjectHealthTool{Resolver: resolver, Stores: stores}, map[string]any{"project": "acme", "format": "json"}},
		{[]string{"health", "--all"}, &tools.CheckProjectHealthTool{Resolver: resolver, Stores: stores}, map[string]any{"all_projects": true}},
	}
	for _, c := range cases {
		_, stdout, stderr := runCLI(t, c.cli...)
		if want := toolText(t, c.tool, c.args); stdout != want {
			t.Errorf("%q printed:\n%s\nwant the tool's output:\n%s\nstderr: %s", c.cli, stdout, want, stderr)
		}
	}
}

// TestQueryCommands_ExitCodes checks the exit codes and streams: no match
// and a healthy-or-warned project succeed, an unhealthy project or a
// missing project or vault fail with 1 (the vault is not created), and bad
// arguments are usage errors.
func TestQueryCommands_ExitCodes(t *testing.T) {
	seedQueryVault(t)
	missingVault := filepath.Join(t.TempDir(), "none.db")
	cases := []struct {
		args      []string
		code      int
		stdoutHas string
		stderrHas string
	}{
		{[]string{"search", "zzz"}, 0, `No results for "zzz"`, ""},
		{[]string{"search", "zzz", "--json"}, 0, `"results": []`, ""},
		{[]string{"health", "acme"}, 1, "Status: UNHEALTHY", ""},
		{[]string{"context", "ghost"}, 1, "", `project not found: "ghost"`},
		{[]string{"search", "x", "--path", missingVault}, 1, "", "No vault exists at " + missingVault},
		{[]string{"health", "--all", "--path", missingVault}, 1, "", "No vault exists"},
		{[]string{"search"}, usageExitCode, "", "search takes one query"},
		{[]string{"search", "a", "b"}, usageExitCode, "", "search takes one query"},
		{[]string{"search", "x", "--limit", "many"}, usageExitCode, "", `--limit must be an integer, got "many"`},
		{[]string{"search", "x", "--match", "fuzzy"}, usageExitCode, "", "--match must be"},
		{[]string{"search", "x", "--bogus"}, usageExitCode, "", `unknown option "--bogus"`},
		{[]string{"context"}, usageExitCode, "", "context takes a project"},
		{[]string{"context", "acme", "--since", "yesterday"}, usageExitCode, "", "since"},
		{[]string{"health"}, usageExitCode, "", "or --all"},
		{[]string{"health", "acme", "--all"}, usageExitCode, "", "not both"},
	}
	for _, c := range cases {
		code, stdout, stderr := runCLI(t, c.args...)
		if code != c.code || !strings.Contains(stdout, c.stdoutHas) || !strings.Contains(stderr, c.stderrHas) {
			t.Errorf("%q = %d\nstdout: %s\nstderr: %s\nwant %d, stdout with %q, stderr with %q", c.args, code, stdout, stderr, c.code, c.stdoutHas, c.stderrHas)
		}
	}
	if _, err := os.Stat(missingVault); !os.IsNotExist(err) {
		t.Errorf("a read-only command created the vault (stat err = %v)", err)
	}
}

// TestQueryCommands_NeverRememberTheProject checks that the read-only
// commands leave the last used project unrecorded.
func TestQueryCommands_NeverRememberTheProject(t *testing.T) {
	seedQueryVault(t)
	if err := config.UpdateGlobalConfig(func(cfg *config.GlobalConfig) {
		cfg.LastProject, cfg.LastSubproject, cfg.LastVaultPath = "", "", ""
	}); err != nil {
		t.Fatal(err)
	}
	before, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"search", "installer", "--project", "acme"}, {"context", "acme"}, {"health", "acme"}} {
		runCLI(t, args...)
	}
	after, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if after.LastProject != before.LastProject || after.LastVaultPath != before.LastVaultPath {
		t.Errorf("last project changed from %q to %q", before.LastProject, after.LastProject)
	}
}

// TestRunSearch_EndOfOptions checks that "--" ends the options, so a query
// starting with "-" can be searched for, and that "--help" after it is a
// query, not a request for help.
func TestRunSearch_EndOfOptions(t *testing.T) {
	seedQueryVault(t)
	for _, args := range [][]string{{"search", "--", "--dry-run"}, {"search", "--project", "acme", "--", "-1"}, {"search", "--", "--help"}} {
		code, stdout, stderr := runCLI(t, args...)
		if code != 0 || strings.Contains(stdout, "Usage:") {
			t.Errorf("%q = %d\nstdout: %s\nstderr: %s\nwant a search", args, code, stdout, stderr)
		}
	}
	p, err := parseArgs([]string{"--path", "v.db", "--", "--path", "x"}, nil, []string{"path"})
	if err != nil || p.values["path"] != "v.db" || len(p.positional) != 2 || p.positional[0] != "--path" {
		t.Errorf("parseArgs = %+v, %v; want --path v.db and two positionals after --", p, err)
	}
}

// TestQueryCommands_SpeakCLI checks that usage errors point at the
// subcommand's own --help and name CLI flags, not tool arguments, and that
// a missing project isn't told to use a tool.
func TestQueryCommands_SpeakCLI(t *testing.T) {
	seedQueryVault(t)
	_, _, stderr := runCLI(t, "context", "acme", "--max-bytes", "200")
	if !strings.Contains(stderr, "--max-bytes must be between") || !strings.Contains(stderr, "Run 'sync82 context --help' for usage.") {
		t.Errorf("stderr = %q, want the flag named and the context help pointer", stderr)
	}
	_, _, stderr = runCLI(t, "search", "x", "--subproject", "s")
	if strings.Contains(stderr, "workspace_root") || !strings.Contains(stderr, "--subproject") {
		t.Errorf("stderr = %q, want CLI flags only", stderr)
	}
	_, _, stderr = runCLI(t, "context", "ghost")
	if strings.Contains(stderr, "create_project") {
		t.Errorf("stderr = %q, want no tool named", stderr)
	}
}
