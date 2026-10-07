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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/mcp-sync82/internal/fsutil"
	"github.com/oito2/mcp-sync82/internal/store"
)

// ExportProject writes one <kind>.md file per kind of the given project
// and subproject in s into outputDir — the standard kinds and any custom
// kind, each rendered as read_memory returns it (entries concatenated in
// date order, with their "## YYYY-MM-DD" headers). A kind with archived
// entries also gets a <kind>.archived.md file holding those entries in the
// same format, so an export keeps the whole history and importing it
// restores the archive.
//
// Nothing is written when a destination exists but isn't a regular file (a
// symlink could redirect the write elsewhere), nor, unless overwrite is
// true, when a destination file already exists; the error then wraps
// errExportWouldOverwrite and names every colliding file. A missing
// outputDir is created, and new files and directories are private to the
// user. It returns the number of files written (0 when the project has no
// content), or an error if listing, reading or writing fails; a write
// failure part-way through reports the files written before it.
//
// It is the entry point shared by the export_memory tool and the "sync82
// export" command.
func ExportProject(ctx context.Context, s *store.Store, project, subproject, outputDir string, overwrite bool) (int, error) {
	return exportProjectCore(ctx, s, project, subproject, outputDir, overwrite)
}

// errExportWouldOverwrite is wrapped by the error of an export whose
// destination files already exist while overwriting was not requested.
var errExportWouldOverwrite = errors.New("already exist; pass overwrite to replace them")

// exportFile is one file an export writes: its name inside the output
// directory and its content.
type exportFile struct {
	name    string
	content string
}

// exportProjectCore implements ExportProject.
func exportProjectCore(ctx context.Context, s *store.Store, project, subproject, outputDir string, overwrite bool) (int, error) {
	files, err := exportFiles(ctx, s, project, subproject)
	if err != nil || len(files) == 0 {
		return 0, err
	}

	var existing []string
	for _, f := range files {
		info, err := os.Lstat(filepath.Join(outputDir, f.name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return 0, fmt.Errorf("check %s: %w", f.name, err)
		case !info.Mode().IsRegular():
			return 0, fmt.Errorf("refusing to write %s: the destination exists and is not a regular file", f.name)
		default:
			existing = append(existing, f.name)
		}
	}
	if len(existing) > 0 && !overwrite {
		return 0, fmt.Errorf("%s in %s %w", strings.Join(existing, ", "), outputDir, errExportWouldOverwrite)
	}

	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return 0, fmt.Errorf("create output directory %s: %w", outputDir, err)
	}
	for i, f := range files {
		content := f.content
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		// AtomicReplaceFile never writes through a symlink: one swapped in
		// after the check above can't redirect the export elsewhere.
		if err := fsutil.AtomicReplaceFile(filepath.Join(outputDir, f.name), []byte(content), 0o600); err != nil {
			written := make([]string, i)
			for j := range written {
				written[j] = files[j].name
			}
			if len(written) == 0 {
				return i, fmt.Errorf("write %s: %w", f.name, err)
			}
			return i, fmt.Errorf("write %s (already written: %s): %w", f.name, strings.Join(written, ", "), err)
		}
	}
	return len(files), nil
}

// exportFiles renders every file an export of the project writes: one per
// kind with content, plus one for each kind's archived entries. It returns
// an error when a kind name is not a valid kind or a store read fails.
func exportFiles(ctx context.Context, s *store.Store, project, subproject string) ([]exportFile, error) {
	kinds, err := s.ListKinds(ctx, project, subproject, true)
	if err != nil {
		return nil, err
	}

	var files []exportFile
	for _, kind := range kinds {
		// The kind comes from the database and becomes part of a file name,
		// so it is validated again here to rule out path traversal.
		if _, err := validateKind(kind); err != nil {
			return nil, fmt.Errorf("refusing to export unexpected kind %q: %w", kind, err)
		}

		content, ok, err := s.ReadContent(ctx, project, subproject, kind)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, exportFile{name: kind + ".md", content: content})
		}

		archived, err := archivedContent(ctx, s, project, subproject, kind)
		if err != nil {
			return nil, err
		}
		if archived != "" {
			files = append(files, exportFile{name: kind + archivedSuffix, content: archived})
		}
	}
	return files, nil
}

// archivedSuffix is the file name suffix, after the kind, of the file
// holding a kind's archived entries in an export.
const archivedSuffix = ".archived.md"

// archivedContent returns a kind's archived entries joined in date order,
// or "" when it has none.
func archivedContent(ctx context.Context, s *store.Store, project, subproject, kind string) (string, error) {
	entries, err := s.ReadEntries(ctx, project, subproject, kind, true)
	if err != nil {
		return "", err
	}
	var bodies []string
	for _, e := range entries {
		if e.Archived {
			bodies = append(bodies, e.Body)
		}
	}
	return strings.Join(bodies, "\n\n"), nil
}
