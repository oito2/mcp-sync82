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
	"errors"
	"fmt"

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// RunImport implements "sync82 import <project> [subproject] <input-dir> [--path <vault>] [--dry-run]" —
// the inverse of "sync82 export", using the same logic as the import_memory
// tool (tools.ImportProject). The vault is created when it does not exist,
// except with --dry-run, which only reports what would change and writes
// nothing. It returns the process exit code: 0 on success, 1 on any
// failure, 2 on invalid arguments (a usage error), with the message written
// to deps.Stderr.
func RunImport(ctx context.Context, args []string, deps memoryCmdDeps) int {
	project, subproject, inputDir, vaultPath, dryRun, err := parseImportArgs(args)
	if err != nil {
		return usageError(deps.Stderr, err)
	}

	vaultPath = deps.Resolver.DBPathOrDefault(vaultPath)
	var s *store.Store
	if dryRun {
		s, err = deps.Stores.GetExisting(ctx, vaultPath)
		if errors.Is(err, store.ErrVaultNotFound) {
			s, err = nil, nil // preview against an empty vault
		}
	} else {
		s, err = deps.Stores.Get(ctx, vaultPath)
	}
	if err != nil {
		fmt.Fprintf(deps.Stderr, "open vault: %v\n", err)
		return 1
	}

	report, err := tools.ImportProject(ctx, s, project, subproject, inputDir, dryRun)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "import: %v\n", err)
		return 1
	}
	fmt.Fprintln(deps.Stdout, tools.FormatImportReport(report, tools.FormatLabel(project, subproject), inputDir, dryRun))
	return 0
}

// parseImportArgs accepts either "<project> <input-dir>" or
// "<project> <subproject> <input-dir>", plus the optional flags
// "--path <vault>" and "--dry-run" anywhere on the line. Project and
// subproject names are normalized and validated. It returns the parsed
// values, or an error (with the usage text for malformed arguments).
func parseImportArgs(args []string) (project, subproject, inputDir, vaultPath string, dryRun bool, err error) {
	const usage = "usage: sync82 import <project> [subproject] <input-dir> [--path <vault>] [--dry-run]"
	parsed, err := parseArgs(args, []string{"dry-run"}, []string{"path"})
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("%w\n%s", err, usage)
	}
	vaultPath, dryRun, positional := parsed.values["path"], parsed.flags["dry-run"], parsed.positional

	switch len(positional) {
	case 2:
		project, inputDir = tools.NormalizeName(positional[0]), positional[1]
	case 3:
		project, subproject, inputDir = tools.NormalizeName(positional[0]), tools.NormalizeName(positional[1]), positional[2]
	default:
		return "", "", "", "", false, fmt.Errorf("%s", usage)
	}
	if err := tools.ValidateTarget(project, subproject); err != nil {
		return "", "", "", "", false, err
	}
	return project, subproject, inputDir, vaultPath, dryRun, nil
}
