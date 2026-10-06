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

// DeleteProjectTool implements delete_project: delete a project or a
// subproject. Deleting a project that has subprojects takes two calls:
// the first lists them and asks for a subproject_action, the second
// carries it out.
type DeleteProjectTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// deleteProjectArgs holds the decoded arguments of the delete_project tool;
// its JSON tags match the property names declared in InputSchema.
type deleteProjectArgs struct {
	Project          string `json:"project"`
	Subproject       string `json:"subproject,omitempty"`
	Confirm          bool   `json:"confirm"`
	SubprojectAction string `json:"subproject_action,omitempty"` // "cancel" | "promote" | "delete_all"
	Path             string `json:"path,omitempty"`
}

// Name returns the MCP tool name, "delete_project".
func (t *DeleteProjectTool) Name() string { return "delete_project" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *DeleteProjectTool) Description() string {
	return "Permanently delete a project or subproject from the vault. Requires confirm: true — ask the user before calling this with confirm: true."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties project and confirm, an optional subproject,
// an optional subproject_action and an optional vault path.
func (t *DeleteProjectTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":    map[string]any{"type": "string", "description": "Project name to delete."},
			"subproject": map[string]any{"type": "string", "description": "Subproject name. When provided, only that subproject is deleted."},
			"confirm":    map[string]any{"type": "boolean", "description": "Must be true to confirm permanent deletion. Ask the user before setting this."},
			"subproject_action": map[string]any{
				"type":        "string",
				"enum":        []string{"cancel", "promote", "delete_all"},
				"description": "Required only when deleting a top-level project that has subprojects.",
			},
			"path": map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"project", "confirm"},
	}
}

// Validate decodes raw into deleteProjectArgs and normalizes the names. It
// returns the arguments, or an error listing every problem: an invalid name,
// confirm not true, or an unknown subproject_action.
func (t *DeleteProjectTool) Validate(raw json.RawMessage) (any, error) {
	var args deleteProjectArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	args.Project = NormalizeName(args.Project)
	args.Subproject = NormalizeName(args.Subproject)

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
	if !args.Confirm {
		problems = append(problems, `"confirm" must be true to confirm permanent deletion — ask the user before proceeding`)
	}
	switch args.SubprojectAction {
	case "", "cancel", "promote", "delete_all":
		// ok
	default:
		problems = append(problems, `"subproject_action" must be one of "cancel", "promote", "delete_all"`)
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
	}
	return args, nil
}

// Execute deletes a subproject, or a top-level project. When the project has
// subprojects and no subproject_action was given, it returns a non-error
// result that asks the caller to choose "cancel", "promote" or "delete_all".
// Promotion moves the subprojects to the vault root first and aborts,
// changing nothing, if that fails. A missing vault yields an error result;
// other failures are returned as errors.
func (t *DeleteProjectTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(deleteProjectArgs)
	dbPath := t.Resolver.DBPathOrDefault(args.Path)

	s, err := t.Stores.GetExisting(ctx, dbPath)
	if errors.Is(err, store.ErrVaultNotFound) {
		return vaultMissingResult(dbPath), nil
	}
	if err != nil {
		return ToolResult{}, err
	}

	// Deleting a specific subproject needs no further decisions.
	if args.Subproject != "" {
		if err := s.DeleteSubproject(ctx, args.Project, args.Subproject); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Text: fmt.Sprintf("Subproject %q deleted.", FormatLabel(args.Project, args.Subproject))}, nil
	}

	// Deleting a top-level project — check for subprojects first. A
	// project that doesn't exist is treated as having none; the "not
	// found" error comes from DeleteProject below.
	parent, err := s.FindProjectByName(ctx, args.Project, nil)
	if err != nil {
		return ToolResult{}, err
	}
	var subs []store.Project
	if parent != nil {
		subs, err = s.ListSubprojects(ctx, parent.ID)
		if err != nil {
			return ToolResult{}, err
		}
	}

	if len(subs) > 0 && args.SubprojectAction == "" {
		lines := make([]string, len(subs))
		for i, sub := range subs {
			lines[i] = "  - " + sub.Name
		}
		text := fmt.Sprintf(
			"Project %q has %d subproject(s):\n%s\n\n"+
				"Call delete_project again with \"subproject_action\" set to one of:\n"+
				"  - \"cancel\"     — abort, do nothing\n"+
				"  - \"promote\"    — move each subproject to the vault root as an independent project\n"+
				"  - \"delete_all\" — delete the project and all its subprojects",
			args.Project, len(subs), strings.Join(lines, "\n"),
		)
		return ToolResult{Text: text}, nil // not an error result: it asks for missing input
	}

	if args.SubprojectAction == "cancel" {
		return ToolResult{Text: fmt.Sprintf("Deletion of %q cancelled.", args.Project)}, nil
	}

	if args.SubprojectAction == "promote" && len(subs) > 0 {
		// PromoteAllSubprojects runs every promotion in one transaction, so
		// a name collision on any subproject leaves all of them in place
		// and deletion is aborted.
		promoted, err := s.PromoteAllSubprojects(ctx, args.Project)
		if err != nil {
			return ToolResult{
				Text:    fmt.Sprintf("Could not promote subprojects: %s. Nothing was changed. Deletion aborted.", err.Error()),
				IsError: true,
			}, nil
		}
		if err := s.DeleteProject(ctx, args.Project); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Text: fmt.Sprintf("Project %q deleted. Subprojects promoted to vault root: %s.", args.Project, strings.Join(promoted, ", "))}, nil
	}

	// subproject_action is "delete_all", or "promote" with nothing to
	// promote, or the project has no subprojects: a plain delete, which
	// also reports "project not found" when args.Project doesn't exist.
	if err := s.DeleteProject(ctx, args.Project); err != nil {
		return ToolResult{}, err
	}
	note := ""
	if len(subs) > 0 {
		names := make([]string, len(subs))
		for i, sub := range subs {
			names[i] = sub.Name
		}
		note = fmt.Sprintf(" (including subprojects: %s)", strings.Join(names, ", "))
	}
	return ToolResult{Text: fmt.Sprintf("Project %q deleted%s.", args.Project, note)}, nil
}
