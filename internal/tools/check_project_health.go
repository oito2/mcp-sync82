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

// CheckProjectHealthTool implements check_project_health: report which of
// the six standard kinds exist for the resolved project. "Exists" means a
// documents row is present (even empty — it was written on purpose) or at
// least one non-archived entries row is present.
type CheckProjectHealthTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// checkProjectHealthArgs holds the decoded arguments of the
// check_project_health tool; its JSON tags match the property names declared
// in InputSchema.
type checkProjectHealthArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
	Format           string `json:"format,omitempty"`
}

// Name returns the MCP tool name, "check_project_health".
func (t *CheckProjectHealthTool) Name() string { return "check_project_health" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *CheckProjectHealthTool) Description() string {
	return "Report which of the six standard memory files exist for a project. Returns an error result (isError: true) when the project is unhealthy — a deliberate signal for the calling agent to act on, not a crash."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional property format plus the project-resolution properties.
func (t *CheckProjectHealthTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
			"format":             formatProperty(),
		},
	}
}

// Validate decodes raw into checkProjectHealthArgs and normalizes format to
// "text" or "json". It returns the arguments, or an error when format is
// invalid.
func (t *CheckProjectHealthTool) Validate(raw json.RawMessage) (any, error) {
	var args checkProjectHealthArgs
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

// Execute resolves the target project and checks that each standard kind
// exists. The result is a text or JSON report; its IsError flag is set when
// any standard kind is missing. Store failures are returned as errors.
func (t *CheckProjectHealthTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(checkProjectHealthArgs)
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, ContextArgs{
		Project: args.Project, Subproject: args.Subproject, Path: args.Path,
		WorkspaceRoot: args.WorkspaceRoot, SearchParentDirs: args.SearchParentDirs,
	})
	if ready != nil {
		return *ready, nil
	}
	if err != nil {
		return ToolResult{}, err
	}

	type kindStatus struct {
		kind   string
		exists bool
	}
	statuses := make([]kindStatus, 0, len(standardKinds))
	isHealthy := true
	for _, k := range standardKinds {
		exists, err := s.KindExists(ctx, rctx.Project, rctx.Subproject, k)
		if err != nil {
			return ToolResult{}, wrapNotFound(err, rctx.Label())
		}
		statuses = append(statuses, kindStatus{kind: k, exists: exists})
		if !exists {
			isHealthy = false
		}
	}

	if args.Format == "json" {
		report := healthReport{Project: rctx.Label(), Vault: rctx.DBPath, Healthy: isHealthy, Files: map[string]bool{}}
		for _, st := range statuses {
			report.Files[st.kind] = st.exists
		}
		return jsonResult(report, !isHealthy)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Health report for project: %s%s\n", rctx.Label(), ContextNote(rctx))
	if isHealthy {
		b.WriteString("Status: HEALTHY ✅\n\nFiles:\n")
	} else {
		b.WriteString("Status: UNHEALTHY ❌\n\nFiles:\n")
	}
	for _, st := range statuses {
		status := "MISSING"
		if st.exists {
			status = "OK"
		}
		fmt.Fprintf(&b, "- %s: %s\n", st.kind, status)
	}
	if !isHealthy {
		b.WriteString("\nRecommendation: Use create_project or init_project_memory to restore missing files.")
	}

	return ToolResult{Text: b.String(), IsError: !isHealthy}, nil
}

// healthReport is check_project_health's JSON result: the project label,
// the vault path, the overall verdict and whether each standard kind
// exists.
type healthReport struct {
	Project string          `json:"project"`
	Vault   string          `json:"vault"`
	Healthy bool            `json:"healthy"`
	Files   map[string]bool `json:"files"`
}
