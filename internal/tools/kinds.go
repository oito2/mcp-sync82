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

package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// maxNameLength is the longest kind, project or subproject name, in
// characters: "<name>.archived.md", the longest file export writes for a
// name, then stays well within the 255-byte file name limit of common
// filesystems.
const maxNameLength = 128

// kindSlugPattern is the slug rule for document/entries "kind" names: a
// leading letter or digit, then letters, digits, hyphens and underscores,
// at most maxNameLength characters in all. It is also the rule for project
// names.
var kindSlugPattern = regexp.MustCompile(fmt.Sprintf(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,%d}$`, maxNameLength-1))

// nameRuleText describes kindSlugPattern in error messages.
var nameRuleText = fmt.Sprintf("must start with a letter or digit, contain only letters, digits, hyphens, and underscores, and be at most %d characters long", maxNameLength)

// validateKind checks a tool call's "filename" argument, raw, trimmed of
// surrounding white space, against the kind slug rule and returns it
// lower-cased: kinds differing only in case are the same kind. It returns
// an error when the name does not match the rule. The argument is named
// "filename" in the tool schemas even though it names a document/entries
// kind rather than a real file.
func validateKind(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if !kindSlugPattern.MatchString(name) {
		return "", fmt.Errorf("invalid filename %q: %s", raw, nameRuleText)
	}
	return strings.ToLower(name), nil
}

// standardKinds are the six kinds init_project_memory sets up for a
// project — protected from delete_memory and always checked by
// check_project_health.
var standardKinds = []string{"memory", "architecture", "stack", "decisions", "progress", "next_steps"}

// isStandardKind reports whether kind is one of the six standardKinds.
func isStandardKind(kind string) bool {
	for _, k := range standardKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// appendOnlyStandardKinds is the set of standard kinds stored as entries
// rows rather than a single overwrite-style document.
var appendOnlyStandardKinds = map[string]bool{"progress": true, "decisions": true}

// isAppendOnlyKind reports whether kind is one of the two standard
// append-only kinds. Custom kinds are append-only or overwrite-style
// depending on which mode the caller used to write them (see
// update_project_memory's per-item "mode"), not by name — this only
// classifies the two fixed standard kinds.
func isAppendOnlyKind(kind string) bool {
	return appendOnlyStandardKinds[kind]
}
