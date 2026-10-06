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
	"slices"
	"testing"
)

// TestDetectInfrastructure verifies that marker files and directories are reported with their labels
// and that absent markers are not.
func TestDetectInfrastructure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM golang:1.26\n")
	if err := os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "turbo.json", "{}")

	var r Result
	detectInfrastructure(dir, &r)

	for _, want := range []string{"Docker", "GitHub Actions", "Turborepo"} {
		if !slices.Contains(r.Infrastructure, want) {
			t.Errorf("Infrastructure = %v, want it to contain %q", r.Infrastructure, want)
		}
	}
	if slices.Contains(r.Infrastructure, "Kubernetes") {
		t.Errorf("Infrastructure = %v, want no Kubernetes (not present)", r.Infrastructure)
	}
}

// TestDetectInfrastructure_NoMarkersIsNoOp verifies that no infrastructure is reported when no marker exists.
func TestDetectInfrastructure_NoMarkersIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectInfrastructure(dir, &r)
	if len(r.Infrastructure) != 0 {
		t.Fatalf("expected no signal, got %+v", r.Infrastructure)
	}
}

// TestDetectInfrastructure_ComposeFileNames verifies that every supported Compose file name is reported as Docker
// Compose.
func TestDetectInfrastructure_ComposeFileNames(t *testing.T) {
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, name, "services: {}\n")

			var r Result
			detectInfrastructure(dir, &r)
			if want := []string{"Docker Compose"}; !slices.Equal(r.Infrastructure, want) {
				t.Errorf("Infrastructure = %v, want %v", r.Infrastructure, want)
			}
		})
	}
}
