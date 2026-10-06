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
const PathDescription = `Base path where the memory is stored. If left blank, uses the default vault path. To use the default user directory, start the path with "HOME" (e.g., "HOME/custom-vault").`

// SearchParentDirsDescription is the description text used on every tool
// schema's optional "search_parent_dirs" field. Looking for .sync82.json
// only at workspace_root itself is the default: a .sync82.json found in a
// parent directory the caller doesn't control could silently redirect
// where memory is stored, since its "path" field is trusted without
// confirmation.
const SearchParentDirsDescription = `If true, also look for .sync82.json in parent directories above workspace_root (useful in monorepos, where the marker file lives at the repo root). Defaults to false — only workspace_root itself is checked.`

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
