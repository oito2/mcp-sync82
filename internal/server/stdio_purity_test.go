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

package server

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// stdioPurityExceptions lists the only source files, relative to the module
// root, allowed to name stdout. cmd/sync82/main.go hands os.Stdout to the
// subcommand dispatcher, which passes it as an io.Writer to the CLI-only
// subcommands (serve mode never writes to it outside the MCP transport). The
// release program is a separate command that never serves MCP.
var stdioPurityExceptions = map[string]bool{
	filepath.Join("cmd", "sync82", "main.go"):      true,
	filepath.Join("scripts", "release", "main.go"): true, // maintainer-only release tool, never runs as the MCP server
}

// stdoutCallPattern matches a direct stdout write or reference.
var stdoutCallPattern = regexp.MustCompile(`\bfmt\.Print(ln|f)?\(|\bos\.Stdout\b`)

// TestStdioPurity_NoStrayStdoutWrites enforces the stdio transport's
// invariant that nothing but the JSON-RPC protocol touches stdout: a stray
// fmt.Println anywhere in the module would corrupt the protocol stream of
// every client.
func TestStdioPurity_NoStrayStdoutWrites(t *testing.T) {
	root := filepath.Join("..", "..")
	var violations []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".dev" || name == ".git" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if stdioPurityExceptions[rel] {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if stdoutCallPattern.Match(content) {
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module source: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("stdout writes outside the allowed files (they would corrupt the stdio protocol): %v", violations)
	}
}

// TestStdoutCallPattern_Detection pins stdoutCallPattern with positive
// and negative snippets, so weakening it fails here first.
func TestStdoutCallPattern_Detection(t *testing.T) {
	for _, snippet := range []string{
		`fmt.Println("hello")`,
		`fmt.Print("hello")`,
		`fmt.Printf("%s\n", name)`,
		`os.Stdout.Write(b)`,
		`fmt.Fprintln(os.Stdout, "hello")`,
	} {
		if !stdoutCallPattern.MatchString(snippet) {
			t.Errorf("expected a match for %q", snippet)
		}
	}
	for _, snippet := range []string{
		`fmt.Fprintln(os.Stderr, "hello")`,
		`fmt.Fprintf(w, "%s", name)`,
		`fmt.Sprintf("%s", name)`,
		`fmt.Errorf("boom: %w", err)`,
		`log.Println("hello")`,
	} {
		if stdoutCallPattern.MatchString(snippet) {
			t.Errorf("expected no match for %q", snippet)
		}
	}
}
