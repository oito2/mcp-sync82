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
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/server"
	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
	"github.com/oito2/mcp-sync82/internal/version"
)

// TestPrintUsage_ListsEveryTopLevelSubcommand checks that the usage text
// names every top-level subcommand.
func TestPrintUsage_ListsEveryTopLevelSubcommand(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()

	for _, want := range []string{"install", "uninstall", "config", "self-update", "export", "import", "version", "help"} {
		if !strings.Contains(out, want) {
			t.Errorf("printUsage output missing %q:\n%s", want, out)
		}
	}
}

// TestBuildRegisteredTools_MatchesEveryToolImplementation checks that every
// tools.Tool implementation defined in package tools is wired into
// tools.Registered, and that Registered references no undefined type.
// Without it, an unregistered tool would compile and pass every other test
// while never appearing in ListTools.
//
// This scans internal/tools's non-test source for every "type XTool
// struct" declaration and cross-checks it against every "&XTool{"
// reference in internal/tools/registry.go, at source level rather than via
// reflection, using only the standard library. It assumes every struct
// named "...Tool" in package tools implements tools.Tool; an unrelated
// "...Tool"-named struct would need excluding here.
func TestBuildRegisteredTools_MatchesEveryToolImplementation(t *testing.T) {
	const toolsDir = "../../internal/tools"
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		t.Fatalf("read %s: %v", toolsDir, err)
	}

	structPattern := regexp.MustCompile(`(?m)^type (\w+Tool) struct\b`)
	implemented := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(toolsDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, m := range structPattern.FindAllStringSubmatch(string(data), -1) {
			implemented[m[1]] = true
		}
	}
	if len(implemented) == 0 {
		t.Fatal("found no \"...Tool\" struct definitions in internal/tools — regex or toolsDir is broken, this test would pass vacuously")
	}

	registrySrc, err := os.ReadFile(filepath.Join(toolsDir, "registry.go"))
	if err != nil {
		t.Fatalf("read registry.go: %v", err)
	}
	usagePattern := regexp.MustCompile(`&(\w+Tool)\{`)
	registered := map[string]bool{}
	for _, m := range usagePattern.FindAllStringSubmatch(string(registrySrc), -1) {
		registered[m[1]] = true
	}

	for name := range implemented {
		if !registered[name] {
			t.Errorf("tools.%s is defined in internal/tools but never registered in tools.Registered (registry.go) — it would never appear in ListTools", name)
		}
	}
	for name := range registered {
		if !implemented[name] {
			t.Errorf("tools.Registered references %s, but no such type is defined in internal/tools", name)
		}
	}
}

// TestRun_DispatchRejectsBadArgumentsWithoutSideEffects checks that run
// routes install, self-update, export and import to their handlers: each
// rejects an invalid argument with its own message (exit code 1 for an
// unknown install target, 2 for the usage errors), without contacting the
// network or creating the default vault in the isolated home.
func TestRun_DispatchRejectsBadArgumentsWithoutSideEffects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cases := []struct {
		args      []string
		code      int
		stderrHas string
	}{
		{[]string{"install", "nonexistent-client"}, 1, `Unknown target: "nonexistent-client"`},
		{[]string{"self-update", "--bogus"}, usageExitCode, `unknown self-update option "--bogus"`},
		{[]string{"export"}, usageExitCode, "usage: sync82 export"},
		{[]string{"import"}, usageExitCode, "usage: sync82 import"},
		{[]string{"install", "--force"}, usageExitCode, `unknown install flag "--force"`},
		{[]string{"config", "get-vault", "extra"}, usageExitCode, "config get-vault takes no arguments"},
		{[]string{"--bogus"}, usageExitCode, `unknown flag "--bogus"`},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), c.args, strings.NewReader(""), &stdout, &stderr); code != c.code {
			t.Errorf("run(%q) = %d, want %d; stderr=%s", c.args, code, c.code, stderr.String())
		}
		if c.code == usageExitCode && !strings.Contains(stderr.String(), "Run 'sync82 --help' for usage.") {
			t.Errorf("run(%q) stderr = %q, want the --help pointer of a usage error", c.args, stderr.String())
		}
		if !strings.Contains(stderr.String(), c.stderrHas) {
			t.Errorf("run(%q) stderr = %q, want it to contain %q", c.args, stderr.String(), c.stderrHas)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".sync82", "knowledge.db")); !os.IsNotExist(err) {
		t.Errorf("default vault exists after rejected commands (stat err = %v)", err)
	}
}

// TestRun_ImportThenExportUseDefaultVault checks that run wires import and
// export to the default vault under the home directory: a file imported
// into a project is written back out by export.
func TestRun_ImportThenExportUseDefaultVault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	inputDir, outputDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "memory.md"), []byte("round trip"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"import", "acme", inputDir}, {"export", "acme", outputDir}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("run(%q) = %d, want 0; stderr=%s", args, code, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".sync82", "knowledge.db")); err != nil {
		t.Errorf("default vault not created under the isolated home: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(outputDir, "memory.md"))
	if err != nil {
		t.Fatalf("read exported memory.md: %v", err)
	}
	if strings.TrimSpace(string(got)) != "round trip" {
		t.Errorf("exported memory.md = %q, want the imported content %q", got, "round trip")
	}
}

// TestRun_Dispatch checks the exit code and output stream of run for help,
// version, config and unknown subcommands.
func TestRun_Dispatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	cases := []struct {
		args       []string
		code       int
		stdoutHas  string
		stderrHas  string
		stdoutNone bool
	}{
		{args: []string{"--help"}, code: 0, stdoutHas: "Usage: sync82"},
		{args: []string{"help"}, code: 0, stdoutHas: "Usage: sync82"},
		{args: []string{"--version"}, code: 0, stdoutHas: version.Get()},
		{args: []string{"version"}, code: 0, stdoutHas: version.Get()},
		{args: []string{"config", "get-vault"}, code: 0, stdoutHas: "No global vault configured"},
		{args: []string{"instal"}, code: 1, stderrHas: `unknown subcommand "instal"`, stdoutNone: true},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), c.args, strings.NewReader(""), &stdout, &stderr)
		if code != c.code {
			t.Errorf("run(%q) = %d, want %d; stderr=%s", c.args, code, c.code, stderr.String())
		}
		if c.stdoutHas != "" && !strings.Contains(stdout.String(), c.stdoutHas) {
			t.Errorf("run(%q) stdout = %q, want it to contain %q", c.args, stdout.String(), c.stdoutHas)
		}
		if c.stderrHas != "" && !strings.Contains(stderr.String(), c.stderrHas) {
			t.Errorf("run(%q) stderr = %q, want it to contain %q", c.args, stderr.String(), c.stderrHas)
		}
		if c.stdoutNone && stdout.Len() != 0 {
			t.Errorf("run(%q) wrote %q to stdout, want nothing", c.args, stdout.String())
		}
	}
}

// connectRegisteredTools starts the server with the real
// tools.Registered list over in-memory transports, backed by a vault in a
// temporary directory and an isolated HOME. It returns a connected client
// session and the number of registered tools; connection failures fail the
// test. Server construction panics if any tool's input schema is invalid,
// so using it also detects a schema typo that would crash startup.
func connectRegisteredTools(t *testing.T) (*mcp.ClientSession, int) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	registered := tools.Registered(tools.NewResolver(filepath.Join(t.TempDir(), "vault.db"), slog.New(slog.DiscardHandler)), mgr)
	s := server.New("sync82-test", "v0", slog.New(slog.DiscardHandler), registered)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, len(registered)
}

// TestBuildRegisteredTools_ServesEveryToolWithValidSchemas registers the
// real tool list and its hand-written schemas with the SDK, so an invalid
// schema (which makes registration panic) or two tools sharing a name
// fails here rather than only when a client starts the server.
func TestBuildRegisteredTools_ServesEveryToolWithValidSchemas(t *testing.T) {
	cs, registered := connectRegisteredTools(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != registered {
		t.Fatalf("server lists %d tools, %d were registered (duplicate names?)", len(res.Tools), registered)
	}
	for _, tool := range res.Tools {
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
	}
}

// callText calls the tool name with args and returns the concatenated text
// of its result. It fails the test on a protocol error or when the result's
// isError flag differs from wantError.
func callText(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, wantError bool) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	if res.IsError != wantError {
		t.Fatalf("%s: isError = %v, want %v; text = %q", name, res.IsError, wantError, text.String())
	}
	return text.String()
}

// TestRegisteredTools_MemoryRoundTrip drives the real tools through an MCP
// client: create a project, write and append memory, read and search it
// back, then delete a custom file.
func TestRegisteredTools_MemoryRoundTrip(t *testing.T) {
	cs, _ := connectRegisteredTools(t)

	callText(t, cs, "create_project", map[string]any{"project": "acme"}, false)
	callText(t, cs, "write_memory", map[string]any{"project": "acme", "filename": "memory", "content": "Acme builds rockets."}, false)
	callText(t, cs, "append_memory", map[string]any{"project": "acme", "filename": "progress", "content": "## 2026-01-01\n- launched"}, false)
	callText(t, cs, "write_memory", map[string]any{"project": "acme", "filename": "notes", "content": "scratch"}, false)

	if text := callText(t, cs, "read_memory", map[string]any{"project": "acme", "filename": "memory"}, false); !strings.Contains(text, "Acme builds rockets.") {
		t.Errorf("read_memory = %q", text)
	}
	if text := callText(t, cs, "search_memory", map[string]any{"query": "launched"}, false); !strings.Contains(text, "acme") {
		t.Errorf("search_memory = %q, want a match in acme", text)
	}
	if text := callText(t, cs, "load_project_context", map[string]any{"project": "acme"}, false); !strings.Contains(text, "launched") {
		t.Errorf("load_project_context = %q", text)
	}

	callText(t, cs, "delete_memory", map[string]any{"project": "acme", "filename": "notes"}, true) // no confirm
	callText(t, cs, "delete_memory", map[string]any{"project": "acme", "filename": "notes", "confirm": true}, false)
	callText(t, cs, "read_memory", map[string]any{"project": "acme", "filename": "notes"}, true)

	// An execution error reaches the client as a tool error result.
	callText(t, cs, "write_memory", map[string]any{"project": "ghost", "filename": "memory", "content": "x"}, true)
}

// TestParseArgs_StrictFlags checks that "--path=X" is parsed as a flag
// rather than a positional argument, that unknown flags like "--force" are
// rejected rather than taken as project names, that repeated flags and a
// missing or empty --path value are errors.
func TestParseArgs_StrictFlags(t *testing.T) {
	p, err := parseArgs([]string{"acme", "--path=/v.db", "out", "--all"}, []string{"all"}, []string{"path"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if p.values["path"] != "/v.db" || !p.flags["all"] || !slices.Equal(p.positional, []string{"acme", "out"}) {
		t.Fatalf("parsed = %+v", p)
	}
	for _, args := range [][]string{
		{"--force", "dir"},
		{"acme", "dir", "--path"},
		{"acme", "dir", "--path", ""},
		{"acme", "dir", "--path", "a", "--path", "b"},
		{"-x"},
	} {
		if _, err := parseArgs(args, []string{"all"}, []string{"path"}); err == nil {
			t.Errorf("parseArgs(%q) = nil error, want a rejection", args)
		}
	}
}

// TestRun_RejectsSurplusArguments checks that subcommands taking no
// arguments, and "install" with more than one target, are usage errors
// (exit code 2).
func TestRun_RejectsSurplusArguments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	for _, args := range [][]string{{"version", "x"}, {"serve", "--foo"}, {"help", "me"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); code != usageExitCode {
			t.Errorf("run(%q) = %d, want %d", args, code, usageExitCode)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := RunInstall(context.Background(), []string{"claude", "extra"}, strings.NewReader(""), &stdout, &stderr, t.TempDir(), "/opt/sync82", nil); code != usageExitCode {
		t.Errorf("install with two targets = %d, want %d", code, usageExitCode)
	}
}

// TestRun_UninstallRejectsUnknownTarget checks that "uninstall" is
// dispatched by run and rejects a target outside installer.Targets with
// exit code 1, listing the available targets.
func TestRun_UninstallRejectsUnknownTarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"uninstall", "nonexistent-client"}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("run(uninstall nonexistent-client) = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Unknown target") || !strings.Contains(stderr.String(), "claude-desktop") {
		t.Errorf("stderr = %q, want the unknown target and the available targets", stderr.String())
	}
}
