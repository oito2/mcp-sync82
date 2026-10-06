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
	"fmt"
	"io"
	"path/filepath"

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// RunExport implements both forms of "sync82 export":
//   - "sync82 export <project> [subproject] <output-dir> [--path <vault>]"
//     writes one project's memory as plain files, using the same rendering
//     as the export_memory tool (tools.ExportProject).
//   - "sync82 export --all <output-dir> [--path <vault>]" does the same for
//     every project and subproject in the vault, one
//     "<output-dir>/<project>[/<subproject>]" subfolder each.
//
// The vault must already exist. It returns the process exit code: 0 on
// success (including nothing to export), 1 on any failure, 2 on invalid
// arguments (a usage error), with the message written to deps.Stderr.
func RunExport(ctx context.Context, args []string, deps memoryCmdDeps) int {
	all, project, subproject, outputDir, vaultPath, err := parseExportArgs(args)
	if err != nil {
		return usageError(deps.Stderr, err)
	}

	vaultPath = deps.Resolver.DBPathOrDefault(vaultPath)
	s, err := deps.Stores.GetExisting(ctx, vaultPath)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "open vault: %v\n", err)
		return 1
	}

	if all {
		return runExportAll(ctx, s, deps.Stdout, deps.Stderr, outputDir)
	}

	count, err := tools.ExportProject(ctx, s, project, subproject, outputDir, true)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "export: %v\n", err)
		return 1
	}

	label := tools.FormatLabel(project, subproject)
	if count == 0 {
		fmt.Fprintf(deps.Stdout, "Nothing to export for %s: no files found.\n", label)
		return 0
	}
	plural := "files"
	if count == 1 {
		plural = "file"
	}
	fmt.Fprintf(deps.Stdout, "Exported %d %s from %s to %s\n", count, plural, label, outputDir)
	return 0
}

// runExportAll exports every top-level project and every subproject in
// the vault into its own "<outputDir>/<project>[/<subproject>]" subfolder
// and prints a per-target line plus a total. It returns the exit code: 0 on
// success or an empty vault, 1 on the first listing or export failure.
func runExportAll(ctx context.Context, s *store.Store, stdout, stderr io.Writer, outputDir string) int {
	projects, err := s.ListTopLevelProjects(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "list projects: %v\n", err)
		return 1
	}
	if len(projects) == 0 {
		fmt.Fprintln(stdout, "Nothing to export: the vault has no projects.")
		return 0
	}

	totalFiles, totalProjects := 0, 0
	for _, p := range projects {
		n, ok := exportOneAndReport(ctx, s, stdout, stderr, p.Name, "", outputDir)
		if !ok {
			return 1
		}
		totalFiles += n
		totalProjects++

		subs, err := s.ListSubprojects(ctx, p.ID)
		if err != nil {
			fmt.Fprintf(stderr, "list subprojects of %q: %v\n", p.Name, err)
			return 1
		}
		for _, sub := range subs {
			n, ok := exportOneAndReport(ctx, s, stdout, stderr, p.Name, sub.Name, outputDir)
			if !ok {
				return 1
			}
			totalFiles += n
			totalProjects++
		}
	}

	fmt.Fprintf(stdout, "\nExported %d project(s), %d file(s) total, to %s\n", totalProjects, totalFiles, outputDir)
	return 0
}

// exportOneAndReport exports a single project/subproject into its own
// subfolder under outputDir and prints a one-line progress report. It
// returns the number of files written and ok, which is false when the
// export itself failed (the error was already written to stderr) — distinct
// from a normal "0 files" outcome, which still counts as success.
func exportOneAndReport(ctx context.Context, s *store.Store, stdout, stderr io.Writer, project, subproject, outputDir string) (count int, ok bool) {
	label := tools.FormatLabel(project, subproject)
	// Project names become folder names here, so a name that could leave
	// outputDir (for example one containing path separators) is refused
	// rather than joined into a path.
	if err := tools.ValidateTarget(project, subproject); err != nil {
		fmt.Fprintf(stderr, "export %s: refusing unsafe project name: %v\n", label, err)
		return 0, false
	}
	dest := filepath.Join(outputDir, project)
	if subproject != "" {
		dest = filepath.Join(dest, subproject)
	}

	count, err := tools.ExportProject(ctx, s, project, subproject, dest, true)
	if err != nil {
		fmt.Fprintf(stderr, "export %s: %v\n", label, err)
		return 0, false
	}
	fmt.Fprintf(stdout, "  %s: %d file(s) -> %s\n", label, count, dest)
	return count, true
}

// parseExportArgs accepts either "<project> <output-dir>" or
// "<project> <subproject> <output-dir>" (both plus an optional trailing
// "--path <vault>"), or "--all <output-dir>" (also plus an optional
// trailing "--path <vault>") to export every project/subproject in the
// vault at once. It returns the parsed target (project and subproject are
// empty with all set) and the optional vault path override, or an error
// carrying the usage text when the arguments are invalid.
func parseExportArgs(args []string) (all bool, project, subproject, outputDir, vaultPath string, err error) {
	usage := "usage: sync82 export <project> [subproject] <output-dir> [--path <vault>]\n       sync82 export --all <output-dir> [--path <vault>]"

	parsed, err := parseArgs(args, []string{"all"}, []string{"path"})
	if err != nil {
		return false, "", "", "", "", fmt.Errorf("%w\n%s", err, usage)
	}
	all, vaultPath, positional := parsed.flags["all"], parsed.values["path"], parsed.positional

	if all {
		if len(positional) != 1 {
			return false, "", "", "", "", fmt.Errorf("%s", usage)
		}
		return true, "", "", positional[0], vaultPath, nil
	}

	switch len(positional) {
	case 2:
		return false, positional[0], "", positional[1], vaultPath, nil
	case 3:
		return false, positional[0], positional[1], positional[2], vaultPath, nil
	default:
		return false, "", "", "", "", fmt.Errorf("%s", usage)
	}
}
