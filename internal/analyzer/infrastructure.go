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
)

// infrastructureMarkers maps a path relative to the workspace root (file or
// directory) to the infrastructure label reported when that path exists.
var infrastructureMarkers = []struct {
	path  string
	label string
}{
	{"Dockerfile", "Docker"},
	{"docker-compose.yml", "Docker Compose"},
	{"docker-compose.yaml", "Docker Compose"},
	{"compose.yaml", "Docker Compose"},
	{"compose.yml", "Docker Compose"},
	{".github/workflows", "GitHub Actions"},
	{"terraform", "Terraform"},
	{"k8s", "Kubernetes"},
	{"kubernetes", "Kubernetes"},
	{".gitlab-ci.yml", "GitLab CI"},
	{"Pulumi.yaml", "Pulumi"},
	{"turbo.json", "Turborepo"},
	{"nx.json", "Nx"},
	{"lerna.json", "Lerna"},
	{"pnpm-workspace.yaml", "pnpm workspaces"},
	{"pnpm-workspace.yml", "pnpm workspaces"},
}

// detectInfrastructure fills Infrastructure from the presence of common
// CI/deployment/monorepo-tooling marker files and directories (Dockerfile,
// docker-compose/compose, GitHub Actions, Terraform, Kubernetes, GitLab CI, Pulumi,
// Turborepo, Nx, Lerna, pnpm workspaces). Each marker is checked purely by
// path existence — no file content is inspected — so it does not depend on
// any language-detection results and can run in any order relative to them.
func detectInfrastructure(root string, r *Result) {
	for _, m := range infrastructureMarkers {
		if _, err := os.Stat(filepath.Join(root, m.path)); err == nil {
			appendUnique(&r.Infrastructure, m.label)
		}
	}
}
