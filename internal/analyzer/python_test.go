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

// TestDetectPython_ViaPyprojectToml verifies Python and framework detection from pyproject.toml.
func TestDetectPython_ViaPyprojectToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"myapp\"\ndependencies = [\"fastapi\", \"pydantic\"]\n")

	var r Result
	detectPython(dir, &r)
	if !slices.Contains(r.Languages, "Python") {
		t.Errorf("Languages = %v, want Python", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "FastAPI") || !slices.Contains(r.Frameworks, "Pydantic") {
		t.Errorf("Frameworks = %v, want FastAPI and Pydantic", r.Frameworks)
	}
}

// TestDetectPython_ViaRequirementsTxt verifies Python and framework detection from requirements.txt,
// including case-insensitive matching.
func TestDetectPython_ViaRequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "Django==4.2\nsqlalchemy==2.0\n")

	var r Result
	detectPython(dir, &r)
	if !slices.Contains(r.Languages, "Python") {
		t.Errorf("Languages = %v, want Python", r.Languages)
	}
	// Case-insensitive: "Django" in the file must still match "django".
	if !slices.Contains(r.Frameworks, "Django") || !slices.Contains(r.Frameworks, "SQLAlchemy") {
		t.Errorf("Frameworks = %v, want Django and SQLAlchemy", r.Frameworks)
	}
}

// TestDetectPython_MissingAllMarkersIsNoOp verifies that detectPython changes nothing when no Python manifest exists.
func TestDetectPython_MissingAllMarkersIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectPython(dir, &r)
	if len(r.Languages) != 0 {
		t.Fatalf("expected no signal when no Python marker file exists, got %+v", r)
	}
}

// TestDetectPython_MatchesRequirementNamesOnly guards against matching
// package names as substrings of the whole file, comments included.
func TestDetectPython_MatchesRequirementNamesOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "# migrated away from flask\nflask-cors==4.0\nflask_sqlalchemy>=3\n-r django-requirements.txt\n")

	var r Result
	detectPython(dir, &r)
	if !slices.Equal(r.Languages, []string{"Python"}) {
		t.Errorf("Languages = %v, want [Python]", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}

// TestDetectPython_RequirementNameExtraction verifies that requirement names are extracted and normalized from
// version specifiers, extras, markers and URL references.
func TestDetectPython_RequirementNameExtraction(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "SQLAlchemy[asyncio]>=2.0  # orm\nPydantic~=2.0\nstarlette ; python_version > '3.8'\n")
	writeFile(t, dir, "pyproject.toml", `[project]
name = "myapp"
description = "uses flask? no"
dependencies = [
    "fastapi>=0.110",  # web
    'uvicorn[standard]',
]
classifiers = ["Framework :: Django"]

[project.optional-dependencies]
web = ["Flask==3.0"]

[tool.poetry.dependencies]
python = "^3.12"
Django = "^5.0"
`)
	writeFile(t, dir, "setup.py", `setup(name="x", install_requires=["fastapi", 'Flask>=2'], keywords=["django"])`)

	var r Result
	detectPython(dir, &r)
	want := []string{"FastAPI", "Django", "Flask", "Starlette", "Pydantic", "SQLAlchemy"}
	if !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}

// TestDetectPython_IgnoresUnrelatedArraysAndSections verifies that arrays and sections that do not declare requirements are
// not reported as dependencies.
func TestDetectPython_IgnoresUnrelatedArraysAndSections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `[project]
name = "myapp"
keywords = ["flask", "django"]

[tool.other]
dependencies = ["fastapi"]
`)
	writeFile(t, dir, "setup.py", `setup(name="x", keywords=["pydantic"])`)

	var r Result
	detectPython(dir, &r)
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}
