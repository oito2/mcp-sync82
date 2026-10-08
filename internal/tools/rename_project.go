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

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// RenameProjectTool implements rename_project: rename a top-level project
// or a specific subproject in place. Only the vault and the global
// config's last-used project change — any .sync82.json elsewhere on disk
// that points at the old name keeps doing so until re-initialized or
// edited by hand.
type RenameProjectTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// renameProjectArgs holds the decoded arguments of the rename_project tool;
// its JSON tags match the property names declared in InputSchema.
type renameProjectArgs struct {
	Project    string `json:"project"`
	Subproject string `json:"subproject,omitempty"`
	NewName    string `json:"new_name"`
	Path       string `json:"path,omitempty"`
}

// Name returns the MCP tool name, "rename_project".
func (t *RenameProjectTool) Name() string { return "rename_project" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *RenameProjectTool) Description() string {
	return "Rename a project or subproject in place. Does not update any .sync82.json elsewhere that already points at the old name — those must be re-initialized (init_project_memory) or edited by hand afterward."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties project and new_name, an optional subproject
// and an optional vault path.
func (t *RenameProjectTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "The project to rename. When \"subproject\" is also given, renames that subproject instead of the top-level project."},
			"subproject": map[string]any{"type": "string", "description": "The subproject to rename, if renaming a subproject rather than the top-level project."},
			"new_name":   map[string]any{"type": "string", "description": "The new name (alphanumeric, hyphens, underscores)."},
			"path":       map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"project", "new_name"},
	}
}

// Validate decodes raw into renameProjectArgs and normalizes the names. It
// returns the arguments, or an error listing every invalid or missing field.
func (t *RenameProjectTool) Validate(raw json.RawMessage) (any, error) {
	var args renameProjectArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	args.Project = NormalizeName(args.Project)
	args.Subproject = NormalizeName(args.Subproject)
	args.NewName = NormalizeName(args.NewName)

	var problems []string
	if args.Project == "" {
		problems = append(problems, `"project" is required and must not be empty`)
	} else if err := validateProjectName("project", args.Project); err != nil {
		problems = append(problems, err.Error())
	}
	if args.Subproject != "" {
		if err := validateProjectName("subproject", args.Subproject); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if args.NewName == "" {
		problems = append(problems, `"new_name" is required and must not be empty`)
	} else if err := validateProjectName("new_name", args.NewName); err != nil {
		problems = append(problems, err.Error())
	} else if args.NewName == firstNonEmpty(args.Subproject, args.Project) {
		problems = append(problems, fmt.Sprintf(`"new_name" is the current name, %q`, args.NewName))
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
	}
	return args, nil
}

// Execute renames the project, or the subproject when one is given, and
// keeps the global config's last-used project in step. A name collision or a
// missing vault yields an error result; other failures are returned as
// errors.
func (t *RenameProjectTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(renameProjectArgs)
	dbPath := t.Resolver.DBPathOrDefault(args.Path)

	s, err := t.Stores.GetExisting(ctx, dbPath)
	if errors.Is(err, store.ErrVaultNotFound) {
		return vaultMissingResult(dbPath), nil
	}
	if err != nil {
		return ToolResult{}, err
	}

	oldLabel := FormatLabel(args.Project, args.Subproject)
	if err := s.RenameProject(ctx, args.Project, args.Subproject, args.NewName); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			// A name collision is an outcome the agent can act on by
			// retrying with a different new_name.
			return ToolResult{Text: err.Error(), IsError: true}, nil
		}
		return ToolResult{}, projectNotFound(err, oldLabel)
	}

	t.followRenameInLastProject(args, dbPath)

	newLabel := args.NewName
	if args.Subproject != "" {
		newLabel = FormatLabel(args.Project, args.NewName)
	}
	return ToolResult{Text: fmt.Sprintf("Renamed %q to %q.", oldLabel, newLabel)}, nil
}

// followRenameInLastProject updates the global config's last-used project
// when it named the project or subproject just renamed in dbPath, so a
// later call resolved from it targets the new name instead of recreating
// the old one. Failures are logged and never block the rename result.
func (t *RenameProjectTool) followRenameInLastProject(args renameProjectArgs, dbPath string) {
	err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) {
		if c.LastProject != args.Project || (c.LastVaultPath != "" && !samePath(c.LastVaultPath, dbPath)) {
			return
		}
		switch {
		case args.Subproject == "":
			c.LastProject = args.NewName
		case c.LastSubproject == args.Subproject:
			c.LastSubproject = args.NewName
		}
	})
	if err != nil {
		t.Resolver.Logger.Error("update last project after rename", "error", err)
	}
}
