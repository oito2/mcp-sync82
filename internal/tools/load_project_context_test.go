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
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
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
// defaults to summaryContextBytes in summary mode and fullContextBytes in
// full mode.
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
	if got := parsed.(loadProjectContextArgs).MaxBytes; got != summaryContextBytes {
		t.Errorf("default max_bytes = %d, want %d", got, summaryContextBytes)
	}
	parsed, err = tool.Validate(mustJSON(t, map[string]any{"project": "acme", "mode": "full"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.(loadProjectContextArgs).MaxBytes; got != fullContextBytes {
		t.Errorf("full mode max_bytes = %d, want %d", got, fullContextBytes)
	}
}

// seedProgress creates project acme in a fresh test vault with a memory
// document and n dated progress entries, one per day from 2026-01-01, each
// body holding "entry N.". It returns a LoadProjectContextTool bound to that
// vault and a background context, and fails the test if any write fails.
func seedProgress(t *testing.T, n int) (*LoadProjectContextTool, context.Context) {
	t.Helper()
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "current memory state"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		date := fmt.Sprintf("2026-01-%02d", i)
		if err := s.AppendEntry(ctx, "acme", "", "progress", date, fmt.Sprintf("## %s\n- entry %d.", date, i)); err != nil {
			t.Fatal(err)
		}
	}
	return &LoadProjectContextTool{Resolver: r, Stores: mgr}, ctx
}

// runLoadContext validates args with tool and executes it, failing the test
// on any error, and returns the response text.
func runLoadContext(t *testing.T, tool *LoadProjectContextTool, ctx context.Context, args map[string]any) string {
	t.Helper()
	parsed, err := tool.Validate(mustJSON(t, args))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return result.Text
}

// TestLoadProjectContextTool_SummaryModeIsDefault verifies that, with no
// mode, only the summaryMaxEntries most recent dated entries are loaded in
// ascending order, current-state files load in full, and a footer reports
// how many entries were shown out of how many.
func TestLoadProjectContextTool_SummaryModeIsDefault(t *testing.T) {
	tool, ctx := seedProgress(t, 15)
	text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme"})

	if !strings.Contains(text, "current memory state") {
		t.Errorf("memory should load in full, got: %s", text)
	}
	for i := 1; i <= 5; i++ {
		if strings.Contains(text, fmt.Sprintf("entry %d.", i)) {
			t.Errorf("entry %d is older than the 10 most recent and should be left out, got: %s", i, text)
		}
	}
	if !strings.Contains(text, "entry 6.") || !strings.Contains(text, "entry 15.") {
		t.Errorf("expected entries 6 to 15, got: %s", text)
	}
	if strings.Index(text, "entry 6.") > strings.Index(text, "entry 15.") {
		t.Errorf("entries should be in ascending order, got: %s", text)
	}
	want := "[older history omitted — progress: 10 of 15 dated entries shown (oldest shown 2026-01-06);"
	if !strings.Contains(text, want) {
		t.Errorf("expected footer %q, got: %s", want, text)
	}
	if !strings.HasSuffix(text, `set "mode" to "full"]`) {
		t.Errorf("the footer should end the response, got: %s", text)
	}
}

// TestLoadProjectContextTool_SummaryWithoutOmissionHasNoFooter verifies
// that summary mode adds no footer when every dated entry fits.
func TestLoadProjectContextTool_SummaryWithoutOmissionHasNoFooter(t *testing.T) {
	tool, ctx := seedProgress(t, 3)
	text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme"})
	if strings.Contains(text, "older history omitted") {
		t.Errorf("no entries were left out, so there should be no footer, got: %s", text)
	}
}

// TestLoadProjectContextTool_FullModeLoadsEverything verifies that full
// mode loads every entry without a footer, as a header followed by one
// section per kind separated by "---".
func TestLoadProjectContextTool_FullModeLoadsEverything(t *testing.T) {
	tool, ctx := seedProgress(t, 3)
	text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme", "mode": "full"})
	want := "# Context: acme\n\n## memory\n\ncurrent memory state\n\n---\n\n## progress\n\n" +
		"## 2026-01-01\n- entry 1.\n\n## 2026-01-02\n- entry 2.\n\n## 2026-01-03\n- entry 3."
	if text != want {
		t.Errorf("full mode output = %q, want %q", text, want)
	}
}

// TestLoadProjectContextTool_ExplicitFiltersOverrideSummaryDefaults
// verifies that an explicit max_entries replaces the summary default, that
// an explicit since lifts the default entry limit, and that the footer also
// reports entries left out by them.
func TestLoadProjectContextTool_ExplicitFiltersOverrideSummaryDefaults(t *testing.T) {
	tool, ctx := seedProgress(t, 15)

	text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme", "max_entries": 2})
	if strings.Contains(text, "entry 13.") || !strings.Contains(text, "entry 14.") || !strings.Contains(text, "entry 15.") {
		t.Errorf("max_entries 2 should keep only entries 14 and 15, got: %s", text)
	}
	if !strings.Contains(text, "progress: 2 of 15 dated entries shown (oldest shown 2026-01-14)") {
		t.Errorf("footer should report 2 of 15, got: %s", text)
	}

	text = runLoadContext(t, tool, ctx, map[string]any{"project": "acme", "since": "2026-01-03"})
	if strings.Contains(text, "entry 2.") || !strings.Contains(text, "entry 3.") || !strings.Contains(text, "entry 15.") {
		t.Errorf("since should keep entries 3 to 15 with no 10-entry limit, got: %s", text)
	}
	if !strings.Contains(text, "progress: 13 of 15 dated entries shown") {
		t.Errorf("footer should report 13 of 15, got: %s", text)
	}

	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "since": "2026-01-03"}))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.(loadProjectContextArgs).MaxEntries; got != 0 {
		t.Errorf("since in summary mode: MaxEntries = %d, want 0", got)
	}
}

// TestLoadProjectContextTool_TruncationNoteNamesCutKinds verifies that
// when the current-state files alone exceed max_bytes, the truncation note
// names the kind cut short and the kinds left out, and the footer about
// omitted history still ends the response.
func TestLoadProjectContextTool_TruncationNoteNamesCutKinds(t *testing.T) {
	tool, ctx := seedProgress(t, 12)
	s, err := tool.Stores.Get(ctx, tool.Resolver.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "architecture", strings.Repeat("architecture line\n", 200)); err != nil {
		t.Fatal(err)
	}

	text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme", "max_bytes": 2048})
	if len(text) > 2048+300 {
		t.Errorf("response is %d bytes, want about 2048", len(text))
	}
	if !strings.Contains(text, "; cut short: architecture; left out: progress") {
		t.Errorf("the note should name the cut and left-out kinds, got: %s", text)
	}
	if !strings.HasSuffix(text, `set "mode" to "full"]`) {
		t.Errorf("the omitted-history footer should still end the response, got: %s", text)
	}
}

// TestLoadProjectContextTool_Validate_RejectsUnknownMode verifies that an
// unknown mode is rejected.
func TestLoadProjectContextTool_Validate_RejectsUnknownMode(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "mode": "brief"})); err == nil {
		t.Fatal(`expected an error for mode "brief"`)
	}
}

// TestLoadProjectContextTool_NeverExceedsMaxBytes verifies that the
// response, truncation note and footer included, fits in max_bytes for
// small budgets, many cut kinds and long lines.
func TestLoadProjectContextTool_NeverExceedsMaxBytes(t *testing.T) {
	tool, ctx := seedProgress(t, 30)
	s, err := tool.Stores.Get(ctx, tool.Resolver.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		kind := fmt.Sprintf("custom_kind_with_a_rather_long_name_%02d", i)
		if err := s.WriteDocument(ctx, "acme", "", kind, strings.Repeat("line of text\n", 40)); err != nil {
			t.Fatal(err)
		}
	}
	for _, maxBytes := range []int{1024, 1500, 2048, 4096, 8192} {
		text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme", "max_bytes": maxBytes})
		if len(text) > maxBytes {
			t.Errorf("max_bytes %d: response is %d bytes", maxBytes, len(text))
		}
		if !strings.Contains(text, "context truncated") {
			t.Errorf("max_bytes %d: no truncation note", maxBytes)
		}
	}
}

// TestLoadProjectContextTool_LongLineKeepsMostOfTheBudget verifies that a
// file made of one huge line is cut near max_bytes instead of falling back
// to the last line break far before it.
func TestLoadProjectContextTool_LongLineKeepsMostOfTheBudget(t *testing.T) {
	tool, ctx := seedProgress(t, 0)
	s, err := tool.Stores.Get(ctx, tool.Resolver.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", strings.Repeat("é", 5000)); err != nil {
		t.Fatal(err)
	}
	text := runLoadContext(t, tool, ctx, map[string]any{"project": "acme", "max_bytes": 4096})
	if len(text) > 4096 {
		t.Fatalf("response is %d bytes, want at most 4096", len(text))
	}
	kept := strings.Count(text, "é") * len("é")
	if kept < 4096/2 {
		t.Errorf("kept %d bytes of the long line, want at least half of the budget", kept)
	}
	if !utf8.ValidString(text) {
		t.Error("the cut split a UTF-8 character")
	}
}

// TestLoadProjectContextTool_ValidateListsEveryProblem checks that every
// invalid argument is reported in one error.
func TestLoadProjectContextTool_ValidateListsEveryProblem(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	_, err := (&LoadProjectContextTool{Resolver: r, Stores: mgr}).Validate(mustJSON(t, map[string]any{
		"project": "acme", "max_entries": -1, "mode": "x", "since": "bad", "max_bytes": 1, "files": []string{"bad name"},
	}))
	if err == nil {
		t.Fatal("invalid arguments accepted")
	}
	for _, want := range []string{"since", "max_entries", "mode", "max_bytes", "bad name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q doesn't mention %s", err, want)
		}
	}
}

// TestLoadProjectContextTool_NamesUnknownFiles checks that a requested
// file that doesn't exist is named, alone or next to files that do.
func TestLoadProjectContextTool_NamesUnknownFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	runTool(t, &InitProjectMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme"})
	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	if text := runTool(t, tool, map[string]any{"project": "acme", "files": []string{"decision"}}).Text; !strings.Contains(text, "[no file named: decision]") || strings.Contains(text, "no content yet") {
		t.Errorf("a misspelled file alone: %q", text)
	}
	text := runTool(t, tool, map[string]any{"project": "acme", "files": []string{"memory", "decision"}}).Text
	if !strings.Contains(text, "## memory") || !strings.Contains(text, "[no file named: decision]") {
		t.Errorf("a misspelled file next to an existing one: %q", text)
	}
}

// TestLoadProjectContextTool_UnknownFilesNoteKeepsToMaxBytes checks that,
// with the smallest max_bytes, many long unknown file names are counted
// instead of named, so the response, notes included, stays within
// max_bytes.
func TestLoadProjectContextTool_UnknownFilesNoteKeepsToMaxBytes(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	runTool(t, &CreateProjectTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme"})
	runTool(t, &WriteMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "memory", "content": strings.Repeat("line of memory content\n", 200)})
	tool := &LoadProjectContextTool{Resolver: r, Stores: mgr}
	files := []string{"memory"}
	for i := range 10 {
		files = append(files, strings.Repeat("x", 120)+fmt.Sprint(i))
	}
	text := runTool(t, tool, map[string]any{"project": "acme", "files": files, "max_bytes": minContextBytes}).Text
	if len(text) > minContextBytes || !strings.Contains(text, "[10 requested files not found]") || !strings.Contains(text, "[context truncated") {
		t.Fatalf("response of %d bytes (max %d):\n%s", len(text), minContextBytes, text)
	}
}
