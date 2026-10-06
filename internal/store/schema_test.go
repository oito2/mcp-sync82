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
	"slices"
	"testing"
)

// TestMigrate_RecordsOnlyOneRowPerVersion verifies that running migrate again
// on a migrated database does not record a version twice.
func TestMigrate_RecordsOnlyOneRowPerVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations rows for version 1 = %d, want 1", count)
	}

	// Re-running migrate on an already-migrated DB must not insert a
	// second row for the same version.
	if err := migrate(context.Background(), s.db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations rows for version 1 after re-migrate = %d, want 1 (still)", count)
	}
}

// TestMigrate_LaterMigrationRunsOnceAgainstAlreadyMigratedVault verifies that
// a later migration with a non-idempotent statement (ALTER TABLE ... ADD
// COLUMN) runs once against a vault that already has version 1 applied and is
// not re-run on a subsequent Open.
func TestMigrate_LaterMigrationRunsOnceAgainstAlreadyMigratedVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	s, err := Open(context.Background(), path) // applies every current migration
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	original := migrations
	next := original[len(original)-1].version + 1
	migrations = append(append([]migration{}, original...), migration{
		version: next,
		stmts:   []string{`ALTER TABLE projects ADD COLUMN test_marker TEXT`},
	})
	t.Cleanup(func() { migrations = original })

	if err := migrate(context.Background(), s.db); err != nil {
		t.Fatalf("migrate to version %d: %v", next, err)
	}
	assertColumnExists(t, s.db, "projects", "test_marker")

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, next).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations rows for version %d = %d, want 1", next, count)
	}
	s.Close()

	// Re-opening the same on-disk vault must not re-run the ALTER TABLE —
	// if migrate() re-executed already-applied migrations, this would
	// fail with a "duplicate column name" error from SQLite.
	s2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer s2.Close()
	assertColumnExists(t, s2.db, "projects", "test_marker")
}

// assertColumnExists fails the test unless table has a column named column in
// db, or when the schema query fails.
func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column name: %v", err)
		}
		if name == column {
			return
		}
	}
	t.Fatalf("column %q not found on table %q", column, table)
}

// TestOpen_RefusesVaultFromNewerSchema verifies that Open fails with
// ErrOpenFailed for a vault whose schema version is newer than the latest
// known migration.
func TestOpen_RefusesVaultFromNewerSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	newer := migrations[len(migrations)-1].version + 1
	if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, newer, now()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(ctx, path); !errors.Is(err, ErrOpenFailed) {
		t.Fatalf("Open of a newer-schema vault: err = %v, want ErrOpenFailed", err)
	}
}

// TestMigrate_LowercasesExistingNames verifies that the lower-casing
// migration lower-cases mixed-case names, except where the lower-cased name
// would collide in the same scope.
func TestMigrate_LowercasesExistingNames(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	// Set up mixed-case rows and remove the lower-casing migration record so
	// the migration runs again.
	for _, stmt := range []string{
		`INSERT INTO projects (name, parent_id, created_at) VALUES ('Acme', NULL, 'x'), ('Beta', NULL, 'x'), ('beta', NULL, 'x')`,
		`INSERT INTO documents (project_id, kind, content, updated_at) VALUES (1, 'Memory', 'kept', 'x')`,
		`INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position) VALUES (1, 'Progress', '2026-01-01', 'log', 0, 'x', 0)`,
		`DELETE FROM schema_migrations WHERE version = 2`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	s.Close()

	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if content, ok, err := s.ReadDocument(ctx, "acme", "", "memory"); err != nil || !ok || content != "kept" {
		t.Fatalf("acme/memory = %q, %v, %v; want the migrated document", content, ok, err)
	}
	if entries, err := s.ReadEntries(ctx, "ACME", "", "PROGRESS", false); err != nil || len(entries) != 1 {
		t.Fatalf("acme/progress = %v, %v; want the migrated entry", entries, err)
	}
	var names []string
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM projects ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if !slices.Equal(names, []string{"Beta", "acme", "beta"}) {
		t.Fatalf("projects = %v, want Acme lower-cased and the colliding Beta left alone", names)
	}
}
