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
	"fmt"
)

// WriteDocument sets the content of the overwrite-style document of the given
// kind for project/subproject, inserting it when it does not exist yet. The
// names are normalized. It returns an error wrapping ErrNotFound when the
// project or subproject does not exist, or a database error. The project
// is resolved and the document upserted in one transaction.
func (s *Store) WriteDocument(ctx context.Context, project, subproject, kind, content string) error {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	return s.withProjectTx(ctx, project, subproject, func(tx *sql.Tx, projectID int64) error {
		if err := writeDocument(ctx, tx, projectID, kind, content); err != nil {
			return fmt.Errorf("write document %s/%s: %w", label(project, subproject), kind, err)
		}
		return nil
	})
}

// execer is the subset of *sql.DB and *sql.Tx used by the write helpers, so
// the same helper runs standalone or inside a transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// writeDocument upserts the document identified by (projectID, kind) with
// content and the current time as its update time. kind must already be
// normalized. It returns the database error, if any.
func writeDocument(ctx context.Context, db execer, projectID int64, kind, content string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO documents (project_id, kind, content, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(project_id, kind) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		projectID, kind, content, now())
	return err
}

// ReadDocument returns the raw content of the overwrite-style document of the
// given kind for project/subproject. ok is false, with no error, when the
// document row does not exist. It returns an error wrapping ErrNotFound when
// the project or subproject does not exist, or a database error.
func (s *Store) ReadDocument(ctx context.Context, project, subproject, kind string) (content string, ok bool, err error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return "", false, err
	}
	content, ok, err = readDocumentIn(ctx, s.db, projectID, kind)
	if err != nil {
		return "", false, fmt.Errorf("read document %s/%s: %w", label(project, subproject), kind, err)
	}
	return content, ok, nil
}

// readDocumentIn reads the content of the document of (projectID, kind)
// from db; ok is false, with no error, when there is none. kind must be
// normalized. It returns the database error, if any.
func readDocumentIn(ctx context.Context, db execer, projectID int64, kind string) (content string, ok bool, err error) {
	err = db.QueryRowContext(ctx,
		`SELECT content FROM documents WHERE project_id = ? AND kind = ?`, projectID, kind,
	).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return content, true, nil
}
