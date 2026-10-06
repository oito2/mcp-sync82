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
	"path/filepath"
	"strings"

	"github.com/oito2/mcp-sync82/internal/analyzer"
	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// initProjectMemoryDescription is the description of init_project_memory:
// a multi-step instruction script for the calling agent (ask about vault
// selection, ask whether this is a subproject, ask whether to auto-detect
// or answer manually) that also describes how the tool reads and creates
// .sync82.json at workspace_root.
const initProjectMemoryDescription = `Initialize memory files for a project or subproject with structured content.

STEP 1 — VAULT SELECTION
If the user has not explicitly specified a vault path, call get_vault_config first.
If global_vault_configured is true, ask:
  "A global vault is configured at <global_vault_path>. Do you want to use it? (yes/no)"
  - Yes → pass that path as "path". No → leave "path" empty.

STEP 2 — PROJECT OR SUBPROJECT?
IMPORTANT: Before calling this tool, determine whether the target is a top-level project or a subproject.

Clues that it IS a subproject: words like "plugin", "module", "package", "extension", "component", "library", "service" in the user's request.
When uncertain: ASK the user — "Is this a subproject of an existing project? If yes, which project?"

If it is a subproject, set both "project" (the parent) and "subproject" (the component name).

If workspace_root is provided:
  - The tool reads .sync82.json at workspace_root itself (pass search_parent_dirs: true to also look in parent directories, e.g. a monorepo root).
  - If found with a "subproject" field: re-initializes that subproject.
  - If found with only "project": re-initializes the root project.
  - If not found: creates a new .sync82.json at workspace_root.

STEP 3 — PROJECT DATA
Ask the user: "How do you want to define the project data?
  1. Auto-analyze (recommended — I will read the project files)
  2. Enter manually"

If option 1: set auto_detect: true with workspace_root.
If option 2: ask these questions (skip any the user can't answer yet):
  1. What does the project do? (description)
  2. What is the main goal?
  3. What is the current phase? (planning / mvp / active / maintenance)
  4. Describe the architecture briefly.
  5. What are the main components? (comma-separated)
  6. What languages are used?
  7. What frameworks and libraries are used?
  8. What infrastructure is used?
  9. What are the immediate next tasks?

Only files that are empty or contain the blank template will be written.`

// needsProjectMessage is init_project_memory's counterpart of
// NeedsInputMessage, returned when no project can be determined from the
// arguments, the local config or the global config.
const needsProjectMessage = `Could not determine which project to initialize.

Please provide one of the following:
- "project": the project name to initialize (optionally with "subproject")
- "workspace_root": path to your project folder — the tool will read .sync82.json if present, or create one once a project is determined

Tip: if this is a new project with no prior session, "project" must be provided explicitly.`

// InitProjectMemoryTool implements init_project_memory: create a project
// or subproject and fill its standard documents from the given answers
// and, optionally, an analysis of the workspace. Documents that already
// have content are left untouched.
type InitProjectMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// initProjectMemoryArgs holds the decoded arguments of the
// init_project_memory tool; its JSON tags match the property names declared
// in InputSchema.
type initProjectMemoryArgs struct {
	Project              string `json:"project,omitempty"`
	Subproject           string `json:"subproject,omitempty"`
	Path                 string `json:"path,omitempty"`
	WorkspaceRoot        string `json:"workspace_root,omitempty"`
	SearchParentDirs     bool   `json:"search_parent_dirs,omitempty"`
	AutoDetect           bool   `json:"auto_detect,omitempty"`
	Description          string `json:"description,omitempty"`
	Goal                 string `json:"goal,omitempty"`
	Phase                string `json:"phase,omitempty"`
	ArchitectureOverview string `json:"architecture_overview,omitempty"`
	Components           string `json:"components,omitempty"`
	Languages            string `json:"languages,omitempty"`
	Frameworks           string `json:"frameworks,omitempty"`
	Infrastructure       string `json:"infrastructure,omitempty"`
	NextSteps            string `json:"next_steps,omitempty"`
}

// Name returns the MCP tool name, "init_project_memory".
func (t *InitProjectMemoryTool) Name() string { return "init_project_memory" }

// Description returns the multi-step instruction script for the calling
// agent.
func (t *InitProjectMemoryTool) Description() string { return initProjectMemoryDescription }

// InputSchema returns the JSON Schema of the tool's arguments: an object
// whose properties are the project-resolution ones, auto_detect and the
// answers used to fill the standard documents.
func (t *InitProjectMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":               map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project."},
			"subproject":            map[string]any{"type": "string", "description": "Subproject name, if this is a component of an existing project."},
			"workspace_root":        map[string]any{"type": "string", "description": "Path to your project folder. Enables .sync82.json auto-discovery for future sessions, and is required when auto_detect is true."},
			"search_parent_dirs":    map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"auto_detect":           map[string]any{"type": "boolean", "description": "When true, analyzes the files at workspace_root to infer description, languages, frameworks, and infrastructure automatically. Requires workspace_root."},
			"description":           map[string]any{"type": "string", "description": "What the project does."},
			"goal":                  map[string]any{"type": "string", "description": "The main goal or objective."},
			"phase":                 map[string]any{"type": "string", "description": "Current phase: planning / mvp / active / maintenance."},
			"architecture_overview": map[string]any{"type": "string", "description": "Brief architecture description."},
			"components":            map[string]any{"type": "string", "description": "Main components, comma-separated."},
			"languages":             map[string]any{"type": "string", "description": "Programming languages used."},
			"frameworks":            map[string]any{"type": "string", "description": "Frameworks and libraries used."},
			"infrastructure":        map[string]any{"type": "string", "description": "Infrastructure and hosting."},
			"next_steps":            map[string]any{"type": "string", "description": "Immediate next tasks, comma or newline separated."},
			"path":                  map[string]any{"type": "string", "description": PathDescription},
		},
	}
}

// Validate decodes raw into initProjectMemoryArgs. It returns the arguments,
// or an error when auto_detect is set without workspace_root, a given
// project or subproject name is invalid, or the answers exceed
// maxContentSize in total.
func (t *InitProjectMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args initProjectMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if args.AutoDetect && args.WorkspaceRoot == "" {
		return nil, fmt.Errorf("workspace_root is required when auto_detect is true")
	}
	// project/subproject are optional here (resolveTarget falls back to
	// .sync82.json or the global config when they're blank), but when
	// given explicitly they must match projectNamePattern.
	if project := strings.TrimSpace(args.Project); project != "" {
		if err := validateProjectName("project", project); err != nil {
			return nil, err
		}
	}
	if subproject := strings.TrimSpace(args.Subproject); subproject != "" {
		if err := validateProjectName("subproject", subproject); err != nil {
			return nil, err
		}
	}
	total := 0
	for _, field := range []string{args.Description, args.Goal, args.Phase, args.ArchitectureOverview, args.Components, args.Languages, args.Frameworks, args.Infrastructure, args.NextSteps} {
		total += len(field)
	}
	if total > maxContentSize {
		return nil, fmt.Errorf("the answers exceed the %d byte limit in total", maxContentSize)
	}
	return args, nil
}

// Execute determines the target (see resolveTarget), creates the project
// when needed and writes the four standard documents that are missing or
// still blank, filled from the answers and the analyzer's findings. With
// workspace_root it also writes .sync82.json unless one already maps the
// workspace to another project. It returns the instructional text when no
// project can be determined, and an error result when the workspace's
// .sync82.json cannot be read; store failures are returned as errors.
func (t *InitProjectMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(initProjectMemoryArgs)

	project, subproject, dbPath, local, err := t.resolveTarget(args)
	if err != nil {
		// A .sync82.json that exists but can't be read is reported instead
		// of being overwritten by the one this call would write.
		return ToolResult{IsError: true, Text: fmt.Sprintf("The .sync82.json for workspace_root %s could not be read: %v. Fix or remove it, then run init_project_memory again.", args.WorkspaceRoot, err)}, nil
	}
	if project == "" {
		return ToolResult{Text: needsProjectMessage}, nil
	}

	var detected analyzer.Result
	if args.AutoDetect && args.WorkspaceRoot != "" {
		detected = analyzer.AnalyzeProject(args.WorkspaceRoot)
	}

	answers := initAnswers{
		Description:          firstNonEmpty(args.Description, detected.Description),
		Goal:                 args.Goal,
		Phase:                args.Phase,
		ArchitectureOverview: args.ArchitectureOverview,
		Components:           firstNonEmptyJoined(args.Components, detected.Components),
		Languages:            firstNonEmptyJoined(args.Languages, detected.Languages),
		Frameworks:           firstNonEmptyJoined(args.Frameworks, detected.Frameworks),
		Infrastructure:       firstNonEmptyJoined(args.Infrastructure, detected.Infrastructure),
		NextSteps:            args.NextSteps,
	}

	s, err := t.Stores.Get(ctx, dbPath)
	if err != nil {
		return ToolResult{}, err
	}

	_, created, err := s.EnsureProject(ctx, project, subproject)
	if err != nil {
		return ToolResult{}, err
	}

	label := FormatLabel(project, subproject)

	var written []string
	for _, tpl := range standardDocumentTemplates {
		shouldWrite := created
		if !shouldWrite {
			blank, err := s.IsBlankOrTemplate(ctx, project, subproject, tpl.kind)
			if err != nil {
				return ToolResult{}, err
			}
			shouldWrite = blank
		}
		if !shouldWrite {
			continue
		}
		if err := s.WriteDocument(ctx, project, subproject, tpl.kind, tpl.render(answers, label)); err != nil {
			return ToolResult{}, err
		}
		written = append(written, tpl.kind)
	}

	extra := ""
	switch {
	case args.WorkspaceRoot == "":
	case local != nil && (local.Config.Project != project || local.Config.Subproject != subproject):
		// The workspace is already mapped to another project: re-pointing
		// it is left to the user rather than done silently.
		extra = fmt.Sprintf(" (.sync82.json at %s points to %q and was left unchanged — edit or remove it to point this workspace at %q)",
			local.ConfigRoot, FormatLabel(local.Config.Project, local.Config.Subproject), label)
	default:
		localCfg := config.LocalConfig{Project: project, Subproject: subproject}
		switch {
		case args.Path != "":
			// A "~/", "HOME/" or absolute path is recorded as given, so the
			// file stays valid for anyone sharing it; a relative path is
			// recorded as the absolute vault path it resolved to.
			localCfg.Path = args.Path
			if !filepath.IsAbs(config.ResolvePath(args.Path)) {
				localCfg.Path = dbPath
			}
		case local != nil:
			// Keep the "path" already recorded in .sync82.json when this
			// call doesn't change it: WriteLocalConfig overwrites the whole
			// file, so an unset Path would drop the custom vault path.
			localCfg.Path = local.Config.Path
		}
		if err := config.WriteLocalConfig(args.WorkspaceRoot, localCfg); err != nil {
			extra = fmt.Sprintf(" (but failed to create .sync82.json: %s)", err.Error())
		} else {
			extra = fmt.Sprintf(" and local config \".sync82.json\" created at %s", args.WorkspaceRoot)
		}
	}

	var message string
	if len(written) > 0 {
		message = fmt.Sprintf("Project %q initialized%s. Files written: %s", label, extra, strings.Join(written, ", "))
	} else {
		message = fmt.Sprintf("Project %q already has content in all files%s. No files were overwritten.", label, extra)
	}

	_ = config.UpdateLastProject(project, subproject, dbPath) // best-effort

	if args.AutoDetect {
		if fields := detectedNonEmptyFieldNames(detected); len(fields) > 0 {
			message += "\n\nAuto-detected fields: " + strings.Join(fields, ", ")
		}
	}

	return ToolResult{Text: message}, nil
}

// resolveTarget determines the project, subproject and vault path
// init_project_memory works on, from args. Unlike Resolver.Resolve, project
// and subproject are filled in independently from args → local config →
// global config, rather than all coming from a single winning tier. For
// example, an explicit "project" argument with no "subproject" still picks
// up a subproject from .sync82.json if one is found.
//
// A .sync82.json naming a different project than an explicit "project"
// argument contributes neither its subproject nor its vault path. When
// workspace_root is given, the global config's last-used project is never
// consulted. local is the .sync82.json found under workspace_root, if any;
// project is empty when nothing determined one. err is set when a
// .sync82.json under workspace_root exists but cannot be read or parsed.
func (t *InitProjectMemoryTool) resolveTarget(args initProjectMemoryArgs) (project, subproject, dbPath string, local *config.LocalConfigResult, err error) {
	project = NormalizeName(args.Project)
	subproject = NormalizeName(args.Subproject)

	if args.WorkspaceRoot != "" {
		localPath, localRoot := "", ""
		found, err := config.ReadLocalConfig(args.WorkspaceRoot, args.SearchParentDirs)
		if err != nil {
			return "", "", "", nil, err
		}
		if found != nil && strings.TrimSpace(found.Config.Project) != "" {
			local = found
			localProject := NormalizeName(found.Config.Project)
			if project == "" || project == localProject {
				project = localProject
				if subproject == "" {
					subproject = NormalizeName(found.Config.Subproject)
				}
				localPath, localRoot = found.Config.Path, found.ConfigRoot
			}
		}
		return project, subproject, t.Resolver.vaultPath(args.Path, localPath, localRoot), local, nil
	}

	if project == "" {
		if globalCfg, err := config.ReadGlobalConfig(); err == nil && globalCfg.LastProject != "" {
			project = globalCfg.LastProject
			if subproject == "" {
				subproject = globalCfg.LastSubproject
			}
			if args.Path == "" && globalCfg.LastVaultPath != "" {
				return project, subproject, absPath(config.ResolvePath(globalCfg.LastVaultPath)), nil, nil
			}
		}
	}
	return project, subproject, t.Resolver.vaultPath(args.Path, "", ""), nil, nil
}

// firstNonEmpty returns primary unless it's empty, in which case it
// returns fallback. It lets an explicit argument override the analyzer's
// result for the same field.
func firstNonEmpty(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

// firstNonEmptyJoined is firstNonEmpty for a comma-separated string
// argument (components, languages, frameworks, infrastructure) whose
// fallback is a list produced by the analyzer, joined with ", ".
func firstNonEmptyJoined(primary string, fallback []string) string {
	if primary != "" {
		return primary
	}
	return strings.Join(fallback, ", ")
}

// detectedNonEmptyFieldNames returns the names of the fields of d the
// analyzer populated, for the "Auto-detected fields: ..." line appended to
// the tool's response.
func detectedNonEmptyFieldNames(d analyzer.Result) []string {
	var fields []string
	if d.Description != "" {
		fields = append(fields, "description")
	}
	if len(d.Languages) > 0 {
		fields = append(fields, "languages")
	}
	if len(d.Frameworks) > 0 {
		fields = append(fields, "frameworks")
	}
	if len(d.Infrastructure) > 0 {
		fields = append(fields, "infrastructure")
	}
	if len(d.Components) > 0 {
		fields = append(fields, "components")
	}
	return fields
}
