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
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// search runs SearchText on s with the given mode and query over project
// acme, up to 100 results, failing the test on error.
func search(t *testing.T, s *Store, mode SearchMode, query string) []SearchResult {
	t.Helper()
	results, _, err := s.SearchText(context.Background(), SearchOptions{Query: query, Mode: mode, Scope: SearchScope{Project: "acme"}, Limit: 100})
	if err != nil {
		t.Fatalf("SearchText(%s, %q): %v", mode, query, err)
	}
	return results
}

// lines returns "kind:line" for each result, in order.
func lines(results []SearchResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Kind + ":" + r.Line
	}
	return out
}

// newAcme returns a test store holding the empty project acme.
func newAcme(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(context.Background(), "acme", ""); err != nil {
		t.Fatal(err)
	}
	return s
}

// assertFTSIntegrity runs the FTS5 integrity check of both indexes, which
// fails when an index no longer matches its content table.
func assertFTSIntegrity(t *testing.T, s *Store) {
	t.Helper()
	for _, table := range []string{"documents_fts", "entries_fts"} {
		if _, err := s.db.Exec(`INSERT INTO ` + table + `(` + table + `, rank) VALUES ('integrity-check', 1)`); err != nil {
			t.Fatalf("%s integrity check: %v", table, err)
		}
	}
}

// TestFoldText_MatchesTheFTS5Tokenizer verifies, letter by letter over the
// Latin, Greek and Cyrillic blocks, that foldText produces the token the
// FTS5 "unicode61 remove_diacritics 2" tokenizer indexes, so line hits
// computed in Go agree with the rows the index selects.
func TestFoldText_MatchesTheFTS5Tokenizer(t *testing.T) {
	s := newTestStore(t)
	for _, stmt := range []string{
		`CREATE VIRTUAL TABLE temp.probe USING fts5(body, tokenize='unicode61 remove_diacritics 2')`,
		`CREATE VIRTUAL TABLE temp.probe_vocab USING fts5vocab(temp, probe, 'row')`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	blocks := [][2]rune{{0x41, 0x7A}, {0xC0, 0x24F}, {0x370, 0x3FF}, {0x400, 0x4FF}, {0x1E00, 0x1EFF}, {0x1F00, 0x1FFF}}
	for _, block := range blocks {
		for r := block[0]; r <= block[1]; r++ {
			if !unicode.IsLetter(r) {
				continue
			}
			word := "q" + string(r) + "q"
			if _, err := s.db.Exec(`DELETE FROM probe`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(`INSERT INTO probe(body) VALUES (?)`, word); err != nil {
				t.Fatal(err)
			}
			var indexed string
			if err := s.db.QueryRow(`SELECT group_concat(term, '|') FROM probe_vocab`).Scan(&indexed); err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(tokenize(word), "|"); got != indexed {
				t.Errorf("U+%04X %c: tokenize = %q, FTS5 indexes %q", r, r, got, indexed)
			}
		}
	}
}

// TestSearchText_WordsIgnoresAccentsAndCase verifies that words mode finds
// accented text from an unaccented query and the other way round.
func TestSearchText_WordsIgnoresAccentsAndCase(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	if err := s.WriteDocument(ctx, "acme", "", "memory", "Sessão de revisão\nDECISÃO tomada"); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]string{
		"sessao":  "memory:Sessão de revisão",
		"SESSÃO":  "memory:Sessão de revisão",
		"decisao": "memory:DECISÃO tomada",
		"decisão": "memory:DECISÃO tomada",
	} {
		if got := lines(search(t, s, SearchWords, query)); !slices.Equal(got, []string{want}) {
			t.Errorf("words %q = %q, want [%q]", query, got, want)
		}
	}
	if got := search(t, s, SearchExact, "sessao"); len(got) != 0 {
		t.Errorf("exact mode should keep accents significant, got %q", lines(got))
	}
}

// TestSearchText_WordsNeedsEveryWordInTheRow verifies that words mode
// selects a row only when it holds every word, wherever they are, and
// reports each line holding one of them.
func TestSearchText_WordsNeedsEveryWordInTheRow(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	if err := s.WriteDocument(ctx, "acme", "", "memory", "the installer\nwrites the config\nunrelated"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", "installer only"); err != nil {
		t.Fatal(err)
	}
	got := lines(search(t, s, SearchWords, "config installer"))
	if !slices.Equal(got, []string{"memory:the installer", "memory:writes the config"}) {
		t.Errorf("words = %q", got)
	}
}

// TestSearchText_PhraseAndPrefix verifies phrase mode, a trailing "*" as a
// prefix in both modes, and the fallback to word lines when the phrase
// spans two lines.
func TestSearchText_PhraseAndPrefix(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	if err := s.WriteDocument(ctx, "acme", "", "memory", "config file written\nfile config\ndecisões sobre o instalador"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", "spans the config\nfile here"); err != nil {
		t.Fatal(err)
	}
	got := lines(search(t, s, SearchPhrase, "config file"))
	slices.Sort(got)
	if !slices.Equal(got, []string{"memory:config file written", "stack:file here", "stack:spans the config"}) {
		t.Errorf("phrase = %q", got)
	}
	if got := lines(search(t, s, SearchWords, "decis* instal*")); !slices.Equal(got, []string{"memory:decisões sobre o instalador"}) {
		t.Errorf("words with prefixes = %q", got)
	}
	if got := lines(search(t, s, SearchPhrase, "sobre o instal*")); !slices.Equal(got, []string{"memory:decisões sobre o instalador"}) {
		t.Errorf("phrase with a prefix = %q", got)
	}
}

// TestSearchText_QuerySyntaxIsTreatedAsText verifies that FTS5 operators and
// punctuation in a query never cause a syntax error, and that a query
// without letters or numbers is reported as ErrNoSearchTerms.
func TestSearchText_QuerySyntaxIsTreatedAsText(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	if err := s.WriteDocument(ctx, "acme", "", "memory", "near or not, a:b c^d e-f \"quoted\" g*h"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`"quoted`, `NEAR(a b)`, `near OR not`, `a:b`, `c^d`, `-e f`, `g*h`, `memory:near`, `(near`, `NOT near`, `{near}`, `near AND`} {
		for _, mode := range []SearchMode{SearchWords, SearchPhrase} {
			if _, _, err := s.SearchText(ctx, SearchOptions{Query: q, Mode: mode, Scope: SearchScope{Project: "acme"}, Limit: 10}); err != nil {
				t.Errorf("%s %q: %v", mode, q, err)
			}
		}
	}
	if got := search(t, s, SearchWords, `NEAR(or not)`); len(got) != 1 {
		t.Errorf("NEAR(...) should search the words near, or, not as text, got %q", lines(got))
	}
	for _, q := range []string{"***", `"" -`, "_"} {
		if _, _, err := s.SearchText(ctx, SearchOptions{Query: q, Mode: SearchWords, Limit: 10}); !errors.Is(err, ErrNoSearchTerms) {
			t.Errorf("%q: err = %v, want ErrNoSearchTerms", q, err)
		}
		if HasSearchTerms(q) {
			t.Errorf("HasSearchTerms(%q) = true, want false", q)
		}
	}
}

// TestSearchText_FiltersByKindAndDate verifies the kinds filter and that a
// date range keeps only dated entries in range, leaving documents and
// undated entries out, in every mode.
func TestSearchText_FiltersByKindAndDate(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle in memory"); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ kind, date, body string }{
		{"progress", "2026-01-10", "needle january"},
		{"progress", "2026-02-10", "needle february"},
		{"progress", "", "needle undated"},
		{"decisions", "2026-02-11", "needle decision"},
	} {
		if err := s.AppendEntry(ctx, "acme", "", e.kind, e.date, e.body); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []SearchMode{SearchWords, SearchPhrase, SearchExact} {
		run := func(opts SearchOptions) []string {
			opts.Query, opts.Mode, opts.Scope, opts.Limit = "needle", mode, SearchScope{Project: "acme"}, 100
			results, _, err := s.SearchText(ctx, opts)
			if err != nil {
				t.Fatalf("%s: %v", mode, err)
			}
			got := lines(results)
			slices.Sort(got)
			return got
		}
		if got := run(SearchOptions{Kinds: []string{"Progress"}}); !slices.Equal(got, []string{"progress:needle february", "progress:needle january", "progress:needle undated"}) {
			t.Errorf("%s kinds = %q", mode, got)
		}
		if got := run(SearchOptions{Since: "2026-02-01"}); !slices.Equal(got, []string{"decisions:needle decision", "progress:needle february"}) {
			t.Errorf("%s since = %q", mode, got)
		}
		if got := run(SearchOptions{Since: "2026-01-01", Until: "2026-02-10", Kinds: []string{"progress"}}); !slices.Equal(got, []string{"progress:needle february", "progress:needle january"}) {
			t.Errorf("%s since+until+kinds = %q", mode, got)
		}
	}
}

// TestSearchText_RanksByRelevanceStably verifies that the row where the
// words weigh most comes first and that pages of a ranked search line up
// with the full result list.
func TestSearchText_RanksByRelevanceStably(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	if err := s.WriteDocument(ctx, "acme", "", "architecture", "a long text that mentions sqlite once among many other words about the design"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", "sqlite sqlite sqlite"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if err := s.AppendEntry(ctx, "acme", "", "progress", fmt.Sprintf("2026-01-%02d", i+1), fmt.Sprintf("sqlite note %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	all := search(t, s, SearchWords, "sqlite")
	if all[0].Kind != "stack" {
		t.Errorf("first result = %q, want the stack document", lines(all)[0])
	}
	var paged []SearchResult
	for offset := 0; offset < len(all); offset += 3 {
		page, _, err := s.SearchText(ctx, SearchOptions{Query: "sqlite", Scope: SearchScope{Project: "acme"}, Offset: offset, Limit: 3})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) > 3 {
			page = page[:3]
		}
		paged = append(paged, page...)
	}
	if !slices.Equal(lines(paged), lines(all)) {
		t.Errorf("pages = %q, want %q", lines(paged), lines(all))
	}
	if again := search(t, s, SearchWords, "sqlite"); !slices.Equal(lines(again), lines(all)) {
		t.Errorf("second run = %q, want %q", lines(again), lines(all))
	}
}

// TestFTSIndex_FollowsEveryWrite verifies that the full-text indexes stay
// in sync with every kind of write: append, replace, update, delete,
// archive, a whole-kind rewrite, a batch write, a kind delete, a rename and
// a project delete.
func TestFTSIndex_FollowsEveryWrite(t *testing.T) {
	s := newAcme(t)
	ctx := context.Background()
	found := func(query string) int { return len(search(t, s, SearchWords, query)) }
	step := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assertFTSIntegrity(t, s)
	}

	step("WriteDocument", s.WriteDocument(ctx, "acme", "", "memory", "alpha"))
	step("WriteDocument again", s.WriteDocument(ctx, "acme", "", "memory", "bravo"))
	if found("alpha") != 0 || found("bravo") != 1 {
		t.Errorf("document update not indexed: alpha %d, bravo %d", found("alpha"), found("bravo"))
	}

	step("AppendEntry", s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "charlie"))
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil {
		t.Fatal(err)
	}
	id := entries[0].ID
	step("UpdateEntry", s.UpdateEntry(ctx, "acme", "", "progress", id, "delta", nil))
	if found("charlie") != 0 || found("delta") != 1 {
		t.Errorf("entry update not indexed: charlie %d, delta %d", found("charlie"), found("delta"))
	}
	_, err = s.ArchiveEntries(ctx, "acme", "", "progress", "2026-02-01", nil)
	step("ArchiveEntries", err)
	if found("delta") != 0 {
		t.Errorf("archived entry still found")
	}
	step("DeleteEntry", s.DeleteEntry(ctx, "acme", "", "progress", id))

	step("ReplaceAllEntries", s.ReplaceAllEntries(ctx, "acme", "", "decisions", []EntrySection{{Date: "2026-01-01", Body: "echo"}}))
	step("ReplaceAllEntries again", s.ReplaceAllEntries(ctx, "acme", "", "decisions", []EntrySection{{Date: "2026-01-01", Body: "foxtrot"}}))
	if found("echo") != 0 || found("foxtrot") != 1 {
		t.Errorf("rewrite not indexed: echo %d, foxtrot %d", found("echo"), found("foxtrot"))
	}
	golf := "golf"
	step("WriteKinds", s.WriteKinds(ctx, "acme", "", []KindWrite{{Kind: "notes", Document: &golf}, {Kind: "decisions", Sections: []EntrySection{{Body: "hotel"}}}}))
	if found("golf") != 1 || found("hotel") != 1 || found("foxtrot") != 0 {
		t.Errorf("batch write not indexed")
	}
	step("DeleteKind", s.DeleteKind(ctx, "acme", "", "notes"))
	if found("golf") != 0 {
		t.Errorf("deleted kind still found")
	}
	step("RenameProject", s.RenameProject(ctx, "acme", "", "acme2"))
	step("RenameProject back", s.RenameProject(ctx, "acme2", "", "acme"))
	if found("bravo") != 1 {
		t.Errorf("rename lost the index")
	}
	step("DeleteProject", s.DeleteProject(ctx, "acme"))
	results, _, err := s.SearchText(ctx, SearchOptions{Query: "bravo hotel", Limit: 10})
	if err != nil || len(results) != 0 {
		t.Errorf("deleted project still found: %v, %v", lines(results), err)
	}
	var rows int
	if err := s.db.QueryRow(`SELECT (SELECT count(*) FROM documents_fts) + (SELECT count(*) FROM entries_fts)`).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("index rows left after deleting the project: %d, %v", rows, err)
	}
}

// TestMigrate_V3IndexesAnExistingV2Vault verifies that opening a vault at
// schema version 2 with data adds the full-text indexes, indexes the
// existing rows, and that opening it again changes nothing.
func TestMigrate_V3IndexesAnExistingV2Vault(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	stmts := append(append([]string{migrationsTableDDL}, schemaV1...), lowercaseNamesV2...)
	stmts = append(stmts,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (1, 'x'), (2, 'x')`,
		`INSERT INTO projects (name, parent_id, created_at) VALUES ('acme', NULL, 'x')`,
		`INSERT INTO documents (project_id, kind, content, updated_at) VALUES (1, 'memory', 'Decisão antiga', 'x')`,
		`INSERT INTO entries (project_id, kind, entry_date, body, archived, created_at, position) VALUES (1, 'progress', '2026-01-01', 'sessão um', 0, 'x', 0)`,
	)
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	for open := 0; open < 2; open++ {
		s, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("Open #%d: %v", open+1, err)
		}
		var version, applied int
		latest := migrations[len(migrations)-1].version
		if err := s.db.QueryRow(`SELECT MAX(version), COUNT(*) FROM schema_migrations`).Scan(&version, &applied); err != nil || version != latest || applied != latest {
			t.Fatalf("Open #%d: schema version %d with %d records (%v), want %d and %d", open+1, version, applied, err, latest, latest)
		}
		if got := lines(search(t, s, SearchWords, "decisao sessao")); len(got) != 0 {
			t.Errorf("words needing both words in one row matched across rows: %q", got)
		}
		if got := lines(search(t, s, SearchWords, "decisao")); !slices.Equal(got, []string{"memory:Decisão antiga"}) {
			t.Errorf("Open #%d: decisao = %q", open+1, got)
		}
		if got := lines(search(t, s, SearchWords, "sessao")); !slices.Equal(got, []string{"progress:sessão um"}) {
			t.Errorf("Open #%d: sessao = %q", open+1, got)
		}
		assertFTSIntegrity(t, s)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// BenchmarkSearchText compares the words, phrase and exact modes on a vault
// with 10,000 entries, plus a words search for "ᏣᎳᎩ", which matches no
// entry and holds a word outside the scripts the index is known to handle,
// so the substring fallback scans every row.
func BenchmarkSearchText(b *testing.B) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(b.TempDir(), "vault.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		b.Fatal(err)
	}
	sections := make([]EntrySection, 10000)
	for i := range sections {
		date := fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1)
		sections[i] = EntrySection{Date: date, Body: fmt.Sprintf("## %s\n- entry %d about the installer, the vault and session %d", date, i, i%97)}
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		b.Fatal(err)
	}
	for _, c := range []struct {
		name, query string
		mode        SearchMode
	}{
		{"words", "session 42", SearchWords},
		{"phrase", "session 42", SearchPhrase},
		{"exact", "session 42", SearchExact},
		{"words-fallback", "ᏣᎳᎩ", SearchWords},
	} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				if _, _, err := s.SearchText(ctx, SearchOptions{Query: c.query, Mode: c.mode, Scope: SearchScope{Project: "acme"}, Limit: 100}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestSearchExact_PrefilterKeepsEverySubstringMatch verifies that the
// FTS5 prefilter never drops a row an exact search must find: every
// substring of every stored line, cut at any position, still finds its
// entry, including cuts inside tokens such as "o_bar" in "foo_barbaz".
func TestSearchExact_PrefilterKeepsEverySubstringMatch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bodies := []string{
		"## 2026-01-01\n- foo_barbaz moved to config.json (v1.2.0)",
		"## 2026-01-02\n- Sessão de manutenção: ação nº 3 — 100% concluída",
		"## 2026-01-03\n- path C:\\Users\\dev\\vault.db, ~/.sync82/knowledge.db",
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, body := range bodies {
		if err := s.AppendEntry(ctx, "acme", "", "progress", body[3:13], body); err != nil {
			t.Fatal(err)
		}
	}
	checked := 0
	for i, body := range bodies {
		line := strings.Split(body, "\n")[1]
		runes := []rune(line)
		for start := 0; start < len(runes); start++ {
			for end := start + 1; end <= len(runes) && end-start <= 12; end++ {
				query := string(runes[start:end])
				if strings.TrimSpace(query) == "" || strings.ContainsAny(query, "\r\n") {
					continue
				}
				results, _, err := s.SearchText(ctx, SearchOptions{Query: query, Mode: SearchExact, Limit: 100})
				if err != nil {
					t.Fatalf("SearchText(%q): %v", query, err)
				}
				found := false
				for _, r := range results {
					if r.Line == line {
						found = true
					}
				}
				if !found {
					t.Fatalf("exact search for %q (prefilter %s) missed entry %d", query, exactPrefilter(query), i)
				}
				checked++
			}
		}
	}
	t.Logf("%d substrings checked", checked)
}

// TestTokenize_MatchesFTS5WhereThePrefilterRelies compares tokenize with
// the words the FTS5 index holds for "q<r>q", for every rune of the Basic
// Multilingual Plane that prefilterSafe accepts: the exact-search
// prefilter is only sound where the two agree. Runes outside that set may
// differ (FTS5's Unicode tables are older than Go's), which prefilterSafe
// keeps out of the prefilter.
func TestTokenize_MatchesFTS5WhereThePrefilterRelies(t *testing.T) {
	s := newTestStore(t)
	for _, stmt := range []string{
		`CREATE VIRTUAL TABLE temp.probe USING fts5(body, tokenize='unicode61 remove_diacritics 2')`,
		`CREATE VIRTUAL TABLE temp.probe_vocab USING fts5vocab(temp, probe, 'row')`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	checked := 0
	for r := rune(0x80); r <= 0xFFFF; r++ {
		if !prefilterSafe(r) {
			continue
		}
		word := "q" + string(r) + "q"
		if _, err := s.db.Exec(`DELETE FROM probe`); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`INSERT INTO probe(body) VALUES (?)`, word); err != nil {
			t.Fatal(err)
		}
		var indexed string
		if err := s.db.QueryRow(`SELECT coalesce(group_concat(term, '|'), '') FROM (SELECT term FROM probe_vocab ORDER BY term)`).Scan(&indexed); err != nil {
			t.Fatal(err)
		}
		tokens := tokenize(word)
		slices.Sort(tokens)
		if got := strings.Join(slices.Compact(tokens), "|"); got != indexed {
			t.Errorf("U+%04X %c: tokenize = %q, FTS5 indexes %q", r, r, got, indexed)
		}
		checked++
	}
	if checked < 1000 {
		t.Fatalf("only %d runes checked", checked)
	}
}

// TestSearchText_NonLatinScripts verifies that words, phrase and exact
// searches find text in scripts whose combining marks the FTS5 tokenizer
// treats as separators: Devanagari, vocalized Hebrew and Arabic.
func TestSearchText_NonLatinScripts(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "नमस्ते दुनिया यह परीक्षण\nשָׁלוֹם עוֹלָם\nمَرْحَبًا بِالْعَالَمِ"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		query string
		mode  SearchMode
	}{
		{"नमस्ते", SearchWords}, {"दुनिया", SearchWords}, {"नमस्ते दुनिया", SearchPhrase},
		{"नमस्ते दुनिया", SearchExact}, {"दुनिया यह", SearchExact},
		{"שָׁלוֹם", SearchWords}, {"שָׁלוֹם עוֹלָם", SearchExact},
		{"مَرْحَبًا", SearchWords}, {"مَرْحَبًا بِالْعَالَمِ", SearchExact},
	} {
		results, _, err := s.SearchText(ctx, SearchOptions{Query: c.query, Mode: c.mode, Limit: 10})
		if err != nil {
			t.Fatalf("%s %q: %v", c.mode, c.query, err)
		}
		if len(results) != 1 {
			t.Errorf("%s %q: %d results, want 1", c.mode, c.query, len(results))
		}
	}
}
