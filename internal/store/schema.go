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
}

// lowercaseNamesV2 lower-cases project, subproject and document kind names
// and entries kinds (names are ASCII slugs, so SQLite's lower() is exact). A
// project or document name whose lower-cased form is already taken in the
// same scope is left unchanged, so no two projects or documents are merged.
// Entries of a kind are merged with those of its lower-cased spelling,
// keeping every entry in one log.
var lowercaseNamesV2 = []string{
	`UPDATE projects SET name = lower(name)
		WHERE name <> lower(name)
		AND NOT EXISTS (SELECT 1 FROM projects other
			WHERE other.id <> projects.id AND other.name = lower(projects.name) AND other.parent_id IS projects.parent_id)`,
	`UPDATE documents SET kind = lower(kind)
		WHERE kind <> lower(kind)
		AND NOT EXISTS (SELECT 1 FROM documents other
			WHERE other.project_id = documents.project_id AND other.kind = lower(documents.kind))`,
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
func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
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
		return fmt.Errorf("vault schema version %d is newer than this sync82 supports (%d); upgrade sync82", current, latest)
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

	return tx.Commit()
}
