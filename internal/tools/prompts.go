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
	"fmt"
	"strings"
)

// PromptArgument is one argument of a sync82 prompt.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptDefinition is one MCP prompt sync82 exposes: its name, texts,
// arguments, and Render, which turns the arguments given by the client
// into the instruction sent to the agent as a user message. Prompts never
// read or write the vault: they only tell the agent which tools to call.
type PromptDefinition struct {
	Name        string
	Title       string
	Description string
	Arguments   []PromptArgument
	Render      func(args map[string]string) (string, error)
}

// promptTargetArguments are the arguments every sync82 prompt takes.
var promptTargetArguments = []PromptArgument{
	{Name: "project", Description: "Project name. If omitted, the agent finds the project from the current workspace."},
	{Name: "subproject", Description: "Subproject name (requires project)."},
}

// Prompts lists the MCP prompts sync82 exposes.
var Prompts = []PromptDefinition{
	{
		Name:        "start_session",
		Title:       "Start a session with the project memory",
		Description: "Load the project's sync82 memory and summarize where the work stands.",
		Arguments:   promptTargetArguments,
		Render:      renderStartSession,
	},
	{
		Name:        "end_session",
		Title:       "Save this session to the project memory",
		Description: "Record what this session did, decided and left pending in the project's sync82 memory.",
		Arguments:   promptTargetArguments,
		Render:      renderEndSession,
	},
}

// promptTarget returns how a prompt names its target project to the
// agent, as tool arguments to pass, from the "project" and "subproject"
// arguments: the names when given (validated and lower-cased), otherwise
// workspace_root. It returns an error for an invalid name or a subproject
// without a project.
func promptTarget(args map[string]string) (string, error) {
	project, subproject := NormalizeName(args["project"]), NormalizeName(args["subproject"])
	if project == "" {
		if subproject != "" {
			return "", fmt.Errorf(`"subproject" requires "project"`)
		}
		return "workspace_root set to the current workspace folder", nil
	}
	if err := ValidateTarget(project, subproject); err != nil {
		return "", err
	}
	if subproject == "" {
		return fmt.Sprintf("project %q", project), nil
	}
	return fmt.Sprintf("project %q and subproject %q", project, subproject), nil
}

// renderStartSession renders the start_session instruction for args.
func renderStartSession(args map[string]string) (string, error) {
	target, err := promptTarget(args)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		fmt.Sprintf("Load the project memory from sync82 by calling load_project_context with %s.", target),
		"Then give me a short summary: what the project is, where it stands now, the most recent decisions and progress, and the next steps.",
		"If the response says older history was omitted, don't load it unless I ask for it.",
	}, "\n"), nil
}

// renderEndSession renders the end_session instruction for args.
func renderEndSession(args map[string]string) (string, error) {
	target, err := promptTarget(args)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		fmt.Sprintf("Save this session to the sync82 project memory with one update_project_memory call, with %s:", target),
		"- progress: what was done in this session, under a \"## YYYY-MM-DD\" header with today's date;",
		"- decisions: each decision made and why, under the same kind of header (leave it out if nothing was decided);",
		"- next_steps: the full, updated list of what is pending;",
		"- memory, architecture or stack: their full updated content, only if they changed.",
		"If something already recorded is now wrong, fix that entry with edit_entry instead of adding one that contradicts it.",
		"Then tell me briefly what you saved.",
	}, "\n"), nil
}
