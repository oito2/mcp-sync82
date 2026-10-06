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
	"slices"
	"strings"
	"testing"
)

// TestMetadata_RealDatabaseErrorIsNotMaskedAsNotFound verifies that Metadata
// returns a genuine database error from the documents query instead of
// falling back to entries and reporting ErrNotFound. The documents table is
// renamed away to force a non-ErrNoRows error while project resolution still
// succeeds.
func TestMetadata_RealDatabaseErrorIsNotMaskedAsNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "hello"); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	if _, err := s.db.ExecContext(ctx, `ALTER TABLE documents RENAME TO documents_moved`); err != nil {
		t.Fatalf("rename documents table: %v", err)
	}

	_, err := s.Metadata(ctx, "acme", "", "memory")
	if err == nil {
		t.Fatal("expected an error once the documents table is gone")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("Metadata error = %v, want a real database error, not ErrNotFound (the old bug masked it as \"file not found\")", err)
	}
	if !strings.Contains(err.Error(), "kind metadata") {
		t.Errorf("Metadata error = %v, want it wrapped with the kind-metadata context", err)
	}
}

// TestMetadata_ArchivedOnlyKind verifies that a log whose entries are all
// archived, which KindExists still reports, has metadata with size 0 instead
// of ErrNotFound, and that a kind with no data at all is still not found.
func TestMetadata_ArchivedOnlyKind(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\nold"); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01"); err != nil {
		t.Fatalf("ArchiveEntries: %v", err)
	}
	if exists, err := s.KindExists(ctx, "acme", "", "progress"); err != nil || !exists {
		t.Fatalf("KindExists = %v, %v; want true, nil", exists, err)
	}

	info, err := s.Metadata(ctx, "acme", "", "progress")
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if info.SizeBytes != 0 || info.LastModified == "" {
		t.Errorf("Metadata = %+v, want size 0 and a modification date", info)
	}

	if _, err := s.Metadata(ctx, "acme", "", "decisions"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Metadata(kind with no data) error = %v, want ErrNotFound", err)
	}
}

// TestStore_ArchiveAwareKindQueries verifies that a log whose entries are all
// archived is hidden by ListKinds unless archived kinds are included, still
// exists for KindExists and KindMode, and has no visible content.
func TestStore_ArchiveAwareKindQueries(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- old"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01"); err != nil {
		t.Fatal(err)
	}

	if kinds, err := s.ListKinds(ctx, "acme", "", false); err != nil || len(kinds) != 0 {
		t.Errorf("ListKinds(visible) = %v, %v; want none", kinds, err)
	}
	if kinds, err := s.ListKinds(ctx, "acme", "", true); err != nil || !slices.Equal(kinds, []string{"progress"}) {
		t.Errorf("ListKinds(with archived) = %v, %v; want [progress]", kinds, err)
	}
	if exists, err := s.KindExists(ctx, "acme", "", "progress"); err != nil || !exists {
		t.Errorf("KindExists = %v, %v; want true for an all-archived log", exists, err)
	}
	if mode, err := s.KindMode(ctx, "acme", "", "progress"); err != nil || mode != KindStorageEntries {
		t.Errorf("KindMode = %q, %v; want entries", mode, err)
	}
	if _, ok, err := s.ReadContent(ctx, "acme", "", "progress"); err != nil || ok {
		t.Errorf("ReadContent ok = %v, %v; want no visible content", ok, err)
	}
}

// TestStore_ReplaceArchivedEntriesKeepsVisibleOnes verifies that
// ReplaceArchivedEntries adds the archived entry while leaving the existing
// non-archived entry in place.
func TestStore_ReplaceArchivedEntriesKeepsVisibleOnes(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- current"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceArchivedEntries(ctx, "acme", "", "progress", []EntrySection{{Date: "2020-01-01", Body: "## 2020-01-01\n- restored"}}); err != nil {
		t.Fatalf("ReplaceArchivedEntries: %v", err)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].Archived || entries[1].Archived {
		t.Fatalf("entries = %+v, want the restored archived entry and the visible one", entries)
	}
}

// TestStore_KindModeAndProjectExists verifies KindMode for a document kind and
// an unknown kind, and ProjectExists for existing and missing projects and
// subprojects.
func TestStore_KindModeAndProjectExists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", "api"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "x"); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		kind string
		want KindStorage
	}{{"memory", KindStorageDocument}, {"notes", KindStorageNone}} {
		if got, err := s.KindMode(ctx, "acme", "", c.kind); err != nil || got != c.want {
			t.Errorf("KindMode(%s) = %q, %v; want %q", c.kind, got, err, c.want)
		}
	}
	for _, c := range []struct {
		project, subproject string
		want                bool
	}{{"acme", "", true}, {"acme", "api", true}, {"acme", "web", false}, {"ghost", "", false}} {
		if got, err := s.ProjectExists(ctx, c.project, c.subproject); err != nil || got != c.want {
			t.Errorf("ProjectExists(%s/%s) = %v, %v; want %v", c.project, c.subproject, got, err, c.want)
		}
	}
}
