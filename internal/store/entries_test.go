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

package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// TestReplaceAllEntries_MultipleDateSections verifies that ReplaceAllEntries
// stores several dated sections and reads them back in order.
func TestReplaceAllEntries_MultipleDateSections(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- entry that should be wiped by the replace"); err != nil {
		t.Fatalf("AppendEntry (seed): %v", err)
	}

	sections := []EntrySection{
		{Date: "2026-01-01", Body: "## 2026-01-01\n- did X"},
		{Date: "2026-01-02", Body: "## 2026-01-02\n- did Y"},
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatalf("ReplaceAllEntries: %v", err)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after replace, got %d", len(entries))
	}
	if entries[0].EntryDate != "2026-01-01" || entries[1].EntryDate != "2026-01-02" {
		t.Fatalf("unexpected order/dates: %+v", entries)
	}
	if entries[0].Body != sections[0].Body || entries[1].Body != sections[1].Body {
		t.Fatalf("unexpected bodies: %+v", entries)
	}
}

// TestReadEntriesSince_FiltersByDateKeepingUndated verifies that the since
// filter drops older dated entries but always keeps undated ones.
func TestReadEntriesSince_FiltersByDateKeepingUndated(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	sections := []EntrySection{
		{Date: "2026-01-01", Body: "## 2026-01-01\n- old"},
		{Date: "2026-06-01", Body: "## 2026-06-01\n- recent"},
		{Body: "undated note, always kept regardless of since"},
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatalf("ReplaceAllEntries: %v", err)
	}

	entries, err := s.ReadEntriesSince(ctx, "acme", "", "progress", "2026-03-01", 0)
	if err != nil {
		t.Fatalf("ReadEntriesSince: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (recent dated + undated), got %d: %+v", len(entries), entries)
	}
	if entries[0].EntryDate != "2026-06-01" || entries[1].EntryDate != "" {
		t.Fatalf("unexpected order/dates: %+v", entries)
	}
}

// TestReadEntriesSince_MaxEntriesKeepsMostRecentInOrder verifies that maxEntries
// keeps the most recent dated entries and returns them in ascending order.
func TestReadEntriesSince_MaxEntriesKeepsMostRecentInOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	sections := []EntrySection{
		{Date: "2026-01-01", Body: "## 2026-01-01\n- first"},
		{Date: "2026-02-01", Body: "## 2026-02-01\n- second"},
		{Date: "2026-03-01", Body: "## 2026-03-01\n- third"},
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatalf("ReplaceAllEntries: %v", err)
	}

	entries, err := s.ReadEntriesSince(ctx, "acme", "", "progress", "", 2)
	if err != nil {
		t.Fatalf("ReadEntriesSince: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (the max_entries cap), got %d: %+v", len(entries), entries)
	}
	// Must still be in ascending chronological order, not reversed.
	if entries[0].EntryDate != "2026-02-01" || entries[1].EntryDate != "2026-03-01" {
		t.Fatalf("unexpected order/dates: %+v", entries)
	}
}

// TestReadEntriesSince_MaxEntriesLargerThanResultReturnsEverything verifies that
// a maxEntries larger than the number of entries returns all of them.
func TestReadEntriesSince_MaxEntriesLargerThanResultReturnsEverything(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	sections := []EntrySection{
		{Date: "2026-01-01", Body: "## 2026-01-01\n- first"},
		{Date: "2026-02-01", Body: "## 2026-02-01\n- second"},
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatalf("ReplaceAllEntries: %v", err)
	}

	entries, err := s.ReadEntriesSince(ctx, "acme", "", "progress", "", 10)
	if err != nil {
		t.Fatalf("ReadEntriesSince: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected both entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].EntryDate != "2026-01-01" || entries[1].EntryDate != "2026-02-01" {
		t.Fatalf("unexpected order/dates: %+v", entries)
	}
}

// TestReadEntriesSince_ExcludesArchived verifies that archived entries are not
// returned.
func TestReadEntriesSince_ExcludesArchived(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- old"); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2099-01-01", nil); err != nil {
		t.Fatalf("ArchiveEntries: %v", err)
	}

	entries, err := s.ReadEntriesSince(ctx, "acme", "", "progress", "", 0)
	if err != nil {
		t.Fatalf("ReadEntriesSince: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected archived entry to be excluded, got %+v", entries)
	}
}

// TestArchiveEntries_NeverArchivesUndated verifies that ArchiveEntries archives
// only dated entries older than the cutoff and reports correct counts.
func TestArchiveEntries_NeverArchivesUndated(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- ancient"); err != nil {
		t.Fatalf("AppendEntry (ancient): %v", err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "", "no date header at all"); err != nil {
		t.Fatalf("AppendEntry (undated): %v", err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- recent"); err != nil {
		t.Fatalf("AppendEntry (recent): %v", err)
	}

	result, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2025-01-01", nil)
	if err != nil {
		t.Fatalf("ArchiveEntries: %v", err)
	}
	if result.Archived != 1 {
		t.Fatalf("expected 1 archived entry (the 2020 one), got %d", result.Archived)
	}
	if result.Kept != 2 {
		t.Fatalf("expected 2 entries kept (undated + recent), got %d", result.Kept)
	}
	if result.NoDate != 1 {
		t.Fatalf("expected 1 undated entry among those kept, got %d", result.NoDate)
	}

	remaining, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 non-archived entries remaining, got %d", len(remaining))
	}

	withArchived, err := s.ReadEntries(ctx, "acme", "", "progress", true)
	if err != nil {
		t.Fatalf("ReadEntries (include archived): %v", err)
	}
	if len(withArchived) != 3 {
		t.Fatalf("expected all 3 entries when including archived, got %d", len(withArchived))
	}
}

// TestDeleteKind_RemovesDocumentAndEntryRows verifies that DeleteKind removes
// both document and entries rows of a kind.
func TestDeleteKind_RemovesDocumentAndEntryRows(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "notes", "some notes"); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	if err := s.DeleteKind(ctx, "acme", "", "notes"); err != nil {
		t.Fatalf("DeleteKind: %v", err)
	}

	if _, ok, err := s.ReadDocument(ctx, "acme", "", "notes"); err != nil || ok {
		t.Fatalf("ReadDocument after delete: ok=%v err=%v, want ok=false", ok, err)
	}
}

// TestDeleteKind_NotFoundWhenKindNeverExisted verifies that DeleteKind returns
// ErrNotFound for a kind with no rows.
func TestDeleteKind_NotFoundWhenKindNeverExisted(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	err := s.DeleteKind(ctx, "acme", "", "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteKind error = %v, want it to wrap ErrNotFound", err)
	}
}

// TestDeleteKind_IsAtomic verifies that DeleteKind is transactional. The
// entries table is renamed away so the second DELETE fails after the
// documents DELETE ran; the document must still exist once the table is
// restored.
func TestDeleteKind_IsAtomic(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "notes", "some notes"); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	if _, err := s.db.ExecContext(ctx, `ALTER TABLE entries RENAME TO entries_moved`); err != nil {
		t.Fatalf("rename entries table: %v", err)
	}

	if err := s.DeleteKind(ctx, "acme", "", "notes"); err == nil {
		t.Fatal("expected DeleteKind to fail once the entries table is gone")
	}

	if _, err := s.db.ExecContext(ctx, `ALTER TABLE entries_moved RENAME TO entries`); err != nil {
		t.Fatalf("restore entries table: %v", err)
	}

	content, ok, err := s.ReadDocument(ctx, "acme", "", "notes")
	if err != nil {
		t.Fatalf("ReadDocument: %v", err)
	}
	if !ok {
		t.Fatal("the documents row was deleted despite the transaction failing — DeleteKind is not atomic")
	}
	if content != "some notes" {
		t.Errorf("content = %q, want %q", content, "some notes")
	}
}

// BenchmarkAppendEntry_LargeKind measures appending to a kind that already
// holds many entries, which exercises the position lookup index.
func BenchmarkAppendEntry_LargeKind(b *testing.B) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(b.TempDir(), "vault.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		b.Fatal(err)
	}
	sections := make([]EntrySection, 50000)
	for i := range sections {
		sections[i] = EntrySection{Date: "2026-01-01", Body: "## 2026-01-01\n- entry"}
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-02", "## 2026-01-02\n- more"); err != nil {
			b.Fatal(err)
		}
	}
}

// TestEntryStats_CountsOnlyActiveEntries verifies that EntryStats counts
// active dated and undated entries apart, leaves out archived entries and a
// preamble, reports the newest active date, returns zeros for a kind with
// no entries, and reports a missing project as ErrNotFound.
func TestEntryStats_CountsOnlyActiveEntries(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	sections := []EntrySection{
		{Body: "# Progress", Preamble: true},
		{Date: "2026-01-01", Body: "## 2026-01-01\n- old"},
		{Date: "2026-03-01", Body: "## 2026-03-01\n- recent"},
		{Date: "2026-02-01", Body: "## 2026-02-01\n- middle"},
		{Body: "an undated note"},
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatalf("ReplaceAllEntries: %v", err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2026-01-15", nil); err != nil {
		t.Fatalf("ArchiveEntries: %v", err)
	}

	st, err := s.EntryStats(ctx, "acme", "", "progress")
	if err != nil {
		t.Fatalf("EntryStats: %v", err)
	}
	if st != (EntryStats{Dated: 2, Undated: 1, LatestDate: "2026-03-01"}) {
		t.Fatalf("EntryStats = %+v, want 2 dated, 1 undated, latest 2026-03-01", st)
	}

	st, err = s.EntryStats(ctx, "acme", "", "decisions")
	if err != nil || st != (EntryStats{}) {
		t.Fatalf("EntryStats on an empty kind = %+v, %v; want zeros, nil", st, err)
	}

	if _, err := s.EntryStats(ctx, "missing", "", "progress"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("EntryStats on a missing project: err = %v, want ErrNotFound", err)
	}
}

// seedEntries creates project acme with subproject api and returns the ids
// of three entries: two progress entries of acme and one progress entry of
// acme/api.
func seedEntries(t *testing.T, ctx context.Context, s *Store) (first, second, other int64) {
	t.Helper()
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", "api"); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	for _, e := range []struct{ sub, date, body string }{
		{"", "2026-01-01", "## 2026-01-01\n- first"},
		{"", "2026-02-01", "## 2026-02-01\n- second"},
		{"api", "2026-01-01", "## 2026-01-01\n- api entry"},
	} {
		if err := s.AppendEntry(ctx, "acme", e.sub, "progress", e.date, e.body); err != nil {
			t.Fatalf("AppendEntry: %v", err)
		}
	}
	main, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(main) != 2 {
		t.Fatalf("ReadEntries = %+v, %v", main, err)
	}
	sub, err := s.ReadEntries(ctx, "acme", "api", "progress", false)
	if err != nil || len(sub) != 1 {
		t.Fatalf("ReadEntries = %+v, %v", sub, err)
	}
	if main[0].ID == 0 || main[1].ID == 0 || sub[0].ID == 0 {
		t.Fatalf("ReadEntries returned entries without ids: %+v %+v", main, sub)
	}
	return main[0].ID, main[1].ID, sub[0].ID
}

// TestReadEntry_OnlyWithinProjectAndKind verifies that ReadEntry returns an
// entry by id and reports ErrNotFound for an id of another subproject or
// kind.
func TestReadEntry_OnlyWithinProjectAndKind(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	first, _, other := seedEntries(t, ctx, s)

	e, err := s.ReadEntry(ctx, "acme", "", "progress", first)
	if err != nil {
		t.Fatalf("ReadEntry: %v", err)
	}
	if e.ID != first || e.EntryDate != "2026-01-01" || e.Body != "## 2026-01-01\n- first" {
		t.Fatalf("ReadEntry = %+v", e)
	}
	if _, err := s.ReadEntry(ctx, "acme", "", "progress", other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadEntry of another subproject's entry: err = %v, want ErrNotFound", err)
	}
	if _, err := s.ReadEntry(ctx, "acme", "", "decisions", first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadEntry with the wrong kind: err = %v, want ErrNotFound", err)
	}
}

// TestUpdateEntry_KeepsPositionAndOptionallyDate verifies that UpdateEntry
// replaces only the body when no date is given, also moves the date when
// one is, keeps the entry's position, and never touches another
// project's entry.
func TestUpdateEntry_KeepsPositionAndOptionallyDate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	first, second, other := seedEntries(t, ctx, s)

	if err := s.UpdateEntry(ctx, "acme", "", "progress", first, "## 2026-01-01\n- first, fixed", nil); err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}
	e, err := s.ReadEntry(ctx, "acme", "", "progress", first)
	if err != nil || e.Body != "## 2026-01-01\n- first, fixed" || e.EntryDate != "2026-01-01" {
		t.Fatalf("after UpdateEntry: %+v, %v", e, err)
	}

	date := "2026-03-01"
	if err := s.UpdateEntry(ctx, "acme", "", "progress", first, "## 2026-03-01\n- moved", &date); err != nil {
		t.Fatalf("UpdateEntry with date: %v", err)
	}
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ID != second || entries[1].ID != first || entries[1].EntryDate != "2026-03-01" {
		t.Fatalf("after moving the date, entries = %+v", entries)
	}

	if err := s.UpdateEntry(ctx, "acme", "", "progress", other, "hijack", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateEntry of another subproject's entry: err = %v, want ErrNotFound", err)
	}
	e, err = s.ReadEntry(ctx, "acme", "api", "progress", other)
	if err != nil || e.Body != "## 2026-01-01\n- api entry" {
		t.Fatalf("the other subproject's entry changed: %+v, %v", e, err)
	}
}

// TestDeleteEntry_RemovesOnlyThatEntry verifies that DeleteEntry removes
// one entry and reports ErrNotFound for an id it does not own or that is
// already gone.
func TestDeleteEntry_RemovesOnlyThatEntry(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	first, second, other := seedEntries(t, ctx, s)

	if err := s.DeleteEntry(ctx, "acme", "", "progress", other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteEntry of another subproject's entry: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteEntry(ctx, "acme", "", "progress", first); err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", true)
	if err != nil || len(entries) != 1 || entries[0].ID != second {
		t.Fatalf("after DeleteEntry, entries = %+v, %v", entries, err)
	}
	if err := s.DeleteEntry(ctx, "acme", "", "progress", first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteEntry: err = %v, want ErrNotFound", err)
	}
	if _, err := s.ReadEntry(ctx, "acme", "api", "progress", other); err != nil {
		t.Fatalf("the other subproject's entry is gone: %v", err)
	}
}

// TestSupersedeEntry_AppendsAndMarksAtomically verifies that SupersedeEntry
// appends the new entry after the others, rewrites the old body through
// mark with the new id, and changes nothing when the id is not found.
func TestSupersedeEntry_AppendsAndMarksAtomically(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	first, _, other := seedEntries(t, ctx, s)
	mark := func(old string, newID int64) string { return fmt.Sprintf("%s\n(superseded by %d)", old, newID) }

	newID, err := s.SupersedeEntry(ctx, "acme", "", "progress", first, "2026-02-01", "## 2026-02-01\n- replacement", mark)
	if err != nil {
		t.Fatalf("SupersedeEntry: %v", err)
	}
	old, err := s.ReadEntry(ctx, "acme", "", "progress", first)
	if err != nil || old.Body != fmt.Sprintf("## 2026-01-01\n- first\n(superseded by %d)", newID) {
		t.Fatalf("old entry = %+v, %v", old, err)
	}
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(entries) != 3 || entries[2].ID != newID || entries[2].Body != "## 2026-02-01\n- replacement" {
		t.Fatalf("entries = %+v, %v; want the new entry last (same date, later position)", entries, err)
	}

	if _, err := s.SupersedeEntry(ctx, "acme", "", "progress", other, "", "x", mark); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SupersedeEntry of another subproject's entry: err = %v, want ErrNotFound", err)
	}
	entries, err = s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(entries) != 3 {
		t.Fatalf("a failed SupersedeEntry appended an entry: %+v, %v", entries, err)
	}
}

// TestUpdateEntry_ConcurrentWithAppends verifies that editing an entry while
// other goroutines append keeps every append, with distinct positions.
func TestUpdateEntry_ConcurrentWithAppends(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	first, _, _ := seedEntries(t, ctx, s)

	const appends = 50
	var wg sync.WaitGroup
	errs := make(chan error, appends*2)
	for i := 0; i < appends; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			errs <- s.AppendEntry(ctx, "acme", "", "progress", "2026-04-01", fmt.Sprintf("## 2026-04-01\n- concurrent %d", i))
		}(i)
		go func(i int) {
			defer wg.Done()
			errs <- s.UpdateEntry(ctx, "acme", "", "progress", first, fmt.Sprintf("## 2026-01-01\n- edit %d", i), nil)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent write: %v", err)
		}
	}

	var n, positions int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(DISTINCT position) FROM entries WHERE kind = 'progress' AND project_id = (SELECT id FROM projects WHERE name = 'acme' AND parent_id IS NULL)`).Scan(&n, &positions); err != nil {
		t.Fatal(err)
	}
	if n != appends+2 || positions != n {
		t.Fatalf("got %d entries with %d distinct positions, want %d of each", n, positions, appends+2)
	}
}

// TestArchiveEntries_WithSummary verifies that the summary entry is added
// with the archived range, that a failing summary leaves every entry as it
// was, that nothing is added when nothing is archived, and that
// ListArchivable lists exactly what gets archived.
func TestArchiveEntries_WithSummary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	sections := []EntrySection{
		{Date: "2026-01-01", Body: "## 2026-01-01\n- a"},
		{Date: "2026-02-01", Body: "## 2026-02-01\n- b"},
		{Date: "2026-06-01", Body: "## 2026-06-01\n- recent"},
		{Body: "undated"},
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatal(err)
	}

	listed, err := s.ListArchivable(ctx, "acme", "", "progress", "2026-03-01")
	if err != nil || len(listed) != 2 || listed[0].EntryDate != "2026-01-01" || listed[1].EntryDate != "2026-02-01" {
		t.Fatalf("ListArchivable = %+v, %v", listed, err)
	}

	failing := func(ArchiveResult) (string, string, error) { return "", "", errors.New("boom") }
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2026-03-01", failing); err == nil || err.Error() != "boom" {
		t.Fatalf("ArchiveEntries with a failing summary: err = %v, want boom", err)
	}
	if active, err := s.ReadEntries(ctx, "acme", "", "progress", false); err != nil || len(active) != 4 {
		t.Fatalf("after a failed archive, active entries = %d, %v; want all 4", len(active), err)
	}

	var seen ArchiveResult
	summary := func(r ArchiveResult) (string, string, error) {
		seen = r
		return "2026-03-01", "## 2026-03-01\n- summary", nil
	}
	result, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2026-03-01", summary)
	if err != nil {
		t.Fatal(err)
	}
	if seen.Archived != 2 || seen.OldestDate != "2026-01-01" || seen.NewestDate != "2026-02-01" {
		t.Errorf("summary saw %+v", seen)
	}
	if result.Archived != 2 || result.Kept != 3 || result.NoDate != 1 || result.SummaryID == 0 {
		t.Errorf("result = %+v", result)
	}
	active, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(active) != 3 || active[0].ID != result.SummaryID || active[0].EntryDate != "2026-03-01" {
		t.Fatalf("active entries = %+v, %v; want the summary first", active, err)
	}

	result, err = s.ArchiveEntries(ctx, "acme", "", "progress", "2026-03-01", summary)
	if err != nil || result.Archived != 0 || result.SummaryID != 0 {
		t.Fatalf("second archive = %+v, %v; want nothing archived (the summary is not older than the cutoff) and no summary", result, err)
	}
}

// TestWriteKinds_AppendAddsAfterExistingEntries verifies that an Append
// write adds its sections after the existing entries without removing them,
// in the same transaction as the other writes.
func TestWriteKinds_AppendAddsAfterExistingEntries(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- first"); err != nil {
		t.Fatal(err)
	}
	doc := "state"
	err := s.WriteKinds(ctx, "acme", "", []KindWrite{
		{Kind: "progress", Append: true, Sections: []EntrySection{{Date: "2026-01-01", Body: "## 2026-01-01\n- second"}}},
		{Kind: "memory", Document: &doc},
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(entries) != 2 || entries[0].Body != "## 2026-01-01\n- first" || entries[1].Body != "## 2026-01-01\n- second" {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	if content, ok, _ := s.ReadDocument(ctx, "acme", "", "memory"); !ok || content != "state" {
		t.Fatalf("memory = %q (ok=%v)", content, ok)
	}
}
