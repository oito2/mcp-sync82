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

import "testing"

// TestValidateKind verifies that validateKind accepts letters, digits, hyphens
// and underscores after a leading alphanumeric character, and rejects empty
// names, leading hyphen or underscore, spaces, slashes, dots and non-ASCII
// characters.
func TestValidateKind(t *testing.T) {
	valid := []string{"memory", "progress", "my-notes", "my_notes", "Notes2", "a"}
	for _, v := range valid {
		if _, err := validateKind(v); err != nil {
			t.Errorf("validateKind(%q) = %v, want no error", v, err)
		}
	}

	invalid := []string{"", "-leading-dash", "_leading-underscore", "has space", "has/slash", "has.dot", "emoji😀"}
	for _, v := range invalid {
		if _, err := validateKind(v); err == nil {
			t.Errorf("validateKind(%q) = nil error, want a validation error", v)
		}
	}
}

// TestIsStandardKind verifies that isStandardKind is true for every entry of
// standardKinds and false for a custom kind.
func TestIsStandardKind(t *testing.T) {
	for _, k := range standardKinds {
		if !isStandardKind(k) {
			t.Errorf("isStandardKind(%q) = false, want true", k)
		}
	}
	if isStandardKind("custom-kind") {
		t.Error("isStandardKind(\"custom-kind\") = true, want false")
	}
}

// TestIsAppendOnlyKind verifies that only progress and decisions are append-
// only kinds.
func TestIsAppendOnlyKind(t *testing.T) {
	if !isAppendOnlyKind("progress") || !isAppendOnlyKind("decisions") {
		t.Error("expected progress and decisions to be append-only")
	}
	for _, k := range []string{"memory", "architecture", "stack", "next_steps", "custom-kind"} {
		if isAppendOnlyKind(k) {
			t.Errorf("isAppendOnlyKind(%q) = true, want false", k)
		}
	}
}
