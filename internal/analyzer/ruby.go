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
	"regexp"
	"strings"
)

// rubyPackages is the ordered table of gem names reported as frameworks.
var rubyPackages = []depPattern{
	{"Rails", []string{"rails"}},
	{"Sinatra", []string{"sinatra"}},
	{"Hanami", []string{"hanami"}},
}

// rubyDetector reads the Gemfile and reports Ruby plus the matching gems.
var rubyDetector = manifestDetector{
	files:    fixedFiles("Gemfile"),
	language: fixedLanguage("Ruby"),
	deps:     func(_, content string) []string { return gemfileGems(content) },
	patterns: rubyPackages,
}

// gemDeclaration matches a Gemfile "gem 'name'" or "gem \"name\"" line
// (optionally with parentheses), capturing the gem name; commented-out
// lines never match since the line must start with "gem".
var gemDeclaration = regexp.MustCompile(`(?m)^[ \t]*gem[ \t]*\(?[ \t]*['"]([^'"]+)['"]`)

// detectRuby fills Languages += Ruby when a Gemfile is present, and
// Frameworks from the gem names it declares.
func detectRuby(root string, r *Result) {
	rubyDetector.detect(root, r)
}

// gemfileGems returns the lowercased names of every gem declared in a
// Gemfile.
func gemfileGems(content string) []string {
	var names []string
	for _, m := range gemDeclaration.FindAllStringSubmatch(content, -1) {
		names = append(names, strings.ToLower(m[1]))
	}
	return names
}
