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

// TestDetectCargoToml verifies Rust, description and framework detection from Cargo.toml.
func TestDetectCargoToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `[package]
name = "myapp"
version = "0.1.0"
description = "a rust service"

[dependencies]
axum = "0.7"
tokio = { version = "1", features = ["full"] }
serde = "1"

[dev-dependencies]
mockall = "0.11"
`)

	var r Result
	detectCargoToml(dir, &r)

	if !slices.Contains(r.Languages, "Rust") {
		t.Errorf("Languages = %v, want Rust", r.Languages)
	}
	if r.Description != "a rust service" {
		t.Errorf("Description = %q, want %q", r.Description, "a rust service")
	}
	for _, want := range []string{"Axum", "Tokio", "Serde"} {
		if !slices.Contains(r.Frameworks, want) {
			t.Errorf("Frameworks = %v, want it to contain %q", r.Frameworks, want)
		}
	}
	if slices.Contains(r.Frameworks, "Diesel") {
		t.Errorf("Frameworks = %v, want no Diesel (not a dependency)", r.Frameworks)
	}
}

// TestDetectCargoToml_DoesNotScanOutsideDependenciesSection verifies that crate names outside dependency tables are not reported.
func TestDetectCargoToml_DoesNotScanOutsideDependenciesSection(t *testing.T) {
	// "diesel" appears only in the [package] description — it is not a
	// dependency key, so it must not be reported.
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `[package]
name = "myapp"
description = "not using diesel here"

[dependencies]
axum = "0.7"
`)

	var r Result
	detectCargoToml(dir, &r)
	if slices.Contains(r.Frameworks, "Diesel") {
		t.Errorf("Frameworks = %v, want Diesel not detected from outside [dependencies]", r.Frameworks)
	}
}

// TestDetectCargoToml_DescriptionScopedToPackageSection verifies that only the [package] description is used.
func TestDetectCargoToml_DescriptionScopedToPackageSection(t *testing.T) {
	// A "description" key inside another table (e.g. tool metadata) must
	// not be picked up as the project's own [package] description.
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `[package]
name = "myapp"
version = "0.1.0"

[package.metadata.somenttool]
description = "not the real project description"

[dependencies]
axum = "0.7"
`)

	var r Result
	detectCargoToml(dir, &r)
	if r.Description != "" {
		t.Fatalf("Description = %q, want empty — line lives outside [package]", r.Description)
	}
}

// TestDetectCargoToml_NeverOverwritesDescription verifies that an already-set description is preserved.
func TestDetectCargoToml_NeverOverwritesDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `description = "from cargo"`+"\n")

	r := Result{Description: "from readme"}
	detectCargoToml(dir, &r)
	if r.Description != "from readme" {
		t.Fatalf("Description = %q, want it untouched", r.Description)
	}
}

// TestDetectCargoToml_MissingFileIsNoOp verifies that a missing Cargo.toml adds no signal.
func TestDetectCargoToml_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectCargoToml(dir, &r)
	if len(r.Languages) != 0 {
		t.Fatalf("expected no signal when Cargo.toml is absent, got %+v", r)
	}
}

// TestDetectCargoToml_MatchesDependencyKeysOnly guards against reporting a
// crate name that only appears inside another dependency's value or as
// part of a longer crate name.
func TestDetectCargoToml_MatchesDependencyKeysOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `[package]
name = "myapp"

[dependencies]
chrono = { version = "0.4", features = ["serde"] }
tokio-postgres = "0.7"
`)

	var r Result
	detectCargoToml(dir, &r)
	if !slices.Equal(r.Languages, []string{"Rust"}) {
		t.Errorf("Languages = %v, want [Rust]", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}

// TestDetectCargoToml_AllDependencyTables verifies that crates are read from every dependency table form,
// including workspace and target tables.
func TestDetectCargoToml_AllDependencyTables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", `[package]
name = "myapp"
description = "not using diesel here"

[workspace.dependencies]
"serde" = "1"

[dev-dependencies]
tokio = { version = "1", features = [
    "full",
] }

[build-dependencies]
sqlx.workspace = true

[target.'cfg(unix)'.dependencies]
rocket = "0.5"

[dependencies.axum]
version = "0.7"
diesel = "ignored: key inside the axum table"

[package.metadata.x]
actix-web = "not a dependency"
`)

	var r Result
	detectCargoToml(dir, &r)
	want := []string{"Axum", "Rocket", "Tokio", "Serde", "sqlx"}
	if !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}

// TestDetectCargoToml_SingleQuotedDescription verifies that a single-quoted description is read.
func TestDetectCargoToml_SingleQuotedDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = 'x'\ndescription = 'single quoted'\n")

	var r Result
	detectCargoToml(dir, &r)
	if r.Description != "single quoted" {
		t.Errorf("Description = %q, want %q", r.Description, "single quoted")
	}
}
