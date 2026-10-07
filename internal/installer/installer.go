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

package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"time"

	"github.com/oito2/mcp-sync82/internal/fsutil"
)

// InstallTimeout bounds how long each of a CLI target's own subcommands
// may run.
const InstallTimeout = 30 * time.Second

// Result is the outcome of installing into, or uninstalling from, a
// single target.
type Result string

// Result values: ResultOK means the registration was written or removed,
// ResultSkip means the target was unsupported, not detected or had nothing
// to remove, and ResultFail means the operation failed.
const (
	ResultOK   Result = "ok"
	ResultSkip Result = "skip"
	ResultFail Result = "fail"
)

// InstallTarget installs sync82 into a single target, dispatching by kind,
// and prints "configured." for a new registration or "updated." when one
// already existed, followed by any warning. A target unsupported on
// env.GOOS or not detected is skipped. stdout/stderr receive the progress
// lines; a CLI target's MCP-add subprocess also inherits them (plus
// stdin), so its output and prompts reach the user directly.
//
// binaryPath is the absolute path of the sync82 binary the client is
// configured to launch.
func InstallTarget(ctx context.Context, t Target, env Env, binaryPath string, stdout, stderr io.Writer) Result {
	if reason := t.unsupported(env); reason != "" {
		return reportUnsupported(stdout, t.Name, reason)
	}
	if !t.Detected(env) {
		return reportNotDetected(stdout, t.Name)
	}
	fmt.Fprintf(stdout, "\nInstalling into %s...\n", t.Name)
	if t.Kind == KindFile {
		return installFileTarget(t, env, binaryPath, stdout, stderr)
	}
	return installCLITarget(ctx, t, binaryPath, stdout, stderr)
}

// reportUnsupported prints the line for a target unavailable on this OS.
func reportUnsupported(stdout io.Writer, name, reason string) Result {
	fmt.Fprintf(stdout, "Skipped: %s (%s).\n", name, reason)
	return ResultSkip
}

// reportNotDetected prints the line for a target whose client is not
// installed.
func reportNotDetected(stdout io.Writer, name string) Result {
	fmt.Fprintf(stdout, "Skipped: %s not detected.\n", name)
	return ResultSkip
}

// reportInstalled prints the success line of an install.
func reportInstalled(stdout io.Writer, name string, updated bool) {
	if updated {
		fmt.Fprintf(stdout, "  ✓  %s — updated.\n", name)
		return
	}
	fmt.Fprintf(stdout, "  ✓  %s — configured.\n", name)
}

// reportFailed prints the failure line of an install or uninstall.
func reportFailed(stdout io.Writer, name string) Result {
	fmt.Fprintf(stdout, "  ✗  %s — failed\n", name)
	return ResultFail
}

// installCLITarget removes every existing registration of sync82 from the
// target, then runs its MCP-add command. "updated." means a registration
// was removed first.
func installCLITarget(ctx context.Context, t Target, binaryPath string, stdout, stderr io.Writer) Result {
	updated, warnings, err := removeCLIRegistration(ctx, t)
	if err != nil {
		fmt.Fprintf(stderr, "[%s] %v\n", t.Name, err)
		reportFailed(stdout, t.Name)
		printWarnings(stdout, warnings)
		return ResultFail
	}

	var args []string
	if t.Args != nil {
		args = t.Args(binaryPath)
	}
	cctx, cancel := context.WithTimeout(ctx, InstallTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, t.Command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintf(stdout, "  ✗  %s — failed (exit code %d)\n", t.Name, exitErr.ExitCode())
		} else {
			fmt.Fprintf(stdout, "  ✗  %s — failed (%v)\n", t.Name, err)
		}
		printWarnings(stdout, warnings)
		return ResultFail
	}
	reportInstalled(stdout, t.Name, updated)
	printWarnings(stdout, warnings)
	return ResultOK
}

// installFileTarget merges sync82's entry into every config file of a file
// target, printing errors to stderr and the status line to stdout. It
// returns ResultFail when the target has no config location or any file
// could not be written, otherwise ResultOK.
func installFileTarget(t Target, env Env, binaryPath string, stdout, stderr io.Writer) Result {
	paths := t.configPaths(env)
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "[%s] no configuration location found\n", t.Name)
		return reportFailed(stdout, t.Name)
	}
	updated, failed := false, false
	for _, path := range paths {
		existed, err := writeEntry(t.Shape, path, binaryPath)
		if err != nil {
			fmt.Fprintf(stderr, "[%s] %v\n", t.Name, err)
			failed = true
			continue
		}
		updated = updated || existed
	}
	if failed {
		return reportFailed(stdout, t.Name)
	}
	reportInstalled(stdout, t.Name, updated)
	return ResultOK
}

// writeEntry merges sync82's entry into the shape's object in the config
// file at path, keeping every other key, the file mode and a symlink at
// path. existed reports whether an entry for sync82 was already there. A
// file with comments or trailing commas is left unchanged and the error
// carries the entry to add by hand.
func writeEntry(shape Shape, path, binaryPath string) (existed bool, err error) {
	if shape.Entry == nil {
		return false, fmt.Errorf("%s: no entry format defined", path)
	}
	cfg, strict, err := readConfig(path)
	if err != nil {
		return false, err
	}
	if v, present := cfg[shape.Key]; present && v != nil {
		if _, ok := v.(map[string]any); !ok {
			// Replacing it with an object holding only sync82 would drop
			// whatever the user keeps there.
			return false, fmt.Errorf("%s: %q is not a JSON object, so the file was left unchanged; fix it, then install again", path, shape.Key)
		}
	}
	servers := asObject(cfg[shape.Key])
	_, existed = servers[serverName]
	entry := shape.Entry(binaryPath)
	if !strict {
		snippet, _ := json.MarshalIndent(map[string]any{shape.Key: map[string]any{serverName: entry}}, "", "  ")
		return existed, fmt.Errorf("%s contains comments or trailing commas, so it was left unchanged. Add this entry to it by hand:\n%s", path, snippet)
	}
	servers[serverName] = entry
	cfg[shape.Key] = servers
	return existed, writeConfig(path, cfg)
}

// writeConfig writes cfg as indented JSON with a trailing newline to
// path, keeping the mode of an existing file. Characters such as "&", "<"
// and ">" are written as they are, not escaped, so values the user wrote
// (URLs, commands) don't change form when the file is rewritten.
func writeConfig(path string, cfg map[string]any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	return fsutil.AtomicWriteFile(path, buf.Bytes(), 0o644)
}

// fileHasEntry reports whether the config file at path holds an entry
// for sync82 under the shape's key. A missing or unreadable file holds
// none.
func fileHasEntry(path string, shape Shape) bool {
	if !fileExists(path) {
		return false
	}
	cfg, _, err := readConfig(path)
	if err != nil {
		return false
	}
	_, ok := asObject(cfg[shape.Key])[serverName]
	return ok
}

// errUnparsableConfig reports a config file whose content is not a JSON
// object, even after removing comments and trailing commas.
var errUnparsableConfig = errors.New("config is not a JSON object")

// readConfig reads the JSON object at path. A missing or blank file yields
// an empty object. Numbers are kept as written (json.Number), so a
// rewrite doesn't round large integers through float64. strict reports
// whether the file is plain JSON; it is false for a file that only parses
// after removing comments and trailing commas (JSONC), which callers must
// not rewrite. Content that parses neither way returns an error wrapping
// errUnparsableConfig.
func readConfig(path string) (cfg map[string]any, strict bool, err error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, true, nil
	}
	if cfg, ok := decodeObject(raw); ok {
		return cfg, true, nil
	}
	if cfg, ok := decodeObject(stripJSONC(raw)); ok {
		return cfg, false, nil
	}
	return nil, false, fmt.Errorf("%s: %w", path, errUnparsableConfig)
}

// decodeObject decodes raw as exactly one JSON object, keeping numbers as
// json.Number. Anything after the object other than white space, even a
// stray "}" or "]", makes it fail.
func decodeObject(raw []byte) (map[string]any, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var cfg map[string]any
	if err := dec.Decode(&cfg); err != nil || cfg == nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return cfg, true
}

// asObject defensively coerces v to a map, returning an empty one for any
// non-object shape, so a merge never panics on an unexpected config file.
func asObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
