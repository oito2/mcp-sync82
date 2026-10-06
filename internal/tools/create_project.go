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
	"fmt"
	"strings"

	"github.com/oito2/mcp-sync82/internal/store"
)

// CreateProjectTool implements create_project: get-or-create a project or
// subproject, creating the parent if it doesn't exist yet. No documents or
// entries are seeded.
type CreateProjectTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// createProjectArgs holds the decoded arguments of the create_project tool;
// its JSON tags match the property names declared in InputSchema.
type createProjectArgs struct {
	Project    string `json:"project"`
	Subproject string `json:"subproject,omitempty"`
	Path       string `json:"path,omitempty"`
}

// Name returns the MCP tool name, "create_project".
func (t *CreateProjectTool) Name() string { return "create_project" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *CreateProjectTool) Description() string {
	return "Create a new project (or subproject of an existing project) in the vault. Safe to call again — an existing project is reported, not duplicated."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property project, an optional subproject and an optional
// vault path.
func (t *CreateProjectTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name (alphanumeric, hyphens, underscores)."},
			"subproject": map[string]any{"type": "string", "description": "Subproject name. When provided, creates it under \"project\" instead of at the vault root."},
			"path":       map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"project"},
	}
}

// Validate decodes raw into createProjectArgs, normalizes the names and
// checks them against the project-name rules. It returns the arguments, or
// an error listing every invalid field.
func (t *CreateProjectTool) Validate(raw json.RawMessage) (any, error) {
	var args createProjectArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	args.Project = NormalizeName(args.Project)
	args.Subproject = NormalizeName(args.Subproject)
	if args.Project == "" {
		return nil, fmt.Errorf(`"project" is required and must not be empty`)
	}

	var problems []string
	if err := validateProjectName("project", args.Project); err != nil {
		problems = append(problems, err.Error())
	}
	if args.Subproject != "" {
		if err := validateProjectName("subproject", args.Subproject); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
	}
	return args, nil
}

// Execute opens the vault at the requested path, creating it if needed, and
// ensures the project or subproject exists, creating its parent when
// necessary. The result says whether it was created or already existed;
// store failures are returned as errors.
func (t *CreateProjectTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(createProjectArgs)
	dbPath := t.Resolver.DBPathOrDefault(args.Path)

	s, err := t.Stores.Get(ctx, dbPath)
	if err != nil {
		return ToolResult{}, err
	}

	_, created, err := s.EnsureProject(ctx, args.Project, args.Subproject)
	if err != nil {
		return ToolResult{}, err
	}

	label := FormatLabel(args.Project, args.Subproject)
	if created {
		return ToolResult{Text: fmt.Sprintf("Project %q created.", label)}, nil
	}
	return ToolResult{Text: fmt.Sprintf("Project %q already exists.", label)}, nil
}
