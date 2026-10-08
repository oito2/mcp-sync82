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
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// TestMigrate_V4KeepsDataAndStopsReusingIds opens a vault at schema
// version 3 holding a project, a subproject, documents and entries with a
// gap in their ids, and checks that version 4 keeps every row with its id,
// keeps the full-text index working, keeps foreign keys enforced, and that
// ids are no longer reused afterwards.
func TestMigrate_V4KeepsDataAndStopsReusingIds(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	stmts := append(append(append([]string{migrationsTableDDL}, schemaV1...), lowercaseNamesV2...), fullTextSearchV3...)
	stmts = append(stmts,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (1, 'x'), (2, 'x'), (3, 'x')`,
		`INSERT INTO projects (id, name, parent_id, created_at) VALUES (1, 'acme', NULL, 'x'), (5, 'api', 1, 'x')`,
		`INSERT INTO documents (project_id, kind, content, updated_at) VALUES (5, 'memory', 'Decisão antiga', 'x')`,
		`INSERT INTO entries (id, project_id, kind, entry_date, body, archived, created_at, position) VALUES
			(3, 1, 'decisions', '2026-01-01', 'sessão um', 0, 'x', 0),
			(9, 1, 'decisions', '2026-01-02', 'drop redis', 0, 'x', 1)`,
	)
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	entries, err := s.ReadEntries(ctx, "acme", "", "decisions", false)
	if err != nil || len(entries) != 2 || entries[0].ID != 3 || entries[1].ID != 9 {
		t.Fatalf("entries after migration = %+v, %v; want ids 3 and 9", entries, err)
	}
	if doc, ok, err := s.ReadDocument(ctx, "acme", "api", "memory"); err != nil || !ok || doc != "Decisão antiga" {
		t.Fatalf("subproject document after migration = %q, %v, %v", doc, ok, err)
	}
	for _, q := range []string{"sessao", "decisao", "redis"} {
		if results, _, err := s.SearchText(ctx, SearchOptions{Query: q, Limit: 10}); err != nil || len(results) != 1 {
			t.Errorf("search %q after migration: %d results, %v", q, len(results), err)
		}
	}
	assertFTSIntegrity(t, s)

	// The newest entry is deleted, then a new one is added: it must not get id 9 again.
	if err := s.DeleteEntry(ctx, "acme", "", "decisions", 9); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-01-03", "keep redis"); err != nil {
		t.Fatal(err)
	}
	entries, _ = s.ReadEntries(ctx, "acme", "", "decisions", false)
	if got := entries[len(entries)-1].ID; got <= 9 {
		t.Errorf("new entry got id %d, want an id above 9", got)
	}
	if err := s.DeleteEntry(ctx, "acme", "", "decisions", 9); !errors.Is(err, ErrNotFound) {
		t.Errorf("a stale entry id: err = %v, want ErrNotFound", err)
	}

	// Foreign keys are enforced again: deleting the project removes its rows.
	if err := s.DeleteProject(ctx, "acme"); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM projects) + (SELECT count(*) FROM documents) + (SELECT count(*) FROM entries)`).Scan(&left); err != nil || left != 0 {
		t.Errorf("rows left after deleting the project: %d, %v", left, err)
	}
	var fk int
	if err := s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d, %v; want 1 after the migration", fk, err)
	}
	if _, _, err := s.EnsureProject(ctx, "beta", ""); err != nil {
		t.Fatal(err)
	}
	projects, _ := s.ListTopLevelProjects(ctx)
	if len(projects) != 1 || projects[0].ID <= 5 {
		t.Errorf("new projects = %+v, want one with an id above 5", projects)
	}
}

// TestStore_EntryIdsAreNeverReused checks, on a new vault, that an entry id
// dropped by a rewrite is not given to a new entry, so a stale id is
// reported as not found instead of naming another entry.
func TestStore_EntryIdsAreNeverReused(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"a", "drop redis"} {
		if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-01-01", body); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := s.ReadEntries(ctx, "acme", "", "decisions", false)
	stale := before[1].ID
	if err := s.ReplaceAllEntries(ctx, "acme", "", "decisions", []EntrySection{{Date: "2026-01-01", Body: "a"}, {Date: "2026-01-03", Body: "keep redis"}}); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ReadEntries(ctx, "acme", "", "decisions", false)
	for _, e := range after {
		if e.ID <= stale {
			t.Errorf("entry %q got id %d, reusing an id up to %d", e.Body, e.ID, stale)
		}
	}
	if err := s.DeleteEntry(ctx, "acme", "", "decisions", stale); !errors.Is(err, ErrNotFound) {
		t.Errorf("stale id: err = %v, want ErrNotFound", err)
	}
}
