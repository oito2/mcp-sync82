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
// restores the archive. A kindsManifestName file recording whether each kind
// is a log or a document is written next to them.
//
// Nothing is written when a destination exists but isn't a regular file (a
// symlink could redirect the write elsewhere), nor, unless overwrite is
// true, when a destination file already exists; the error then wraps
// errExportWouldOverwrite and names every colliding file. A missing
// outputDir is created, and new files and directories are private to the
// user. It returns an ExportReport — the number of memory files written
// (0 when the project has no content; the kinds manifest is not counted),
// the kinds skipped because their name can't be a file name and the stale
// .md files left in outputDir — or an error if listing, reading or writing
// fails; a write failure part-way through reports the files written before
// it.
//
// It is the entry point shared by the export_memory tool and the "sync82
// export" command.
func ExportProject(ctx context.Context, s *store.Store, project, subproject, outputDir string, overwrite bool) (ExportReport, error) {
	return exportProjectCore(ctx, s, project, subproject, outputDir, overwrite)
}

// ExportReport is the outcome of an export: Written is the number of files
// written, Skipped the kinds left out because their name isn't a valid
// kind name (too long to be a file name, for example), in name order, and
// Stale the .md files already in the output directory that this export
// didn't write — files of kinds the project no longer has, which an import
// of the folder would bring back — in name order.
type ExportReport struct {
	Written int
	Skipped []string
	Stale   []string
}

// errExportWouldOverwrite is wrapped by the error of an export whose
// destination files already exist while overwriting was not requested.
var errExportWouldOverwrite = errors.New("already exist; pass overwrite to replace them")

// exportFile is one file an export writes: its name inside the output
// directory and its content. meta marks the kinds manifest, which is not
// counted among the files written.
type exportFile struct {
	name    string
	content string
	meta    bool
}

// exportProjectCore implements ExportProject.
func exportProjectCore(ctx context.Context, s *store.Store, project, subproject, outputDir string, overwrite bool) (ExportReport, error) {
	files, skipped, err := exportFiles(ctx, s, project, subproject)
	report := ExportReport{Skipped: skipped}
	if err != nil || len(files) == 0 {
		return report, err
	}

	var existing []string
	for _, f := range files {
		info, err := os.Lstat(filepath.Join(outputDir, f.name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return report, fmt.Errorf("check %s: %w", f.name, err)
		case !info.Mode().IsRegular():
			return report, fmt.Errorf("refusing to write %s: the destination exists and is not a regular file", f.name)
		default:
			existing = append(existing, f.name)
		}
	}
	if len(existing) > 0 && !overwrite {
		return report, fmt.Errorf("%s in %s %w", strings.Join(existing, ", "), outputDir, errExportWouldOverwrite)
	}

	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return report, fmt.Errorf("create output directory %s: %w", outputDir, err)
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
				report.Written = countMemoryFiles(files[:i])
				return report, fmt.Errorf("write %s: %w", f.name, err)
			}
			report.Written = countMemoryFiles(files[:i])
			return report, fmt.Errorf("write %s (already written: %s): %w", f.name, strings.Join(written, ", "), err)
		}
	}
	report.Written = countMemoryFiles(files)
	report.Stale = staleExportFiles(outputDir, files)
	return report, nil
}

// exportFiles renders every file an export of the project writes: one per
// kind with content, plus one for each kind's archived entries. A kind
// whose name isn't a valid kind name is left out and listed in skipped. It
// returns an error when a store read fails.
func exportFiles(ctx context.Context, s *store.Store, project, subproject string) (files []exportFile, skipped []string, err error) {
	kinds, err := s.ListKinds(ctx, project, subproject, true)
	if err != nil {
		return nil, nil, err
	}
	modes, err := s.KindModes(ctx, project, subproject)
	if err != nil {
		return nil, nil, err
	}
	manifest := kindsManifest{Version: kindsManifestVersion, Kinds: map[string]string{}}

	for _, kind := range kinds {
		// The kind comes from the database and becomes part of a file
		// name, so it is validated again here: a name that could leave the
		// output directory, or that is too long to be a file name, is
		// skipped instead of failing the whole export.
		if valid, err := validateKind(kind); err != nil || valid != kind {
			skipped = append(skipped, kind)
			continue
		}

		content, ok, err := s.ReadContent(ctx, project, subproject, kind)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			files = append(files, exportFile{name: kind + ".md", content: content})
		}
		switch modes[kind] {
		case store.KindStorageDocument:
			manifest.Kinds[kind] = kindStorageDocument
		case store.KindStorageEntries:
			manifest.Kinds[kind] = kindStorageLog
		}

		archived, err := archivedContent(ctx, s, project, subproject, kind)
		if err != nil {
			return nil, nil, err
		}
		if archived != "" {
			files = append(files, exportFile{name: kind + archivedSuffix, content: archived})
		}
	}
	if len(files) > 0 {
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("encode %s: %w", kindsManifestName, err)
		}
		files = append(files, exportFile{name: kindsManifestName, content: string(data), meta: true})
	}
	return files, skipped, nil
}

// kindsManifestName is the file an export writes next to the .md files to
// record how each kind is stored, so an import restores a custom log as a
// log rather than as a document. The leading dot keeps it out of the
// "*.md" files an import reads as kinds.
const kindsManifestName = ".sync82-kinds.json"

// kindsManifestVersion is the format version of kindsManifest.
const kindsManifestVersion = 1

// The storage values of kindsManifest.Kinds.
const (
	kindStorageLog      = "log"
	kindStorageDocument = "document"
)

// kindsManifest is the content of kindsManifestName: its format version
// and, for each exported kind, "log" (entries) or "document".
type kindsManifest struct {
	Version int               `json:"version"`
	Kinds   map[string]string `json:"kinds"`
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

// FormatExportNotes returns the lines an export result adds after its
// summary, each starting with a newline: the kinds report.Skipped left out
// and the stale files of report.Stale. It returns "" when there is nothing
// to add.
func FormatExportNotes(report ExportReport) string {
	var b strings.Builder
	if len(report.Skipped) > 0 {
		fmt.Fprintf(&b, "\nSkipped (the name can't be a file name; rename the file to export it): %s", strings.Join(report.Skipped, ", "))
	}
	if len(report.Stale) > 0 {
		fmt.Fprintf(&b, "\nAlso in the folder, from an earlier export (an import would bring them back; delete them if those files were removed): %s", strings.Join(report.Stale, ", "))
	}
	return b.String()
}

// countMemoryFiles returns how many of files are memory files, leaving out
// the kinds manifest.
func countMemoryFiles(files []exportFile) int {
	n := 0
	for _, f := range files {
		if !f.meta {
			n++
		}
	}
	return n
}

// staleExportFiles returns the names of the .md files in outputDir that an
// import would read as kinds but files doesn't hold, in name order. A
// directory that can't be read gives none.
func staleExportFiles(outputDir string, files []exportFile) []string {
	written := make(map[string]bool, len(files))
	for _, f := range files {
		written[f.name] = true
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil
	}
	var stale []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || written[name] || !strings.HasSuffix(name, ".md") {
			continue
		}
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".md"), ".archived")
		if kind, err := validateKind(base); err == nil && !nonKindNames[kind] {
			stale = append(stale, name)
		}
	}
	return stale
}
