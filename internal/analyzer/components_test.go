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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// mkdirs creates each directory named in names under base, including parents.
// It fails the test on any I/O error.
func mkdirs(t *testing.T, base string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(base, n), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", n, err)
		}
	}
}

// TestDetectComponents_MonorepoDirsWin verifies that monorepo subdirectories take precedence over top-level
// directories.
func TestDetectComponents_MonorepoDirsWin(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, filepath.Join(dir, "packages"), "core", "cli")
	mkdirs(t, filepath.Join(dir, "apps"), "web")
	mkdirs(t, filepath.Join(dir, "packages"), "node_modules", ".hidden")
	// Unrelated top-level dirs that must NOT be used since the monorepo
	// dirs already yielded a valid (1-20) count.
	mkdirs(t, dir, "some-other-top-level-dir")

	var r Result
	detectComponents(dir, &r)

	want := map[string]bool{"packages/core": true, "packages/cli": true, "apps/web": true}
	if len(r.Components) != len(want) {
		t.Fatalf("Components = %v, want exactly %v", r.Components, want)
	}
	for _, c := range r.Components {
		if !want[c] {
			t.Errorf("unexpected component %q", c)
		}
	}
}

// TestDetectComponents_SkipsNoiseDirs verifies that hidden, dependency and build-output directories are skipped.
func TestDetectComponents_SkipsNoiseDirs(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, filepath.Join(dir, "packages"), "core", "node_modules", "dist", "build", ".git", "coverage", "tmp", "temp")

	var r Result
	detectComponents(dir, &r)

	if len(r.Components) != 1 || r.Components[0] != "packages/core" {
		t.Fatalf("Components = %v, want only [packages/core]", r.Components)
	}
}

// TestDetectComponents_FallsBackToTopLevel verifies that top-level directories are used when no monorepo directory
// exists.
func TestDetectComponents_FallsBackToTopLevel(t *testing.T) {
	dir := t.TempDir()
	// No monorepo dirs at all.
	mkdirs(t, dir, "cmd", "internal", "docs")
	mkdirs(t, dir, "node_modules") // must be skipped

	var r Result
	detectComponents(dir, &r)

	want := map[string]bool{"cmd": true, "internal": true, "docs": true}
	if len(r.Components) != len(want) {
		t.Fatalf("Components = %v, want exactly %v", r.Components, want)
	}
}

// TestDetectComponents_TooManyTopLevelDirsYieldsEmpty verifies that more top-level directories than the limit yield no
// components.
func TestDetectComponents_TooManyTopLevelDirsYieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	names := make([]string, 0, 16)
	for i := 0; i < 16; i++ {
		names = append(names, fmt.Sprintf("dir%d", i))
	}
	mkdirs(t, dir, names...)

	var r Result
	detectComponents(dir, &r)
	if len(r.Components) != 0 {
		t.Fatalf("Components = %v, want empty when top-level count exceeds 15", r.Components)
	}
}

// TestDetectComponents_NoDirectoriesAtAllYieldsEmpty verifies that a workspace without directories yields no components.
func TestDetectComponents_NoDirectoriesAtAllYieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectComponents(dir, &r)
	if len(r.Components) != 0 {
		t.Fatalf("Components = %v, want empty for a directory with no subdirectories", r.Components)
	}
}

// TestDetectComponents_SkipsVendorAndBuildDirs verifies that vendor and build directories are excluded.
func TestDetectComponents_SkipsVendorAndBuildDirs(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "src", "vendor", "target", "venv", ".venv", "bin", "obj", "__pycache__", "docs")

	var r Result
	detectComponents(dir, &r)
	if want := []string{"docs", "src"}; !slices.Equal(r.Components, want) {
		t.Fatalf("Components = %v, want %v", r.Components, want)
	}
}
