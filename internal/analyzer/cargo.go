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

// cargoDescriptionPattern matches a single- or double-quoted description assignment, capturing the
// text in group 1 (double quotes) or group 2 (single quotes).
var cargoDescriptionPattern = regexp.MustCompile(`(?m)^\s*description\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// rustCrates is the ordered table of Rust crates reported as frameworks.
var rustCrates = []depPattern{
	{"Axum", []string{"axum"}},
	{"Actix", []string{"actix-web"}},
	{"Rocket", []string{"rocket"}},
	{"Tokio", []string{"tokio"}},
	{"Serde", []string{"serde"}},
	{"sqlx", []string{"sqlx"}},
	{"Diesel", []string{"diesel"}},
}

// cargoDepTables lists the Cargo.toml table names whose keys are crate names.
var cargoDepTables = []string{"dependencies", "dev-dependencies", "build-dependencies"}

// cargoDetector reads Cargo.toml and reports Rust plus the matching crates.
var cargoDetector = manifestDetector{
	files:    fixedFiles("Cargo.toml"),
	language: fixedLanguage("Rust"),
	deps:     func(_, content string) []string { return cargoDependencyNames(content) },
	patterns: rustCrates,
}

// detectCargoToml fills Languages += Rust, Description (if unset) from
// Cargo.toml's [package] description field, and Frameworks from the
// crate names declared as dependencies.
func detectCargoToml(root string, r *Result) {
	read := cargoDetector.detect(root, r)
	content, ok := read["Cargo.toml"]
	if !ok {
		return
	}

	pkgSection := extractTOMLSection(content, "package")
	if m := cargoDescriptionPattern.FindStringSubmatch(pkgSection); m != nil {
		setDescription(r, m[1]+m[2])
	}
}

// cargoDependencyNames returns the crate names declared in a Cargo.toml:
// every key of a [dependencies], [dev-dependencies], [build-dependencies]
// table (also under "workspace." or "target.<cfg>."), and the crate named
// by a "[<table>.<crate>]" header. Keys of any other table, and keys
// inside a "[<table>.<crate>]" table, are ignored.
func cargoDependencyNames(content string) []string {
	var names []string
	inTable := false
	for _, line := range strings.Split(content, "\n") {
		if header, ok := tomlHeader(line); ok {
			var crate string
			inTable, crate = cargoDepHeader(header)
			if crate != "" {
				names = append(names, crate)
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			inTable = false
			continue
		}
		if !inTable {
			continue
		}
		if key, ok := tomlKey(line); ok {
			names = append(names, key)
		}
	}
	return names
}

// cargoDepHeader classifies a Cargo.toml table name: isTable reports
// whether it is a dependency table whose keys are crate names, and crate
// is the crate a "<table>.<crate>" header names (quotes removed).
func cargoDepHeader(header string) (isTable bool, crate string) {
	h := header
	if rest, ok := strings.CutPrefix(h, "workspace."); ok {
		h = rest
	} else if rest, ok := strings.CutPrefix(h, "target."); ok {
		h = skipTOMLKeySegment(rest)
	}
	for _, table := range cargoDepTables {
		if h == table {
			return true, ""
		}
		if rest, ok := strings.CutPrefix(h, table+"."); ok {
			return false, strings.Trim(strings.TrimSpace(rest), `"'`)
		}
	}
	return false, ""
}

// skipTOMLKeySegment returns s with its first dotted-key segment (quoted
// or bare) and the following "." removed, or "" when s has a single
// segment.
func skipTOMLKeySegment(s string) string {
	if s != "" && (s[0] == '"' || s[0] == '\'') {
		end := strings.IndexByte(s[1:], s[0])
		if end < 0 {
			return ""
		}
		s = s[end+2:]
		rest, _ := strings.CutPrefix(s, ".")
		return rest
	}
	_, rest, found := strings.Cut(s, ".")
	if !found {
		return ""
	}
	return rest
}
