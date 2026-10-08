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
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

// resolveOutputSchema returns tool's output schema, resolved for
// validation. It fails the test when the schema is not a valid JSON Schema
// object.
func resolveOutputSchema(t *testing.T, tool OutputSchemaTool) *jsonschema.Resolved {
	t.Helper()
	data, err := json.Marshal(tool.OutputSchema())
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("%s output schema: %v", tool.Name(), err)
	}
	if schema.Type != "object" {
		t.Fatalf("%s output schema type = %q, want object", tool.Name(), schema.Type)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("%s output schema does not resolve: %v", tool.Name(), err)
	}
	return resolved
}

// checkStructured runs tool with args and checks that the result carries
// structured content valid against the tool's output schema, as JSON.
func checkStructured(t *testing.T, tool OutputSchemaTool, args map[string]any) {
	t.Helper()
	res := runTool(t, tool, args)
	if res.Structured == nil {
		t.Fatalf("%s %v: no structured content (text %q)", tool.Name(), args, res.Text)
	}
	data, err := json.Marshal(res.Structured)
	if err != nil {
		t.Fatal(err)
	}
	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		t.Fatal(err)
	}
	if err := resolveOutputSchema(t, tool).Validate(instance); err != nil {
		t.Errorf("%s %v: structured content %s does not match the output schema: %v", tool.Name(), args, data, err)
	}
}

// TestOutputSchemas_StructuredContentConforms runs every tool with an
// output schema through its result branches — missing and empty vault,
// empty and populated project, no and many results, text and JSON,
// warnings and the vault-wide health check — and validates each
// structured content against the declared schema.
func TestOutputSchemas_StructuredContentConforms(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	listProjects := &ListProjectsTool{Resolver: r, Stores: mgr}
	listFiles := &ListFilesTool{Resolver: r, Stores: mgr}
	search := &SearchMemoryTool{Resolver: r, Stores: mgr}
	health := &CheckProjectHealthTool{Resolver: r, Stores: mgr}

	checkStructured(t, listProjects, map[string]any{"path": filepath.Join(t.TempDir(), "none.db")})
	checkStructured(t, listProjects, map[string]any{})
	checkStructured(t, health, map[string]any{"all_projects": true})

	ctx := context.Background()
	s := seedHealthyProject(t, r, mgr, time.Now().UTC().Format("2006-01-02"))
	if _, _, err := s.EnsureProject(ctx, "empty", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "stack", ""); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if err := s.AppendEntry(ctx, "acme", "", "progress", "", "undated note about the installer"); err != nil {
			t.Fatal(err)
		}
	}

	for _, format := range []string{"text", "json"} {
		checkStructured(t, listProjects, map[string]any{"format": format})
		checkStructured(t, listFiles, map[string]any{"project": "acme", "format": format})
		checkStructured(t, listFiles, map[string]any{"project": "acme", "metadata": true, "format": format})
		checkStructured(t, listFiles, map[string]any{"project": "empty", "format": format})
		checkStructured(t, search, map[string]any{"query": "nothing-matches-this", "format": format})
		checkStructured(t, search, map[string]any{"query": "installer", "limit": 2, "context_lines": 1, "format": format})
		checkStructured(t, search, map[string]any{"query": "installer", "match": "exact", "format": format})
		checkStructured(t, health, map[string]any{"project": "acme", "format": format})
		checkStructured(t, health, map[string]any{"project": "empty", "format": format})
		checkStructured(t, health, map[string]any{"all_projects": true, "format": format})
	}
}

// TestOutputSchemas_RejectNonConformingContent makes sure the validation
// above can fail: a report with a wrongly typed field is rejected.
func TestOutputSchemas_RejectNonConformingContent(t *testing.T) {
	resolved := resolveOutputSchema(t, &CheckProjectHealthTool{})
	if err := resolved.Validate(map[string]any{"vault": 1, "healthy": true}); err == nil {
		t.Error("a numeric vault was accepted")
	}
	if err := resolved.Validate(map[string]any{"vault": "/v.db"}); err == nil {
		t.Error("a report without healthy was accepted")
	}
}
