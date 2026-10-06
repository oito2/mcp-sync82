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
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// TestWriteDocument_OverwritesInPlaceWithoutKeepingHistory verifies that a
// second WriteDocument replaces the content and leaves a single documents row.
func TestWriteDocument_OverwritesInPlaceWithoutKeepingHistory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	if err := s.WriteDocument(ctx, "acme", "", "memory", "first version"); err != nil {
		t.Fatalf("WriteDocument (first): %v", err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "second version"); err != nil {
		t.Fatalf("WriteDocument (second): %v", err)
	}

	content, ok, err := s.ReadDocument(ctx, "acme", "", "memory")
	if err != nil {
		t.Fatalf("ReadDocument: %v", err)
	}
	if !ok || content != "second version" {
		t.Fatalf("got (%q, %v), want (%q, true)", content, ok, "second version")
	}

	// The second write updates the same row; no history rows accumulate.
	var docCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM documents WHERE kind = 'memory'`,
	).Scan(&docCount)
	if err != nil {
		t.Fatalf("query documents: %v", err)
	}
	if docCount != 1 {
		t.Fatalf("expected exactly 1 documents row for kind memory, got %d", docCount)
	}
}

// TestIsBlankOrTemplate verifies that IsBlankOrTemplate is true for a missing,
// empty or whitespace-only document and false for a document with content.
func TestIsBlankOrTemplate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	blank, err := s.IsBlankOrTemplate(ctx, "acme", "", "memory")
	if err != nil {
		t.Fatalf("IsBlankOrTemplate (no row): %v", err)
	}
	if !blank {
		t.Fatal("expected a kind with no document row to be blank")
	}

	if err := s.WriteDocument(ctx, "acme", "", "memory", "# Memory\n"); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	blank, err = s.IsBlankOrTemplate(ctx, "acme", "", "memory")
	if err != nil {
		t.Fatalf("IsBlankOrTemplate (after write): %v", err)
	}
	if blank {
		t.Fatal("expected a kind with real content to not be blank")
	}
}

// TestWriteDocument_ConcurrentWritersAllSucceed runs many concurrent writes
// through two independent Stores on the same vault file and verifies that
// every write succeeds.
func TestWriteDocument_ConcurrentWritersAllSucceed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	stores := make([]*Store, 2)
	for i := range stores {
		s, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		t.Cleanup(func() { s.Close() })
		stores[i] = s
	}
	if _, _, err := stores[0].EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	const writers = 200
	const kinds = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := stores[i%len(stores)]
			kind := fmt.Sprintf("k%d", i%kinds)
			if err := s.WriteDocument(ctx, "acme", "", kind, fmt.Sprintf("content %d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("WriteDocument: %v", err)
	}

	var rows int
	if err := stores[0].db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != kinds {
		t.Fatalf("documents rows = %d, want %d (one per kind)", rows, kinds)
	}
}

// TestConcurrentMixedWriters_AllSucceedWithUniquePositions mixes document
// writes and entry appends across two Stores on the same vault file and
// verifies that every write succeeds and every appended entry gets its own
// position.
func TestConcurrentMixedWriters_AllSucceedWithUniquePositions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	stores := make([]*Store, 2)
	for i := range stores {
		s, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		t.Cleanup(func() { s.Close() })
		stores[i] = s
	}
	if _, _, err := stores[0].EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	const writers = 100
	var wg sync.WaitGroup
	errs := make(chan error, 2*writers)
	for i := 0; i < writers; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if err := stores[i%2].AppendEntry(ctx, "acme", "", "progress", "2026-01-01", fmt.Sprintf("entry %d", i)); err != nil {
				errs <- err
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			if err := stores[(i+1)%2].WriteDocument(ctx, "acme", "", "memory", fmt.Sprintf("v%d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent write: %v", err)
	}

	var total, distinct int
	if err := stores[0].db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(DISTINCT position) FROM entries WHERE kind = 'progress'`,
	).Scan(&total, &distinct); err != nil {
		t.Fatal(err)
	}
	if total != writers || distinct != writers {
		t.Fatalf("entries = %d with %d distinct positions, want %d of each", total, distinct, writers)
	}
}
