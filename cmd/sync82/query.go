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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/oito2/mcp-sync82/internal/tools"
)

// RunSearch implements "sync82 search <query> [--project P] [--subproject S]
// [--match words|phrase|exact] [--kinds k1,k2] [--since D] [--until D]
// [--limit N] [--offset N] [--context-lines N] [--json] [--path V]" through
// the search_memory tool, so its output is the tool's. It returns the
// process exit code: 0 on success (no match included), 1 on a failure such
// as a missing vault or project, 2 on invalid arguments.
func RunSearch(ctx context.Context, args []string, deps memoryCmdDeps) int {
	p, err := parseArgs(args, []string{"json"}, []string{"project", "subproject", "match", "kinds", "since", "until", "limit", "offset", "context-lines", "path"})
	if err != nil {
		return commandUsageError(deps.Stderr, "search", err)
	}
	if len(p.positional) != 1 {
		return commandUsageError(deps.Stderr, "search", fmt.Errorf("search takes one query (quote it when it has spaces), got %d arguments", len(p.positional)))
	}
	toolArgs := map[string]any{"query": p.positional[0]}
	copyValues(toolArgs, p, map[string]string{"project": "project", "subproject": "subproject", "match": "match", "since": "since", "until": "until", "path": "path"})
	if err := copyInts(toolArgs, p, map[string]string{"limit": "limit", "offset": "offset", "context-lines": "context_lines"}); err != nil {
		return commandUsageError(deps.Stderr, "search", err)
	}
	if kinds, ok := p.values["kinds"]; ok {
		toolArgs["kinds"] = splitList(kinds)
	}
	if p.flags["json"] {
		toolArgs["format"] = "json"
	}
	return runToolCommand(ctx, "search", &tools.SearchMemoryTool{Resolver: deps.Resolver, Stores: deps.Stores}, toolArgs, deps)
}

// RunContext implements "sync82 context <project> [subproject] [--full]
// [--since D] [--max-entries N] [--max-bytes N] [--files f1,f2] [--path V]"
// through the load_project_context tool, so its output is the tool's. It
// returns 0 on success, 1 on a failure such as a missing vault or project,
// and 2 on invalid arguments.
func RunContext(ctx context.Context, args []string, deps memoryCmdDeps) int {
	p, err := parseArgs(args, []string{"full"}, []string{"since", "max-entries", "max-bytes", "files", "path"})
	if err != nil {
		return commandUsageError(deps.Stderr, "context", err)
	}
	toolArgs, err := projectArgs("context", p.positional)
	if err != nil {
		return commandUsageError(deps.Stderr, "context", err)
	}
	copyValues(toolArgs, p, map[string]string{"since": "since", "path": "path"})
	if err := copyInts(toolArgs, p, map[string]string{"max-entries": "max_entries", "max-bytes": "max_bytes"}); err != nil {
		return commandUsageError(deps.Stderr, "context", err)
	}
	if files, ok := p.values["files"]; ok {
		toolArgs["files"] = splitList(files)
	}
	if p.flags["full"] {
		toolArgs["mode"] = "full"
	}
	return runToolCommand(ctx, "context", &tools.LoadProjectContextTool{Resolver: deps.Resolver, Stores: deps.Stores}, toolArgs, deps)
}

// RunHealth implements "sync82 health <project> [subproject] | --all
// [--json] [--stale-days N] [--path V]" through the check_project_health
// tool, so its output is the tool's. It returns 0 when every checked
// project is healthy (warnings included), 1 when one is unhealthy or on a
// failure such as a missing vault, and 2 on invalid arguments.
func RunHealth(ctx context.Context, args []string, deps memoryCmdDeps) int {
	p, err := parseArgs(args, []string{"all", "json"}, []string{"stale-days", "path"})
	if err != nil {
		return commandUsageError(deps.Stderr, "health", err)
	}
	var toolArgs map[string]any
	switch {
	case p.flags["all"] && len(p.positional) > 0:
		return commandUsageError(deps.Stderr, "health", fmt.Errorf("health takes either a project or --all, not both"))
	case p.flags["all"]:
		toolArgs = map[string]any{"all_projects": true}
	default:
		if toolArgs, err = projectArgs("health", p.positional); err != nil {
			return commandUsageError(deps.Stderr, "health", fmt.Errorf("%w, or --all for every project", err))
		}
	}
	copyValues(toolArgs, p, map[string]string{"path": "path"})
	if err := copyInts(toolArgs, p, map[string]string{"stale-days": "stale_days"}); err != nil {
		return commandUsageError(deps.Stderr, "health", err)
	}
	if p.flags["json"] {
		toolArgs["format"] = "json"
	}
	return runToolCommand(ctx, "health", &tools.CheckProjectHealthTool{Resolver: deps.Resolver, Stores: deps.Stores}, toolArgs, deps)
}

// projectArgs returns the tool arguments naming the project, and the
// subproject when given, of positional, which must hold one or two names;
// command names the subcommand in the error.
func projectArgs(command string, positional []string) (map[string]any, error) {
	switch len(positional) {
	case 1:
		return map[string]any{"project": positional[0]}, nil
	case 2:
		return map[string]any{"project": positional[0], "subproject": positional[1]}, nil
	default:
		return nil, fmt.Errorf("%s takes a project and an optional subproject, got %d arguments", command, len(positional))
	}
}

// copyValues copies each value flag of p named in names (flag → tool
// argument) into toolArgs.
func copyValues(toolArgs map[string]any, p parsedArgs, names map[string]string) {
	for flag, arg := range names {
		if v, ok := p.values[flag]; ok {
			toolArgs[arg] = v
		}
	}
}

// copyInts copies each integer flag of p named in names (flag → tool
// argument) into toolArgs. It returns an error for a value that is not an
// integer; the tool checks the range.
func copyInts(toolArgs map[string]any, p parsedArgs, names map[string]string) error {
	for flag, arg := range names {
		v, ok := p.values[flag]
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("--%s must be an integer, got %q", flag, v)
		}
		toolArgs[arg] = n
	}
	return nil
}

// splitList splits a comma-separated flag value into its trimmed, non-empty
// items.
func splitList(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// cliWording rewrites the parts of a tool's error messages that name tool
// arguments or other tools into the flags and commands of the CLI.
var cliWording = strings.NewReplacer(
	`"project" or "workspace_root"`, "--project",
	`"max_bytes"`, "--max-bytes",
	`"max_entries"`, "--max-entries",
	`"context_lines"`, "--context-lines",
	`"stale_days"`, "--stale-days",
	`"subproject"`, "--subproject",
	`"project"`, "--project",
	`"match"`, "--match",
	`"since"`, "--since",
	`"until"`, "--until",
	`"limit"`, "--limit",
	`"offset"`, "--offset",
	`"mode"`, "--full",
	"; use create_project first", "",
)

// runToolCommand runs tool with toolArgs, as an MCP call would, and prints
// its result; command names the subcommand in a usage error, and error
// messages name CLI flags (cliWording). Invalid arguments are a usage
// error (2). A result carrying a
// report goes to stdout, and one without (a missing vault, for example) to
// stderr; either way an error result exits with 1. A failure of the tool
// itself is printed to stderr and exits with 1.
func runToolCommand(ctx context.Context, command string, tool tools.Tool, toolArgs map[string]any, deps memoryCmdDeps) int {
	raw, err := json.Marshal(toolArgs)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "Error: encode arguments: %v\n", err)
		return 1
	}
	parsed, err := tool.Validate(raw)
	if err != nil {
		return commandUsageError(deps.Stderr, command, errors.New(cliWording.Replace(err.Error())))
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "Error: %s\n", cliWording.Replace(err.Error()))
		return 1
	}
	text := strings.TrimRight(result.Text, "\n")
	if result.IsError && result.Structured == nil {
		fmt.Fprintln(deps.Stderr, text)
		return 1
	}
	fmt.Fprintln(deps.Stdout, text)
	if result.IsError {
		return 1
	}
	return 0
}
