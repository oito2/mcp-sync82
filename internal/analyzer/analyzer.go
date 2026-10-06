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

// Package analyzer auto-detects a project's description, languages,
// frameworks, infrastructure, and components from its source files —
// README.md, package.json, composer.json, Cargo.toml, pyproject.toml/
// requirements.txt/setup.py, go.mod, pom.xml/build.gradle(.kts), Gemfile
// and *.csproj/*.fsproj/*.vbproj. Frameworks are detected by exact
// dependency name, and every slice in the Result has a deterministic
// order for a given set of files.
package analyzer

// Result holds everything AnalyzeProject could detect about a workspace.
// Every field starts empty; each detector fills in what it can without
// ever overwriting a field a previous detector already set (Description
// is "first non-empty wins"; the slice fields accumulate, deduplicated,
// across every detector that recognizes something).
type Result struct {
	Description    string
	Languages      []string
	Frameworks     []string
	Infrastructure []string
	Components     []string
}

// AnalyzeProject inspects the directory workspaceRoot for stack/description signals,
// running each detector in a fixed order. Description is set by the first
// detector that finds one: README.md, then package.json's, then
// composer.json's, then Cargo.toml's [package] description; the
// Python/Go/Java/Kotlin/Ruby/.NET detectors never set it. Within each
// slice field, entries appear in detector order, and within one detector
// in the order of its pattern table or of the files it scans.
//
// Every detector is a no-op — including on any I/O error, not just a
// missing marker file — since this is a best-effort convenience feature
// for init_project_memory's auto_detect, not a hard requirement; a
// permission error on one file must not prevent detecting everything
// else. It therefore never fails: the returned Result is the zero value
// when nothing is detected.
func AnalyzeProject(workspaceRoot string) Result {
	var r Result

	detectReadme(workspaceRoot, &r)
	detectPackageJSON(workspaceRoot, &r)
	detectComposer(workspaceRoot, &r)
	detectCargoToml(workspaceRoot, &r)
	detectPython(workspaceRoot, &r)
	detectGoMod(workspaceRoot, &r)
	detectJava(workspaceRoot, &r)
	detectRuby(workspaceRoot, &r)
	detectDotNet(workspaceRoot, &r)
	detectInfrastructure(workspaceRoot, &r)
	detectComponents(workspaceRoot, &r)

	return r
}
