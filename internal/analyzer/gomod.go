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

// goModulePaths is the ordered table of Go module paths reported as frameworks.
var goModulePaths = []depPattern{
	{"Gin", []string{"github.com/gin-gonic/gin"}},
	{"Echo", []string{"github.com/labstack/echo"}},
	{"Fiber", []string{"github.com/gofiber/fiber"}},
	{"Zap", []string{"go.uber.org/zap"}},
}

// goModDetector reads go.mod and reports Go plus the matching required modules.
var goModDetector = manifestDetector{
	files:    fixedFiles("go.mod"),
	language: fixedLanguage("Go"),
	deps:     func(_, content string) []string { return goRequiredModules(content) },
	patterns: goModulePaths,
}

// goMajorSuffix matches a module path's trailing "/vN" major-version
// element.
var goMajorSuffix = regexp.MustCompile(`/v[0-9]+$`)

// detectGoMod fills Languages += Go and Frameworks from the module paths
// go.mod requires, compared exactly after removing a "/vN" major-version
// suffix (so .../fiber/v2 matches .../fiber).
func detectGoMod(root string, r *Result) {
	goModDetector.detect(root, r)
}

// goRequiredModules returns the module paths listed by go.mod's require
// directives, both single-line ("require path version") and block
// ("require ( ... )") forms, with "//" comments and surrounding quotes
// removed and any "/vN" major-version suffix stripped.
func goRequiredModules(content string) []string {
	var paths []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		var path string
		switch {
		case inBlock:
			if fields[0] == ")" {
				inBlock = false
				continue
			}
			path = fields[0]
		case fields[0] == "require(" || (fields[0] == "require" && len(fields) > 1 && fields[1] == "("):
			inBlock = true
			continue
		case fields[0] == "require" && len(fields) > 1:
			path = fields[1]
		default:
			continue
		}

		path = strings.Trim(path, "\"`")
		paths = append(paths, goMajorSuffix.ReplaceAllString(path, ""))
	}
	return paths
}
