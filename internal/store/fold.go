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
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// foldExceptions maps the runes that the FTS5 "unicode61 remove_diacritics
// 2" tokenizer folds differently from the general rule in foldRune to the
// rune it produces: Latin letters with two diacritics, which it only
// lower-cases, the dotless i, which it keeps, and U+037F, which it leaves
// as is.
var foldExceptions = map[rune]rune{
	0x01E0: 0x01E1, 0x01E1: 0x01E1, // Ǡ ǡ
	0x01E2: 0x01E3, 0x01E3: 0x01E3, // Ǣ ǣ
	0x01EE: 0x01EF, 0x01EF: 0x01EF, // Ǯ ǯ
	0x01FC: 0x01FD, 0x01FD: 0x01FD, // Ǽ ǽ
	0x01FE: 0x01FF, 0x01FF: 0x01FF, // Ǿ ǿ
	0x0131: 0x0131, // ı
	0x037F: 0x037F, // Ϳ
}

// foldText returns s folded the way the FTS5 "unicode61 remove_diacritics 2"
// tokenizer folds the text it indexes: lower-cased, with the diacritics of
// Latin letters removed ("Sessão" → "sessao") and the combining marks of
// decomposed text dropped. Letters of other scripts keep their diacritics.
func foldText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		foldRune(&b, r)
	}
	return b.String()
}

// foldRune writes the folded form of r to b, as described in foldText: a
// combining mark is dropped, a rune of foldExceptions is replaced by its
// mapping, a Latin letter is decomposed and written without its marks, and
// any other rune is lower-cased. Lower-casing goes through the upper case
// so variant forms such as "ſ" or "ϐ" fold to their base letter.
func foldRune(b *strings.Builder, r rune) {
	if unicode.Is(unicode.Mn, r) {
		return
	}
	if m, ok := foldExceptions[r]; ok {
		b.WriteRune(m)
		return
	}
	if r < utf8RuneSelf {
		b.WriteRune(unicode.ToLower(r))
		return
	}
	decomposed := norm.NFD.String(string(r))
	base := []rune(decomposed)[0]
	if !unicode.Is(unicode.Latin, base) {
		b.WriteRune(unicode.ToLower(unicode.ToUpper(r)))
		return
	}
	for _, c := range decomposed {
		if !unicode.Is(unicode.Mn, c) {
			b.WriteRune(unicode.ToLower(unicode.ToUpper(c)))
		}
	}
}

// utf8RuneSelf is the first rune that is not ASCII.
const utf8RuneSelf = 0x80

// isTokenRune reports whether r belongs to a search token, as in the FTS5
// unicode61 tokenizer: letters, numbers, private-use characters and the
// combining marks foldText drops. Every other rune separates tokens.
func isTokenRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.Is(unicode.Co, r) || unicode.Is(unicode.Mn, r)
}

// tokenize splits s into folded tokens, the same words the FTS5 index
// holds for s.
func tokenize(s string) []string {
	var tokens []string
	for _, field := range strings.FieldsFunc(s, func(r rune) bool { return !isTokenRune(r) }) {
		if t := foldText(field); t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// searchTerm is one token of a words or phrase query: the folded text and
// whether it matches as a prefix (the query word ended with "*").
type searchTerm struct {
	text   string
	prefix bool
}

// parseSearchTerms splits a words or phrase query into searchTerms. The
// query is split on white space; each word is tokenized like indexed text,
// and a word ending in "*" makes its last token a prefix. Any other
// punctuation only separates tokens, so FTS5 syntax in the query is never
// interpreted.
func parseSearchTerms(query string) []searchTerm {
	var terms []searchTerm
	for _, word := range strings.Fields(query) {
		tokens := tokenize(word)
		for i, t := range tokens {
			terms = append(terms, searchTerm{text: t, prefix: i == len(tokens)-1 && strings.HasSuffix(word, "*")})
		}
	}
	return terms
}

// HasSearchTerms reports whether query holds at least one searchable word
// for the words and phrase search modes, that is, at least one letter or
// number.
func HasSearchTerms(query string) bool {
	return len(parseSearchTerms(query)) > 0
}

// ftsQuery builds the FTS5 MATCH expression for terms: in words mode every
// term quoted, all of them required; in phrase mode one quoted phrase.
// A prefix term (or, in phrase mode, a prefix last term) gets "*". Terms
// only hold letters and numbers, so quoting them needs no escaping.
func ftsQuery(terms []searchTerm, mode SearchMode) string {
	if mode == SearchPhrase {
		texts := make([]string, len(terms))
		for i, t := range terms {
			texts[i] = t.text
		}
		q := `"` + strings.Join(texts, " ") + `"`
		if terms[len(terms)-1].prefix {
			q += "*"
		}
		return q
	}
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = `"` + t.text + `"`
		if t.prefix {
			parts[i] += "*"
		}
	}
	return strings.Join(parts, " ")
}

// termMatches reports whether the folded token tok matches term.
func termMatches(tok string, term searchTerm) bool {
	if term.prefix {
		return strings.HasPrefix(tok, term.text)
	}
	return tok == term.text
}

// lineHasAnyTerm reports whether one of the tokens of line matches one of
// terms.
func lineHasAnyTerm(line string, terms []searchTerm) bool {
	for _, tok := range tokenize(line) {
		for _, term := range terms {
			if termMatches(tok, term) {
				return true
			}
		}
	}
	return false
}

// lineHasPhrase reports whether the tokens of line hold terms as a
// consecutive sequence.
func lineHasPhrase(line string, terms []searchTerm) bool {
	tokens := tokenize(line)
	for start := 0; start+len(terms) <= len(tokens); start++ {
		matched := true
		for i, term := range terms {
			if !termMatches(tokens[start+i], term) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
