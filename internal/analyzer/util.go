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
	"io"
	"os"
	"strings"
)

// maxAnalyzedFileSize caps how large a single marker file (Cargo.toml,
// package.json, README.md, ...) readMarkerFile will read into memory.
// These are all meant to be small, hand-written manifests — a file above
// this size is far more likely to be a mistake (a generated lockfile with
// the wrong name) than a real one worth scanning.
const maxAnalyzedFileSize = 5 * 1024 * 1024 // 5 MiB

// readMarkerFile reads root/name and returns its content, or ("", false)
// if the file doesn't exist, can't be read, exceeds maxAnalyzedFileSize,
// isn't a regular file (a FIFO, a device, a directory), or is reached
// through a symlink pointing outside root. Every detector's marker-file
// read goes through this instead of calling os.ReadFile directly, so these
// rules and "missing file is a no-op, not an error" are enforced in one
// place. The read itself is bounded, so a file growing after the size
// check is still never read past the cap.
func readMarkerFile(root, name string) (string, bool) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return "", false
	}
	defer r.Close()

	info, err := r.Stat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxAnalyzedFileSize {
		return "", false
	}
	f, err := r.Open(name)
	if err != nil {
		return "", false
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return "", false
	}

	data, err := io.ReadAll(io.LimitReader(f, maxAnalyzedFileSize+1))
	if err != nil || len(data) > maxAnalyzedFileSize {
		return "", false
	}
	return string(data), true
}

// appendUnique appends value to *slice unless it's already present —
// used everywhere a detector adds to Languages/Frameworks/Infrastructure
// so no field ever ends up with the same entry twice, whether from one
// detector or several.
func appendUnique(slice *[]string, value string) {
	for _, v := range *slice {
		if v == value {
			return
		}
	}
	*slice = append(*slice, value)
}

// extractTOMLSection returns the raw text between a "[name]" header line
// and the next line starting with "[" (or EOF), or "" when the header is
// absent. It is a line-based scan, not a full TOML parser.
func extractTOMLSection(content, name string) string {
	header := "[" + name + "]"
	lines := strings.Split(content, "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return ""
	}

	var b strings.Builder
	for _, line := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			break
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
