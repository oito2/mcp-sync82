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

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestSearchMemoryTool_ScopedToProjectAndSubprojects verifies the search
// scope: project plus subproject searches only that subproject, a project
// alone searches it and all its subprojects, and no project searches the whole
// vault.
func TestSearchMemoryTool_ScopedToProjectAndSubprojects(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "oito2", "sync82", "memory", "needle in sync82"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "oito2", "perci", "memory", "needle in perci"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle in acme"); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}

	// project + subproject: just that subproject.
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"query": "needle", "project": "oito2", "subproject": "sync82"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "sync82") || strings.Contains(result.Text, "perci") || strings.Contains(result.Text, "acme") {
		t.Fatalf("expected only sync82 result, got: %s", result.Text)
	}

	// project only: that project + all its subprojects, not acme.
	parsed2, err := tool.Validate(mustJSON(t, map[string]string{"query": "needle", "project": "oito2"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result2, err := tool.Execute(ctx, parsed2)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result2.Text, "sync82") || !strings.Contains(result2.Text, "perci") || strings.Contains(result2.Text, "acme") {
		t.Fatalf("expected sync82 and perci but not acme, got: %s", result2.Text)
	}

	// no project: everything.
	parsed3, err := tool.Validate(mustJSON(t, map[string]string{"query": "needle"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result3, err := tool.Execute(ctx, parsed3)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result3.Text, "sync82") || !strings.Contains(result3.Text, "perci") || !strings.Contains(result3.Text, "acme") {
		t.Fatalf("expected all three projects, got: %s", result3.Text)
	}
}

// TestSearchMemoryTool_TrimsProjectAndSubprojectWhitespace verifies that
// leading and trailing whitespace in an explicit project name is trimmed, so
// it resolves as the trimmed name does.
func TestSearchMemoryTool_TrimsProjectAndSubprojectWhitespace(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle here"); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}

	// Whitespace around an explicit project name must resolve the same as
	// the trimmed name, as in Resolver.Resolve, rename_project and
	// init_project_memory.
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"query": "needle", "project": " acme "}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "needle here") {
		t.Fatalf("expected whitespace in project name to be trimmed and the match found, got: %s", result.Text)
	}
}

// TestSearchMemoryTool_NoResults verifies the message returned when nothing
// matches the query.
func TestSearchMemoryTool_NoResults(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"query": "nonexistent-term"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `No results for "nonexistent-term"`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestSearchMemoryTool_SubstringFallbackIsReported verifies that a search
// answered by the substring fallback says so in the text and sets
// substring_fallback in the structured report.
func TestSearchMemoryTool_SubstringFallbackIsReported(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "price 100₽ today"); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	result := runTool(t, tool, map[string]any{"query": "100₽", "project": "acme"})
	if result.IsError || !strings.Contains(result.Text, "acme/memory:1  price 100₽ today") || !strings.Contains(result.Text, "the words were matched inside the text instead") {
		t.Fatalf("result = %q, want the match and the fallback note", result.Text)
	}
	if report, ok := result.Structured.(searchReport); !ok || !report.SubstringFallback || len(report.Results) != 1 {
		t.Fatalf("structured = %+v, want substring_fallback and one result", result.Structured)
	}
}

// TestSearchMemoryTool_PaginationTruncationMessage verifies that, when matches
// exceed the limit, the output contains only limit matches and a message
// saying the limit was reached.
func TestSearchMemoryTool_PaginationTruncationMessage(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle one\nneedle two\nneedle three"); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"query": "needle", "project": "acme", "limit": 2}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "limit of 2 results reached") {
		t.Fatalf("expected a truncation message, got: %s", result.Text)
	}
	if strings.Count(result.Text, "needle") != 2 {
		t.Fatalf("expected exactly 2 match lines, got: %s", result.Text)
	}
}

// TestSearchMemoryTool_LiteralPercentNotWildcard verifies that "%" in the
// query matches literally and is not treated as a wildcard.
func TestSearchMemoryTool_LiteralPercentNotWildcard(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", "100% done\nother line"); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"query": "100%", "project": "acme", "match": "exact"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "100% done") || strings.Contains(result.Text, "other line") {
		t.Fatalf("expected only the literal match, got: %s", result.Text)
	}
}

// TestSearchMemoryTool_DoesNotFallBackToLastUsedProjectOnItsOwn verifies that,
// without a project or workspace_root, the search covers every project instead
// of being scoped to the last-used project from the global config.
func TestSearchMemoryTool_DoesNotFallBackToLastUsedProjectOnItsOwn(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "oito2", "", "memory", "shared-term in oito2"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "shared-term in acme"); err != nil {
		t.Fatal(err)
	}
	// Set "oito2" as the last-used project. Most tools would scope to it
	// through tier 3, but search_memory must not unless workspace_root is
	// also given.
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{LastProject: "oito2"} }); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"query": "shared-term"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "oito2") || !strings.Contains(result.Text, "acme") {
		t.Fatalf("expected results from both projects (search everything, not scoped to last-used), got: %s", result.Text)
	}
}

// TestSearchMemoryTool_ExplicitPathWinsOverWorkspaceRootLocalConfig verifies
// that an explicit "path" argument selects the vault to search even when a
// workspace_root with a .sync82.json is also given and no project is set.
func TestSearchMemoryTool_ExplicitPathWinsOverWorkspaceRootLocalConfig(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	// The term exists only in a vault distinct from the default vault that
	// the local config resolves to.
	customVault := t.TempDir() + "/custom.db"
	cs, err := mgr.Get(ctx, customVault)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cs.EnsureProject(ctx, "oito2", ""); err != nil {
		t.Fatal(err)
	}
	if err := cs.WriteDocument(ctx, "oito2", "", "memory", "only-in-custom-vault"); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{
		"query":          "only-in-custom-vault",
		"workspace_root": workspace,
		"path":           customVault,
	}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "only-in-custom-vault") {
		t.Fatalf("expected a match from the explicit path's vault, got: %s", result.Text)
	}
}

// TestSearchMemoryTool_WorkspaceRootLocalConfigProblems verifies that, with
// workspace_root and no project, an unreadable .sync82.json or one naming an
// invalid project yields an error result, while a workspace without a
// .sync82.json searches the whole vault.
func TestSearchMemoryTool_WorkspaceRootLocalConfigProblems(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle"); err != nil {
		t.Fatal(err)
	}
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	search := func(workspace string) ToolResult {
		t.Helper()
		parsed, err := tool.Validate(mustJSON(t, map[string]string{"query": "needle", "workspace_root": workspace}))
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		result, err := tool.Execute(ctx, parsed)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return result
	}

	corrupt := t.TempDir()
	if err := os.WriteFile(filepath.Join(corrupt, ".sync82.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := search(corrupt); !result.IsError || !strings.Contains(result.Text, "could not be read") {
		t.Errorf("corrupt .sync82.json: result = %+v, want an error result saying it could not be read", result)
	}

	invalid := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalid, ".sync82.json"), []byte(`{"project": "../etc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := search(invalid); !result.IsError || strings.Contains(result.Text, "needle") {
		t.Errorf("invalid project name: result = %+v, want an error result and no search", result)
	}

	if result := search(t.TempDir()); result.IsError || !strings.Contains(result.Text, "needle") {
		t.Errorf("no .sync82.json: result = %+v, want a whole-vault search finding the match", result)
	}
}

// TestSearchMemoryTool_Validate verifies that validation rejects a missing
// query, a limit above 1000 and a negative offset, and accepts limit 0 by
// applying the default.
func TestSearchMemoryTool_Validate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{})); err == nil {
		t.Fatal("expected a validation error for a missing query")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"query": "x", "limit": 0})); err != nil {
		t.Fatalf("expected limit=0 to default rather than error, got: %v", err)
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"query": "x", "limit": 2000})); err == nil {
		t.Fatal("expected a validation error for limit > 1000")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"query": "x", "offset": -1})); err == nil {
		t.Fatal("expected a validation error for negative offset")
	}
}

// TestSearchMemoryTool_ValidateBounds verifies that validation rejects
// context_lines above 20, a multi-line query, and a subproject given without a
// project, and accepts context_lines of exactly 20.
func TestSearchMemoryTool_ValidateBounds(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	for _, args := range []map[string]any{
		{"query": "x", "context_lines": 21}, // context_lines above the maximum
		{"query": "a\nb"},                   // a multi-line query can never match a single line
		{"query": "x", "subproject": "api"}, // subproject without a project
	} {
		if _, err := tool.Validate(mustJSON(t, args)); err == nil {
			t.Errorf("Validate(%v) = nil error, want a rejection", args)
		}
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"query": "x", "context_lines": 20})); err != nil {
		t.Errorf("context_lines=20 rejected: %v", err)
	}
}

// TestSearchMemoryTool_OutputIsBounded verifies that a search with many long
// matches and the maximum context returns a response close to
// maxSearchResponseSize, with a truncation notice, instead of an unbounded
// one.
func TestSearchMemoryTool_OutputIsBounded(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	line := "needle " + strings.Repeat("x", 2000)
	if err := s.WriteDocument(ctx, "acme", "", "memory", strings.TrimSuffix(strings.Repeat(line+"\n", 2000), "\n")); err != nil {
		t.Fatal(err)
	}

	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"query": "needle", "limit": 1000, "context_lines": 20}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Text) > maxSearchResponseSize+4096 {
		t.Fatalf("response is %d bytes, want it bounded near %d", len(result.Text), maxSearchResponseSize)
	}
	if !strings.Contains(result.Text, "output truncated") {
		t.Errorf("response does not say it was truncated")
	}
}

// TestSearchMemoryTool_LabelsEntriesWithTheirDate verifies that a match inside
// a dated log entry is labeled with the project, kind and entry date.
func TestSearchMemoryTool_LabelsEntriesWithTheirDate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- shipped"); err != nil {
		t.Fatal(err)
	}
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"query": "shipped"}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "acme/progress[2026-01-01]:2  - shipped") {
		t.Fatalf("Text = %q, want the entry date in the label", result.Text)
	}
}

// TestSearchMemoryTool_WordsModeByDefault verifies that the default match
// finds every word in any order with accents ignored, and that kinds and
// since narrow the search.
func TestSearchMemoryTool_WordsModeByDefault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "Instalador grava a configuração"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-01-05", "## 2026-01-05\n- Decisão: configuração no XDG"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-03-05", "## 2026-03-05\n- Decisão: configuração por workspace"); err != nil {
		t.Fatal(err)
	}
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}

	text := runTool(t, tool, map[string]any{"project": "acme", "query": "configuracao instal*"}).Text
	if !strings.Contains(text, "acme/memory:1  Instalador grava a configuração") || strings.Contains(text, "decisions") {
		t.Errorf("words search = %q", text)
	}
	text = runTool(t, tool, map[string]any{"project": "acme", "query": "decisao configuracao", "kinds": []string{"Decisions"}, "since": "2026-02-01"}).Text
	if !strings.Contains(text, "workspace") || strings.Contains(text, "XDG") || strings.Contains(text, "memory") {
		t.Errorf("kinds + since search = %q", text)
	}
	text = runTool(t, tool, map[string]any{"project": "acme", "query": "configuração no", "match": "phrase"}).Text
	if !strings.Contains(text, "XDG") || strings.Contains(text, "workspace") {
		t.Errorf("phrase search = %q", text)
	}
}

// TestSearchMemoryTool_ValidateSearchOptions verifies the validation of
// match, kinds, since and until, and of a query with no words.
func TestSearchMemoryTool_ValidateSearchOptions(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	for name, args := range map[string]map[string]any{
		"unknown match":      {"query": "x", "match": "fuzzy"},
		"no words":           {"query": "%%%"},
		"no words in phrase": {"query": "-- --", "match": "phrase"},
		"invalid kind":       {"query": "x", "kinds": []string{"../etc"}},
		"invalid since":      {"query": "x", "since": "2026-13-01"},
		"invalid until":      {"query": "x", "until": "yesterday"},
		"since after until":  {"query": "x", "since": "2026-02-01", "until": "2026-01-01"},
	} {
		if _, err := tool.Validate(mustJSON(t, args)); err == nil {
			t.Errorf("%s: Validate(%v) = nil error, want a rejection", name, args)
		}
	}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"query": "%%%", "match": "exact"}))
	if err != nil {
		t.Fatalf("exact mode should accept punctuation: %v", err)
	}
	if got := parsed.(searchMemoryArgs).Match; got != "exact" {
		t.Errorf("Match = %q", got)
	}
	parsed, err = tool.Validate(mustJSON(t, map[string]any{"query": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.(searchMemoryArgs).Match; got != "words" {
		t.Errorf("default Match = %q, want words", got)
	}
}

// TestSearchMemoryTool_ClipsLongLines verifies that a match on one huge line
// is clipped, so the response stays far below maxSearchResponseSize, in
// both formats.
func TestSearchMemoryTool_ClipsLongLines(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle "+strings.Repeat("ã", 1<<20)); err != nil {
		t.Fatal(err)
	}
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	for _, format := range []string{"text", "json"} {
		text := runTool(t, tool, map[string]any{"project": "acme", "query": "needle", "format": format}).Text
		if len(text) > 2*maxSearchLineBytes {
			t.Errorf("%s response is %d bytes, want the line clipped near %d", format, len(text), maxSearchLineBytes)
		}
		if !strings.Contains(text, "…") || !utf8.ValidString(text) {
			t.Errorf("%s response should end the clipped line with … on a character boundary", format)
		}
	}
}

// TestSearchMemoryTool_SubprojectWithoutProjectIsReported verifies that a
// subproject given with a workspace that has no .sync82.json is an error
// result instead of a search of the whole vault.
func TestSearchMemoryTool_SubprojectWithoutProjectIsReported(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle"); err != nil {
		t.Fatal(err)
	}
	result := runTool(t, &SearchMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"query": "needle", "workspace_root": t.TempDir(), "subproject": "api"})
	if !result.IsError || !strings.Contains(result.Text, `"subproject" was given`) {
		t.Fatalf("result = %+v, want an error result instead of a vault-wide search", result)
	}
}

// TestSearchMemoryTool_RejectsInvalidProjectName verifies that an invalid
// explicit project or subproject name is a validation error instead of a
// misleading "project not found".
func TestSearchMemoryTool_RejectsInvalidProjectName(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	for _, args := range []map[string]any{
		{"query": "x", "project": "bad name!"},
		{"query": "x", "project": "acme", "subproject": "../up"},
	} {
		if _, err := tool.Validate(mustJSON(t, args)); err == nil || !strings.Contains(err.Error(), "must start with a letter or digit") {
			t.Errorf("Validate(%v) = %v, want a name error", args, err)
		}
	}
}

// TestSearchMemoryTool_ResponseCapCountsEncodedCopies fills the results
// with lines of "<", which JSON encodes in six bytes each, and checks that
// the whole response as sent — the text and the structured content, both
// JSON-encoded — stays within about maxSearchResponseSize (a 4096-byte
// allowance for the envelope), in text and JSON
// format, while reporting where to continue.
func TestSearchMemoryTool_ResponseCapCountsEncodedCopies(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	line := "needle " + strings.Repeat("<", 4000)
	if err := s.WriteDocument(ctx, "acme", "", "memory", strings.TrimSuffix(strings.Repeat(line+"\n", 600), "\n")); err != nil {
		t.Fatal(err)
	}
	tool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	for _, format := range []string{"text", "json"} {
		res := runTool(t, tool, map[string]any{"query": "needle", "limit": 1000, "format": format})
		sent, err := json.Marshal(map[string]any{
			"content":           []any{map[string]any{"type": "text", "text": res.Text}},
			"structuredContent": res.Structured,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(sent) > maxSearchResponseSize+4096 {
			t.Errorf("%s format: %d bytes sent, want at most about %d", format, len(sent), maxSearchResponseSize)
		}
		report, ok := res.Structured.(searchReport)
		if !ok || report.NextOffset == 0 || len(report.Results) == 0 {
			t.Errorf("%s format: %d results, next offset %d; want a truncated page with a next offset", format, len(report.Results), report.NextOffset)
		}
	}
}
