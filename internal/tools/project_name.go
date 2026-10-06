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
	"strings"
)

// projectNamePattern is the format of a project or subproject name:
// alphanumeric characters, hyphens and underscores, starting with a letter
// or digit. It is the same rule kindSlugPattern applies to kind slugs.
var projectNamePattern = kindSlugPattern

// validateProjectName checks name against projectNamePattern. field names
// the argument in the error message (e.g. "project" or "subproject"). It
// returns an error when name does not match.
func validateProjectName(field, name string) error {
	if !projectNamePattern.MatchString(name) {
		return fmt.Errorf(
			"%q must start with a letter or digit and contain only letters, digits, hyphens, and underscores: %q",
			field, name,
		)
	}
	return nil
}

// NormalizeName returns a project or subproject name as it is stored:
// trimmed and lower-cased, so names differing only in case are the same
// project.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// ValidateTarget checks project and, when non-empty, subproject against
// the project-name rules. Every path that creates or resolves a project
// goes through it, so a name that could escape a directory when used as a
// folder name ("../x", "a/b") never reaches the vault.
func ValidateTarget(project, subproject string) error {
	if err := validateProjectName("project", project); err != nil {
		return err
	}
	if subproject != "" {
		return validateProjectName("subproject", subproject)
	}
	return nil
}
