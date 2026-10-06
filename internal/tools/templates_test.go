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

// TestToList verifies that toList turns comma-, semicolon- or newline-
// separated text into a bullet list, re-bullets items that already start with
// a dash, drops empty pieces, and returns a "-" placeholder for blank input.
func TestToList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"empty string yields placeholder", "", "-"},
		{"whitespace only yields placeholder", "   ", "-"},
		{"comma separated", "a, b, c", "- a\n- b\n- c"},
		{"semicolon separated", "a; b; c", "- a\n- b\n- c"},
		{"newline separated", "a\nb\nc", "- a\n- b\n- c"},
		{"already bulleted items get re-bulleted not doubled", "- a, - b", "- a\n- b"},
		{"drops empty pieces from consecutive separators", "a,,b", "- a\n- b"},
		{"mixed separators", "a, b; c\nd", "- a\n- b\n- c\n- d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toList(tt.raw); got != tt.want {
				t.Errorf("toList(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestRenderMemoryTemplate verifies that renderMemoryTemplate emits the memory
// document sections with the project name and the supplied answers.
func TestRenderMemoryTemplate(t *testing.T) {
	a := initAnswers{Description: "does things", Goal: "be useful", Phase: "mvp", Components: "cli, server"}
	got := renderMemoryTemplate(a, "acme")

	for _, want := range []string{
		"# Memory", "## Overview", "- Name: acme", "- Description: does things", "- Goal: be useful",
		"## Current Status", "- Phase: mvp", "## Key Components", "- cli", "- server", "## Important Notes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderMemoryTemplate output missing %q; got:\n%s", want, got)
		}
	}
}

// TestRenderArchitectureTemplate verifies that renderArchitectureTemplate
// emits the architecture document sections with the supplied overview and
// components.
func TestRenderArchitectureTemplate(t *testing.T) {
	a := initAnswers{ArchitectureOverview: "a monolith", Components: "api"}
	got := renderArchitectureTemplate(a, "acme")
	for _, want := range []string{"# Architecture", "## Overview", "a monolith", "## Components", "- api", "## Data Flow", "## External Integrations"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderArchitectureTemplate output missing %q; got:\n%s", want, got)
		}
	}
}

// TestRenderStackTemplate verifies that renderStackTemplate emits the stack
// document sections with the supplied languages, frameworks and
// infrastructure.
func TestRenderStackTemplate(t *testing.T) {
	a := initAnswers{Languages: "Go", Frameworks: "Gin", Infrastructure: "Docker"}
	got := renderStackTemplate(a, "acme")
	for _, want := range []string{"# Stack", "## Languages", "- Go", "## Frameworks", "- Gin", "## Libraries", "## Infrastructure", "- Docker", "## Dev Tools"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderStackTemplate output missing %q; got:\n%s", want, got)
		}
	}
}

// TestRenderNextStepsTemplate verifies that renderNextStepsTemplate emits the
// next-steps document sections with the supplied steps under "Now".
func TestRenderNextStepsTemplate(t *testing.T) {
	a := initAnswers{NextSteps: "ship v1"}
	got := renderNextStepsTemplate(a, "acme")
	for _, want := range []string{"# Next Steps", "## Now", "- ship v1", "## Soon", "## Later", "## Ideas"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderNextStepsTemplate output missing %q; got:\n%s", want, got)
		}
	}
}
