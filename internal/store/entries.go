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

// Entry is a single row of an append-only log. ID is the row's identifier,
// unique within the vault and stable for the life of the row (rewriting the
// whole kind, or importing it, creates new rows). EntryDate is empty when
// the entry is undated.
type Entry struct {
	ID        int64
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
// ones, or the archived ones when Archived is set). With Append, Sections
// are added as new non-archived entries after the existing ones instead,
// as AppendEntry adds one. Document takes precedence when non-nil.
type KindWrite struct {
	Kind     string
	Document *string
	Sections []EntrySection
	Archived bool
	Append   bool
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
		switch {
		case w.Document != nil:
			err = writeDocument(ctx, tx, projectID, kind, *w.Document)
		case w.Append:
			err = appendEntriesIn(ctx, tx, projectID, kind, w.Sections)
		default:
			err = replaceEntriesIn(ctx, tx, projectID, kind, w.Sections, w.Archived)
		}
		if err != nil {
			return fmt.Errorf("write %s/%s: %w", label(project, subproject), kind, err)
		}
	}
	return tx.Commit()
}

// appendEntriesIn adds sections as new non-archived entries of
// (projectID, kind), each at the next insertion position, as AppendEntry
// does. kind must be normalized; db should be a transaction when several
// writes must apply together. It returns the first database error.
func appendEntriesIn(ctx context.Context, db execer, projectID int64, kind string, sections []EntrySection) error {
	for _, section := range sections {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position)
			VALUES (?, ?, ?, ?, 0, ?,
				(SELECT COALESCE(MAX(position), -1) + 1 FROM entries WHERE project_id = ? AND kind = ?))`,
			projectID, kind, nullableString(section.Date), section.Body, now(), projectID, kind,
		); err != nil {
			return err
		}
	}
	return nil
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

	query := `SELECT id, entry_date, body, archived, created_at FROM entries WHERE project_id = ? AND kind = ?`
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
		if err := rows.Scan(&e.ID, &date, &e.Body, &archived, &e.CreatedAt); err != nil {
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

	query := `SELECT id, entry_date, body, archived, created_at FROM entries WHERE project_id = ? AND kind = ? AND archived = 0`
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
		if err := rows.Scan(&e.ID, &date, &e.Body, &archived, &e.CreatedAt); err != nil {
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

// ReadEntry returns the entry with the given id, archived or not, when it
// belongs to (project, kind). It returns an error wrapping ErrNotFound when
// the project or subproject does not exist or no such entry belongs to it,
// or a database error.
func (s *Store) ReadEntry(ctx context.Context, project, subproject, kind string, id int64) (Entry, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return Entry{}, err
	}
	return readEntryIn(ctx, s.db, projectID, kind, id, label(project, subproject))
}

// readEntryIn reads the entry with the given id from db when it belongs to
// (projectID, kind). where names the project in error messages. It returns
// an error wrapping ErrNotFound when there is no such entry, or a database
// error.
func readEntryIn(ctx context.Context, db execer, projectID int64, kind string, id int64, where string) (Entry, error) {
	e := Entry{ID: id}
	var date sql.NullString
	var archived int
	err := db.QueryRowContext(ctx,
		`SELECT entry_date, body, archived, created_at FROM entries WHERE id = ? AND project_id = ? AND kind = ?`,
		id, projectID, kind,
	).Scan(&date, &e.Body, &archived, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, fmt.Errorf("entry not found: %s/%s entry %d: %w", where, kind, id, ErrNotFound)
	}
	if err != nil {
		return Entry{}, fmt.Errorf("read entry %s/%s entry %d: %w", where, kind, id, err)
	}
	e.EntryDate = date.String
	e.Archived = archived != 0
	return e, nil
}

// UpdateEntry replaces the body of the entry with the given id, keeping its
// position and archived flag. When entryDate is non-nil it also sets the
// entry's date ("" for undated); otherwise the date is kept. It returns an
// error wrapping ErrNotFound when the project or subproject does not exist
// or no such entry belongs to (project, kind), or a database error.
func (s *Store) UpdateEntry(ctx context.Context, project, subproject, kind string, id int64, body string, entryDate *string) error {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return err
	}
	query := `UPDATE entries SET body = ? WHERE id = ? AND project_id = ? AND kind = ?`
	args := []any{body, id, projectID, kind}
	if entryDate != nil {
		query = `UPDATE entries SET body = ?, entry_date = ? WHERE id = ? AND project_id = ? AND kind = ?`
		args = []any{body, nullableString(*entryDate), id, projectID, kind}
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update entry %s/%s entry %d: %w", label(project, subproject), kind, id, err)
	}
	return requireOneRow(res, label(project, subproject), kind, id)
}

// DeleteEntry deletes the entry with the given id, archived or not. It
// returns an error wrapping ErrNotFound when the project or subproject does
// not exist or no such entry belongs to (project, kind), or a database
// error.
func (s *Store) DeleteEntry(ctx context.Context, project, subproject, kind string, id int64) error {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM entries WHERE id = ? AND project_id = ? AND kind = ?`, id, projectID, kind)
	if err != nil {
		return fmt.Errorf("delete entry %s/%s entry %d: %w", label(project, subproject), kind, id, err)
	}
	return requireOneRow(res, label(project, subproject), kind, id)
}

// SupersedeEntry appends a new, non-archived entry with entryDate (empty for
// undated) and body, as AppendEntry does, and replaces the body of the
// entry with the given id by mark(oldBody, newID), all in one transaction.
// It returns the new entry's id. It returns an error wrapping ErrNotFound
// when the project or subproject does not exist or no such entry belongs
// to (project, kind), or a database error.
func (s *Store) SupersedeEntry(ctx context.Context, project, subproject, kind string, id int64, entryDate, body string, mark func(oldBody string, newID int64) string) (int64, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return 0, err
	}
	where := label(project, subproject)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("supersede entry %s/%s entry %d: %w", where, kind, id, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	old, err := readEntryIn(ctx, tx, projectID, kind, id, where)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position)
		VALUES (?, ?, ?, ?, 0, ?,
			(SELECT COALESCE(MAX(position), -1) + 1 FROM entries WHERE project_id = ? AND kind = ?))`,
		projectID, kind, nullableString(entryDate), body, now(), projectID, kind)
	if err != nil {
		return 0, fmt.Errorf("supersede entry %s/%s entry %d: %w", where, kind, id, err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("supersede entry %s/%s entry %d: %w", where, kind, id, err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE entries SET body = ? WHERE id = ?`, mark(old.Body, newID), id,
	); err != nil {
		return 0, fmt.Errorf("supersede entry %s/%s entry %d: %w", where, kind, id, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("supersede entry %s/%s entry %d: %w", where, kind, id, err)
	}
	return newID, nil
}

// requireOneRow returns an error wrapping ErrNotFound, naming entry id of
// kind in where, when res affected no row, or the error of reading the
// affected row count.
func requireOneRow(res sql.Result, where, kind string, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("entry %s/%s entry %d: %w", where, kind, id, err)
	}
	if n == 0 {
		return fmt.Errorf("entry not found: %s/%s entry %d: %w", where, kind, id, ErrNotFound)
	}
	return nil
}

// EntryStats summarizes the non-archived entries of a kind: Dated counts
// the dated ones (those ReadEntriesSince filters with since and
// maxEntries), Undated the undated ones other than a preamble, and
// LatestDate is the newest entry date ("" when none is dated).
type EntryStats struct {
	Dated      int
	Undated    int
	LatestDate string
}

// EntryStats returns the EntryStats of (project, kind), all zero for a kind
// with no entries. It returns an error wrapping ErrNotFound when the project
// or subproject does not exist, or a database error.
func (s *Store) EntryStats(ctx context.Context, project, subproject, kind string) (EntryStats, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return EntryStats{}, err
	}
	var st EntryStats
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(entry_date),
		        COUNT(*) FILTER (WHERE entry_date IS NULL AND position >= 0),
		        COALESCE(MAX(entry_date), '')
		 FROM entries WHERE project_id = ? AND kind = ? AND archived = 0`,
		projectID, kind,
	).Scan(&st.Dated, &st.Undated, &st.LatestDate)
	if err != nil {
		return EntryStats{}, fmt.Errorf("entry stats %s/%s: %w", label(project, subproject), kind, err)
	}
	return st, nil
}

// ArchiveResult summarizes the outcome of an ArchiveEntries call: Archived is
// the number of entries archived by the call, OldestDate and NewestDate the
// range of their dates ("" when none was archived), Kept the number of
// non-archived entries remaining afterwards (a summary entry included), and
// NoDate how many of those are undated. SummaryID is the id of the summary
// entry, or 0 when none was added.
type ArchiveResult struct {
	Archived   int
	OldestDate string
	NewestDate string
	Kept       int
	NoDate     int
	SummaryID  int64
}

// ArchiveSummary builds the summary entry ArchiveEntries adds after
// archiving: given the archived range (Archived, OldestDate and
// NewestDate set), it returns the entry's date ("" for undated) and body.
// An error aborts the whole call.
type ArchiveSummary func(archived ArchiveResult) (entryDate, body string, err error)

// ArchiveEntries marks every non-archived, dated entry older than cutoff
// (format "YYYY-MM-DD") as archived. Undated entries are never archived.
// When summary is not nil and at least one entry is archived, the entry it
// builds is appended as a new, non-archived entry. Everything happens in one
// transaction, so a failure leaves the entries unchanged. It returns the
// counts as an ArchiveResult. It returns an error wrapping ErrNotFound when
// the project or subproject does not exist, summary's error, or a database
// error.
func (s *Store) ArchiveEntries(ctx context.Context, project, subproject, kind, cutoff string, summary ArchiveSummary) (ArchiveResult, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return ArchiveResult{}, err
	}
	where := label(project, subproject)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: %w", where, kind, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	var result ArchiveResult
	var oldest, newest sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*), MIN(entry_date), MAX(entry_date) FROM entries
		 WHERE project_id = ? AND kind = ? AND archived = 0
		   AND entry_date IS NOT NULL AND entry_date < ?`,
		projectID, kind, cutoff,
	).Scan(&result.Archived, &oldest, &newest)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: %w", where, kind, err)
	}
	result.OldestDate, result.NewestDate = oldest.String, newest.String

	if _, err := tx.ExecContext(ctx,
		`UPDATE entries SET archived = 1
		 WHERE project_id = ? AND kind = ? AND archived = 0
		   AND entry_date IS NOT NULL AND entry_date < ?`,
		projectID, kind, cutoff,
	); err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: %w", where, kind, err)
	}

	if summary != nil && result.Archived > 0 {
		date, body, err := summary(result)
		if err != nil {
			return ArchiveResult{}, err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position)
			VALUES (?, ?, ?, ?, 0, ?,
				(SELECT COALESCE(MAX(position), -1) + 1 FROM entries WHERE project_id = ? AND kind = ?))`,
			projectID, kind, nullableString(date), body, now(), projectID, kind)
		if err != nil {
			return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: add summary: %w", where, kind, err)
		}
		if result.SummaryID, err = res.LastInsertId(); err != nil {
			return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: add summary: %w", where, kind, err)
		}
	}

	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE entry_date IS NULL)
		 FROM entries WHERE project_id = ? AND kind = ? AND archived = 0`,
		projectID, kind,
	).Scan(&result.Kept, &result.NoDate)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: count remaining: %w", where, kind, err)
	}
	if err := tx.Commit(); err != nil {
		return ArchiveResult{}, fmt.Errorf("archive entries %s/%s: %w", where, kind, err)
	}
	return result, nil
}

// ListArchivable returns the entries ArchiveEntries would archive with the
// same cutoff: non-archived, dated entries older than cutoff, in reading
// order. It returns an error wrapping ErrNotFound when the project or
// subproject does not exist, or a database error.
func (s *Store) ListArchivable(ctx context.Context, project, subproject, kind, cutoff string) ([]Entry, error) {
	project, subproject, kind = normalizeName(project), normalizeName(subproject), normalizeName(kind)
	projectID, err := s.resolveProjectID(ctx, project, subproject)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, entry_date, body, archived, created_at FROM entries
		 WHERE project_id = ? AND kind = ? AND archived = 0
		   AND entry_date IS NOT NULL AND entry_date < ?`+entryOrder,
		projectID, kind, cutoff)
	if err != nil {
		return nil, fmt.Errorf("list archivable %s/%s: %w", label(project, subproject), kind, err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var date sql.NullString
		var archived int
		if err := rows.Scan(&e.ID, &date, &e.Body, &archived, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry row: %w", err)
		}
		e.EntryDate = date.String
		e.Archived = archived != 0
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list archivable %s/%s: %w", label(project, subproject), kind, err)
	}
	return out, nil
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
