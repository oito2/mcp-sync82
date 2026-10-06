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

package analyzer

import (
	"slices"
	"testing"
)

// TestDetectRuby_ViaGemfile verifies Ruby and framework detection from a Gemfile.
func TestDetectRuby_ViaGemfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "source 'https://rubygems.org'\n\ngem 'rails', '~> 7.1'\ngem 'pg'\n")

	var r Result
	detectRuby(dir, &r)
	if !slices.Contains(r.Languages, "Ruby") {
		t.Errorf("Languages = %v, want Ruby", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Rails") {
		t.Errorf("Frameworks = %v, want Rails", r.Frameworks)
	}
	if slices.Contains(r.Frameworks, "Sinatra") {
		t.Errorf("Frameworks = %v, want no Sinatra (not in Gemfile)", r.Frameworks)
	}
}

// TestDetectRuby_MissingFileIsNoOp verifies that a missing Gemfile adds no signal.
func TestDetectRuby_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectRuby(dir, &r)
	if len(r.Languages) != 0 {
		t.Fatalf("expected no signal when Gemfile is absent, got %+v", r)
	}
}

// TestDetectRuby_MatchesGemNamesOnly guards against matching gem names as
// substrings of the whole Gemfile, comments included.
func TestDetectRuby_MatchesGemNamesOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "# no rails here\n# gem 'hanami'\ngem 'sinatra-contrib'\n")

	var r Result
	detectRuby(dir, &r)
	if !slices.Equal(r.Languages, []string{"Ruby"}) {
		t.Errorf("Languages = %v, want [Ruby]", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}

// TestDetectRuby_DoubleQuotedAndParenthesizedGems verifies that double-quoted and parenthesized gem declarations
// are recognized.
func TestDetectRuby_DoubleQuotedAndParenthesizedGems(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", "gem \"hanami\", \"~> 2.1\"\n  gem('sinatra')\ngem 'Rails' # comment\n")

	var r Result
	detectRuby(dir, &r)
	want := []string{"Rails", "Sinatra", "Hanami"}
	if !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}
