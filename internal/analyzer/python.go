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
	"regexp"
	"strings"
)

// pythonPackages is the ordered table of normalized Python package names
// reported as frameworks.
var pythonPackages = []depPattern{
	{"FastAPI", []string{"fastapi"}},
	{"Django", []string{"django"}},
	{"Flask", []string{"flask"}},
	{"Starlette", []string{"starlette"}},
	{"Pydantic", []string{"pydantic"}},
	{"SQLAlchemy", []string{"sqlalchemy"}},
}

// pythonDetector reads pyproject.toml, requirements.txt and setup.py.
var pythonDetector = manifestDetector{
	files:    fixedFiles("pyproject.toml", "requirements.txt", "setup.py"),
	language: fixedLanguage("Python"),
	deps:     pythonDependencyNames,
	patterns: pythonPackages,
}

// pythonArrayKey matches a TOML or Python assignment of a list literal,
// capturing the key name; the match ends right after the opening "[".
var pythonArrayKey = regexp.MustCompile(`^\s*["']?([A-Za-z0-9_.\-]+)["']?\s*=\s*\[`)

// setupInstallRequires matches the start of an install_requires list in
// setup.py; the match ends right after the opening "[".
var setupInstallRequires = regexp.MustCompile(`\binstall_requires\s*=\s*\[`)

// detectPython fills Languages += Python when any of pyproject.toml,
// requirements.txt, or setup.py is present, and Frameworks from the
// normalized package names they declare as requirements.
func detectPython(root string, r *Result) {
	pythonDetector.detect(root, r)
}

// pythonDependencyNames returns the normalized requirement names declared
// in one Python manifest file.
func pythonDependencyNames(file, content string) []string {
	var reqs []string
	switch file {
	case "requirements.txt":
		for _, line := range strings.Split(content, "\n") {
			if i := strings.IndexByte(line, '#'); i >= 0 {
				line = line[:i]
			}
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "-") {
				continue
			}
			reqs = append(reqs, line)
		}
	case "pyproject.toml":
		reqs = pyprojectRequirements(content)
	case "setup.py":
		if loc := setupInstallRequires.FindStringIndex(content); loc != nil {
			reqs, _ = quotedStrings(content[loc[1]:])
		}
	}

	var names []string
	for _, req := range reqs {
		if name := pythonRequirementName(req); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// pyprojectRequirements returns the requirement strings and package names
// declared in a pyproject.toml: the "dependencies" array of [project],
// every array of [project.optional-dependencies] and [dependency-groups],
// and every key of a Poetry dependencies table
// ([tool.poetry.dependencies], [tool.poetry.dev-dependencies],
// [tool.poetry.group.<name>.dependencies]).
func pyprojectRequirements(content string) []string {
	var reqs []string
	section := ""
	lines := strings.Split(content, "\n")
	offset := 0 // byte offset of lines[i] in content
	for i := 0; i < len(lines); offset, i = offset+len(lines[i])+1, i+1 {
		line := lines[i]
		if header, ok := tomlHeader(line); ok {
			section = header
			continue
		}

		if isPoetryDepTable(section) {
			if key, ok := tomlKey(line); ok && key != "python" {
				reqs = append(reqs, key)
			}
			continue
		}

		m := pythonArrayKey.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		key := line[m[2]:m[3]]
		wanted := (section == "project" && key == "dependencies") ||
			section == "project.optional-dependencies" ||
			section == "dependency-groups"

		rest := content[offset+m[1]:]
		items, end := quotedStrings(rest)
		if wanted {
			reqs = append(reqs, items...)
		}
		for range strings.Count(rest[:end], "\n") {
			offset += len(lines[i]) + 1
			i++
		}
	}
	return reqs
}

// isPoetryDepTable reports whether section is a Poetry table whose keys
// are package names.
func isPoetryDepTable(section string) bool {
	if section == "tool.poetry.dependencies" || section == "tool.poetry.dev-dependencies" {
		return true
	}
	return strings.HasPrefix(section, "tool.poetry.group.") && strings.HasSuffix(section, ".dependencies")
}

// pythonRequirementName returns the normalized package name of a
// requirement string: the text before the first of "[<>=~!; @(,", or
// whitespace, lowercased, with "_" and "." replaced by "-".
func pythonRequirementName(req string) string {
	req = strings.TrimSpace(req)
	if i := strings.IndexAny(req, "[<>=~!; @(,\t"); i >= 0 {
		req = req[:i]
	}
	req = strings.ToLower(req)
	return strings.NewReplacer("_", "-", ".", "-").Replace(req)
}
