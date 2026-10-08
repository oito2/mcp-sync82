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
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// TestArchiveMemoryTool_Validate_RejectsBadFilename verifies that validation
// rejects filenames that are not an appendable log kind (memory, architecture,
// a custom kind, or empty).
func TestArchiveMemoryTool_Validate_RejectsBadFilename(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}

	for _, filename := range []string{"memory", "architecture", "custom-notes", ""} {
		if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": filename})); err == nil {
			t.Errorf("expected a validation error for filename %q", filename)
		}
	}
}

// TestArchiveMemoryTool_Validate_NormalizesFilename verifies that filename is
// matched case-insensitively and returned lower-cased, as in the other
// memory tools.
func TestArchiveMemoryTool_Validate_NormalizesFilename(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": " Progress "}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := parsed.(archiveMemoryArgs).Filename; got != "progress" {
		t.Fatalf("Filename = %q, want %q", got, "progress")
	}
}

// TestArchiveMemoryTool_Validate_KeepDaysDefaultAndBounds verifies that
// keep_days defaults to 90 (also when 0), rejects negative values and values
// above maxKeepDays, and accepts maxKeepDays itself.
func TestArchiveMemoryTool_Validate_KeepDaysDefaultAndBounds(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := parsed.(archiveMemoryArgs).KeepDays; got != 90 {
		t.Fatalf("default keep_days = %d, want 90", got)
	}

	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "keep_days": 0})); err != nil {
		t.Fatalf("Validate with keep_days=0 should apply the default, got error: %v", err)
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "keep_days": -1})); err == nil {
		t.Fatal("expected a validation error for keep_days=-1")
	}
	// Values that would overflow the cutoff date computation must be rejected.
	for _, days := range []int64{maxKeepDays + 1, math.MaxInt64} {
		if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "keep_days": days})); err == nil {
			t.Errorf("expected a validation error for keep_days=%d", days)
		}
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "keep_days": maxKeepDays})); err != nil {
		t.Errorf("keep_days=%d should be accepted, got %v", maxKeepDays, err)
	}
}

// recentEntryDate returns yesterday's UTC date in YYYY-MM-DD form, an entry
// date that is always newer than the cutoff of any keep_days of at least one.
func recentEntryDate() string {
	return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
}

// TestArchiveMemoryTool_ArchivesOldEntriesOnly verifies that only entries
// older than the keep_days cutoff are archived, while recent entries are kept
// and undated entries are counted separately and never archived.
func TestArchiveMemoryTool_ArchivesOldEntriesOnly(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	// One old entry (should be archived), one recent entry (kept), one
	// undated entry (must never be archived regardless of age). The recent
	// date is relative to the clock, so it stays within keep_days.
	recent := recentEntryDate()
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\nold stuff"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", recent, "## "+recent+"\nrecent stuff"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "", "undated stuff"); err != nil {
		t.Fatal(err)
	}

	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "keep_days": 90}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(result.Text, "Archived 1 entry from acme/progress") {
		t.Fatalf("Text = %q, want it to report 1 archived entry", result.Text)
	}
	if !strings.Contains(result.Text, "2 kept, 1 undated") {
		t.Fatalf("Text = %q, want it to report 2 kept and 1 undated", result.Text)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", true)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	archived := map[string]bool{}
	for _, e := range entries {
		archived[e.EntryDate] = e.Archived
	}
	want := map[string]bool{"2020-01-01": true, recent: false, "": false}
	if len(entries) != 3 || !reflect.DeepEqual(archived, want) {
		t.Fatalf("archived state by date = %v (%d entries), want %v", archived, len(entries), want)
	}
}

// TestArchiveMemoryTool_NothingToArchive verifies that the tool reports there
// is nothing to archive when every entry is recent.
func TestArchiveMemoryTool_NothingToArchive(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	recent := recentEntryDate()
	if err := s.AppendEntry(ctx, "acme", "", "decisions", recent, "## "+recent+"\nrecent decision"); err != nil {
		t.Fatal(err)
	}

	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "decisions"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "Nothing to archive in acme/decisions") {
		t.Fatalf("Text = %q, want the nothing-to-archive message", result.Text)
	}
}

// TestArchiveMemoryTool_ProjectNotFound verifies that executing against a
// nonexistent project returns an error.
func TestArchiveMemoryTool_ProjectNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "ghost", "filename": "progress"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err == nil {
		t.Fatal("expected an execution error for a project that doesn't exist")
	}
}

// TestArchiveMemoryTool_RefusesProjectFromLastSession verifies that, with no
// project given, the tool returns an error result instead of falling back to
// the last-used project from the global config, because archiving cannot be
// undone through the tools.
func TestArchiveMemoryTool_RefusesProjectFromLastSession(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) {
		*c = config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}
	}); err != nil {
		t.Fatal(err)
	}
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"filename": "progress"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Text, "last session") {
		t.Fatalf("result = %+v, want an error result about the last session", result)
	}
}

// TestArchiveMemoryTool_RejectsUnknownArgument verifies that validation
// rejects an unknown argument (a misspelled "keepDays") and names it in the
// error, rather than ignoring it and archiving with the default keep_days.
func TestArchiveMemoryTool_RejectsUnknownArgument(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	_, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "keepDays": 365}))
	if err == nil || !strings.Contains(err.Error(), "keepDays") {
		t.Fatalf("err = %v, want it to name the unknown argument", err)
	}
}

// seedOldLog creates project acme in the default vault of r with two
// progress entries a year old and one from today, and returns the store.
func seedOldLog(t *testing.T, r *Resolver, mgr *store.Manager) (*store.Store, string, string) {
	t.Helper()
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old1, old2 := now.AddDate(-1, 0, 0).Format("2006-01-02"), now.AddDate(-1, 0, 1).Format("2006-01-02")
	today := now.Format("2006-01-02")
	for _, e := range [][2]string{{old1, "first old step"}, {old2, "second old step"}, {today, "today's step"}} {
		if err := s.AppendEntry(ctx, "acme", "", "progress", e[0], "## "+e[0]+"\n- "+e[1]); err != nil {
			t.Fatal(err)
		}
	}
	return s, old1, old2
}

// TestArchiveMemoryTool_DryRunChangesNothing verifies that a dry run lists
// the entries it would archive and leaves them active.
func TestArchiveMemoryTool_DryRunChangesNothing(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, old1, old2 := seedOldLog(t, r, mgr)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}

	text := runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "dry_run": true, "summary": "x"}).Text
	for _, want := range []string{
		"Dry run — nothing was changed. Would archive 2 entries from acme/progress",
		"(from " + old1 + " to " + old2 + ")",
		"] entry 1: - first old step",
		"] entry 2: - second old step",
		"would be added as a new active entry dated today",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("dry run report lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "today's step") {
		t.Errorf("a recent entry was listed: %s", text)
	}
	if active, err := s.ReadEntries(context.Background(), "acme", "", "progress", false); err != nil || len(active) != 3 {
		t.Fatalf("after a dry run, active entries = %d, %v; want 3", len(active), err)
	}
}

// TestArchiveMemoryTool_DryRunDoesNotCreateVault verifies that a dry run on
// a missing vault returns the missing-vault result without creating it.
func TestArchiveMemoryTool_DryRunDoesNotCreateVault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	vault := filepath.Join(t.TempDir(), "missing.db")
	result := runTool(t, &ArchiveMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "progress", "dry_run": true, "path": vault})
	if !result.IsError || !strings.Contains(result.Text, "No vault exists") {
		t.Fatalf("result = %+v, want the missing-vault result", result)
	}
	if _, err := os.Stat(vault); !os.IsNotExist(err) {
		t.Fatalf("the dry run created the vault: %v", err)
	}
}

// TestArchiveMemoryTool_SummaryWithoutHeaderGetsOne verifies that a summary
// without a date header is added under a generated header dated today and
// naming the archived range, and that a second archive keeps it active.
func TestArchiveMemoryTool_SummaryWithoutHeaderGetsOne(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, old1, old2 := seedOldLog(t, r, mgr)

	text := runTool(t, &ArchiveMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "progress", "summary": "- Two old steps, both done."}).Text
	if !strings.Contains(text, "Archived 2 entries") || !strings.Contains(text, "→ summary added as entry") {
		t.Fatalf("result = %s", text)
	}
	active, err := s.ReadEntries(context.Background(), "acme", "", "progress", false)
	if err != nil || len(active) != 2 {
		t.Fatalf("active entries = %+v, %v", active, err)
	}
	today := time.Now().UTC().Format("2006-01-02")
	want := "## " + today + " — Summary of 2 archived entries (from " + old1 + " to " + old2 + ")\n- Two old steps, both done."
	if active[1].EntryDate != today || active[1].Body != want {
		t.Fatalf("summary entry = %+v, want body %q dated %s", active[1], want, today)
	}

	text = runTool(t, &ArchiveMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "progress"}).Text
	if !strings.Contains(text, "Nothing to archive") {
		t.Fatalf("the summary was archived again: %s", text)
	}
}

// TestArchiveMemoryTool_SummaryWithHeaderKeepsIt verifies that a summary
// with its own date header is stored as given, dated by that header, and
// that a header older than the cutoff is rejected.
func TestArchiveMemoryTool_SummaryWithHeaderKeepsIt(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, _, _ := seedOldLog(t, r, mgr)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "summary": "## 2020-12-31\n- too old"})); err == nil || !strings.Contains(err.Error(), "older than the archive cutoff") {
		t.Fatalf("a summary dated before the cutoff: err = %v", err)
	}

	date := archiveCutoff(90)
	summary := "## " + date + "\n- The year in one line."
	runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "summary": summary})
	active, err := s.ReadEntries(context.Background(), "acme", "", "progress", false)
	if err != nil || len(active) != 2 || active[0].EntryDate != date || active[0].Body != summary {
		t.Fatalf("active entries = %+v, %v", active, err)
	}
}

// TestArchiveMemoryTool_SummaryNotWrittenWhenNothingIsArchived verifies
// that a summary is dropped, and the result says so, when no entry is old
// enough, and that a blank summary is rejected.
func TestArchiveMemoryTool_SummaryNotWrittenWhenNothingIsArchived(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, _, _ := seedOldLog(t, r, mgr)
	tool := &ArchiveMemoryTool{Resolver: r, Stores: mgr}

	text := runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "keep_days": 1000, "summary": "- nothing"}).Text
	if !strings.Contains(text, "Nothing to archive") || !strings.Contains(text, "The summary was not written.") {
		t.Fatalf("result = %s", text)
	}
	if active, err := s.ReadEntries(context.Background(), "acme", "", "progress", false); err != nil || len(active) != 3 {
		t.Fatalf("active entries = %d, %v; want 3", len(active), err)
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "summary": "  \n<!-- entry:4 -->\n"})); err == nil {
		t.Fatal("a blank summary should be rejected")
	}
}

// TestFormatArchiveDryRun_Bounds verifies the dry-run report for nothing to
// archive, the cap on listed entries, the cut of long first lines and an
// entry with only its header.
func TestFormatArchiveDryRun_Bounds(t *testing.T) {
	if got := formatArchiveDryRun(nil, "acme/progress", 90, true); !strings.Contains(got, "nothing would be archived") || !strings.Contains(got, "summary would not be written") {
		t.Errorf("empty report = %q", got)
	}

	entries := make([]store.Entry, maxDryRunEntries+5)
	for i := range entries {
		entries[i] = store.Entry{ID: int64(i + 1), EntryDate: "2026-01-01", Body: "## 2026-01-01\n- " + strings.Repeat("x", maxDryRunLineRunes+10)}
	}
	entries[0].Body = "## 2026-01-01\n\n"
	got := formatArchiveDryRun(entries, "acme/progress", 90, false)
	if !strings.Contains(got, "- [2026-01-01] entry 1: (empty)") {
		t.Errorf("an entry with only its header should show (empty):\n%s", got)
	}
	if !strings.Contains(got, "- "+strings.Repeat("x", maxDryRunLineRunes-2)+"…") {
		t.Errorf("long first lines should be cut with an ellipsis")
	}
	if !strings.Contains(got, "- … and 5 more; read them with read_memory") || strings.Contains(got, "entry 201:") {
		t.Errorf("the listing should stop at %d entries", maxDryRunEntries)
	}
	if !strings.HasSuffix(got, "pass a summary when archiving.") {
		t.Errorf("without a summary the report should suggest one")
	}
}
