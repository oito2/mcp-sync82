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
	"reflect"
	"testing"

	"github.com/oito2/mcp-sync82/internal/store"
)

// TestExtractFirstDate verifies that extractFirstDate returns the date of the
// first valid "## YYYY-MM-DD" header, and an empty string when there is none
// or when the matching header is not a real calendar date.
func TestExtractFirstDate(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"no header", "just some text\nno header here", ""},
		{"simple header", "## 2026-01-01\n- did X", "2026-01-01"},
		{"header not at line start is still matched by ^ per line", "## 2026-01-01\nsome text\n## 2026-01-02\nmore", "2026-01-01"},
		{"header with extra spaces", "##   2026-03-15\nbody", "2026-03-15"},
		// The header pattern only checks the digit shape (4-2-2), so
		// extractFirstDate must also reject a match that is not a real date.
		{"invalid month is rejected", "## 2026-13-01\n- did X", ""},
		{"invalid day is rejected", "## 2026-02-30\n- did X", ""},
		{"day zero is rejected", "## 2026-01-00\n- did X", ""},
		// An invalid header is skipped; the first valid one after it
		// provides the date.
		{"an invalid first header is skipped in favor of a later valid one", "## 2026-13-01\nbad\n## 2026-01-02\ngood", "2026-01-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractFirstDate(tt.content); got != tt.want {
				t.Errorf("extractFirstDate(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

// TestSplitByDateHeader verifies how splitByDateHeader divides content into
// sections: undated content becomes one section, blank content none, an
// undated preamble is flagged, and a header with an invalid calendar date
// yields a section with an empty Date.
func TestSplitByDateHeader(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []store.EntrySection
	}{
		{
			name:    "no header at all becomes one undated section",
			content: "just plain notes\nwith no header",
			want:    []store.EntrySection{{Body: "just plain notes\nwith no header"}},
		},
		{
			name:    "empty content produces no sections",
			content: "   \n  ",
			want:    nil,
		},
		{
			name:    "single dated section",
			content: "## 2026-01-01\n- did X\n- did Y",
			want:    []store.EntrySection{{Date: "2026-01-01", Body: "## 2026-01-01\n- did X\n- did Y"}},
		},
		{
			name:    "multiple dated sections",
			content: "## 2026-01-01\n- did X\n\n## 2026-01-02\n- did Y",
			want: []store.EntrySection{
				{Date: "2026-01-01", Body: "## 2026-01-01\n- did X"},
				{Date: "2026-01-02", Body: "## 2026-01-02\n- did Y"},
			},
		},
		{
			name:    "undated preamble before the first header",
			content: "some undated preamble\n\n## 2026-01-01\n- did X",
			want: []store.EntrySection{
				{Body: "some undated preamble", Preamble: true},
				{Date: "2026-01-01", Body: "## 2026-01-01\n- did X"},
			},
		},
		{
			name:    "invalid calendar date in a non-first section is not stored as Date",
			content: "## 2026-01-01\n- did X\n\n## 2026-13-99\n- did Y",
			want: []store.EntrySection{
				{Date: "2026-01-01", Body: "## 2026-01-01\n- did X"},
				{Date: "", Body: "## 2026-13-99\n- did Y"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitByDateHeader(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitByDateHeader(%q) =\n%+v\nwant\n%+v", tt.content, got, tt.want)
			}
		})
	}
}

// TestDateHeaderQuirks verifies header edge cases: "##" with the date on the
// next line is not a header, a header inside a code fence (``` or ~~~) is
// ignored, and an invalid first header does not hide a later valid one.
func TestDateHeaderQuirks(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{"##\n2026-01-02\n- x", ""},
		{"```\n## 2026-01-02\n```\n- x", ""},
		{"## 2026-13-01\n- bad\n\n## 2026-01-02\n- good", "2026-01-02"},
		{"~~~\n## 2026-01-01\n~~~\n\n## 2026-02-02\n- real", "2026-02-02"},
	}
	for _, c := range cases {
		if got := extractFirstDate(c.content); got != c.want {
			t.Errorf("extractFirstDate(%q) = %q, want %q", c.content, got, c.want)
		}
	}

	sections := splitByDateHeader("## 2026-01-01\n```\n## 2026-05-05\n```\n- done")
	if len(sections) != 1 || sections[0].Date != "2026-01-01" {
		t.Errorf("splitByDateHeader = %+v, want one section: a fenced header doesn't start a new one", sections)
	}
}
