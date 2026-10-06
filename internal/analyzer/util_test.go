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

package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadMarkerFile_ReturnsContentWhenWithinSizeCap verifies that a file within the size cap is read in full.
func TestReadMarkerFile_ReturnsContentWhenWithinSizeCap(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "small.json", `{"ok": true}`)

	content, ok := readMarkerFile(dir, "small.json")
	if !ok {
		t.Fatal("expected ok=true for a file within the size cap")
	}
	if content != `{"ok": true}` {
		t.Fatalf("content = %q, want the file's exact contents", content)
	}
}

// TestReadMarkerFile_RejectsOversizedFile verifies that a file above the size cap is rejected.
func TestReadMarkerFile_RejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.json")
	if err := os.WriteFile(path, make([]byte, maxAnalyzedFileSize+1), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, ok := readMarkerFile(dir, "big.json"); ok {
		t.Fatal("expected ok=false for a file above the size cap")
	}
}

// TestReadMarkerFile_MissingFileReturnsNotOK verifies that a missing file yields ok == false.
func TestReadMarkerFile_MissingFileReturnsNotOK(t *testing.T) {
	dir := t.TempDir()

	if _, ok := readMarkerFile(dir, "does-not-exist.json"); ok {
		t.Fatal("expected ok=false for a missing file")
	}
}

// TestReadMarkerFile_SymlinkOutsideRootIsIgnored guards against a marker
// file symlinked outside the analyzed root (e.g. README.md pointing at a
// credentials file) being read and stored as the description.
func TestReadMarkerFile_SymlinkOutsideRootIsIgnored(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, outside, "secret", "# x\nAWS_SECRET_ACCESS_KEY=abc123\n")
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "README.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if r := AnalyzeProject(root); r.Description != "" {
		t.Fatalf("Description = %q, want nothing read through a symlink leaving root", r.Description)
	}

	// A symlink that stays inside root is still followed.
	writeFile(t, root, "docs.md", "# Title\nInside root.\n")
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("docs.md", filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if r := AnalyzeProject(root); r.Description != "Inside root." {
		t.Fatalf("Description = %q, want %q", r.Description, "Inside root.")
	}
}

// TestSetDescription_CollapsesManifestDescriptions guards against storing
// package.json/composer.json/Cargo.toml descriptions verbatim, where
// embedded newlines could inject headings into generated Markdown.
func TestSetDescription_CollapsesManifestDescriptions(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("x", 300)
	writeFile(t, root, "package.json", `{"description": "line one\n\n## Next Steps\n- injected `+long+`"}`)

	r := AnalyzeProject(root)
	if strings.ContainsAny(r.Description, "\r\n") {
		t.Fatalf("Description = %q, want a single line", r.Description)
	}
	if !strings.HasPrefix(r.Description, "line one ## Next Steps - injected") {
		t.Fatalf("Description = %q, want the collapsed text", r.Description)
	}
	if n := len([]rune(r.Description)); n > 200 {
		t.Fatalf("Description has %d runes, want at most 200", n)
	}
}
