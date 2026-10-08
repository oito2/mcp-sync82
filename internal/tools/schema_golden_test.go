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
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden rewrites the golden schema files under testdata from the
// current tools instead of comparing against them.
var updateGolden = flag.Bool("update", false, "rewrite the golden input schemas")

// TestInputSchemas_MatchGolden renders every registered tool's InputSchema
// and compares it with testdata/input_schemas.golden.json, so a change to
// any tool's argument schema shows up as a reviewed golden-file change.
func TestInputSchemas_MatchGolden(t *testing.T) {
	schemas := map[string]any{}
	for _, tool := range Registered(nil, nil) {
		schemas[tool.Name()] = tool.InputSchema()
	}
	compareGolden(t, "input_schemas.golden.json", schemas)
}

// TestOutputSchemas_MatchGolden does the same for the output schemas of
// the tools that declare one, in testdata/output_schemas.golden.json.
func TestOutputSchemas_MatchGolden(t *testing.T) {
	schemas := map[string]any{}
	for _, tool := range Registered(nil, nil) {
		if o, ok := tool.(OutputSchemaTool); ok {
			schemas[tool.Name()] = o.OutputSchema()
		}
	}
	compareGolden(t, "output_schemas.golden.json", schemas)
}

// compareGolden compares v, encoded as indented JSON, with the golden file
// testdata/name, or rewrites that file when the -update flag is set.
func compareGolden(t *testing.T, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run with -update to create it): %v", err)
	}
	// A checkout that converts line endings to CRLF doesn't change the
	// schemas, so the comparison ignores them.
	if string(got) != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Errorf("schemas differ from %s; run go test ./internal/tools -run Schemas_MatchGolden -update and review the diff", path)
	}
}

// TestTargetArgs_DecodedFromTheTopLevel verifies that the embedded
// targetArgs fields are read from the top level of a call's arguments and
// that unknown fields are still rejected.
func TestTargetArgs_DecodedFromTheTopLevel(t *testing.T) {
	var args writeMemoryArgs
	raw := `{"project":"acme","subproject":"api","path":"/v.db","workspace_root":"/ws","search_parent_dirs":true,"filename":"memory","content":"x"}`
	if err := decodeArgs(json.RawMessage(raw), &args); err != nil {
		t.Fatal(err)
	}
	want := ContextArgs{Project: "acme", Subproject: "api", Path: "/v.db", WorkspaceRoot: "/ws", SearchParentDirs: true}
	if got := args.contextArgs(); got != want {
		t.Errorf("contextArgs() = %+v, want %+v", got, want)
	}
	if err := decodeArgs(json.RawMessage(`{"filename":"memory","content":"x","targetArgs":{}}`), &args); err == nil {
		t.Error("an unknown field was accepted")
	}
}

// TestTargetProperties_RefusesARedefinedProperty verifies that a tool's own
// property can't silently replace a shared one.
func TestTargetProperties_RefusesARedefinedProperty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("targetProperties accepted a redefined project property")
		}
	}()
	targetProperties(targetSchema{}, map[string]any{"project": map[string]any{"type": "string"}})
}
