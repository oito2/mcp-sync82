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
	// ExpectedSubprojects is the number of subprojects the caller expects
	// "delete_all" to delete; nil when not given.
	ExpectedSubprojects *int   `json:"expected_subprojects,omitempty"`
	Path                string `json:"path,omitempty"`
}

// Name returns the MCP tool name, "delete_project".
func (t *DeleteProjectTool) Name() string { return "delete_project" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *DeleteProjectTool) Description() string {
	return "Permanently delete a project or subproject from the vault. Requires confirm: true — ask the user before calling this with confirm: true. A top-level project with subprojects also needs subproject_action; with \"delete_all\", pass expected_subprojects, the number of subprojects you showed the user, and nothing is deleted when the vault holds another number."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties project and confirm, an optional subproject,
// an optional subproject_action, an optional expected_subprojects and an
// optional vault path.
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
			"expected_subprojects": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "Required with subproject_action \"delete_all\": the number of subprojects shown to the user. Nothing is deleted when the project has another number.",
			},
			"path": map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"project", "confirm"},
	}
}

// Validate decodes raw into deleteProjectArgs and normalizes the names. It
// returns the arguments, or an error listing every problem: an invalid name,
// confirm not true, an unknown subproject_action, or an expected_subprojects
// that is negative or given without subproject_action "delete_all".
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
	if n := args.ExpectedSubprojects; n != nil {
		if *n < 0 {
			problems = append(problems, `"expected_subprojects" must be 0 or more`)
		}
		if args.SubprojectAction != "delete_all" {
			problems = append(problems, `"expected_subprojects" is only used with "subproject_action": "delete_all"`)
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
	}
	return args, nil
}

// Execute deletes a subproject, or a top-level project. When the project has
// subprojects and no subproject_action was given, or "delete_all" was given
// without expected_subprojects, it returns a non-error result that lists the
// subprojects and asks the caller to choose "cancel", "promote" or
// "delete_all". A top-level project is deleted only when its number of
// subprojects, checked in the deleting transaction, is expected_subprojects
// for "delete_all" and zero otherwise; any other number is an error result
// that lists the current subprojects and deletes nothing. Promotion and
// deletion happen in one transaction, so a failure changes nothing. After a
// deletion, a last used project (or subproject) that no longer exists is
// forgotten, and one that was promoted is followed to its new name. A
// missing vault yields an error result, as does a failed promotion; a
// missing project and other failures are returned as errors.
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
			return ToolResult{}, projectNotFound(err, FormatLabel(args.Project, args.Subproject))
		}
		t.forgetLastProject(args.Project, args.Subproject, false, dbPath)
		return ToolResult{Text: fmt.Sprintf("Subproject %q deleted.", FormatLabel(args.Project, args.Subproject))}, nil
	}

	// Deleting a top-level project — check for subprojects first. A
	// project that doesn't exist is treated as having none; the "not
	// found" error comes from DeleteProjectExpecting below.
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
		return ToolResult{Text: subprojectChoiceText(args.Project, subs), ProjectsUnchanged: true}, nil // not an error result: it asks for missing input
	}
	if args.SubprojectAction == "delete_all" && args.ExpectedSubprojects == nil && parent != nil {
		text := "\"delete_all\" also needs \"expected_subprojects\". " + subprojectChoiceText(args.Project, subs)
		return ToolResult{Text: text, ProjectsUnchanged: true}, nil
	}

	if args.SubprojectAction == "cancel" {
		return ToolResult{Text: fmt.Sprintf("Deletion of %q cancelled.", args.Project), ProjectsUnchanged: true}, nil
	}

	if args.SubprojectAction == "promote" && len(subs) > 0 {
		// Every promotion and the deletion run in one transaction, so a name
		// collision on any subproject leaves everything in place.
		promoted, err := s.PromoteSubprojectsAndDelete(ctx, args.Project)
		if err != nil {
			return ToolResult{
				Text:    fmt.Sprintf("Could not promote subprojects: %s. Nothing was changed. Deletion aborted.", err.Error()),
				IsError: true,
			}, nil
		}
		t.forgetLastProject(args.Project, "", true, dbPath)
		return ToolResult{Text: fmt.Sprintf("Project %q deleted. Subprojects promoted to vault root: %s.", args.Project, strings.Join(promoted, ", "))}, nil
	}

	// subproject_action is "delete_all", or "promote" with nothing to
	// promote, or the project has no subprojects: a delete that checks the
	// number of subprojects in its own transaction, so one created since
	// the listing above is never deleted unseen. It also reports "project
	// not found" when args.Project doesn't exist.
	expected := 0
	if args.SubprojectAction == "delete_all" && args.ExpectedSubprojects != nil {
		expected = *args.ExpectedSubprojects
	}
	if err := s.DeleteProjectExpecting(ctx, args.Project, expected); errors.Is(err, store.ErrSubprojectCount) {
		var current []store.Project
		if parent != nil {
			if current, err = s.ListSubprojects(ctx, parent.ID); err != nil {
				return ToolResult{}, err
			}
		}
		text := fmt.Sprintf("Nothing was deleted: project %q has %d subproject(s), not %d. ", args.Project, len(current), expected) +
			subprojectChoiceText(args.Project, current)
		return ToolResult{Text: text, IsError: true, ProjectsUnchanged: true}, nil
	} else if err != nil {
		return ToolResult{}, projectNotFound(err, args.Project)
	}
	t.forgetLastProject(args.Project, "", false, dbPath)
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

// subprojectChoiceText lists subs, the subprojects of project, and tells the
// caller to call delete_project again with a subproject_action, giving the
// expected_subprojects that "delete_all" needs.
func subprojectChoiceText(project string, subs []store.Project) string {
	lines := make([]string, len(subs))
	for i, sub := range subs {
		lines[i] = "  - " + sub.Name
	}
	return fmt.Sprintf(
		"Project %q has %d subproject(s):\n%s\n\n"+
			"Show this list to the user, then call delete_project again with \"subproject_action\" set to one of:\n"+
			"  - \"cancel\"     — abort, do nothing\n"+
			"  - \"promote\"    — move each subproject to the vault root as an independent project\n"+
			"  - \"delete_all\" — delete the project and all its subprojects; also pass \"expected_subprojects\": %d",
		project, len(subs), strings.Join(lines, "\n"), len(subs),
	)
}

// forgetLastProject updates the global config after project (or its
// subproject, when not empty) was deleted from the vault at dbPath, so the
// last used project never names something that no longer exists: a
// remembered target inside what was deleted is cleared, except that with
// promoted set a remembered subproject of project follows its promotion to
// the vault root. A remembered target in another vault is left alone. A
// failure is only logged.
func (t *DeleteProjectTool) forgetLastProject(project, subproject string, promoted bool, dbPath string) {
	project, subproject = NormalizeName(project), NormalizeName(subproject)
	err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) {
		if c.LastProject != project || (c.LastVaultPath != "" && !samePath(c.LastVaultPath, dbPath)) {
			return
		}
		switch {
		case subproject != "" && c.LastSubproject != subproject:
			// Another subproject of the same project: still there.
		case promoted && c.LastSubproject != "":
			c.LastProject, c.LastSubproject = c.LastSubproject, ""
		default:
			c.LastProject, c.LastSubproject, c.LastVaultPath = "", "", ""
		}
	})
	if err != nil {
		t.Resolver.Logger.Error("forget last project after delete", "error", err)
	}
}
