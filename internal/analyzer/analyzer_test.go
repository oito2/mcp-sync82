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
	"reflect"
	"slices"
	"testing"
)

// TestAnalyzeProject_ReadmeDescriptionBeatsPackageJSON verifies that the README description takes precedence over the
// package.json description.
func TestAnalyzeProject_ReadmeDescriptionBeatsPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# App\nDescription from the README.\n")
	writeFile(t, dir, "package.json", `{"description": "description from package.json", "dependencies": {"react": "^18"}}`)

	r := AnalyzeProject(dir)
	if r.Description != "Description from the README." {
		t.Fatalf("Description = %q, want the README's description to win", r.Description)
	}
	if !slices.Contains(r.Frameworks, "React") {
		t.Errorf("Frameworks = %v, want React (package.json signals still apply)", r.Frameworks)
	}
}

// TestAnalyzeProject_PolyglotAccumulatesLanguagesAcrossDetectors verifies that languages from several manifests accumulate
// in the Result.
func TestAnalyzeProject_PolyglotAccumulatesLanguagesAcrossDetectors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeFile(t, dir, "package.json", `{"dependencies": {}}`)

	r := AnalyzeProject(dir)
	if !slices.Contains(r.Languages, "Go") || !slices.Contains(r.Languages, "JavaScript") {
		t.Fatalf("Languages = %v, want both Go and JavaScript detected", r.Languages)
	}
}

// TestAnalyzeProject_JavaRubyDotNetAreWiredIn verifies that the Java, Ruby and .NET detectors run as part of
// AnalyzeProject.
func TestAnalyzeProject_JavaRubyDotNetAreWiredIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", "<project></project>")
	writeFile(t, dir, "Gemfile", "gem 'rails'\n")
	writeFile(t, dir, "App.csproj", `<Project Sdk="Microsoft.NET.Sdk"></Project>`)

	r := AnalyzeProject(dir)
	for _, want := range []string{"Java", "Ruby", "C#"} {
		if !slices.Contains(r.Languages, want) {
			t.Errorf("Languages = %v, want it to contain %q", r.Languages, want)
		}
	}
}

// TestAnalyzeProject_EmptyDirectoryYieldsZeroValueResult verifies that an empty directory produces a zero-value Result.
func TestAnalyzeProject_EmptyDirectoryYieldsZeroValueResult(t *testing.T) {
	dir := t.TempDir()
	r := AnalyzeProject(dir)
	if r.Description != "" || len(r.Languages) != 0 || len(r.Frameworks) != 0 ||
		len(r.Infrastructure) != 0 || len(r.Components) != 0 {
		t.Fatalf("expected a zero-value Result for an empty directory, got %+v", r)
	}
}

// TestAnalyzeProject_FullSignalMix verifies the combined Result for a workspace with description, language,
// framework, infrastructure and component signals.
func TestAnalyzeProject_FullSignalMix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# sync82\nAn MCP server that keeps project memory.\n")
	writeFile(t, dir, "go.mod", "module github.com/oito2/mcp-sync82\n\ngo 1.26\n")
	writeFile(t, dir, "Dockerfile", "FROM golang:1.26\n")
	mkdirs(t, dir, "cmd", "internal")

	r := AnalyzeProject(dir)
	if r.Description != "An MCP server that keeps project memory." {
		t.Errorf("Description = %q", r.Description)
	}
	if !slices.Contains(r.Languages, "Go") {
		t.Errorf("Languages = %v, want Go", r.Languages)
	}
	if !slices.Contains(r.Infrastructure, "Docker") {
		t.Errorf("Infrastructure = %v, want Docker", r.Infrastructure)
	}
	want := map[string]bool{"cmd": true, "internal": true}
	if len(r.Components) != len(want) {
		t.Errorf("Components = %v, want %v", r.Components, want)
	}
}

// TestAnalyzeProject_DeterministicOrder checks that the order of
// Frameworks is the same on every run, even though detectors may range
// over maps.
func TestAnalyzeProject_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"description": "Polyglot fixture",
		"dependencies": {"openai": "1", "zod": "1", "react": "1", "express": "1",
			"@prisma/client": "1", "next": "1", "hono": "1", "axios": "1"},
		"devDependencies": {"typescript": "5"}
	}`)
	writeFile(t, dir, "composer.json", `{"require": {"slim/slim": "4", "laravel/framework": "11"}}`)
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"x\"\n\n[dependencies]\nserde = \"1\"\ntokio = \"1\"\naxum = \"0.7\"\n")
	writeFile(t, dir, "requirements.txt", "sqlalchemy\nflask\nfastapi\n")
	writeFile(t, dir, "go.mod", "module example.com/x\n\nrequire (\n\tgo.uber.org/zap v1.27.0\n\tgithub.com/gin-gonic/gin v1.10.0\n)\n")
	writeFile(t, dir, "build.gradle.kts", "plugins {\n    id(\"org.springframework.boot\") version \"3.2.0\"\n}\ndependencies {\n    implementation(\"io.ktor:ktor-server-core:2.3.0\")\n}\n")
	writeFile(t, dir, "Gemfile", "gem 'hanami'\ngem 'sinatra'\n")
	writeFile(t, dir, "App.csproj", `<Project><ItemGroup>
<PackageReference Include="Microsoft.EntityFrameworkCore" Version="8.0.0" />
<FrameworkReference Include="Microsoft.AspNetCore.App" />
</ItemGroup></Project>`)
	writeFile(t, dir, "Dockerfile", "FROM scratch\n")
	writeFile(t, dir, "compose.yaml", "services: {}\n")
	mkdirs(t, dir, "web", "api")

	want := Result{
		Description: "Polyglot fixture",
		Languages:   []string{"TypeScript", "PHP", "Rust", "Python", "Go", "Kotlin", "Ruby", "C#"},
		Frameworks: []string{
			"React", "Next.js", "Express", "Hono",
			"Zod", "Prisma", "Axios", "OpenAI SDK",
			"Laravel", "Slim",
			"Axum", "Tokio", "Serde",
			"FastAPI", "Flask", "SQLAlchemy",
			"Gin", "Zap",
			"Spring Boot", "Ktor",
			"Sinatra", "Hanami",
			"ASP.NET Core", "Entity Framework Core",
		},
		Infrastructure: []string{"Docker", "Docker Compose"},
		Components:     []string{"api", "web"},
	}

	for i := range 20 {
		if got := AnalyzeProject(dir); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: AnalyzeProject =\n%+v\nwant\n%+v", i, got, want)
		}
	}
}
