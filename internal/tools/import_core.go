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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oito2/mcp-sync82/internal/fsutil"
	"github.com/oito2/mcp-sync82/internal/store"
)

// ImportProject is the inverse of ExportProject: it reads every
// "<kind>.md" file in inputDir and writes it into the given project and
// subproject of s the way write_memory would (an append-only kind's
// content is split into dated sections with splitByDateHeader; an
// overwrite-style kind is written as-is). A "<kind>.archived.md" file
// replaces that kind's archived entries; without one, the archived entries
// already in the vault are kept. The project is created when missing.
//
// The import is all or nothing: every file is read and checked first, and
// the writes are then applied in one transaction, so a failure leaves the
// vault unchanged. With dryRun, nothing is written and the report says
// what the import would do. Files that cannot be imported are listed in
// the report as skipped. It returns an error when the project or
// subproject name is invalid, inputDir cannot be read, it holds more than
// maxImportFiles ".md" files or more than maxImportTotalBytes of them, two
// files differ only in case, the kinds manifest is unusable, or a store
// operation fails. s may be nil only with dryRun, for a vault that does not
// exist yet: every file is then reported as new.
//
// It is the entry point shared by the import_memory tool and the "sync82
// import" command.
func ImportProject(ctx context.Context, s *store.Store, project, subproject, inputDir string, dryRun bool) (ImportReport, error) {
	return importProjectCore(ctx, s, project, subproject, inputDir, dryRun)
}

// ImportReport lists the file names of an import by outcome: Created for
// kinds that don't exist yet, Overwritten for kinds whose content the
// file replaces, Skipped for files that aren't imported (not a valid kind
// name, empty, too large, or not a regular file).
type ImportReport struct {
	Created     []string
	Overwritten []string
	Skipped     []string
}

// Imported returns the number of files the import writes: those created
// plus those overwritten.
func (r ImportReport) Imported() int {
	return len(r.Created) + len(r.Overwritten)
}

// Bounds of one import: the most ".md" files it reads, and the most bytes
// they may hold in total, all of which stay in memory until the single
// write transaction.
const (
	maxImportFiles      = 256
	maxImportTotalBytes = 64 << 20
)

// nonKindNames are the base names, lower-cased, of Markdown files commonly
// kept next to an export (in a repository, for example) that are never
// imported as memory kinds.
var nonKindNames = map[string]bool{
	"readme":          true,
	"changelog":       true,
	"license":         true,
	"contributing":    true,
	"code_of_conduct": true,
}

// errNotRegularFile reports a path that is a symlink, directory or special
// file (FIFO, device, socket) rather than a regular file.
var errNotRegularFile = errors.New("not a regular file")

// readRegularFile reads path when it is a regular file — not a symlink or
// special file — of at most limit bytes. A file over the limit returns
// ok=false with no error; a non-regular file, or one that is not the file
// first inspected (replaced between the check and the open, for example by
// a symlink), returns errNotRegularFile; other failures are returned as
// errors. The read is bounded, so a file growing after the size check is
// never read past the limit.
func readRegularFile(path string, limit int64) (data []byte, ok bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, errNotRegularFile
	}
	if info.Size() > limit {
		return nil, false, nil
	}
	// Opened without following a symlink and without blocking on a FIFO,
	// either of which may have replaced the file since the Lstat above.
	f, err := os.OpenFile(path, os.O_RDONLY|fsutil.OpenNonblock|fsutil.OpenNoFollow, 0)
	if err != nil {
		if now, lerr := os.Lstat(path); lerr == nil && !now.Mode().IsRegular() {
			return nil, false, errNotRegularFile
		}
		return nil, false, err
	}
	defer f.Close()
	if opened, err := f.Stat(); err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, false, errNotRegularFile
	}
	data, err = io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return nil, false, nil
	}
	return data, true, nil
}

// importProjectCore implements ImportProject.
func importProjectCore(ctx context.Context, s *store.Store, project, subproject, inputDir string, dryRun bool) (ImportReport, error) {
	var report ImportReport
	if err := ValidateTarget(project, subproject); err != nil {
		return report, err
	}
	dirEntries, err := os.ReadDir(inputDir)
	if err != nil {
		return report, fmt.Errorf("read input directory %s: %w", inputDir, err)
	}

	var names []string
	for _, e := range dirEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names) // deterministic order — easier to reason about and to test
	// The manifest of an export says how each kind was stored; it decides
	// the storage of a kind the project doesn't have yet.
	manifest, err := readKindsManifest(filepath.Join(inputDir, kindsManifestName))
	if err != nil {
		return report, err
	}
	// A kind with a "<kind>.archived.md" file is a log: its "<kind>.md"
	// holds its active entries, not a document.
	hasArchive := map[string]bool{}
	for _, name := range names {
		if base, ok := strings.CutSuffix(name, archivedSuffix); ok {
			hasArchive[strings.ToLower(base)] = true
		}
	}
	if len(names) > maxImportFiles {
		return report, fmt.Errorf("input directory %s has %d .md files; at most %d are imported at once", inputDir, len(names), maxImportFiles)
	}

	if s == nil && !dryRun {
		return report, errors.New("import needs an open vault unless it is a dry run")
	}
	exists := false
	if s != nil {
		if exists, err = s.ProjectExists(ctx, project, subproject); err != nil {
			return report, err
		}
	}

	// First pass: read and check every file, deciding each write, before
	// anything is written.
	var writes []store.KindWrite
	var totalBytes int
	seen := map[string]string{}
	for _, name := range names {
		archived := strings.HasSuffix(name, archivedSuffix)
		base := strings.TrimSuffix(name, ".md")
		if archived {
			base = strings.TrimSuffix(name, archivedSuffix)
		}
		kind, err := validateKind(base)
		if err != nil || nonKindNames[kind] {
			// Not every ".md" file in the directory is a memory kind (a name
			// that isn't a valid kind, or a README.md next to the exported
			// files): skip it instead of failing the whole import.
			report.Skipped = append(report.Skipped, name)
			continue
		}
		key := kind
		if archived {
			key += archivedSuffix
		}
		if other, dup := seen[key]; dup {
			return ImportReport{}, fmt.Errorf("%s and %s are the same file once names are lower-cased; keep only one", other, name)
		}
		seen[key] = name

		// Symlinks and special files are skipped: a symlink could pull a
		// file from outside the input directory into the vault, and a FIFO
		// or device would block or never end. Files over maxContentSize
		// are skipped without being read in full.
		data, ok, err := readRegularFile(filepath.Join(inputDir, name), maxContentSize)
		if errors.Is(err, errNotRegularFile) || (err == nil && !ok) {
			report.Skipped = append(report.Skipped, name)
			continue
		}
		if err != nil {
			return ImportReport{}, fmt.Errorf("read %s: %w", name, err)
		}
		if totalBytes += len(data); totalBytes > maxImportTotalBytes {
			return ImportReport{}, fmt.Errorf("the .md files in %s hold more than %d MB in total; import them in smaller groups", inputDir, maxImportTotalBytes>>20)
		}
		content := stripEntryMarkers(string(data))
		if strings.TrimSpace(content) == "" {
			// An empty file has nothing to import; skip it instead of
			// overwriting existing content with blank.
			report.Skipped = append(report.Skipped, name)
			continue
		}

		mode := store.KindStorageNone
		if exists {
			if mode, err = s.KindMode(ctx, project, subproject, kind); err != nil {
				return ImportReport{}, err
			}
		}
		write := store.KindWrite{Kind: kind}
		switch {
		case archived && mode == store.KindStorageDocument:
			return ImportReport{}, fmt.Errorf("%s: %q is an overwrite-style document, which has no archived entries", name, kind)
		case archived:
			write.Sections, write.Archived = splitByDateHeader(content), true
		case isAppendOnlyKind(kind) || mode == store.KindStorageEntries ||
			(mode == store.KindStorageNone && (hasArchive[kind] || manifest[kind] == kindStorageLog)):
			write.Sections = splitByDateHeader(content)
		default:
			write.Document = &content
		}
		writes = append(writes, write)
		if mode == store.KindStorageNone {
			report.Created = append(report.Created, name)
		} else {
			report.Overwritten = append(report.Overwritten, name)
		}
	}

	if dryRun || len(writes) == 0 {
		return report, nil
	}

	// Second pass: create the project if needed and apply every write in
	// one transaction. EnsureProject lets an import restore a backup into
	// a brand-new project without a separate create_project call.
	if _, _, err := s.EnsureProject(ctx, project, subproject); err != nil {
		return ImportReport{}, err
	}
	if err := s.WriteKinds(ctx, project, subproject, writes); err != nil {
		return ImportReport{}, err
	}
	return report, nil
}

// FormatImportReport renders r as the text the import_memory tool and the
// "sync82 import" command print. target is the project label the files go
// into, inputDir the directory they came from, and dryRun selects the
// wording for an import that wrote nothing.
func FormatImportReport(r ImportReport, target, inputDir string, dryRun bool) string {
	if r.Imported() == 0 && len(r.Skipped) == 0 {
		return fmt.Sprintf("Nothing to import for %s: %s contains no .md files.", target, inputDir)
	}
	verb := "Imported"
	if dryRun {
		verb = "Dry run, nothing written — would import"
	}
	text := fmt.Sprintf("%s %d %s into %s from %s", verb, r.Imported(), pluralize(r.Imported(), "file", "files"), target, inputDir)
	if len(r.Created) > 0 {
		text += "\nNew: " + strings.Join(r.Created, ", ")
	}
	if len(r.Overwritten) > 0 {
		text += "\nOverwritten: " + strings.Join(r.Overwritten, ", ")
	}
	if len(r.Skipped) > 0 {
		text += "\nSkipped (not a valid kind name, empty, too large, or not a regular file): " + strings.Join(r.Skipped, ", ")
	}
	return text
}

// readKindsManifest reads the kindsManifest at path and returns its kinds,
// lower-cased, mapped to "log" or "document". A missing manifest, as in a
// folder written by hand, gives an empty map. It returns an error for a manifest that is not a regular file, is
// larger than 1 MB, isn't valid JSON, or has another format version.
func readKindsManifest(path string) (map[string]string, error) {
	data, ok, err := readRegularFile(path, 1<<20)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return map[string]string{}, nil
	case errors.Is(err, errNotRegularFile):
		return nil, fmt.Errorf("%s is not a regular file", kindsManifestName)
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", kindsManifestName, err)
	case !ok:
		return nil, fmt.Errorf("%s is larger than 1 MB", kindsManifestName)
	}
	var m kindsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s is not valid: %w", kindsManifestName, err)
	}
	if m.Version != kindsManifestVersion {
		return nil, fmt.Errorf("%s has format version %d; this sync82 reads version %d", kindsManifestName, m.Version, kindsManifestVersion)
	}
	kinds := make(map[string]string, len(m.Kinds))
	for kind, storage := range m.Kinds {
		kinds[strings.ToLower(kind)] = storage
	}
	return kinds, nil
}
