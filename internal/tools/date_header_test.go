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
	"strings"
	"testing"
	"time"

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

// TestExtractFirstDate_MixedFenceMarkers verifies that a ~~~ line inside a
// ``` block doesn't close it, so a date header example after it is still
// ignored.
func TestExtractFirstDate_MixedFenceMarkers(t *testing.T) {
	content := "intro\n```\n~~~\n## 2020-01-01\n```\n## 2026-05-05\n- real"
	if got := extractFirstDate(content); got != "2026-05-05" {
		t.Fatalf("extractFirstDate = %q, want 2026-05-05 (the header inside the fence is an example)", got)
	}
}

// TestDateHeaderMatches_InlineTripleBackticks verifies that a line opening
// with an inline ```code``` span doesn't hide the headers after it, while
// a real fence still does.
func TestDateHeaderMatches_InlineTripleBackticks(t *testing.T) {
	content := "```go test``` runs the suite.\n## 2026-01-01\n- did x\n```\n## 2026-01-02\n```\n## 2026-01-03\n- y"
	var dates []string
	for _, m := range dateHeaderMatches(content) {
		dates = append(dates, content[m[2]:m[3]])
	}
	if got := strings.Join(dates, ","); got != "2026-01-01,2026-01-03" {
		t.Errorf("headers = %s, want 2026-01-01,2026-01-03", got)
	}
}

// TestDateHeaderMatches_LinearTime checks that about 2 MB of alternating
// fences and headers is scanned quickly; a scan that rescans the fences
// for every header takes tens of seconds on it.
func TestDateHeaderMatches_LinearTime(t *testing.T) {
	var b strings.Builder
	for b.Len() < 2<<20 {
		b.WriteString("```\nx\n```\n## 2026-01-01\n- y\n")
	}
	start := time.Now()
	if n := len(dateHeaderMatches(b.String())); n == 0 {
		t.Fatal("no header found")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("scanning 2 MB took %s", elapsed)
	}
}

// BenchmarkDateHeaderMatches measures the scan of about 1 MB of
// alternating fences and headers.
func BenchmarkDateHeaderMatches(b *testing.B) {
	var sb strings.Builder
	for sb.Len() < 1<<20 {
		sb.WriteString("```\nx\n```\n## 2026-01-01\n- y\n")
	}
	content := sb.String()
	b.SetBytes(int64(len(content)))
	for b.Loop() {
		dateHeaderMatches(content)
	}
}
