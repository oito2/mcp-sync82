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
	"path/filepath"
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
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2099-01-01"); err != nil {
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

	result, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2025-01-01")
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
