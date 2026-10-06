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
	"strings"
)

// ReadContent returns the readable content of a kind, from whichever table
// holds it. A document takes precedence; otherwise the bodies of the
// non-archived entries are joined with a blank line between them. ok is false
// when the kind has neither a document nor non-archived entries. It returns
// an error wrapping ErrNotFound when the project or subproject does not
// exist, or a database error.
func (s *Store) ReadContent(ctx context.Context, project, subproject, kind string) (content string, ok bool, err error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	docContent, found, err := s.ReadDocument(ctx, project, subproject, kind)
	if err != nil {
		return "", false, err
	}
	if found {
		return docContent, true, nil
	}

	entries, err := s.ReadEntries(ctx, project, subproject, kind, false)
	if err != nil {
		return "", false, err
	}
	if len(entries) == 0 {
		return "", false, nil
	}

	bodies := make([]string, len(entries))
	for i, e := range entries {
		bodies[i] = e.Body
	}
	return strings.Join(bodies, "\n\n"), true, nil
}

// KindExists reports whether a kind has any data for the project: a documents
// row (even with empty content) or at least one entries row, archived or not.
// Errors are those of ReadDocument and ReadEntries.
func (s *Store) KindExists(ctx context.Context, project, subproject, kind string) (bool, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	_, found, err := s.ReadDocument(ctx, project, subproject, kind)
	if err != nil {
		return false, err
	}
	if found {
		return true, nil
	}

	entries, err := s.ReadEntries(ctx, project, subproject, kind, true)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// KindStorage names the table a kind's data lives in for a project.
type KindStorage string

const (
	// KindStorageNone means the kind has no data yet.
	KindStorageNone KindStorage = ""
	// KindStorageDocument means the kind is a single overwrite-style
	// documents row.
	KindStorageDocument KindStorage = "document"
	// KindStorageEntries means the kind is a log of entries rows
	// (archived or not).
	KindStorageEntries KindStorage = "entries"
)

// KindMode reports which table holds a kind's data for the project, or
// KindStorageNone when it has none. A kind present in both tables reports
// KindStorageDocument, the table ReadContent reads first. It returns an
// error wrapping ErrNotFound when the project or subproject does not exist,
// or a database error.
func (s *Store) KindMode(ctx context.Context, project, subproject, kind string) (KindStorage, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return KindStorageNone, err
	}
	var inDocuments, inEntries bool
	err = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM documents WHERE project_id = ? AND kind = ?),
		       EXISTS(SELECT 1 FROM entries WHERE project_id = ? AND kind = ?)`,
		projectID, kind, projectID, kind,
	).Scan(&inDocuments, &inEntries)
	if err != nil {
		return KindStorageNone, fmt.Errorf("kind mode %s/%s: %w", label(project, subproject), kind, err)
	}
	switch {
	case inDocuments:
		return KindStorageDocument, nil
	case inEntries:
		return KindStorageEntries, nil
	default:
		return KindStorageNone, nil
	}
}

// KindMetadata describes the size and freshness of a kind's content.
// SizeBytes is the content length in bytes, EstimatedTokens a rough estimate
// (SizeBytes / 4), and LastModified the date of the last change.
type KindMetadata struct {
	SizeBytes       int
	EstimatedTokens int
	LastModified    string // "YYYY-MM-DD"
}

// Metadata computes KindMetadata for a document or entries kind. For a
// document it uses the content size and update date; for entries it sums the
// sizes of the non-archived bodies and uses the latest creation date of the
// non-archived entries, or of all entries when every one is archived (size
// 0), so a kind KindExists reports always has metadata. Sizes use
// length(CAST(x AS BLOB)) because SQLite's length() counts characters, not
// bytes, for TEXT values. It returns an error wrapping ErrNotFound when the
// project does not exist or the kind has no document and no entries, or a
// database error.
func (s *Store) Metadata(ctx context.Context, project, subproject, kind string) (KindMetadata, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return KindMetadata{}, err
	}

	var size int
	var updatedAt string
	err = s.db.QueryRowContext(ctx,
		`SELECT length(CAST(content AS BLOB)), updated_at FROM documents WHERE project_id = ? AND kind = ?`,
		projectID, kind,
	).Scan(&size, &updatedAt)
	if err == nil {
		return KindMetadata{SizeBytes: size, EstimatedTokens: size / 4, LastModified: dateOnly(updatedAt)}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return KindMetadata{}, fmt.Errorf("kind metadata %s/%s: %w", label(project, subproject), kind, err)
	}

	var totalSize int
	var lastCreated sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(CASE WHEN archived = 0 THEN length(CAST(body AS BLOB)) ELSE 0 END), 0),
		        COALESCE(MAX(CASE WHEN archived = 0 THEN created_at END), MAX(created_at))
		 FROM entries WHERE project_id = ? AND kind = ?`,
		projectID, kind,
	).Scan(&totalSize, &lastCreated)
	if err != nil {
		return KindMetadata{}, fmt.Errorf("kind metadata %s/%s: %w", label(project, subproject), kind, err)
	}
	if !lastCreated.Valid {
		return KindMetadata{}, fmt.Errorf("file not found: %s/%s: %w", label(project, subproject), kind, ErrNotFound)
	}
	return KindMetadata{SizeBytes: totalSize, EstimatedTokens: totalSize / 4, LastModified: dateOnly(lastCreated.String)}, nil
}

// dateOnly returns the leading "YYYY-MM-DD" part of an RFC 3339 timestamp, or
// the input unchanged when it is shorter than ten bytes.
func dateOnly(rfc3339 string) string {
	if len(rfc3339) >= 10 {
		return rfc3339[:10]
	}
	return rfc3339
}

// ListKinds returns every kind known for the project, the union of document
// kinds and distinct entries kinds, sorted by name. A kind whose entries are
// all archived is included only when includeArchived is true. It returns an
// error wrapping ErrNotFound when the project or subproject does not exist,
// or a database error.
func (s *Store) ListKinds(ctx context.Context, project, subproject string, includeArchived bool) ([]string, error) {
	project, subproject = normalizeName(project), normalizeName(subproject)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT kind FROM documents WHERE project_id = ?
		UNION
		SELECT kind FROM entries WHERE project_id = ?`
	if !includeArchived {
		query += ` AND archived = 0`
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY kind`, projectID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list kinds %s: %w", label(project, subproject), err)
	}
	defer rows.Close()

	var kinds []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan kind row: %w", err)
		}
		kinds = append(kinds, k)
	}
	return kinds, rows.Err()
}
