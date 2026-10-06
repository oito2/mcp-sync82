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
	"strings"
	"testing"
)

// TestLoadProjectContextTool_ConcatenatesNonBlankFiles verifies that the
// context starts with a project header, includes one section per non-blank
// kind, and skips kinds with blank content.
func TestLoadProjectContextTool_ConcatenatesNonBlankFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "# Memory content"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "architecture", "  "); err != nil { // whitespace-only content
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- did X"); err != nil {
		t.Fatal(err)
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.HasPrefix(result.Text, "# Context: acme\n\n") {
		t.Fatalf("expected the context header, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "## memory") || !strings.Contains(result.Text, "# Memory content") {
		t.Fatalf("expected memory section, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "## progress") || !strings.Contains(result.Text, "did X") {
		t.Fatalf("expected progress section, got: %s", result.Text)
	}
	if strings.Contains(result.Text, "## architecture") {
		t.Fatalf("expected the blank architecture file to be skipped, got: %s", result.Text)
	}
}

// TestLoadProjectContextTool_NoContentYet verifies the placeholder returned
// for a project without content.
func TestLoadProjectContextTool_NoContentYet(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "# Context: acme\n\n(no content yet)"; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestLoadProjectContextTool_FilesFilter verifies that the "files" argument
// restricts the context to the listed kinds.
func TestLoadProjectContextTool_FilesFilter(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "memory content"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", "stack content"); err != nil {
		t.Fatal(err)
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "files": []string{"memory"}}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "memory content") {
		t.Fatalf("expected memory content, got: %s", result.Text)
	}
	if strings.Contains(result.Text, "stack content") {
		t.Fatalf("expected stack to be filtered out, got: %s", result.Text)
	}
}

// TestLoadProjectContextTool_SinceFiltersDatedEntriesButKeepsUndated verifies
// that "since" drops dated log entries before the cutoff while keeping later
// and undated entries, and does not affect overwrite-style documents.
func TestLoadProjectContextTool_SinceFiltersDatedEntriesButKeepsUndated(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- old entry"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-06-01", "## 2026-06-01\n- recent entry"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "", "undated entry"); err != nil {
		t.Fatal(err)
	}
	// An overwrite-style document holds current state, not history, so
	// since and max_entries must not affect it.
	if err := s.WriteDocument(ctx, "acme", "", "memory", "current memory state"); err != nil {
		t.Fatal(err)
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "since": "2026-03-01"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(result.Text, "old entry") {
		t.Fatalf("expected the pre-cutoff dated entry to be excluded, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "recent entry") {
		t.Fatalf("expected the post-cutoff dated entry to be included, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "undated entry") {
		t.Fatalf("expected the undated entry to always be included, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "current memory state") {
		t.Fatalf("expected the overwrite-style file to be unaffected by since, got: %s", result.Text)
	}
}

// TestLoadProjectContextTool_MaxEntriesKeepsOnlyTheMostRecent verifies that
// "max_entries" keeps only the given number of most recent log entries.
func TestLoadProjectContextTool_MaxEntriesKeepsOnlyTheMostRecent(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-01-01", "2026-02-01", "2026-03-01"} {
		if err := s.AppendEntry(ctx, "acme", "", "progress", date, "## "+date+"\n- entry "+date); err != nil {
			t.Fatal(err)
		}
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "max_entries": 1}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "entry 2026-03-01") {
		t.Fatalf("expected only the most recent entry to be included, got: %s", result.Text)
	}
	if strings.Contains(result.Text, "entry 2026-01-01") || strings.Contains(result.Text, "entry 2026-02-01") {
		t.Fatalf("expected older entries to be excluded, got: %s", result.Text)
	}
}

// TestLoadProjectContextTool_Validate_RejectsBadSinceAndNegativeMaxEntries
// verifies that validation rejects a malformed "since" date and a negative
// "max_entries".
func TestLoadProjectContextTool_Validate_RejectsBadSinceAndNegativeMaxEntries(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "since": "not-a-date"})); err == nil {
		t.Fatal("expected an error for a malformed \"since\" date")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "max_entries": -1})); err == nil {
		t.Fatal("expected an error for a negative \"max_entries\"")
	}
}

// TestLoadProjectContextTool_MaxEntriesCountsOnlyDatedEntries verifies that a
// "# Progress" title before the first dated entry is kept first when the log
// is stored and does not count as an entry, so max_entries=1 returns the
// newest dated entry.
func TestLoadProjectContextTool_MaxEntriesCountsOnlyDatedEntries(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	content := "# Progress\n\n## 2026-01-01\n- a\n\n## 2026-02-01\n- b\n\n## 2026-03-01\n- c"
	if err := writeMemoryCore(ctx, s, "acme", "", "progress", content); err != nil {
		t.Fatal(err)
	}

	full, ok, err := s.ReadContent(ctx, "acme", "", "progress")
	if err != nil || !ok {
		t.Fatalf("ReadContent: ok=%v err=%v", ok, err)
	}
	if full != "# Progress\n\n## 2026-01-01\n- a\n\n## 2026-02-01\n- b\n\n## 2026-03-01\n- c" {
		t.Fatalf("ReadContent = %q, want the title kept first", full)
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "files": []string{"progress"}, "max_entries": 1}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "## 2026-03-01") {
		t.Fatalf("Text = %q, want the newest dated entry", result.Text)
	}
	if strings.Contains(result.Text, "## 2026-02-01") {
		t.Fatalf("Text = %q, want only one dated entry", result.Text)
	}
}

// TestLoadProjectContextTool_BoundedWithCurrentStateFirst verifies that
// current-state documents come before long logs, that the output is cut near
// max_bytes with a truncation notice so a long log cannot fill the agent
// context, that max_bytes below the minimum is rejected, and that max_bytes
// defaults to defaultContextBytes.
func TestLoadProjectContextTool_BoundedWithCurrentStateFirst(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", "Go"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-01-01", "## 2026-01-01\n- "+strings.Repeat("d", 1000)); err != nil {
			t.Fatal(err)
		}
	}

	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "max_bytes": 10000}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Text) > 10000+300 {
		t.Fatalf("response is %d bytes, want about 10000", len(result.Text))
	}
	if !strings.Contains(result.Text, "## stack\n\nGo") {
		t.Errorf("stack (current state) should come before the long decisions log")
	}
	if !strings.Contains(result.Text, "context truncated") {
		t.Errorf("response doesn't say it was truncated")
	}

	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "max_bytes": 10})); err == nil {
		t.Error("expected max_bytes below the minimum to be rejected")
	}
	parsed, err = tool.Validate(mustJSON(t, map[string]any{"project": "acme"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.(loadProjectContextArgs).MaxBytes; got != defaultContextBytes {
		t.Errorf("default max_bytes = %d, want %d", got, defaultContextBytes)
	}
}
