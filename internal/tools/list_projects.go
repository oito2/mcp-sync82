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

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/oito2/mcp-sync82/internal/store"
)

// ListProjectsTool implements list_projects: list every top-level
// project, each followed by its subprojects. It doesn't target a specific
// project, so it only resolves the vault path.
type ListProjectsTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// listProjectsArgs holds the decoded arguments of the list_projects tool;
// its JSON tags match the property names declared in InputSchema.
type listProjectsArgs struct {
	Path   string `json:"path,omitempty"`
	Format string `json:"format,omitempty"`
}

// Name returns the MCP tool name, "list_projects".
func (t *ListProjectsTool) Name() string { return "list_projects" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ListProjectsTool) Description() string {
	return "List every project and subproject currently in the vault."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional properties format and path.
func (t *ListProjectsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":   map[string]any{"type": "string", "description": PathDescription},
			"format": formatProperty(),
		},
	}
}

// Validate decodes raw into listProjectsArgs and normalizes format to "text"
// or "json". It returns the arguments, or an error when format is invalid.
func (t *ListProjectsTool) Validate(raw json.RawMessage) (any, error) {
	var args listProjectsArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	format, err := validateFormat(args.Format)
	if err != nil {
		return nil, err
	}
	args.Format = format
	return args, nil
}

// Execute lists every top-level project with its subprojects in the vault. A
// vault that does not exist is reported as an empty list rather than
// created. The output is text or JSON, with the list also returned as
// structured content; store failures are returned as errors.
func (t *ListProjectsTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(listProjectsArgs)
	dbPath := t.Resolver.DBPathOrDefault(args.Path)

	s, err := t.Stores.GetExisting(ctx, dbPath)
	if errors.Is(err, store.ErrVaultNotFound) {
		if args.Format == "json" {
			return jsonResult(projectList{Vault: dbPath, Projects: []projectEntry{}}, false)
		}
		return ToolResult{Text: fmt.Sprintf("No projects found: no vault exists at %s yet. Use create_project to add one.", dbPath),
			Structured: projectList{Vault: dbPath, Projects: []projectEntry{}}}, nil
	}
	if err != nil {
		return ToolResult{}, err
	}

	tree, err := s.ProjectTree(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	if len(tree) == 0 && args.Format != "json" {
		return ToolResult{Text: "No projects found. Use create_project to add one.", Structured: projectList{Vault: dbPath, Projects: []projectEntry{}}}, nil
	}

	list := projectList{Vault: dbPath, Projects: []projectEntry{}}
	var lines []string
	for _, p := range tree {
		lines = append(lines, "- "+p.Name)
		for _, sub := range p.Subprojects {
			lines = append(lines, "  └─ "+sub)
		}
		list.Projects = append(list.Projects, projectEntry{Name: p.Name, Subprojects: p.Subprojects})
	}

	if args.Format == "json" {
		return jsonResult(list, false)
	}
	return ToolResult{Text: "Projects in vault:\n" + strings.Join(lines, "\n"), Structured: list}, nil
}

// OutputSchema returns the JSON Schema of projectList, the structured
// content of every list_projects result.
func (t *ListProjectsTool) OutputSchema() map[string]any {
	return schemaObject(map[string]any{
		"vault": schemaString(),
		"projects": schemaArray(schemaObject(map[string]any{
			"name":        schemaString(),
			"subprojects": schemaArray(schemaString()),
		}, "name", "subprojects")),
	}, "vault", "projects")
}

// projectList is list_projects' JSON result: the vault path and its
// top-level projects.
type projectList struct {
	Vault    string         `json:"vault"`
	Projects []projectEntry `json:"projects"`
}

// projectEntry is one top-level project in a projectList.
type projectEntry struct {
	Name        string   `json:"name"`
	Subprojects []string `json:"subprojects"`
}
