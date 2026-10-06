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
	"strings"
	"testing"
)

// TestExportProjectCore_RefusesKindThatWouldEscapeOutputDir verifies that
// ExportProject refuses a kind such as "../evil" instead of writing outside
// the output directory. The kind is written with Store.WriteDocument directly,
// bypassing the kind validation done by the tools layer, to simulate an
// unsanitized kind reaching the export.
func TestExportProjectCore_RefusesKindThatWouldEscapeOutputDir(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "../evil", "malicious content"); err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	if _, err := ExportProject(ctx, s, "acme", "", outputDir, false); err == nil {
		t.Fatal("expected an error refusing to export an invalid kind, got nil")
	} else if !strings.Contains(err.Error(), "refusing to export") {
		t.Fatalf("error = %q, want it to mention refusing to export", err.Error())
	}
}
