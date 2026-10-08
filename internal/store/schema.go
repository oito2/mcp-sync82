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
	"fmt"
)

// migration is one versioned step of the vault schema: a fixed list of
// statements applied exactly once and recorded in schema_migrations under its
// version number. Statements may be non-idempotent (ALTER TABLE ... ADD
// COLUMN, data backfills) because migrate skips any version already
// recorded.
type migration struct {
	version int
	stmts   []string
}

// migrations is the ordered list of every schema version. New versions are
// appended with a higher number; the statements of an existing version must
// not change, since vaults may already have applied them.
var migrations = []migration{
	{version: 1, stmts: schemaV1},
	{version: 2, stmts: lowercaseNamesV2},
	{version: 3, stmts: fullTextSearchV3},
	{version: 4, stmts: idsNeverReusedV4},
}

// idsNeverReusedV4 rebuilds entries and projects with an AUTOINCREMENT key,
// so the id of a deleted row is never given to a later one: an entry id an
// agent read earlier can then only name that entry, or none. Every row
// keeps its id, so entries_fts, keyed by entry ids, and the references
// between tables stay valid. The indexes and the entries_fts triggers,
// dropped with the old tables, are created again, and entries_fts is
// rebuilt from the new table. It runs with foreign keys off (see migrate),
// so dropping the old tables deletes no row of another table.
var idsNeverReusedV4 = []string{
	`DROP TRIGGER IF EXISTS entries_fts_insert`,
	`DROP TRIGGER IF EXISTS entries_fts_delete`,
	`DROP TRIGGER IF EXISTS entries_fts_update`,
	`CREATE TABLE entries_v4 (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		kind          TEXT NOT NULL,
		entry_date    TEXT NULL,
		body          TEXT NOT NULL,
		archived      INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL,
		position      INTEGER NOT NULL
	)`,
	`INSERT INTO entries_v4 (id, project_id, kind, entry_date, body, archived, created_at, position)
		SELECT id, project_id, kind, entry_date, body, archived, created_at, position FROM entries`,
	`DROP TABLE entries`,
	`ALTER TABLE entries_v4 RENAME TO entries`,
	`CREATE INDEX idx_entries_lookup ON entries(project_id, kind, archived, entry_date)`,
	`CREATE INDEX idx_entries_position ON entries(project_id, kind, position)`,
	`CREATE TRIGGER entries_fts_insert AFTER INSERT ON entries BEGIN
		INSERT INTO entries_fts(rowid, body) VALUES (new.id, new.body);
	END`,
	`CREATE TRIGGER entries_fts_delete AFTER DELETE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, body) VALUES ('delete', old.id, old.body);
	END`,
	`CREATE TRIGGER entries_fts_update AFTER UPDATE OF body ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, body) VALUES ('delete', old.id, old.body);
		INSERT INTO entries_fts(rowid, body) VALUES (new.id, new.body);
	END`,
	`INSERT INTO entries_fts(entries_fts) VALUES ('rebuild')`,

	`CREATE TABLE projects_v4 (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT NOT NULL,
		parent_id     INTEGER NULL REFERENCES projects_v4(id) ON DELETE CASCADE,
		created_at    TEXT NOT NULL
	)`,
	`INSERT INTO projects_v4 (id, name, parent_id, created_at)
		SELECT id, name, parent_id, created_at FROM projects`,
	`DROP TABLE projects`,
	`ALTER TABLE projects_v4 RENAME TO projects`,
	`CREATE UNIQUE INDEX idx_projects_toplevel_name ON projects(name) WHERE parent_id IS NULL`,
	`CREATE UNIQUE INDEX idx_projects_child_name ON projects(parent_id, name) WHERE parent_id IS NOT NULL`,
}

// fullTextSearchV3 adds the full-text indexes searched by the words and
// phrase search modes: documents_fts over documents.content and entries_fts
// over entries.body, both external-content FTS5 tables keyed by the source
// row id and tokenized with "unicode61 remove_diacritics 2" (case and Latin
// diacritics ignored). Triggers keep each index in sync with every insert,
// delete and change of the indexed column, including deletes cascaded from
// a project; a change of entries.archived alone does not touch the index,
// since searches filter archived entries themselves. The two rebuild
// statements index the rows that already exist. Every statement can run
// again on a vault that already has them.
var fullTextSearchV3 = []string{
	`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
		content, content='documents', content_rowid='id',
		tokenize='unicode61 remove_diacritics 2')`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
		body, content='entries', content_rowid='id',
		tokenize='unicode61 remove_diacritics 2')`,

	`CREATE TRIGGER IF NOT EXISTS documents_fts_insert AFTER INSERT ON documents BEGIN
		INSERT INTO documents_fts(rowid, content) VALUES (new.id, new.content);
	END`,
	`CREATE TRIGGER IF NOT EXISTS documents_fts_delete AFTER DELETE ON documents BEGIN
		INSERT INTO documents_fts(documents_fts, rowid, content) VALUES ('delete', old.id, old.content);
	END`,
	`CREATE TRIGGER IF NOT EXISTS documents_fts_update AFTER UPDATE OF content ON documents BEGIN
		INSERT INTO documents_fts(documents_fts, rowid, content) VALUES ('delete', old.id, old.content);
		INSERT INTO documents_fts(rowid, content) VALUES (new.id, new.content);
	END`,

	`CREATE TRIGGER IF NOT EXISTS entries_fts_insert AFTER INSERT ON entries BEGIN
		INSERT INTO entries_fts(rowid, body) VALUES (new.id, new.body);
	END`,
	`CREATE TRIGGER IF NOT EXISTS entries_fts_delete AFTER DELETE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, body) VALUES ('delete', old.id, old.body);
	END`,
	`CREATE TRIGGER IF NOT EXISTS entries_fts_update AFTER UPDATE OF body ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, body) VALUES ('delete', old.id, old.body);
		INSERT INTO entries_fts(rowid, body) VALUES (new.id, new.body);
	END`,

	`INSERT INTO documents_fts(documents_fts) VALUES ('rebuild')`,
	`INSERT INTO entries_fts(entries_fts) VALUES ('rebuild')`,
}

// lowercaseNamesV2 lower-cases project, subproject and document kind names
// and entries kinds (names are ASCII slugs, so SQLite's lower() is exact). A
// project or document name whose lower-cased form is already taken in the
// same scope is left unchanged, so no two projects or documents are merged.
// When several spellings of one name need lower-casing and none is lower
// case yet (for example "Foo" and "FOO"), only the one with the smallest id
// is renamed: SQLite checks the guard for every row before updating any,
// so renaming them all would break the unique indexes and fail the whole
// migration. Entries of a kind are merged with those of its lower-cased
// spelling, keeping every entry in one log.
//
// A vault that already recorded version 2 never runs these statements
// again.
var lowercaseNamesV2 = []string{
	`UPDATE projects SET name = lower(name)
		WHERE name <> lower(name)
		AND NOT EXISTS (SELECT 1 FROM projects other
			WHERE other.id <> projects.id
			  AND other.parent_id IS projects.parent_id
			  AND lower(other.name) = lower(projects.name)
			  AND (other.name = lower(other.name) OR other.id < projects.id))`,
	`UPDATE documents SET kind = lower(kind)
		WHERE kind <> lower(kind)
		AND NOT EXISTS (SELECT 1 FROM documents other
			WHERE other.id <> documents.id
			  AND other.project_id = documents.project_id
			  AND lower(other.kind) = lower(documents.kind)
			  AND (other.kind = lower(other.kind) OR other.id < documents.id))`,
	`UPDATE entries SET kind = lower(kind) WHERE kind <> lower(kind)`,
}

// migrationsTableDDL creates the schema_migrations tracking table. It runs
// before the version check, which queries that table, so it must exist
// whether or not any migration is pending.
const migrationsTableDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version       INTEGER PRIMARY KEY,
	applied_at    TEXT NOT NULL
)`

// schemaV1 is the initial vault schema, split into individual statements so
// it can be applied without multi-statement Exec support.
//
// Project names are unique among siblings, enforced by two partial unique
// indexes rather than a single table-level UNIQUE(parent_id, name)
// constraint: in SQL, NULL is never equal to NULL for uniqueness purposes,
// so that constraint alone would allow two top-level projects
// (parent_id IS NULL) with the same name. One index enforces uniqueness
// among top-level projects, the other among the children of a given
// parent.
var schemaV1 = []string{
	`CREATE TABLE IF NOT EXISTS projects (
		id            INTEGER PRIMARY KEY,
		name          TEXT NOT NULL,
		parent_id     INTEGER NULL REFERENCES projects(id) ON DELETE CASCADE,
		created_at    TEXT NOT NULL
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_toplevel_name
		ON projects(name) WHERE parent_id IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_child_name
		ON projects(parent_id, name) WHERE parent_id IS NOT NULL`,

	`CREATE TABLE IF NOT EXISTS documents (
		id            INTEGER PRIMARY KEY,
		project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		kind          TEXT NOT NULL,
		content       TEXT NOT NULL DEFAULT '',
		updated_at    TEXT NOT NULL,
		UNIQUE(project_id, kind)
	)`,

	`CREATE TABLE IF NOT EXISTS entries (
		id            INTEGER PRIMARY KEY,
		project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		kind          TEXT NOT NULL,
		entry_date    TEXT NULL,
		body          TEXT NOT NULL,
		archived      INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL,
		position      INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_entries_lookup
		ON entries(project_id, kind, archived, entry_date)`,
	// Lets AppendEntry find MAX(position) for a (project, kind) without
	// scanning every entry of that kind.
	`CREATE INDEX IF NOT EXISTS idx_entries_position
		ON entries(project_id, kind, position)`,
}

// migrate applies every pending entry of migrations in version order inside a
// single transaction, recording each in schema_migrations. It is safe to call
// on every Open: versions already recorded are skipped. It returns an error
// when the vault's schema version is newer than the latest known version, or
// when any statement fails, in which case nothing is applied.
//
// The migrations run on one connection with foreign keys off, which SQLite
// only allows outside a transaction, so a table can be rebuilt without its
// drop cascading to the rows that reference it. Before committing, every
// reference is checked with foreign_key_check; foreign keys are turned back
// on before the connection is returned to the pool.
func migrate(ctx context.Context, db *sql.DB) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for migration: %w", err)
	}
	defer func() {
		if _, ferr := conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA foreign_keys = ON`); ferr != nil && err == nil {
			err = fmt.Errorf("enable foreign keys after migration: %w", ferr)
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	if _, err := tx.ExecContext(ctx, migrationsTableDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current)
	if err != nil {
		return fmt.Errorf("check migration state: %w", err)
	}

	// A vault migrated by a newer sync82 may hold data this binary does not
	// understand, so it is refused rather than read or written.
	if latest := migrations[len(migrations)-1].version; current > latest {
		return fmt.Errorf("vault schema version %d is newer than this sync82 supports (%d): %w", current, latest, ErrSchemaTooNew)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		for _, stmt := range m.stmts {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply schema migration %d: %w", m.version, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, m.version, now(),
		); err != nil {
			return fmt.Errorf("record schema migration %d: %w", m.version, err)
		}
	}

	if err := checkForeignKeys(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// checkForeignKeys returns an error when any row of tx's database refers to
// a row that does not exist, as reported by PRAGMA foreign_key_check.
func checkForeignKeys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("check foreign keys: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowid sql.NullInt64
		var parent string
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("check foreign keys: %w", err)
		}
		return fmt.Errorf("check foreign keys: row %d of %s refers to a missing %s row", rowid.Int64, table, parent)
	}
	return rows.Err()
}
