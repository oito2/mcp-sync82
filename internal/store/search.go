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

// SearchMode selects how SearchText matches a query.
type SearchMode string

// The search modes. SearchWords finds rows holding every word of the query
// and SearchPhrase rows holding the words in that order, both through the
// full-text indexes, ignoring case and Latin diacritics, ranked by
// relevance. SearchExact finds rows holding the query as a literal,
// case-insensitive substring, in reading order.
const (
	SearchWords  SearchMode = "words"
	SearchPhrase SearchMode = "phrase"
	SearchExact  SearchMode = "exact"
)

// SearchOptions describes a SearchText call. Mode defaults to SearchWords
// when empty. Kinds, when not empty, limits the search to those kinds
// (normalized names). Since and Until ("YYYY-MM-DD", inclusive), when set,
// limit the search to dated entries in that range; documents and undated
// entries are then left out. Offset and Limit select the page of matching
// lines; ContextLines, when positive, adds that many lines before and after
// each match.
type SearchOptions struct {
	Query        string
	Mode         SearchMode
	Scope        SearchScope
	Kinds        []string
	Since        string
	Until        string
	Offset       int
	Limit        int
	ContextLines int
}

// SearchResult is a single matching line within a document or entry. Project
// is the top-level project name and Subproject the subproject name (empty for
// a top-level project). For an entry, EntryID is its id (0 for a document)
// and EntryDate its date ("" if undated); LineNumber is the 1-based line within the document or entry body. Line is
// the matching line, and ContextBefore and ContextAfter hold the surrounding
// lines when context was requested.
type SearchResult struct {
	Project       string
	Subproject    string
	Kind          string
	EntryID       int64
	EntryDate     string
	LineNumber    int
	Line          string
	ContextBefore []string
	ContextAfter  []string
}

// searchRow is one document or entry row read by a search: the owning project
// and subproject names, the kind, the entry id (0 for documents), the entry
// date (empty for documents and undated entries) and the text content.
type searchRow struct {
	project    string
	subproject string
	kind       string
	entryID    int64
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

// SearchText searches the documents and non-archived entries in scope
// and returns their matching lines. In SearchWords and SearchPhrase modes
// the full-text indexes select the rows, ordered by relevance (bm25), then
// by project, subproject, kind and reading order; a line matches when it
// holds one of the query's words (words mode) or the whole phrase (phrase
// mode), and a row whose phrase spans several lines reports the lines
// holding any of its words. In SearchExact mode the query is a literal,
// case-insensitive substring (Unicode-aware: "DECISÃO" matches "decisão")
// and matches are ordered by project, subproject, kind, then reading order.
//
// It returns the matching lines skipping the first opts.Offset, up to
// opts.Limit+1 of them: the extra one lets the caller detect that more
// results exist. The boolean reports whether the scan stopped at
// maxScannedRows or maxScannedBytes, in which case later matches are
// missing. It returns an error wrapping ErrNotFound when the scoped
// project or subproject does not exist, an error wrapping ErrNoSearchTerms
// when a words or phrase query has no letter or number, or a database
// error.
func (s *Store) SearchText(ctx context.Context, opts SearchOptions) ([]SearchResult, bool, error) {
	if opts.Mode == "" {
		opts.Mode = SearchWords
	}
	var terms []searchTerm
	if opts.Mode != SearchExact {
		terms = parseSearchTerms(opts.Query)
		if len(terms) == 0 {
			return nil, false, fmt.Errorf("search %q: %w", opts.Query, ErrNoSearchTerms)
		}
	}
	kinds := make([]string, len(opts.Kinds))
	for i, k := range opts.Kinds {
		kinds[i] = normalizeName(k)
	}
	opts.Kinds = kinds

	projectIDs, err := s.resolveScopeProjectIDs(ctx, opts.Scope)
	if err != nil {
		return nil, false, err
	}

	var rows []searchRow
	var truncated bool
	if opts.Mode == SearchExact {
		rows, truncated, err = s.fetchMatchingRows(ctx, opts, projectIDs)
	} else {
		rows, truncated, err = s.fetchRankedRows(ctx, ftsQuery(terms, opts.Mode), opts, projectIDs)
	}
	if err != nil {
		return nil, false, err
	}

	type match struct {
		SearchResult
		row int
	}
	var all []match
	for rowIndex, r := range rows {
		lines := strings.Split(r.content, "\n")
		for _, i := range matchingLines(lines, opts.Mode, opts.Query, terms) {
			res := SearchResult{
				Project:    r.project,
				Subproject: r.subproject,
				Kind:       r.kind,
				EntryID:    r.entryID,
				EntryDate:  r.entryDate,
				LineNumber: i + 1,
				Line:       lines[i],
			}
			if opts.ContextLines > 0 {
				res.ContextBefore = contextSlice(lines, i, -opts.ContextLines)
				res.ContextAfter = contextSlice(lines, i, opts.ContextLines)
			}
			all = append(all, match{res, rowIndex})
		}
	}

	if opts.Mode == SearchExact {
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
	}

	if opts.Offset >= len(all) {
		return []SearchResult{}, truncated, nil
	}
	end := opts.Offset + opts.Limit + 1
	if end > len(all) {
		end = len(all)
	}
	out := make([]SearchResult, 0, end-opts.Offset)
	for _, m := range all[opts.Offset:end] {
		out = append(out, m.SearchResult)
	}
	return out, truncated, nil
}

// matchingLines returns the indexes of the lines of one matched row that
// count as hits for mode: lines holding query as a case-insensitive
// substring (SearchExact), the whole phrase of terms (SearchPhrase) or any
// of terms (SearchWords). When no single line holds the phrase, because
// it spans lines, the lines holding any of its terms are returned instead.
func matchingLines(lines []string, mode SearchMode, query string, terms []searchTerm) []int {
	needle := strings.ToLower(query)
	hit := func(line string) bool { return lineHasAnyTerm(line, terms) }
	switch mode {
	case SearchExact:
		hit = func(line string) bool { return strings.Contains(strings.ToLower(line), needle) }
	case SearchPhrase:
		hit = func(line string) bool { return lineHasPhrase(line, terms) }
	}
	var out []int
	for i, line := range lines {
		if hit(line) {
			out = append(out, i)
		}
	}
	if len(out) == 0 && mode == SearchPhrase {
		return matchingLines(lines, SearchWords, query, terms)
	}
	return out
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

// fetchMatchingRows runs the case-insensitive LIKE query of a SearchExact
// search against documents and then entries (archived entries excluded),
// scoped to projectIDs (nil means no scope) and filtered by opts.Kinds,
// opts.Since and opts.Until, each in a fixed order. Documents are skipped
// when a date filter is set. It returns the matching rows, at most
// maxScannedRows per query and about maxScannedBytes of content in total.
// truncated reports whether either bound cut the result short. It returns a
// database error on failure.
func (s *Store) fetchMatchingRows(ctx context.Context, opts SearchOptions, projectIDs []int64) (rows []searchRow, truncated bool, err error) {
	like := "%" + escapeLike(strings.ToLower(opts.Query)) + "%"

	type query struct {
		what string
		sql  string
		args []any
	}
	var queries []query

	// The first two columns resolve to the top-level project name and the
	// subproject name respectively, whether the matched row belongs to a
	// subproject (p has a parent) or a top-level project (p has none).
	if opts.Since == "" && opts.Until == "" {
		docFilter, docArgs := searchFilters("d", false, opts, projectIDs)
		queries = append(queries, query{"documents", `
		SELECT COALESCE(parent.name, p.name), CASE WHEN parent.id IS NOT NULL THEN p.name ELSE '' END, d.kind, 0, '', d.content
		FROM documents d
		JOIN projects p ON p.id = d.project_id
		LEFT JOIN projects parent ON parent.id = p.parent_id
		WHERE ` + lowerFunc + `(d.content) LIKE ? ESCAPE '\'` + docFilter + `
		ORDER BY d.project_id, d.kind
		LIMIT ?`, append(append([]any{like}, docArgs...), maxScannedRows+1)})
	}

	entryFilter, entryArgs := searchFilters("e", true, opts, projectIDs)
	queries = append(queries, query{"entries", `
		SELECT COALESCE(parent.name, p.name), CASE WHEN parent.id IS NOT NULL THEN p.name ELSE '' END, e.kind, e.id, COALESCE(e.entry_date, ''), e.body
		FROM entries e
		JOIN projects p ON p.id = e.project_id
		LEFT JOIN projects parent ON parent.id = p.parent_id
		WHERE e.archived = 0 AND ` + lowerFunc + `(e.body) LIKE ? ESCAPE '\'` + entryFilter + `
		ORDER BY e.project_id, e.kind, (e.entry_date IS NULL AND e.position >= 0), e.entry_date, e.position
		LIMIT ?`, append(append([]any{like}, entryArgs...), maxScannedRows+1)})

	var total int
	for _, q := range queries {
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

// fetchRankedRows runs the full-text query match (an FTS5 expression built
// by ftsQuery) of a SearchWords or SearchPhrase search against
// documents_fts and entries_fts in one query, scoped and filtered like
// fetchMatchingRows. Rows come in relevance order (bm25, best first), ties
// broken by project, subproject, kind and reading order, so the order is
// the same on every call. It returns at most maxScannedRows rows and about
// maxScannedBytes of content; truncated reports whether either bound cut
// the result short. It returns a database error on failure.
func (s *Store) fetchRankedRows(ctx context.Context, match string, opts SearchOptions, projectIDs []int64) (rows []searchRow, truncated bool, err error) {
	var branches []string
	var args []any
	if opts.Since == "" && opts.Until == "" {
		docFilter, docArgs := searchFilters("d", false, opts, projectIDs)
		branches = append(branches, `
		SELECT COALESCE(parent.name, p.name) AS project, CASE WHEN parent.id IS NOT NULL THEN p.name ELSE '' END AS subproject,
			d.kind AS kind, 0 AS entry_id, '' AS entry_date, d.content AS content,
			bm25(documents_fts) AS score, 0 AS undated, 0 AS position, d.id AS row_id
		FROM documents_fts
		JOIN documents d ON d.id = documents_fts.rowid
		JOIN projects p ON p.id = d.project_id
		LEFT JOIN projects parent ON parent.id = p.parent_id
		WHERE documents_fts MATCH ?`+docFilter)
		args = append(append(args, match), docArgs...)
	}
	entryFilter, entryArgs := searchFilters("e", true, opts, projectIDs)
	branches = append(branches, `
		SELECT COALESCE(parent.name, p.name) AS project, CASE WHEN parent.id IS NOT NULL THEN p.name ELSE '' END AS subproject,
			e.kind AS kind, e.id AS entry_id, COALESCE(e.entry_date, '') AS entry_date, e.body AS content,
			bm25(entries_fts) AS score, (e.entry_date IS NULL AND e.position >= 0) AS undated, e.position AS position, e.id AS row_id
		FROM entries_fts
		JOIN entries e ON e.id = entries_fts.rowid
		JOIN projects p ON p.id = e.project_id
		LEFT JOIN projects parent ON parent.id = p.parent_id
		WHERE entries_fts MATCH ? AND e.archived = 0`+entryFilter)
	args = append(append(args, match), entryArgs...)

	query := `SELECT project, subproject, kind, entry_id, entry_date, content FROM (` +
		strings.Join(branches, "\n\t\tUNION ALL") + `)
		ORDER BY score, project, subproject, kind, undated, entry_date, position, row_id
		LIMIT ?`
	args = append(args, maxScannedRows+1)

	var total int
	n, cut, err := s.readSearchRows(ctx, query, args, &rows, &total)
	if err != nil {
		return nil, false, fmt.Errorf("search: %w", err)
	}
	return rows, cut || n > maxScannedRows, nil
}

// searchFilters builds the extra WHERE conditions, each starting with
// " AND ", and their arguments for a search over the table aliased alias
// ("d" for documents, "e" for entries): the project scope (projectIDs, nil
// for none), opts.Kinds and, for entries (dated true), opts.Since and
// opts.Until, which also exclude undated entries. alias is interpolated
// into the SQL and must be a trusted constant.
func searchFilters(alias string, dated bool, opts SearchOptions, projectIDs []int64) (string, []any) {
	clause, args := scopeClause(alias+".project_id", projectIDs)
	if len(opts.Kinds) > 0 {
		placeholders := make([]string, len(opts.Kinds))
		for i, k := range opts.Kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		clause += fmt.Sprintf(" AND %s.kind IN (%s)", alias, strings.Join(placeholders, ","))
	}
	if dated {
		if opts.Since != "" {
			clause += fmt.Sprintf(" AND %s.entry_date >= ?", alias)
			args = append(args, opts.Since)
		}
		if opts.Until != "" {
			clause += fmt.Sprintf(" AND %s.entry_date <= ?", alias)
			args = append(args, opts.Until)
		}
	}
	return clause, args
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
		if err := result.Scan(&r.project, &r.subproject, &r.kind, &r.entryID, &r.entryDate, &r.content); err != nil {
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
