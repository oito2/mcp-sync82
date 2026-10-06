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
	"os"
	"path/filepath"
	"strings"
)

// monorepoDirs lists the top-level directories whose subdirectories are
// treated as monorepo components.
var monorepoDirs = []string{"packages", "apps", "libs", "modules"}

// skipComponentDirs holds the dependency, build-output, and cache directory
// names that are never reported as components.
var skipComponentDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	"tmp":          true,
	"temp":         true,
	"vendor":       true,
	"target":       true,
	"venv":         true,
	"bin":          true,
	"obj":          true,
	"__pycache__":  true,
}

// shouldSkipComponentDir reports whether a directory named name is
// excluded from Components: hidden directories (including .venv) and the
// dependency, build-output, and cache directories in skipComponentDirs.
func shouldSkipComponentDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return skipComponentDirs[name]
}

// detectComponents fills Components by listing the immediate
// subdirectories of any monorepo package directory (packages/, apps/,
// libs/, modules/) present at root, as "dir/name" entries — used only if
// that yields 1-20 entries. Otherwise falls back to the workspace root's
// own top-level directories, used only if there are 1-15 of them; if
// neither condition holds, Components stays empty.
func detectComponents(root string, r *Result) {
	var monorepoEntries []string
	for _, dir := range monorepoDirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || shouldSkipComponentDir(e.Name()) {
				continue
			}
			monorepoEntries = append(monorepoEntries, dir+"/"+e.Name())
		}
	}

	if len(monorepoEntries) >= 1 && len(monorepoEntries) <= 20 {
		r.Components = monorepoEntries
		return
	}

	topEntries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var topLevel []string
	for _, e := range topEntries {
		if !e.IsDir() || shouldSkipComponentDir(e.Name()) {
			continue
		}
		topLevel = append(topLevel, e.Name())
	}
	if len(topLevel) >= 1 && len(topLevel) <= 15 {
		r.Components = topLevel
	}
}
