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
	"testing"
	"unicode"
	"unicode/utf8"
)

// benchmarkLine is a Portuguese log line with accented letters, the
// typical content search folds line by line.
const benchmarkLine = "## 2026-10-07\n- Sessão de manutenção: correção da migração, ações de configuração e revisão das decisões após a análise de desempenho (ação nº 3)."

// BenchmarkFoldText measures folding one accented line.
func BenchmarkFoldText(b *testing.B) {
	b.SetBytes(int64(len(benchmarkLine)))
	for b.Loop() {
		foldText(benchmarkLine)
	}
}

// BenchmarkTokenize measures tokenizing one accented line, which search
// does for every line of every matched row.
func BenchmarkTokenize(b *testing.B) {
	b.SetBytes(int64(len(benchmarkLine)))
	for b.Loop() {
		tokenize(benchmarkLine)
	}
}

// TestFoldRune_CacheAndFastPathMatchTheSlowFold verifies that the ASCII
// fast path and the cached folds give what foldRuneSlow computes, on the
// first and on later calls, for ASCII, the Basic Multilingual Plane and
// runes above it, which are never cached.
func TestFoldRune_CacheAndFastPathMatchTheSlowFold(t *testing.T) {
	check := func(r rune) {
		var want strings.Builder
		if r < utf8RuneSelf {
			want.WriteRune(unicode.ToLower(r))
		} else {
			foldRuneSlow(&want, r)
		}
		for range 2 {
			if got := foldText(string(r)); got != want.String() {
				t.Fatalf("foldText(U+%04X) = %q, want %q", r, got, want.String())
			}
		}
	}
	for r := rune(0); r <= maxCachedRune; r++ {
		if utf8.ValidRune(r) {
			check(r)
		}
	}
	for _, r := range []rune{0x10400, 0x1D400, 0x1F600} {
		check(r)
		if _, cached := foldCache.Load(r); cached {
			t.Errorf("U+%04X above maxCachedRune was cached", r)
		}
	}
}

// TestExactPrefilter checks the FTS5 expression built for exact queries:
// the first token is left out when the query begins inside it, the last
// one is a prefix when the query ends inside it, and a query without a
// usable token gives "" (full scan).
func TestExactPrefilter(t *testing.T) {
	for query, want := range map[string]string{
		"session 42":   `"42"*`,
		" session 42 ": `"session" "42"`,
		"foo_bar":      `"bar"*`,
		"(Ação) nº 3.": `"acao" "nº" "3"`,
		"100%":         "",
		"%_-.":         "",
		"oba":          "",
		"-x":           `"x"*`,
		"config.json":  `"json"*`,
		"a b c":        `"b" "c"*`,
		"x नमस्ते दुनिया":  "",
		"custo 100₽ total": `"total"*`,
		"a Ϳb c":           `"c"*`,
	} {
		if got := exactPrefilter(query); got != want {
			t.Errorf("exactPrefilter(%q) = %s, want %s", query, got, want)
		}
	}
}
