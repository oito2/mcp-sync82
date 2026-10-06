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
	"unicode/utf8"
)

// writeFile writes content to dir/name, creating parent directories. It fails the
// test on any I/O error.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

// TestDetectReadme verifies description extraction from README.md for the supported heading
// styles and the lines that must be skipped.
func TestDetectReadme(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "simple description line",
			content: "# My Project\n\nA tool that does things.\n",
			want:    "A tool that does things.",
		},
		{
			name:    "skips badges and blank lines",
			content: "# My Project\n\n![build](badge.svg)\n\nThe real description.\n",
			want:    "The real description.",
		},
		{
			name:    "skips HTML and blockquote lines",
			content: "# My Project\n<div align=\"center\">\n> a quote\nActual description here.\n",
			want:    "Actual description here.",
		},
		{
			name:    "skips markdown table rows",
			content: "# My Project\n| a | b |\n|---|---|\nReal one.\n",
			want:    "Real one.",
		},
		{
			name:    "skips emoji-first lines",
			content: "# My Project\n🚀 exciting stuff\nPlain description.\n",
			want:    "Plain description.",
		},
		{
			name:    "strips bold and code markers",
			content: "# My Project\n**Bold** and `code` description.\n",
			want:    "Bold and code description.",
		},
		{
			name:    "no heading at all yields no description",
			content: "just some text\nno heading here\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "README.md", tt.content)

			var r Result
			detectReadme(dir, &r)
			if r.Description != tt.want {
				t.Errorf("Description = %q, want %q", r.Description, tt.want)
			}
		})
	}
}

// TestDetectReadme_TruncatesTo200Runes verifies that a long description is truncated to 200 runes without
// splitting a multi-byte character.
func TestDetectReadme_TruncatesTo200Runes(t *testing.T) {
	dir := t.TempDir()
	// Starts with an ASCII character (so it passes the ASCII-start
	// filter) but is otherwise all multi-byte runes, to catch a
	// byte-vs-rune truncation bug: slicing by byte count could split a
	// multi-byte character in half.
	long := "A" + strings.Repeat("é", 250)
	writeFile(t, dir, "README.md", "# Title\n"+long+"\n")

	var r Result
	detectReadme(dir, &r)
	got := []rune(r.Description)
	if len(got) != 200 {
		t.Fatalf("Description length = %d runes, want 200", len(got))
	}
	if !utf8.ValidString(r.Description) {
		t.Fatalf("Description is not valid UTF-8 after truncation: %q", r.Description)
	}
}

// TestDetectReadme_NeverOverwritesExistingDescription verifies that an already-set description is preserved.
func TestDetectReadme_NeverOverwritesExistingDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Title\nfrom readme\n")

	r := Result{Description: "already set"}
	detectReadme(dir, &r)
	if r.Description != "already set" {
		t.Fatalf("Description = %q, want it untouched", r.Description)
	}
}

// TestDetectReadme_MissingFileIsNoOp verifies that a missing README.md adds no signal.
func TestDetectReadme_MissingFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectReadme(dir, &r)
	if r.Description != "" {
		t.Fatalf("Description = %q, want empty when README.md is absent", r.Description)
	}
}

// TestDetectReadme_SetextAndCodeFences checks that setext titles are
// recognized as headings, and that lines inside code fences are treated
// neither as headings nor as the description.
func TestDetectReadme_SetextAndCodeFences(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "setext title",
			content: "My Project\n==========\n\nA setext description.\n",
			want:    "A setext description.",
		},
		{
			name:    "fence before the heading is skipped",
			content: "```bash\n# install\nnpm i x\n```\n# Title\nReal description.\n",
			want:    "Real description.",
		},
		{
			name:    "fence only, no heading outside it",
			content: "```\n# install\nnpm i x\n```\n",
			want:    "",
		},
		{
			name:    "fence after the heading is skipped",
			content: "# Title\n~~~~\ncode line\n~~~\nstill code\n~~~~\nAfter the fence.\n",
			want:    "After the fence.",
		},
		{
			name:    "setext underline inside a fence is ignored",
			content: "```\ncode\n====\n```\nTitle\n=====\nDescription.\n",
			want:    "Description.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "README.md", tt.content)

			var r Result
			detectReadme(dir, &r)
			if r.Description != tt.want {
				t.Errorf("Description = %q, want %q", r.Description, tt.want)
			}
		})
	}
}
