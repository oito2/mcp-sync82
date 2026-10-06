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
	"strings"
	"time"

	"github.com/oito2/mcp-sync82/internal/store"
)

// maxContentSize is the largest content, in bytes, accepted for a single
// write_memory, append_memory, update_project_memory or imported file. It
// is generous for curated project notes but stops one oversized call from
// growing the vault or the process's memory without bound.
const maxContentSize = 10 * 1024 * 1024 // 10MB

// errAppendToDocumentKind is returned when an append targets a kind stored
// as a single overwrite-style document, whose content only write_memory
// replaces. Appending there would add an entry no read ever returns.
var errAppendToDocumentKind = errors.New("is an overwrite-style file; use write_memory to replace its content instead of appending")

// validateAppendInput checks a (kind, content) pair against
// append_memory's rules: kind must be a valid slug, content must be
// non-empty and within maxContentSize, and content must contain a
// "## YYYY-MM-DD" date header when kind is one of the two standard
// append-only kinds. It returns the lower-cased kind, or the first
// violated rule as an error; a standard overwrite-style kind yields an
// error wrapping errAppendToDocumentKind. AppendMemoryTool.Validate and
// appendMemoryCore both use it, so every append path enforces the same
// rules.
func validateAppendInput(kind, content string) (string, error) {
	kind, err := validateKind(kind)
	if err != nil {
		return "", err
	}
	if isStandardKind(kind) && !isAppendOnlyKind(kind) {
		return "", fmt.Errorf("%q %w", kind, errAppendToDocumentKind)
	}
	if content == "" {
		return "", fmt.Errorf(`"content" is required and must not be empty`)
	}
	if len(content) > maxContentSize {
		return "", fmt.Errorf(`"content" exceeds the %d byte limit`, maxContentSize)
	}
	if isAppendOnlyKind(kind) && extractFirstDate(content) == "" {
		today := time.Now().UTC().Format("2006-01-02")
		return "", fmt.Errorf(
			"content appended to %q must contain a date header in the format \"## YYYY-MM-DD\".\nExample:\n## %s\n- Your entry here",
			kind, today,
		)
	}
	return kind, nil
}

// appendMemoryCore appends content as a new entry of kind in the given
// project and subproject of s. It is shared by append_memory and
// update_project_memory. The input is validated with validateAppendInput;
// a kind already stored as a document is refused with an error wrapping
// errAppendToDocumentKind, and a missing project yields the error built
// by wrapNotFound. For the date-headed kinds the entry's date is taken
// from its first valid date header.
func appendMemoryCore(ctx context.Context, s *store.Store, project, subproject, kind, content string) error {
	kind, err := validateAppendInput(kind, content)
	if err != nil {
		return err
	}
	mode, err := s.KindMode(ctx, project, subproject, kind)
	if err != nil {
		return wrapNotFound(err, FormatLabel(project, subproject))
	}
	if mode == store.KindStorageDocument {
		return fmt.Errorf("%q %w", kind, errAppendToDocumentKind)
	}

	entryDate := ""
	if isAppendOnlyKind(kind) {
		entryDate = extractFirstDate(content)
	}
	if err := s.AppendEntry(ctx, project, subproject, kind, entryDate, content); err != nil {
		return wrapNotFound(err, FormatLabel(project, subproject))
	}
	return nil
}

// validateWriteInput checks a (kind, content) pair against write_memory's
// rules: kind must be a valid slug and content must be non-empty and
// within maxContentSize. It returns the lower-cased kind, or an error
// listing every violated rule.
func validateWriteInput(kind, content string) (string, error) {
	var problems []string

	validKind, err := validateKind(kind)
	if err != nil {
		problems = append(problems, err.Error())
	} else {
		kind = validKind
	}
	if content == "" {
		problems = append(problems, `"content" is required and must not be empty`)
	} else if len(content) > maxContentSize {
		problems = append(problems, fmt.Sprintf(`"content" exceeds the %d byte limit`, maxContentSize))
	}
	if len(problems) > 0 {
		return "", fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
	}
	return kind, nil
}

// writeMemoryCore replaces the whole content of kind in the given project
// and subproject of s. It is shared by write_memory and
// update_project_memory. The kind keeps the storage it already has:
// progress, decisions and any custom kind written by appending stay a log
// of dated entries (replaced via splitByDateHeader), everything else is an
// overwrite-style document. Invalid input is rejected with
// validateWriteInput's error, and a missing project yields the error built
// by wrapNotFound.
func writeMemoryCore(ctx context.Context, s *store.Store, project, subproject, kind, content string) error {
	kind, err := validateWriteInput(kind, content)
	if err != nil {
		return err
	}

	asEntries := isAppendOnlyKind(kind)
	if !asEntries && !isStandardKind(kind) {
		mode, err := s.KindMode(ctx, project, subproject, kind)
		if err != nil {
			return wrapNotFound(err, FormatLabel(project, subproject))
		}
		asEntries = mode == store.KindStorageEntries
	}

	if asEntries {
		err = s.ReplaceAllEntries(ctx, project, subproject, kind, splitByDateHeader(content))
	} else {
		err = s.WriteDocument(ctx, project, subproject, kind, content)
	}
	if err != nil {
		return wrapNotFound(err, FormatLabel(project, subproject))
	}
	return nil
}
