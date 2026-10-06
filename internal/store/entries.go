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

// EntrySection is a single dated or undated chunk of append-only content,
// such as one "## YYYY-MM-DD" section of a log. The store persists
// already-parsed sections and does not split raw text itself.
type EntrySection struct {
	Date string // "YYYY-MM-DD", empty for undated content
	Body string
	// Preamble marks undated content that precedes the first dated section
	// (such as a title). It is stored with position preamblePosition so it
	// reads back first instead of after every dated entry.
	Preamble bool
}

// preamblePosition is the position of a preamble entry. Every other entry
// has a position >= 0, so a negative position identifies a preamble.
const preamblePosition = -1

// entryOrder orders entries the way they read: preamble first, then dated
// entries by date, then other undated entries, ties broken by insertion
// position.
const entryOrder = ` ORDER BY (entry_date IS NULL AND position >= 0), entry_date, position`

// Entry is a single row of an append-only log. EntryDate is empty when the
// entry is undated.
type Entry struct {
	EntryDate string // "", if undated
	Body      string
	Archived  bool
	CreatedAt string
}

// AppendEntry inserts one new, non-archived entry with the given date (empty
// for undated) and body at the next insertion position for its (project,
// kind) pair. The position is computed by a subquery in the same INSERT
// statement, so concurrent appends cannot read the same position. It
// returns an error wrapping ErrNotFound when the project or subproject does
// not exist, or a database error.
func (s *Store) AppendEntry(ctx context.Context, project, subproject, kind, entryDate, body string) error {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return err
	}

	date := nullableString(entryDate)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position)
		VALUES (?, ?, ?, ?, 0, ?,
			(SELECT COALESCE(MAX(position), -1) + 1 FROM entries WHERE project_id = ? AND kind = ?))`,
		projectID, kind, date, body, now(), projectID, kind)
	if err != nil {
		return fmt.Errorf("append entry %s/%s: %w", label(project, subproject), kind, err)
	}
	return nil
}

// ReplaceAllEntries atomically replaces every non-archived entries row for
// (project, kind) with the given sections, in order. Archived rows are left
// untouched. The result reads the same as appending the sections one by one
// with AppendEntry. It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) ReplaceAllEntries(ctx context.Context, project, subproject, kind string, sections []EntrySection) error {
	return s.replaceEntries(ctx, project, subproject, kind, sections, false)
}

// ReplaceArchivedEntries atomically replaces every archived entries row for
// (project, kind) with the given sections, stored as archived. Non-archived
// rows are left untouched. It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) ReplaceArchivedEntries(ctx context.Context, project, subproject, kind string, sections []EntrySection) error {
	return s.replaceEntries(ctx, project, subproject, kind, sections, true)
}

// replaceEntries deletes the (project, kind) entries rows whose archived flag
// equals archived and inserts sections in their place with that same flag,
// positioned after every remaining row, all in one transaction. It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) replaceEntries(ctx context.Context, project, subproject, kind string, sections []EntrySection, archived bool) error {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace entries %s/%s: %w", label(project, subproject), kind, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	if err := replaceEntriesIn(ctx, tx, projectID, kind, sections, archived); err != nil {
		return fmt.Errorf("replace entries %s/%s: %w", label(project, subproject), kind, err)
	}
	return tx.Commit()
}

// replaceEntriesIn deletes the (projectID, kind) entries rows whose archived
// flag equals archived and inserts sections in their place with that flag,
// positioned after every remaining row. A Preamble section gets
// preamblePosition. kind must be normalized. db should be a transaction so
// the delete and the inserts apply together. It returns the first database
// error.
func replaceEntriesIn(ctx context.Context, db execer, projectID int64, kind string, sections []EntrySection, archived bool) error {
	flag := 0
	if archived {
		flag = 1
	}
	if _, err := db.ExecContext(ctx,
		`DELETE FROM entries WHERE project_id = ? AND kind = ? AND archived = ?`, projectID, kind, flag,
	); err != nil {
		return err
	}

	var base int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) + 1 FROM entries WHERE project_id = ? AND kind = ?`, projectID, kind,
	).Scan(&base); err != nil {
		return err
	}

	createdAt := now()
	for i, section := range sections {
		position := base + i
		if section.Preamble {
			position = preamblePosition
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			projectID, kind, nullableString(section.Date), section.Body, flag, createdAt, position,
		); err != nil {
			return err
		}
	}
	return nil
}

// KindWrite is one kind's new content in a WriteKinds batch: Document for an
// overwrite-style kind, or Sections replacing its entries (the non-archived
// ones, or the archived ones when Archived is set). Document takes
// precedence when non-nil.
type KindWrite struct {
	Kind     string
	Document *string
	Sections []EntrySection
	Archived bool
}

// WriteKinds applies every write to (project, subproject) in one transaction:
// either all of them take effect or none does. Kind names are normalized.
// It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) WriteKinds(ctx context.Context, project, subproject string, writes []KindWrite) error {
	project, subproject = normalizeName(project), normalizeName(subproject)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("write %s: %w", label(project, subproject), err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	for _, w := range writes {
		kind := normalizeName(w.Kind)
		if w.Document != nil {
			err = writeDocument(ctx, tx, projectID, kind, *w.Document)
		} else {
			err = replaceEntriesIn(ctx, tx, projectID, kind, w.Sections, w.Archived)
		}
		if err != nil {
			return fmt.Errorf("write %s/%s: %w", label(project, subproject), kind, err)
		}
	}
	return tx.Commit()
}

// ReadEntries returns entries for (project, kind) in reading order: a
// preamble first, then dated entries by date, then other undated entries,
// ties broken by insertion position. Archived entries are excluded unless
// includeArchived is true. A kind with no entries yields an empty slice and
// no error. It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) ReadEntries(ctx context.Context, project, subproject, kind string, includeArchived bool) ([]Entry, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return nil, err
	}

	query := `SELECT entry_date, body, archived, created_at FROM entries WHERE project_id = ? AND kind = ?`
	if !includeArchived {
		query += ` AND archived = 0`
	}
	query += entryOrder

	rows, err := s.db.QueryContext(ctx, query, projectID, kind)
	if err != nil {
		return nil, fmt.Errorf("read entries %s/%s: %w", label(project, subproject), kind, err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		var date sql.NullString
		var archived int
		if err := rows.Scan(&date, &e.Body, &archived, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry row: %w", err)
		}
		e.EntryDate = date.String
		e.Archived = archived != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReadEntriesSince returns non-archived entries for (project, kind), filtered
// in SQL so the full history is never loaded just to return recent entries.
//
// Both filters apply to dated entries only; undated entries (a preamble,
// undated notes) are always kept, as in ArchiveEntries. since, if non-empty
// (format "YYYY-MM-DD"), keeps only dated entries on or after it. maxEntries,
// if > 0, then keeps only the most recent maxEntries of those dated entries.
// The result is in the same reading order ReadEntries uses. It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) ReadEntriesSince(ctx context.Context, project, subproject, kind, since string, maxEntries int) ([]Entry, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return nil, err
	}

	dated := `entry_date IS NOT NULL`
	var datedArgs []any
	if since != "" {
		dated += ` AND entry_date >= ?`
		datedArgs = append(datedArgs, since)
	}

	query := `SELECT entry_date, body, archived, created_at FROM entries WHERE project_id = ? AND kind = ? AND archived = 0`
	args := []any{projectID, kind}
	if maxEntries > 0 {
		query += ` AND (entry_date IS NULL OR id IN (
			SELECT id FROM entries WHERE project_id = ? AND kind = ? AND archived = 0 AND ` + dated + `
			ORDER BY entry_date DESC, position DESC LIMIT ?))`
		args = append(args, projectID, kind)
		args = append(args, datedArgs...)
		args = append(args, maxEntries)
	} else {
		query += ` AND (entry_date IS NULL OR (` + dated + `))`
		args = append(args, datedArgs...)
	}
	query += entryOrder

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read entries since %s/%s: %w", label(project, subproject), kind, err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		var date sql.NullString
		var archived int
		if err := rows.Scan(&date, &e.Body, &archived, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry row: %w", err)
		}
		e.EntryDate = date.String
		e.Archived = archived != 0
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read entries since %s/%s: %w", label(project, subproject), kind, err)
	}
	return out, nil
}

// ArchiveResult summarizes the outcome of an ArchiveEntries call: Archived is
// the number of entries archived by the call, Kept the number of non-archived
// entries remaining afterwards, and NoDate how many of those are undated.
type ArchiveResult struct {
	Archived int
	Kept     int
	NoDate   int
}

// ArchiveEntries marks every non-archived, dated entry older than cutoff
// (format "YYYY-MM-DD") as archived. Undated entries are never archived. It
// returns the counts as an ArchiveResult. It returns an error wrapping ErrNotFound when the project or subproject does not exist, or a database error.
func (s *Store) ArchiveEntries(ctx context.Context, project, subproject, kind, cutoff string) (ArchiveResult, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return ArchiveResult{}, err
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE entries SET archived = 1
		 WHERE project_id = ? AND kind = ? AND archived = 0
		   AND entry_date IS NOT NULL AND entry_date < ?`,
		projectID, kind, cutoff,
	)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: %w", label(project, subproject), kind, err)
	}
	archived, err := res.RowsAffected()
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: %w", label(project, subproject), kind, err)
	}

	var kept, noDate int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE entry_date IS NULL)
		 FROM entries WHERE project_id = ? AND kind = ? AND archived = 0`,
		projectID, kind,
	).Scan(&kept, &noDate)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: count remaining: %w", label(project, subproject), kind, err)
	}

	return ArchiveResult{Archived: int(archived), Kept: kept, NoDate: noDate}, nil
}

// DeleteKind deletes every row (document and entries, archived or not) of a
// kind in one transaction, so a kind is never left half-deleted. The store
// does not protect any kind from deletion. It returns an error wrapping
// ErrNotFound when the project or subproject does not exist or the kind has
// no rows, or a database error.
func (s *Store) DeleteKind(ctx context.Context, project, subproject, kind string) error {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete %s/%s: %w", label(project, subproject), kind, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	docRes, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE project_id = ? AND kind = ?`, projectID, kind)
	if err != nil {
		return fmt.Errorf("delete %s/%s: %w", label(project, subproject), kind, err)
	}
	docsDeleted, err := docRes.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete %s/%s: %w", label(project, subproject), kind, err)
	}

	entryRes, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE project_id = ? AND kind = ?`, projectID, kind)
	if err != nil {
		return fmt.Errorf("delete %s/%s: %w", label(project, subproject), kind, err)
	}
	entriesDeleted, err := entryRes.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete %s/%s: %w", label(project, subproject), kind, err)
	}

	if docsDeleted == 0 && entriesDeleted == 0 {
		return fmt.Errorf("file not found: %s/%s: %w", label(project, subproject), kind, ErrNotFound)
	}
	return tx.Commit()
}
