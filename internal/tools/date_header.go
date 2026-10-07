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
	"regexp"
	"strings"
	"time"

	"github.com/oito2/mcp-sync82/internal/store"
)

// dateHeaderPattern matches a "## YYYY-MM-DD" section header line — the
// format append_memory enforces for the two standard append-only kinds
// (progress, decisions) and splitByDateHeader uses to break text into
// dated sections. It only checks the digit shape (4-2-2), not that the
// date is a real calendar date; callers parse the date themselves. The
// date must be on the same line as "##".
var dateHeaderPattern = regexp.MustCompile(`(?m)^##[ \t]+(\d{4}-\d{2}-\d{2})\b`)

// fencePattern matches a line opening or closing a fenced code block.
var fencePattern = regexp.MustCompile("(?m)^[ \t]*(```|~~~)")

// dateHeaderMatches returns the index pairs (in the form of
// regexp.FindAllStringSubmatchIndex: the whole match, then the date
// group) of every "## YYYY-MM-DD" header in content outside fenced code
// blocks, where such a line is example text rather than a header. A block
// opened by ``` closes only at the next ```, and one opened by ~~~ only at
// the next ~~~, so the other marker inside a block is just text.
func dateHeaderMatches(content string) [][]int {
	fences := fencePattern.FindAllStringSubmatchIndex(content, -1)
	inFence := func(pos int) bool {
		marker := ""
		for _, f := range fences {
			if f[0] > pos {
				break
			}
			if m := content[f[2]:f[3]]; marker == "" {
				marker = m
			} else if marker == m {
				marker = ""
			}
		}
		return marker != ""
	}
	var out [][]int
	for _, m := range dateHeaderPattern.FindAllStringSubmatchIndex(content, -1) {
		if !inFence(m[0]) {
			out = append(out, m)
		}
	}
	return out
}

// extractFirstDate returns the date of the first "## YYYY-MM-DD" header
// in content (outside code fences) whose date is a real calendar date, or
// "" if there is none. A header such as "## 2026-13-99" is skipped.
func extractFirstDate(content string) string {
	for _, m := range dateHeaderMatches(content) {
		date := content[m[2]:m[3]]
		if _, err := time.Parse("2006-01-02", date); err == nil {
			return date
		}
	}
	return ""
}

// splitByDateHeader splits append-only content into dated sections, one
// per "## YYYY-MM-DD" header, as a sequence of append_memory calls would
// store them. Content before the first header becomes an undated section
// marked as the preamble, so it keeps reading first. A header whose date
// is not a real calendar date yields an undated section. Content with no
// header at all becomes a single undated section, and blank content yields
// none.
func splitByDateHeader(content string) []store.EntrySection {
	matches := dateHeaderMatches(content)
	if len(matches) == 0 {
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			return nil
		}
		return []store.EntrySection{{Body: trimmed}}
	}

	var sections []store.EntrySection
	if preamble := strings.TrimSpace(content[:matches[0][0]]); preamble != "" {
		sections = append(sections, store.EntrySection{Body: preamble, Preamble: true})
	}

	for i, m := range matches {
		start := m[0]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		date := content[m[2]:m[3]]
		if _, err := time.Parse("2006-01-02", date); err != nil {
			// Not a real calendar date (e.g. "2026-13-99") — treat the
			// section as undated rather than persisting an invalid date.
			date = ""
		}
		sections = append(sections, store.EntrySection{
			Date: date,
			Body: strings.TrimSpace(content[start:end]),
		})
	}

	return sections
}
