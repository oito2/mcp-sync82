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
	"encoding/json"
	"strings"
)

// depPattern maps a display name (a framework or notable library) to the
// dependency names that signal it. A pattern matches when any of its
// names is present in a project's dependency set.
type depPattern struct {
	display string
	names   []string
}

// matchDeps appends, unique and in the order of patterns, the display
// name of every pattern with at least one name present in deps. The
// result therefore depends only on the pattern table order, never on map
// iteration order.
func matchDeps(deps map[string]bool, patterns []depPattern, target *[]string) {
	for _, p := range patterns {
		for _, name := range p.names {
			if deps[name] {
				appendUnique(target, p.display)
				break
			}
		}
	}
}

// manifestDetector describes a detector that reads a list of manifest
// files from root, adds a language for every file it could read, collects
// the dependency names each file declares, and reports the patterns that
// match the combined set.
type manifestDetector struct {
	// files returns the manifest file names to read, relative to root,
	// in the order they are processed.
	files func(root string) []string
	// language returns the language added when file was read.
	language func(file string) string
	// deps returns the dependency names declared in one file's content,
	// already in the normalized form used by patterns.
	deps func(file, content string) []string
	// patterns is the ordered table of known dependencies.
	patterns []depPattern
}

// detect runs d against root: every file returned by d.files is read
// through readMarkerFile (unreadable files are skipped), its language is
// added to r.Languages, its dependency names are collected, and the
// matching patterns are appended to r.Frameworks. It returns the content
// of every file that was read, keyed by file name.
func (d manifestDetector) detect(root string, r *Result) map[string]string {
	read := make(map[string]string)
	deps := make(map[string]bool)
	for _, file := range d.files(root) {
		content, ok := readMarkerFile(root, file)
		if !ok {
			continue
		}
		read[file] = content
		appendUnique(&r.Languages, d.language(file))
		for _, name := range d.deps(file, content) {
			deps[name] = true
		}
	}
	if len(read) > 0 {
		matchDeps(deps, d.patterns, &r.Frameworks)
	}
	return read
}

// fixedFiles returns a files function that always yields names.
func fixedFiles(names ...string) func(string) []string {
	return func(string) []string { return names }
}

// fixedLanguage returns a language function that always yields lang.
func fixedLanguage(lang string) func(string) string {
	return func(string) string { return lang }
}

// decodeJSONManifest parses content as a JSON object and returns its
// "description" field (empty when absent or not a string) and the keys of
// every object-valued field named in depFields, transformed by normalize
// when it is not nil. Each field is decoded on its own, so a field with
// an unexpected type is ignored without affecting the others. ok is false
// only when content is not a JSON object.
func decodeJSONManifest(content string, normalize func(string) string, depFields ...string) (description string, deps map[string]bool, ok bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &fields); err != nil {
		return "", nil, false
	}

	if raw, found := fields["description"]; found {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			description = s
		}
	}

	deps = make(map[string]bool)
	for _, field := range depFields {
		raw, found := fields[field]
		if !found {
			continue
		}
		var entries map[string]json.RawMessage
		if json.Unmarshal(raw, &entries) != nil {
			continue
		}
		for name := range entries {
			if normalize != nil {
				name = normalize(name)
			}
			deps[name] = true
		}
	}
	return description, deps, true
}

// quotedStrings scans s, which starts right after an opening "[", up to
// the matching unquoted "]" (or the end of s), and returns the contents of
// every single- or double-quoted string found at any nesting depth, plus
// the index in s just past the closing bracket. Text after an unquoted
// "#" up to the end of its line is skipped as a comment. A backslash
// inside a double-quoted string escapes the next character.
func quotedStrings(s string) (items []string, end int) {
	depth := 1
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return items, i + 1
			}
		case '"', '\'':
			var b strings.Builder
			j := i + 1
			for ; j < len(s) && s[j] != c; j++ {
				if c == '"' && s[j] == '\\' && j+1 < len(s) {
					j++
				}
				b.WriteByte(s[j])
			}
			items = append(items, b.String())
			i = j
		}
	}
	return items, len(s)
}

// tomlKey returns the bare name of the key assigned on a TOML line
// ("name = ..." or "\"name\" = ..."), with any dotted suffix of an
// unquoted key removed ("serde.workspace = true" yields "serde"). ok is
// false when the line is not a key assignment.
func tomlKey(line string) (key string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == '#' || line[0] == '[' {
		return "", false
	}
	if line[0] == '"' || line[0] == '\'' {
		end := strings.IndexByte(line[1:], line[0])
		if end < 0 {
			return "", false
		}
		key = line[1 : end+1]
		if !strings.HasPrefix(strings.TrimSpace(line[end+2:]), "=") &&
			!strings.HasPrefix(strings.TrimSpace(line[end+2:]), ".") {
			return "", false
		}
		return key, key != ""
	}
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", false
	}
	key = strings.TrimSpace(line[:eq])
	if dot := strings.IndexByte(key, '.'); dot >= 0 {
		key = strings.TrimSpace(key[:dot])
	}
	if key == "" || strings.ContainsAny(key, " \t\"'") {
		return "", false
	}
	return key, true
}

// tomlHeader returns the table name of a TOML "[name]" header line, with
// a trailing comment removed and surrounding whitespace trimmed. ok is
// false for any other line, including "[[name]]" array-of-tables headers.
func tomlHeader(line string) (name string, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") || strings.HasPrefix(line, "[[") {
		return "", false
	}
	end := strings.IndexByte(line, ']')
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(line[1:end]), true
}
