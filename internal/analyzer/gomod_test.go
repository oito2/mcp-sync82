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

// TestDetectGoMod verifies Go and framework detection from go.mod requirements.
func TestDetectGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/myapp\n\ngo 1.26\n\nrequire (\n\tgithub.com/gofiber/fiber/v2 v2.52.0\n\tgo.uber.org/zap v1.27.0\n)\n")

	var r Result
	detectGoMod(dir, &r)
	if !slices.Contains(r.Languages, "Go") {
		t.Errorf("Languages = %v, want Go", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Fiber") {
		t.Errorf("Frameworks = %v, want Fiber (version-suffixed module path)", r.Frameworks)
	}
	if !slices.Contains(r.Frameworks, "Zap") {
		t.Errorf("Frameworks = %v, want Zap", r.Frameworks)
	}
	if slices.Contains(r.Frameworks, "Gin") {
		t.Errorf("Frameworks = %v, want no Gin (not required)", r.Frameworks)
	}
}

// TestDetectGoMod_MissingFileIsNoOp verifies that a missing go.mod adds no signal.
func TestDetectGoMod_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectGoMod(dir, &r)
	if len(r.Languages) != 0 {
		t.Fatalf("expected no signal when go.mod is absent, got %+v", r)
	}
}

// TestDetectGoMod_MatchesModulePathsExactly guards against matching
// module paths as substrings, which would report a longer path sharing a
// prefix.
func TestDetectGoMod_MatchesModulePathsExactly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/myapp

go 1.26

require github.com/gin-gonic/gin-contrib-not v1.0.0
require (
	go.uber.org/zapx v1.0.0 // go.uber.org/zap
)
replace github.com/labstack/echo/v4 => ../echo
`)

	var r Result
	detectGoMod(dir, &r)
	if !slices.Equal(r.Languages, []string{"Go"}) {
		t.Errorf("Languages = %v, want [Go]", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}

// TestDetectGoMod_SingleLineAndBlockRequires verifies that both single-line and block require directives are read.
func TestDetectGoMod_SingleLineAndBlockRequires(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/myapp\n\nrequire go.uber.org/zap v1.27.0\n\nrequire (\n\tgithub.com/labstack/echo/v4 v4.12.0 // indirect\n\tgithub.com/gin-gonic/gin v1.10.0\n)\n")

	var r Result
	detectGoMod(dir, &r)
	want := []string{"Gin", "Echo", "Zap"}
	if !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}
