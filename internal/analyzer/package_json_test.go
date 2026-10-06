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
	"slices"
	"testing"
)

// TestDetectPackageJSON_TypeScriptViaDependency verifies that a typescript dependency yields the TypeScript language.
func TestDetectPackageJSON_TypeScriptViaDependency(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"description": "a TS project",
		"dependencies": {"react": "^18.0.0"},
		"devDependencies": {"typescript": "^5.0.0"}
	}`)

	var r Result
	detectPackageJSON(dir, &r)
	if r.Description != "a TS project" {
		t.Errorf("Description = %q, want %q", r.Description, "a TS project")
	}
	if !slices.Contains(r.Languages, "TypeScript") {
		t.Errorf("Languages = %v, want it to contain TypeScript", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "React") {
		t.Errorf("Frameworks = %v, want it to contain React", r.Frameworks)
	}
}

// TestDetectPackageJSON_TypeScriptViaTsconfig verifies that a tsconfig.json file yields the TypeScript language.
func TestDetectPackageJSON_TypeScriptViaTsconfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies": {}}`)
	writeFile(t, dir, "tsconfig.json", `{}`)

	var r Result
	detectPackageJSON(dir, &r)
	if !slices.Contains(r.Languages, "TypeScript") {
		t.Errorf("Languages = %v, want TypeScript (via tsconfig.json)", r.Languages)
	}
}

// TestDetectPackageJSON_JavaScriptWithoutTypeScriptSignal verifies that JavaScript is reported when there is no TypeScript signal.
func TestDetectPackageJSON_JavaScriptWithoutTypeScriptSignal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies": {"express": "^4.0.0"}}`)

	var r Result
	detectPackageJSON(dir, &r)
	if !slices.Contains(r.Languages, "JavaScript") {
		t.Errorf("Languages = %v, want JavaScript", r.Languages)
	}
	if slices.Contains(r.Languages, "TypeScript") {
		t.Errorf("Languages = %v, want no TypeScript", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Express") {
		t.Errorf("Frameworks = %v, want Express", r.Frameworks)
	}
}

// TestDetectPackageJSON_NotableLibraries verifies that notable libraries are reported as frameworks.
func TestDetectPackageJSON_NotableLibraries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"zod": "^3.0.0", "@modelcontextprotocol/sdk": "^1.0.0"}
	}`)

	var r Result
	detectPackageJSON(dir, &r)
	if !slices.Contains(r.Frameworks, "Zod") || !slices.Contains(r.Frameworks, "MCP SDK") {
		t.Errorf("Frameworks = %v, want Zod and MCP SDK", r.Frameworks)
	}
}

// TestDetectPackageJSON_NeverOverwritesDescription verifies that an already-set description is preserved.
func TestDetectPackageJSON_NeverOverwritesDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"description": "from package.json"}`)

	r := Result{Description: "from readme"}
	detectPackageJSON(dir, &r)
	if r.Description != "from readme" {
		t.Fatalf("Description = %q, want it untouched", r.Description)
	}
}

// TestDetectPackageJSON_MalformedJSONIsNoOp verifies that malformed JSON adds no signal.
func TestDetectPackageJSON_MalformedJSONIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{not valid json`)

	var r Result
	detectPackageJSON(dir, &r)
	if r.Description != "" || len(r.Languages) != 0 || len(r.Frameworks) != 0 {
		t.Fatalf("expected no signal from malformed package.json, got %+v", r)
	}
}

// TestDetectPackageJSON_MissingFileIsNoOp verifies that a missing package.json adds no signal.
func TestDetectPackageJSON_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectPackageJSON(dir, &r)
	if r.Description != "" || len(r.Languages) != 0 {
		t.Fatalf("expected no signal when package.json is absent, got %+v", r)
	}
}

// TestDetectPackageJSON_UnexpectedFieldTypesAreIgnored guards against one
// field with an unexpected type making the whole file fail to decode,
// dropping every signal.
func TestDetectPackageJSON_UnexpectedFieldTypesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"description": {"text": "not a string"},
		"dependencies": {"vue": "^3", "zod": {"x": 1}},
		"devDependencies": {"typescript": {"x": 1}}
	}`)

	var r Result
	detectPackageJSON(dir, &r)
	if r.Description != "" {
		t.Errorf("Description = %q, want empty", r.Description)
	}
	if !slices.Equal(r.Languages, []string{"TypeScript"}) {
		t.Errorf("Languages = %v, want [TypeScript]", r.Languages)
	}
	if want := []string{"Vue", "Zod"}; !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}

// TestDetectPackageJSON_NonObjectDependenciesStillAddsLanguage verifies that a non-object dependencies field is ignored while
// the language is still reported.
func TestDetectPackageJSON_NonObjectDependenciesStillAddsLanguage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"description": "kept", "dependencies": ["react"], "devDependencies": {"express": "4"}}`)

	var r Result
	detectPackageJSON(dir, &r)
	if r.Description != "kept" {
		t.Errorf("Description = %q, want %q", r.Description, "kept")
	}
	if !slices.Equal(r.Languages, []string{"JavaScript"}) {
		t.Errorf("Languages = %v, want [JavaScript]", r.Languages)
	}
	if want := []string{"Express"}; !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}
