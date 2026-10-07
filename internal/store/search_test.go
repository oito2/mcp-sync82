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
	"slices"
	"strings"
	"testing"
)

// TestSearchText_UnderscoreAndBackslashAreLiteral verifies that '_' and '\\'
// in a query match literally instead of acting as LIKE metacharacters.
func TestSearchText_UnderscoreAndBackslashAreLiteral(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	content := "file_name.go\nfileXname.go\nback\\slash"
	if err := s.WriteDocument(ctx, "acme", "", "stack", content); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	// "_" is the LIKE wildcard for any single character; it must match only a
	// literal underscore, not "fileXname.go".
	results, _, err := s.SearchText(ctx, SearchOptions{Query: "file_name", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (underscore): %v", err)
	}
	if len(results) != 1 || results[0].Line != "file_name.go" {
		t.Fatalf("expected exactly 1 literal match for \"file_name\", got %+v", results)
	}

	// The escape character itself ("\") must also match literally.
	backslashResults, _, err := s.SearchText(ctx, SearchOptions{Query: `back\slash`, Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (backslash): %v", err)
	}
	if len(backslashResults) != 1 || backslashResults[0].Line != `back\slash` {
		t.Fatalf("expected exactly 1 literal match for backslash query, got %+v", backslashResults)
	}
}

// TestSearchText_Pagination verifies the offset and limit behavior of
// SearchText, including the extra result that signals more matches.
func TestSearchText_Pagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	content := "line one\nneedle here\nline three\nanother needle\nline five"
	if err := s.WriteDocument(ctx, "acme", "", "architecture", content); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	// limit=1 with 2 total matches returns limit+1=2 results, which is how
	// the caller detects that more results exist.
	page1, _, err := s.SearchText(ctx, SearchOptions{Query: "needle", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 1, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (page1): %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 results (limit+1) on page1, got %d", len(page1))
	}

	// offset=1 with 2 total matches: only the last one remains.
	page2, _, err := s.SearchText(ctx, SearchOptions{Query: "needle", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 1, Limit: 1, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (page2): %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 result on page2, got %d", len(page2))
	}
	if page2[0].Line != "another needle" {
		t.Fatalf("page2 line = %q, want %q", page2[0].Line, "another needle")
	}

	// A literal "%" in the query must match literally, not as a LIKE
	// wildcard.
	if err := s.WriteDocument(ctx, "acme", "", "stack", "100% done\nanything"); err != nil {
		t.Fatalf("WriteDocument (percent): %v", err)
	}
	percentResults, _, err := s.SearchText(ctx, SearchOptions{Query: "100%", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (percent): %v", err)
	}
	if len(percentResults) != 1 {
		t.Fatalf("expected exactly 1 literal match for \"100%%\", got %d", len(percentResults))
	}
}

// TestSearchText_CapsScannedRowsAtMaxScannedRows verifies that a search over
// more than maxScannedRows matching rows reports itself as truncated, in
// exact mode and in the ranked words mode.
func TestSearchText_CapsScannedRowsAtMaxScannedRows(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	// One matching row per entry exercises the row-level cap directly: each
	// becomes exactly one searchRow before line splitting. The rows are
	// written in one transaction to keep the test fast.
	sections := make([]EntrySection, maxScannedRows+5)
	for i := range sections {
		sections[i] = EntrySection{Body: "cap-target line"}
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatalf("ReplaceAllEntries: %v", err)
	}

	results, truncated, err := s.SearchText(ctx, SearchOptions{Query: "cap-target", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: maxScannedRows * 2, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(results) != maxScannedRows {
		t.Fatalf("len(results) = %d, want %d (the maxScannedRows ceiling)", len(results), maxScannedRows)
	}
	if !truncated {
		t.Fatal("truncated = false, want the cut reported")
	}

	results, truncated, err = s.SearchText(ctx, SearchOptions{Query: "cap target", Mode: SearchWords, Scope: SearchScope{Project: "acme"}, Limit: maxScannedRows * 2})
	if err != nil {
		t.Fatalf("SearchText (words): %v", err)
	}
	if len(results) != maxScannedRows || !truncated {
		t.Fatalf("words mode: %d results, truncated %v; want %d and true", len(results), truncated, maxScannedRows)
	}
}

// TestSearchText_UnicodeCaseInsensitive verifies case-insensitive matching
// beyond ASCII, such as "DECISÃO" finding "Decisão".
func TestSearchText_UnicodeCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "Decisão: usar ÁGUA"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"decisão", "DECISÃO", "água", "ÁGUA"} {
		results, truncated, err := s.SearchText(ctx, SearchOptions{Query: q, Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 0})
		if err != nil {
			t.Fatalf("SearchText(%q): %v", q, err)
		}
		if len(results) != 1 || truncated {
			t.Errorf("SearchText(%q) = %d results (truncated %v), want 1", q, len(results), truncated)
		}
	}
}

// TestSearchText_EntriesCarryTheirDateInReadingOrder verifies that matches in
// a log carry their entry date and come in reading order.
func TestSearchText_EntriesCarryTheirDateInReadingOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"2026-03-01", "2026-01-01", "2026-02-01"} {
		if err := s.AppendEntry(ctx, "acme", "", "progress", d, "## "+d+"\n- needle"); err != nil {
			t.Fatal(err)
		}
	}
	results, _, err := s.SearchText(ctx, SearchOptions{Query: "needle", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatal(err)
	}
	var dates []string
	for _, r := range results {
		dates = append(dates, r.EntryDate)
	}
	if !slices.Equal(dates, []string{"2026-01-01", "2026-02-01", "2026-03-01"}) {
		t.Fatalf("entry dates = %v, want reading (date) order", dates)
	}
}

// TestSearchText_ScopeFiltersBySubproject verifies that SearchScope limits
// results to one subproject or to a project with all of its
// subprojects.
func TestSearchText_ScopeFiltersBySubproject(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatalf("EnsureProject (sync82): %v", err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatalf("EnsureProject (perci): %v", err)
	}
	if err := s.WriteDocument(ctx, "oito2", "sync82", "memory", "shared-term in sync82"); err != nil {
		t.Fatalf("WriteDocument (sync82): %v", err)
	}
	if err := s.WriteDocument(ctx, "oito2", "perci", "memory", "shared-term in perci"); err != nil {
		t.Fatalf("WriteDocument (perci): %v", err)
	}

	scoped, _, err := s.SearchText(ctx, SearchOptions{Query: "shared-term", Mode: SearchExact, Scope: SearchScope{Project: "oito2", Subproject: "sync82"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (scoped): %v", err)
	}
	if len(scoped) != 1 {
		t.Fatalf("expected 1 result scoped to sync82, got %d", len(scoped))
	}
	// Project must be the top-level name and Subproject the child name, not
	// swapped; a count-only assertion would not detect a swap.
	if scoped[0].Project != "oito2" || scoped[0].Subproject != "sync82" {
		t.Fatalf("got Project=%q Subproject=%q, want Project=oito2 Subproject=sync82", scoped[0].Project, scoped[0].Subproject)
	}

	unscoped, _, err := s.SearchText(ctx, SearchOptions{Query: "shared-term", Mode: SearchExact, Scope: SearchScope{Project: "oito2"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText (project-wide): %v", err)
	}
	if len(unscoped) != 2 {
		t.Fatalf("expected 2 results across all of oito2's subprojects, got %d", len(unscoped))
	}
	for _, r := range unscoped {
		if r.Project != "oito2" {
			t.Errorf("got Project=%q, want oito2 for every result under oito2's subprojects", r.Project)
		}
	}
}

// TestSearchText_ReportsTopLevelProjectNameNotItsOwnRow verifies that results
// report the top-level project in Project and the subproject in Subproject.
func TestSearchText_ReportsTopLevelProjectNameNotItsOwnRow(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "top-level-marker"); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}

	results, _, err := s.SearchText(ctx, SearchOptions{Query: "top-level-marker", Mode: SearchExact, Scope: SearchScope{}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Project != "acme" || results[0].Subproject != "" {
		t.Fatalf("got Project=%q Subproject=%q, want Project=acme Subproject=\"\" for a top-level project", results[0].Project, results[0].Subproject)
	}
}

// TestSearchText_ContextLinesAreClampedToTheContent verifies that requested
// context lines are clamped at the start and end of the content.
func TestSearchText_ContextLinesAreClampedToTheContent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "one\ntwo\nneedle\nfour"); err != nil {
		t.Fatal(err)
	}

	results, _, err := s.SearchText(ctx, SearchOptions{Query: "needle", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 5})
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if !slices.Equal(r.ContextBefore, []string{"one", "two"}) || !slices.Equal(r.ContextAfter, []string{"four"}) {
		t.Fatalf("context = %q / %q, want the lines around the match, clamped to the document", r.ContextBefore, r.ContextAfter)
	}
}

// TestSearchText_EntryHitsCarryTheEntryID verifies that a match in an entry
// carries that entry's id and a match in a document carries 0.
func TestSearchText_EntryHitsCarryTheEntryID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle in memory"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- needle"); err != nil {
		t.Fatal(err)
	}
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadEntries = %+v, %v", entries, err)
	}

	results, _, err := s.SearchText(ctx, SearchOptions{Query: "needle", Mode: SearchExact, Scope: SearchScope{Project: "acme"}, Offset: 0, Limit: 10, ContextLines: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v, want 2", results)
	}
	for _, r := range results {
		want := int64(0)
		if r.Kind == "progress" {
			want = entries[0].ID
		}
		if r.EntryID != want {
			t.Errorf("%s hit has EntryID %d, want %d", r.Kind, r.EntryID, want)
		}
	}
}

// TestSearchText_NegativePagingAndCRLF verifies that a negative offset or
// limit doesn't panic, and that CRLF content yields lines without "\r".
func TestSearchText_NegativePagingAndCRLF(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "first needle\r\nsecond needle\r\n"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []SearchMode{SearchWords, SearchExact} {
		results, _, err := s.SearchText(ctx, SearchOptions{Query: "needle", Mode: mode, Scope: SearchScope{Project: "acme"}, Offset: -3, Limit: -1})
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if len(results) != 1 {
			t.Errorf("%s with negative paging: %d results, want 1 (limit 0 plus the look-ahead one)", mode, len(results))
		}
		results, _, err = s.SearchText(ctx, SearchOptions{Query: "needle", Mode: mode, Scope: SearchScope{Project: "acme"}, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range results {
			if strings.HasSuffix(r.Line, "\r") {
				t.Errorf("%s: line %q keeps its carriage return", mode, r.Line)
			}
		}
	}
}
