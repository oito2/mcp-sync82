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
	"slices"
	"testing"
)

// TestDetectDotNet_ViaCsproj verifies C# and framework detection from a .csproj file.
func TestDetectDotNet_ViaCsproj(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyApp.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" />
    <PackageReference Include="Microsoft.EntityFrameworkCore" Version="8.0.0" />
  </ItemGroup>
</Project>`)

	var r Result
	detectDotNet(dir, &r)
	if !slices.Contains(r.Languages, "C#") {
		t.Errorf("Languages = %v, want C#", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "ASP.NET Core") {
		t.Errorf("Frameworks = %v, want ASP.NET Core", r.Frameworks)
	}
	if !slices.Contains(r.Frameworks, "Entity Framework Core") {
		t.Errorf("Frameworks = %v, want Entity Framework Core", r.Frameworks)
	}
	if slices.Contains(r.Frameworks, "Blazor") {
		t.Errorf("Frameworks = %v, want no Blazor (not referenced)", r.Frameworks)
	}
}

// TestDetectDotNet_ViaFsproj verifies F# detection from a .fsproj file.
func TestDetectDotNet_ViaFsproj(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyApp.fsproj", `<Project Sdk="Microsoft.NET.Sdk"></Project>`)

	var r Result
	detectDotNet(dir, &r)
	if !slices.Contains(r.Languages, "F#") {
		t.Errorf("Languages = %v, want F#", r.Languages)
	}
	if slices.Contains(r.Languages, "C#") {
		t.Errorf("Languages = %v, want no C# from a .fsproj alone", r.Languages)
	}
}

// TestDetectDotNet_MissingProjectFileIsNoOp verifies that no project file adds no signal.
func TestDetectDotNet_MissingProjectFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectDotNet(dir, &r)
	if len(r.Languages) != 0 {
		t.Fatalf("expected no signal when no .csproj/.fsproj/.vbproj exists, got %+v", r)
	}
}

// TestDetectDotNet_MatchesPackageReferencesExactly guards against matching
// package names as substrings of the whole lowercased file.
func TestDetectDotNet_MatchesPackageReferencesExactly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "App.csproj", `<Project Sdk="Microsoft.NET.Sdk">
  <!-- Microsoft.EntityFrameworkCore -->
  <PropertyGroup><Description>Blazor-like Microsoft.AspNetCore clone</Description></PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Blazored.LocalStorage" Version="4.5.0" />
    <PackageReference Include="Microsoft.AspNetCore.Something.Else" />
  </ItemGroup>
</Project>`)

	var r Result
	detectDotNet(dir, &r)
	if !slices.Equal(r.Languages, []string{"C#"}) {
		t.Errorf("Languages = %v, want [C#]", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}

// TestDetectDotNet_EachProjectFileCaseInsensitive verifies that project-file extensions are matched case-insensitively
// and each file is parsed on its own.
func TestDetectDotNet_EachProjectFileCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "A.fsproj", `<Project><ItemGroup><packagereference include="microsoft.entityframeworkcore.sqlite" /></ItemGroup></Project>`)
	writeFile(t, dir, "B.csproj", `<Project><ItemGroup>
<PackageReference
    Include="Microsoft.AspNetCore.Components.WebAssembly"
    Version="8.0.0" />
<FrameworkReference Include="Microsoft.AspNetCore.App" />
</ItemGroup></Project>`)

	var r Result
	detectDotNet(dir, &r)
	if want := []string{"F#", "C#"}; !slices.Equal(r.Languages, want) {
		t.Errorf("Languages = %v, want %v", r.Languages, want)
	}
	if want := []string{"ASP.NET Core", "Entity Framework Core", "Blazor"}; !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}

// TestDetectDotNet_ReadsAtMost32ProjectFiles verifies that project files beyond the cap are not read.
func TestDetectDotNet_ReadsAtMost32ProjectFiles(t *testing.T) {
	dir := t.TempDir()
	for i := range maxDotNetProjectFiles {
		writeFile(t, dir, fmt.Sprintf("P%02d.csproj", i), "<Project></Project>")
	}
	writeFile(t, dir, "Z.fsproj", `<Project><ItemGroup><PackageReference Include="Microsoft.EntityFrameworkCore" /></ItemGroup></Project>`)

	var r Result
	detectDotNet(dir, &r)
	if !slices.Equal(r.Languages, []string{"C#"}) {
		t.Errorf("Languages = %v, want [C#] (the 33rd project file is not read)", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}
