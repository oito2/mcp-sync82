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
	"database/sql/driver"
	"fmt"
	"sort"
	"strings"

	"modernc.org/sqlite"
)

// lowerFunc is the name of the SQL function that lower-cases text with Go's
// Unicode-aware strings.ToLower; SQLite's own lower() and LIKE only fold
// ASCII letters.
const lowerFunc = "sync82_lower"

// init registers lowerFunc as a deterministic one-argument SQL function. It
// lower-cases string arguments and returns any other value unchanged. It
// panics if the registration fails.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction(lowerFunc, 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if s, ok := args[0].(string); ok {
			return strings.ToLower(s), nil
		}
		return args[0], nil
	})
}

// SearchScope narrows a search to a project (and optionally a specific
// subproject). An empty Project means the whole vault; a Project with an
// empty Subproject means that project plus all of its subprojects.
type SearchScope struct {
	Project    string
	Subproject string
}

// SearchResult is a single matching line within a document or entry. Project
// is the top-level project name and Subproject the subproject name (empty for
// a top-level project). For an entry, EntryDate is its date ("" if undated);
// LineNumber is the 1-based line within the document or entry body. Line is
// the matching line, and ContextBefore and ContextAfter hold the surrounding
// lines when context was requested.
type SearchResult struct {
	Project       string
	Subproject    string
	Kind          string
	EntryDate     string
	LineNumber    int
	Line          string
	ContextBefore []string
	ContextAfter  []string
}

// searchRow is one document or entry row read by a search: the owning project
// and subproject names, the kind, the entry date (empty for documents and
// undated entries) and the text content.
type searchRow struct {
	project    string
	subproject string
	kind       string
	entryDate  string
	content    string
}

// maxScannedRows and maxScannedBytes bound how much matching content a search
// reads into memory, independent of the requested page size: each of the two
// queries in fetchMatchingRows stops at maxScannedRows rows, and reading stops
// once the content read exceeds maxScannedBytes. A search that hits either
// bound reports itself as truncated.
const (
	maxScannedRows  = 5000
	maxScannedBytes = 64 << 20
)

// SearchText performs a case-insensitive substring search over every document
// and non-archived entries row in scope, matching query literally (no
// tokenization or wildcards). Case folding is Unicode-aware ("DECISÃO"
// matches "decisão"). contextLines, when positive, adds that many lines
// before and after each match.
//
// It returns the matching lines skipping the first offset, up to limit+1 of
// them: the extra one lets the caller detect that more results exist. The
// boolean reports whether the scan stopped at maxScannedRows or
// maxScannedBytes, in which case later matches are missing. Matches are
// ordered by project, subproject, kind, then row reading order (entries in
// reading order), then line number, so pagination is stable. It returns an
// error wrapping ErrNotFound when the scoped project or subproject does not
// exist, or a database error.
func (s *Store) SearchText(ctx context.Context, query string, scope SearchScope, offset, limit, contextLines int) ([]SearchResult, bool, error) {
	projectIDs, err := s.resolveScopeProjectIDs(ctx, scope)
	if err != nil {
		return nil, false, err
	}

	rows, truncated, err := s.fetchMatchingRows(ctx, query, projectIDs)
	if err != nil {
		return nil, false, err
	}

	type match struct {
		SearchResult
		row int
	}
	needle := strings.ToLower(query)
	var all []match
	for rowIndex, r := range rows {
		lines := strings.Split(r.content, "\n")
		for i, line := range lines {
			if !strings.Contains(strings.ToLower(line), needle) {
				continue
			}
			res := SearchResult{
				Project:    r.project,
				Subproject: r.subproject,
				Kind:       r.kind,
				EntryDate:  r.entryDate,
				LineNumber: i + 1,
				Line:       line,
			}
			if contextLines > 0 {
				res.ContextBefore = contextSlice(lines, i, -contextLines)
				res.ContextAfter = contextSlice(lines, i, contextLines)
			}
			all = append(all, match{res, rowIndex})
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.Subproject != b.Subproject {
			return a.Subproject < b.Subproject
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.row != b.row {
			return a.row < b.row
		}
		return a.LineNumber < b.LineNumber
	})

	if offset >= len(all) {
		return []SearchResult{}, truncated, nil
	}
	end := offset + limit + 1
	if end > len(all) {
		end = len(all)
	}
	out := make([]SearchResult, 0, end-offset)
	for _, m := range all[offset:end] {
		out = append(out, m.SearchResult)
	}
	return out, truncated, nil
}

// contextSlice returns the |delta| lines before (delta < 0) or after
// (delta > 0) index i of lines, clamped to the bounds of lines. The result
// shares memory with lines.
func contextSlice(lines []string, i, delta int) []string {
	if delta < 0 {
		start := i + delta
		if start < 0 {
			start = 0
		}
		return lines[start:i]
	}
	end := i + 1 + delta
	if end > len(lines) {
		end = len(lines)
	}
	return lines[i+1 : end]
}

// resolveScopeProjectIDs returns nil (meaning no filter, search everything)
// or the IDs of the projects in scope: the named subproject alone, or the
// project together with all its subprojects. It returns an error wrapping
// ErrNotFound when the project or subproject does not exist, or a database
// error.
func (s *Store) resolveScopeProjectIDs(ctx context.Context, scope SearchScope) ([]int64, error) {
	if scope.Project == "" {
		return nil, nil
	}

	parent, err := s.FindProjectByName(ctx, scope.Project, nil)
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, fmt.Errorf("project not found: %q: %w", scope.Project, ErrNotFound)
	}

	if scope.Subproject != "" {
		sub, err := s.FindProjectByName(ctx, scope.Subproject, &parent.ID)
		if err != nil {
			return nil, err
		}
		if sub == nil {
			return nil, fmt.Errorf("project not found: %q/%q: %w", scope.Project, scope.Subproject, ErrNotFound)
		}
		return []int64{sub.ID}, nil
	}

	ids := []int64{parent.ID}
	subs, err := s.ListSubprojects(ctx, parent.ID)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		ids = append(ids, sub.ID)
	}
	return ids, nil
}

// fetchMatchingRows runs the case-insensitive LIKE query against documents
// and then entries (archived entries excluded), optionally scoped to
// projectIDs (nil means no scope), each in a fixed order. It returns the
// matching rows, at most maxScannedRows per query and about maxScannedBytes of
// content in total. truncated reports whether either bound cut the result
// short. It returns a database error on failure.
func (s *Store) fetchMatchingRows(ctx context.Context, query string, projectIDs []int64) (rows []searchRow, truncated bool, err error) {
	like := "%" + escapeLike(strings.ToLower(query)) + "%"

	// The first two columns resolve to the top-level project name and the
	// subproject name respectively, whether the matched row belongs to a
	// subproject (p has a parent) or a top-level project (p has none).
	docClause, docScopeArgs := scopeClause("d.project_id", projectIDs)
	docSQL := `
		SELECT COALESCE(parent.name, p.name), CASE WHEN parent.id IS NOT NULL THEN p.name ELSE '' END, d.kind, '', d.content
		FROM documents d
		JOIN projects p ON p.id = d.project_id
		LEFT JOIN projects parent ON parent.id = p.parent_id
		WHERE ` + lowerFunc + `(d.content) LIKE ? ESCAPE '\'` + docClause + `
		ORDER BY d.project_id, d.kind
		LIMIT ?`
	docArgs := append([]any{like}, append(docScopeArgs, maxScannedRows+1)...)

	entryClause, entryScopeArgs := scopeClause("e.project_id", projectIDs)
	entrySQL := `
		SELECT COALESCE(parent.name, p.name), CASE WHEN parent.id IS NOT NULL THEN p.name ELSE '' END, e.kind, COALESCE(e.entry_date, ''), e.body
		FROM entries e
		JOIN projects p ON p.id = e.project_id
		LEFT JOIN projects parent ON parent.id = p.parent_id
		WHERE e.archived = 0 AND ` + lowerFunc + `(e.body) LIKE ? ESCAPE '\'` + entryClause + `
		ORDER BY e.project_id, e.kind, (e.entry_date IS NULL AND e.position >= 0), e.entry_date, e.position
		LIMIT ?`
	entryArgs := append([]any{like}, append(entryScopeArgs, maxScannedRows+1)...)

	var total int
	for _, q := range []struct {
		what string
		sql  string
		args []any
	}{{"documents", docSQL, docArgs}, {"entries", entrySQL, entryArgs}} {
		n, cut, err := s.readSearchRows(ctx, q.sql, q.args, &rows, &total)
		if err != nil {
			return nil, false, fmt.Errorf("search %s: %w", q.what, err)
		}
		if cut || n > maxScannedRows {
			truncated = true
		}
		if cut {
			break
		}
	}
	return rows, truncated, nil
}

// readSearchRows runs one search query and appends its rows to *rows, at most
// maxScannedRows of them, adding their content size to *total. It reads at
// most maxScannedRows+1 rows and returns how many it read in n. cut is true
// when it stopped early because *total exceeded maxScannedBytes (the row that
// crossed the limit is not appended). It returns a query or scan error.
func (s *Store) readSearchRows(ctx context.Context, query string, args []any, rows *[]searchRow, total *int) (n int, cut bool, err error) {
	result, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, false, err
	}
	defer result.Close()
	for result.Next() {
		n++
		if n > maxScannedRows {
			return n, false, nil
		}
		var r searchRow
		if err := result.Scan(&r.project, &r.subproject, &r.kind, &r.entryDate, &r.content); err != nil {
			return n, false, fmt.Errorf("scan search row: %w", err)
		}
		*total += len(r.content)
		if *total > maxScannedBytes {
			return n, true, nil
		}
		*rows = append(*rows, r)
	}
	return n, false, result.Err()
}

// scopeClause builds an " AND <column> IN (?,?,...)" SQL fragment and its
// arguments from ids, or ("", nil) when ids is nil (no scope filter). column
// is interpolated into the SQL and must be a trusted constant.
func scopeClause(column string, ids []int64) (string, []any) {
	if ids == nil {
		return "", nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	return fmt.Sprintf(" AND %s IN (%s)", column, strings.Join(placeholders, ",")), args
}

// escapeLike escapes the SQLite LIKE metacharacters (%, _) and the backslash
// escape character in q, so q matches as a literal substring rather than a
// wildcard pattern.
func escapeLike(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return q
}
