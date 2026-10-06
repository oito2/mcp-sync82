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
	"regexp"
	"strings"
)

// dotNetPackages matches lowercased PackageReference/FrameworkReference
// Include values exactly.
var dotNetPackages = []depPattern{
	{"ASP.NET Core", []string{
		"microsoft.aspnetcore.app",
		"microsoft.aspnetcore.all",
		"microsoft.aspnetcore.openapi",
		"microsoft.aspnetcore.authentication.jwtbearer",
		"microsoft.aspnetcore.signalr",
	}},
	{"Entity Framework Core", []string{
		"microsoft.entityframeworkcore",
		"microsoft.entityframeworkcore.sqlserver",
		"microsoft.entityframeworkcore.sqlite",
		"microsoft.entityframeworkcore.inmemory",
		"microsoft.entityframeworkcore.cosmos",
		"microsoft.entityframeworkcore.design",
		"microsoft.entityframeworkcore.tools",
		"npgsql.entityframeworkcore.postgresql",
		"pomelo.entityframeworkcore.mysql",
	}},
	{"Blazor", []string{
		"microsoft.aspnetcore.components.webassembly",
		"microsoft.aspnetcore.components.webassembly.server",
		"microsoft.aspnetcore.components.webassembly.devserver",
	}},
}

// maxDotNetProjectFiles caps how many project files detectDotNet reads.
const maxDotNetProjectFiles = 32

// dotNetProjectLanguages maps a .NET project-file extension to its
// language.
var dotNetProjectLanguages = map[string]string{
	".csproj": "C#",
	".fsproj": "F#",
	".vbproj": "Visual Basic .NET",
}

// dotNetDetector reads the top-level .NET project files and reports their
// languages plus the matching package references.
var dotNetDetector = manifestDetector{
	files: dotNetProjectFiles,
	language: func(file string) string {
		return dotNetProjectLanguages[strings.ToLower(file[strings.LastIndexByte(file, '.'):])]
	},
	deps:     func(_, content string) []string { return dotNetPackageReferences(content) },
	patterns: dotNetPackages,
}

// dotNetReference matches the Include value of a PackageReference or
// FrameworkReference element.
var dotNetReference = regexp.MustCompile(`(?is)<(?:PackageReference|FrameworkReference)\b[^>]*?\bInclude\s*=\s*["']([^"']+)["']`)

// detectDotNet fills Languages (C#, F#, and/or Visual Basic .NET,
// depending on which project-file extensions are present) and Frameworks
// from the package references of root's own top-level
// *.csproj/*.fsproj/*.vbproj files, each file parsed on its own.
func detectDotNet(root string, r *Result) {
	dotNetDetector.detect(root, r)
}

// dotNetProjectFiles lists, in name order, at most maxDotNetProjectFiles
// non-directory entries of root with a .NET project-file extension.
func dotNetProjectFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		i := strings.LastIndexByte(name, '.')
		if i < 0 || dotNetProjectLanguages[strings.ToLower(name[i:])] == "" {
			continue
		}
		files = append(files, name)
		if len(files) == maxDotNetProjectFiles {
			break
		}
	}
	return files
}

// dotNetPackageReferences returns the lowercased Include values of every
// PackageReference and FrameworkReference in one project file.
func dotNetPackageReferences(content string) []string {
	var names []string
	for _, m := range dotNetReference.FindAllStringSubmatch(content, -1) {
		names = append(names, strings.ToLower(strings.TrimSpace(m[1])))
	}
	return names
}
