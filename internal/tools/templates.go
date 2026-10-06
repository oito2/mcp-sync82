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
	"time"
)

// toList splits raw on commas, semicolons, or newlines, trims each piece,
// strips a leading "- " a caller may have already typed, drops empty
// pieces, and renders the result as "- item" lines. If nothing remains,
// it renders a single "-" placeholder bullet.
func toList(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})

	var items []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "- ")
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		items = append(items, "- "+p)
	}
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, "\n")
}

// initAnswers is init_project_memory's merged view of explicit tool
// arguments and analyzer-detected fields (an explicit argument wins; see
// InitProjectMemoryTool.Execute), fed into the four standard document
// templates below.
type initAnswers struct {
	Description          string
	Goal                 string
	Phase                string
	ArchitectureOverview string
	Components           string
	Languages            string
	Frameworks           string
	Infrastructure       string
	NextSteps            string
}

// standardDocumentTemplates lists the four overwrite-style documents
// init_project_memory seeds, each with the function that renders it. The
// append-only decisions and progress kinds have no template: they start
// empty.
var standardDocumentTemplates = []struct {
	kind   string
	render func(a initAnswers, label string) string
}{
	{"memory", renderMemoryTemplate},
	{"architecture", renderArchitectureTemplate},
	{"stack", renderStackTemplate},
	{"next_steps", renderNextStepsTemplate},
}

// renderMemoryTemplate renders the "memory" document from a for the
// project called label, stamping today's UTC date as the last update.
func renderMemoryTemplate(a initAnswers, label string) string {
	var b strings.Builder
	b.WriteString("# Memory\n\n")
	b.WriteString("## Overview\n")
	fmt.Fprintf(&b, "- Name: %s\n", label)
	fmt.Fprintf(&b, "- Description: %s\n", a.Description)
	fmt.Fprintf(&b, "- Goal: %s\n\n", a.Goal)
	b.WriteString("## Current Status\n")
	fmt.Fprintf(&b, "- Phase: %s\n", a.Phase)
	fmt.Fprintf(&b, "- Last updated: %s\n\n", time.Now().UTC().Format("2006-01-02"))
	b.WriteString("## Key Components\n")
	b.WriteString(toList(a.Components))
	b.WriteString("\n\n## Important Notes\n-")
	return b.String()
}

// renderArchitectureTemplate renders the "architecture" document from a.
func renderArchitectureTemplate(a initAnswers, _ string) string {
	var b strings.Builder
	b.WriteString("# Architecture\n\n")
	b.WriteString("## Overview\n")
	b.WriteString(a.ArchitectureOverview)
	b.WriteString("\n\n## Components\n")
	b.WriteString(toList(a.Components))
	b.WriteString("\n\n## Data Flow\n-\n\n## External Integrations\n-")
	return b.String()
}

// renderStackTemplate renders the "stack" document from a.
func renderStackTemplate(a initAnswers, _ string) string {
	var b strings.Builder
	b.WriteString("# Stack\n\n")
	b.WriteString("## Languages\n")
	b.WriteString(toList(a.Languages))
	b.WriteString("\n\n## Frameworks\n")
	b.WriteString(toList(a.Frameworks))
	b.WriteString("\n\n## Libraries\n-\n\n## Infrastructure\n")
	b.WriteString(toList(a.Infrastructure))
	b.WriteString("\n\n## Dev Tools\n-")
	return b.String()
}

// renderNextStepsTemplate renders the "next_steps" document from a.
func renderNextStepsTemplate(a initAnswers, _ string) string {
	var b strings.Builder
	b.WriteString("# Next Steps\n\n")
	b.WriteString("## Now\n")
	b.WriteString(toList(a.NextSteps))
	b.WriteString("\n\n## Soon\n-\n\n## Later\n-\n\n## Ideas\n-")
	return b.String()
}
