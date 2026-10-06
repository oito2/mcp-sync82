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

// TestDetectComposer_LanguageAndFramework verifies PHP and framework detection from composer.json.
func TestDetectComposer_LanguageAndFramework(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{
		"description": "a laravel app",
		"require": {"php": "^8.2", "laravel/framework": "^11.0"},
		"require-dev": {"phpunit/phpunit": "^10.0"}
	}`)

	var r Result
	detectComposer(dir, &r)
	if r.Description != "a laravel app" {
		t.Errorf("Description = %q, want %q", r.Description, "a laravel app")
	}
	if !slices.Contains(r.Languages, "PHP") {
		t.Errorf("Languages = %v, want it to contain PHP", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Laravel") {
		t.Errorf("Frameworks = %v, want it to contain Laravel", r.Frameworks)
	}
}

// TestDetectComposer_RequireDevOnlyStillMatches verifies that packages in require-dev are matched.
func TestDetectComposer_RequireDevOnlyStillMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{
		"require": {"php": "^8.1"},
		"require-dev": {"moodle/moodle-cs": "^2.0"}
	}`)

	var r Result
	detectComposer(dir, &r)
	if !slices.Contains(r.Frameworks, "Moodle Coding Standard") {
		t.Errorf("Frameworks = %v, want it to contain Moodle Coding Standard", r.Frameworks)
	}
}

// TestDetectComposer_NeverOverwritesDescription verifies that an already-set description is preserved.
func TestDetectComposer_NeverOverwritesDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{"description": "from composer"}`)

	r := Result{Description: "from readme"}
	detectComposer(dir, &r)
	if r.Description != "from readme" {
		t.Fatalf("Description = %q, want it untouched", r.Description)
	}
}

// TestDetectComposer_MalformedJSONIsNoOp verifies that malformed JSON adds no signal.
func TestDetectComposer_MalformedJSONIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{not valid json`)

	var r Result
	detectComposer(dir, &r)
	if r.Description != "" || len(r.Languages) != 0 || len(r.Frameworks) != 0 {
		t.Fatalf("expected no signal from malformed composer.json, got %+v", r)
	}
}

// TestDetectComposer_MissingFileIsNoOp verifies that a missing composer.json adds no signal.
func TestDetectComposer_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectComposer(dir, &r)
	if r.Description != "" || len(r.Languages) != 0 {
		t.Fatalf("expected no signal when composer.json is absent, got %+v", r)
	}
}

// TestAnalyzeProject_DetectsPHPComposerProject verifies that AnalyzeProject reports a Composer project end to end.
func TestAnalyzeProject_DetectsPHPComposerProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{
		"description": "a moodle plugin",
		"require": {"php": "^8.1", "symfony/symfony": "^6.0"}
	}`)

	r := AnalyzeProject(dir)
	if r.Description != "a moodle plugin" {
		t.Errorf("Description = %q, want %q", r.Description, "a moodle plugin")
	}
	if !slices.Contains(r.Languages, "PHP") {
		t.Errorf("Languages = %v, want PHP", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Symfony") {
		t.Errorf("Frameworks = %v, want Symfony", r.Frameworks)
	}
}

// TestDetectComposer_UnexpectedFieldTypesAreIgnored guards against one
// field with an unexpected type making the whole file fail to decode,
// dropping even the PHP language.
func TestDetectComposer_UnexpectedFieldTypesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{
		"description": 42,
		"require": "laravel/framework",
		"require-dev": {"Slim/Slim": {"x": 1}}
	}`)

	var r Result
	detectComposer(dir, &r)
	if r.Description != "" {
		t.Errorf("Description = %q, want empty", r.Description)
	}
	if !slices.Equal(r.Languages, []string{"PHP"}) {
		t.Errorf("Languages = %v, want [PHP]", r.Languages)
	}
	if want := []string{"Slim"}; !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}
