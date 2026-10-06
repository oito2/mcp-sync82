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
	"strings"
	"testing"
)

// TestValidateProjectName verifies validateProjectName directly, independent
// of the tools that call it: it accepts alphanumeric names with hyphens and
// underscores, and rejects empty names, a leading hyphen or underscore,
// spaces, slashes and non-ASCII characters.
func TestValidateProjectName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"oito2", false},
		{"sync82", false},
		{"a", false},
		{"1abc", false},
		{"under_score", false},
		{"-abc", true},
		{"_abc", true},
		{"", true},
		{"has space", true},
		{"café", true},
		{"slash/es", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateProjectName("project", c.name)
			if (err != nil) != c.wantErr {
				t.Errorf("validateProjectName(%q) error = %v, wantErr %v", c.name, err, c.wantErr)
			}
		})
	}
}

// TestValidateProjectName_ErrorNamesTheField verifies that the error message
// names the field passed by the caller, so "project" and "subproject" can be
// told apart in a validation error.
func TestValidateProjectName_ErrorNamesTheField(t *testing.T) {
	err := validateProjectName("subproject", "bad name")
	if err == nil {
		t.Fatal("expected an error for an invalid subproject name")
	}
	if got := err.Error(); !strings.Contains(got, `"subproject"`) {
		t.Errorf("error = %q, want it to mention the field name %q", got, "subproject")
	}
}
