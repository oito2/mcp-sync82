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
	"strings"
	"unicode"
)

// detectReadme sets Description from the first non-empty line after
// README.md's first heading — an ATX "#" heading, or a setext title (a
// text line underlined with "=") — provided that line doesn't start with
// #, !, [, <, >, or | (filters out badges, HTML, blockquotes, tables) and
// starts with an ASCII-printable character (filters emoji-first lines).
// Lines inside ``` or ~~~ code fences are never treated as a heading or
// as the description.
func detectReadme(root string, r *Result) {
	if r.Description != "" {
		return
	}

	content, ok := readMarkerFile(root, "README.md")
	if !ok {
		return
	}

	lines := strings.Split(content, "\n")
	inFence := markdownFenceTracker()
	headingIndex := -1
	for i, line := range lines {
		if inFence(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			headingIndex = i
			break
		}
		if trimmed != "" && i+1 < len(lines) && isSetextTitleUnderline(lines[i+1]) {
			headingIndex = i + 1
			break
		}
	}
	if headingIndex == -1 {
		return
	}

	for _, line := range lines[headingIndex+1:] {
		if inFence(line) {
			continue
		}
		candidate := strings.TrimSpace(line)
		if candidate == "" {
			continue
		}
		if strings.ContainsAny(candidate[:1], "#![<>|") {
			continue
		}
		if !isASCIIPrintableStart(candidate) {
			continue
		}
		setDescription(r, candidate)
		return
	}
}

// markdownFenceTracker returns a function to be called on each line in
// order; it reports whether that line is a code-fence delimiter or lies
// inside a fenced code block. A fence opens with a line starting with at
// least three backticks or tildes and closes with a line starting with
// at least as many of the same character.
func markdownFenceTracker() func(line string) bool {
	var fenceChar byte
	fenceLen := 0
	return func(line string) bool {
		trimmed := strings.TrimSpace(line)
		n := 0
		if trimmed != "" && (trimmed[0] == '`' || trimmed[0] == '~') {
			for n < len(trimmed) && trimmed[n] == trimmed[0] {
				n++
			}
		}
		if fenceLen == 0 {
			if n >= 3 {
				fenceChar, fenceLen = trimmed[0], n
				return true
			}
			return false
		}
		if n >= fenceLen && trimmed[0] == fenceChar && strings.TrimSpace(trimmed[n:]) == "" {
			fenceLen = 0
		}
		return true
	}
}

// isSetextTitleUnderline reports whether line consists only of "="
// characters (at least one), ignoring surrounding whitespace.
func isSetextTitleUnderline(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed != "" && strings.Trim(trimmed, "=") == ""
}

// isASCIIPrintableStart reports whether the first byte of s is a printable
// ASCII character. s must not be empty.
func isASCIIPrintableStart(s string) bool {
	c := s[0]
	return c >= 0x20 && c < 0x7F
}

// cleanDescription strips **bold**/`code` markers and truncates to 200
// characters (runes, not bytes — truncating by byte could split a
// multi-byte UTF-8 character in half).
func cleanDescription(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.TrimSpace(s)

	runes := []rune(s)
	if len(runes) > 200 {
		runes = runes[:200]
	}
	return string(runes)
}

// setDescription stores s as r's description when none is set yet, with
// newlines and other control characters collapsed into single spaces and
// cleanDescription applied — manifest descriptions end up on one line of
// generated Markdown, where a newline could otherwise inject headings.
func setDescription(r *Result, s string) {
	if r.Description != "" {
		return
	}
	s = strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return ' '
		}
		return c
	}, s)
	r.Description = cleanDescription(strings.Join(strings.Fields(s), " "))
}
