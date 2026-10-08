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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/oito2/mcp-sync82/internal/config"
)

// PathDescription is the description text used on every tool schema's
// optional "path" field.
const PathDescription = `Base path where the memory is stored. If left blank, uses the default vault path. A leading "~", "HOME" or "$HOME" (e.g. "~/vaults/work.db", "HOME/custom-vault") is expanded to the user's home directory.`

// SearchParentDirsDescription is the description text used on every tool
// schema's optional "search_parent_dirs" field. Looking for .sync82.json
// only at workspace_root itself is the default: a .sync82.json found in a
// parent directory the caller doesn't control could silently redirect
// where memory is stored, since its "path" field is trusted without
// confirmation.
const SearchParentDirsDescription = `If true, also look for .sync82.json in parent directories above workspace_root (useful in monorepos, where the marker file lives at the repo root). Defaults to false — only workspace_root itself is checked.`

// Description texts of the project-resolution schema properties shared by
// the project-scoped tools. projectRefusesLastDescription is the project
// text of the tools that refuse a project taken only from the last session.
const (
	projectDescription            = `Project name. If omitted, auto-discovered from workspace_root or the last used project.`
	projectRefusesLastDescription = `Project name. If omitted, auto-discovered from workspace_root; unlike other tools, the last used project is refused.`
	subprojectDescription         = `Subproject name.`
	workspaceRootDescription      = `Path to your project folder, used to auto-discover the project via .sync82.json.`
)

// targetSchema selects the description texts targetProperties gives the
// project, subproject and workspace_root properties. An empty field keeps
// the default text: projectDescription, subprojectDescription and
// workspaceRootDescription.
type targetSchema struct {
	Project       string
	Subproject    string
	WorkspaceRoot string
}

// targetProperties returns the JSON Schema properties of targetArgs —
// project, subproject, path, workspace_root and search_parent_dirs — with
// the texts selected by opts, merged with a tool's own properties, own.
// It panics when own redefines one of the shared properties, which only a
// programming error can cause.
func targetProperties(opts targetSchema, own map[string]any) map[string]any {
	text := func(custom, fallback string) string {
		if custom != "" {
			return custom
		}
		return fallback
	}
	props := map[string]any{
		"project":            map[string]any{"type": "string", "description": text(opts.Project, projectDescription)},
		"subproject":         map[string]any{"type": "string", "description": text(opts.Subproject, subprojectDescription)},
		"path":               map[string]any{"type": "string", "description": PathDescription},
		"workspace_root":     map[string]any{"type": "string", "description": text(opts.WorkspaceRoot, workspaceRootDescription)},
		"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
	}
	for name, schema := range own {
		if _, shared := props[name]; shared {
			panic("targetProperties: property " + name + " is already defined")
		}
		props[name] = schema
	}
	return props
}

// pluralize returns singular when n == 1, plural otherwise — the
// "file"/"files", "entry"/"entries" choice several tool responses make,
// shared instead of reimplemented at each call site.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// absoluteDirArg validates the directory argument raw, named field in
// error messages: it must be non-blank and, after "~"/"HOME" expansion,
// absolute — a relative path would depend on the working directory the MCP
// client started the server in. It returns the expanded, cleaned path, or
// an error when the value is blank or not absolute.
func absoluteDirArg(field, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%q is required and must not be empty", field)
	}
	dir := config.ResolvePath(raw)
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%q must be an absolute path (or start with ~/ or HOME/), got %q", field, raw)
	}
	return filepath.Clean(dir), nil
}

// formatProperty returns the JSON Schema property of the optional "format"
// argument shared by the listing and reporting tools.
func formatProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"text", "json"},
		"default":     "text",
		"description": `"text" (default) for readable text, or "json" for a JSON document with the same information, also returned as structured content.`,
	}
}

// validateFormat returns the "format" argument's value, defaulting to
// "text" when empty. It returns an error for anything other than "text" or
// "json".
func validateFormat(format string) (string, error) {
	switch format {
	case "", "text":
		return "text", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf(`"format" must be "text" or "json", got %q`, format)
	}
}

// jsonResult returns a ToolResult whose text is v as an indented JSON
// document and whose structured content is v itself. isError sets the
// result's IsError flag. It returns an error when v cannot be marshaled.
func jsonResult(v any, isError bool) (ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ToolResult{}, fmt.Errorf("encode result: %w", err)
	}
	return ToolResult{Text: string(data), Structured: v, IsError: isError}, nil
}

// Building blocks of the tools' hand-written output schemas.

// schemaString, schemaInteger and schemaBoolean return the JSON Schema of a
// string, an integer and a boolean.
func schemaString() map[string]any  { return map[string]any{"type": "string"} }
func schemaInteger() map[string]any { return map[string]any{"type": "integer"} }
func schemaBoolean() map[string]any { return map[string]any{"type": "boolean"} }

// schemaArray returns the JSON Schema of an array whose items follow items.
func schemaArray(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

// schemaObject returns the JSON Schema of an object with properties, of
// which required must be present.
func schemaObject(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// problemsError returns the error a Validate returns for problems, one
// line each under "invalid arguments:", or nil when there is none.
func problemsError(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
}
