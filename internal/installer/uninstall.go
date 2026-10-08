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

package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
)

// UninstallTarget removes sync82's registration from a single target. A
// CLI target is cleaned through its own CLI and skipped when its client is
// not detected; a file target has the sync82 key deleted from each of its
// RemovePaths files whether or not its client is still installed. A
// target with nothing registered is reported as "not configured, skipping"
// and returns ResultSkip. Warnings are printed after the status line.
func UninstallTarget(ctx context.Context, t Target, env Env, stdout, stderr io.Writer) Result {
	if reason := t.unsupported(env); reason != "" {
		return reportUnsupported(stdout, t.Name, reason)
	}
	if t.Kind == KindCLI {
		return uninstallCLITarget(ctx, t, env, stdout, stderr)
	}
	return uninstallFileTarget(t, env, stdout, stderr)
}

// reportNotConfigured prints the line for a target without a registration.
func reportNotConfigured(stdout io.Writer, name string) Result {
	fmt.Fprintf(stdout, "  ⚠  %s — not configured, skipping\n", name)
	return ResultSkip
}

// uninstallCLITarget removes sync82 from a CLI target through the
// client's own subcommands. It returns ResultSkip when the client is not
// detected, nothing was registered, or only warnings were produced, and
// ResultFail when the removal errored.
func uninstallCLITarget(ctx context.Context, t Target, env Env, stdout, stderr io.Writer) Result {
	if !t.Detected(env) {
		return reportNotDetected(stdout, t.Name)
	}
	fmt.Fprintf(stdout, "\nRemoving from %s...\n", t.Name)
	removed, warnings, err := removeCLIRegistration(ctx, t)
	var result Result
	switch {
	case err != nil:
		fmt.Fprintf(stderr, "[%s] %v\n", t.Name, err)
		result = reportFailed(stdout, t.Name)
	case removed:
		fmt.Fprintf(stdout, "  ✓  %s — removed.\n", t.Name)
		result = ResultOK
	case len(warnings) > 0:
		fmt.Fprintf(stdout, "  ⚠  %s — nothing removed\n", t.Name)
		result = ResultSkip
	default:
		result = reportNotConfigured(stdout, t.Name)
	}
	printWarnings(stdout, warnings)
	return result
}

// uninstallFileTarget deletes sync82's entry from each of a file target's
// remove paths that holds one. It returns ResultSkip when none does,
// ResultFail when any file could not be read or cleaned, and ResultManual
// when the only problem is a file with comments, left for the user to edit.
func uninstallFileTarget(t Target, env Env, stdout, stderr io.Writer) Result {
	var found []string
	unreadable := false
	for _, path := range t.removePaths(env) {
		has, err := fileHasEntry(path, t.Shape)
		if err != nil {
			fmt.Fprintf(stderr, "[%s] %v\n", t.Name, err)
			unreadable = true
			continue
		}
		if has {
			found = append(found, path)
		}
	}
	if unreadable && len(found) == 0 {
		return reportFailed(stdout, t.Name)
	}
	if len(found) == 0 {
		return reportNotConfigured(stdout, t.Name)
	}

	fmt.Fprintf(stdout, "\nRemoving from %s...\n", t.Name)
	failed, manual := unreadable, false
	for _, path := range found {
		err := removeEntry(t.Shape, path)
		if err != nil {
			fmt.Fprintf(stderr, "[%s] %v\n", t.Name, err)
		}
		var manualErr *manualEditError
		switch {
		case errors.As(err, &manualErr):
			manual = true
		case err != nil:
			failed = true
		}
	}
	if failed {
		return reportFailed(stdout, t.Name)
	}
	if manual {
		return reportManual(stdout, t.Name)
	}
	fmt.Fprintf(stdout, "  ✓  %s — removed.\n", t.Name)
	return ResultOK
}

// removeEntry deletes the sync82 key from the shape's object in the config
// file at path, keeping every other key, the file mode and a symlink at
// path. A file with comments or trailing commas is left unchanged and the
// error, a *manualEditError, says what to remove by hand.
func removeEntry(shape Shape, path string) error {
	cfg, strict, raw, err := loadConfig(path)
	if err != nil {
		return err
	}
	servers, ok := cfg[shape.Key].(map[string]any)
	if !ok {
		return nil
	}
	if _, ok := servers[serverName]; !ok {
		return nil
	}
	if !strict {
		return &manualEditError{fmt.Sprintf("%s contains comments or trailing commas, so it was left unchanged. Remove the %q entry from its %q object by hand", path, serverName, shape.Key)}
	}
	return writeServers(path, raw, shape.Key, func(servers []jsonMember) []jsonMember {
		return slices.DeleteFunc(servers, func(m jsonMember) bool { return m.key == serverName })
	})
}
