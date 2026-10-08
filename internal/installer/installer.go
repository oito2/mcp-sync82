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
// to remove, ResultManual means a config file holds comments or trailing
// commas, so it was left unchanged and the entry printed to add by hand,
// and ResultFail means the operation failed.
const (
	ResultOK     Result = "ok"
	ResultSkip   Result = "skip"
	ResultManual Result = "manual"
	ResultFail   Result = "fail"
)

// manualEditError reports a config file left unchanged because it holds
// comments or trailing commas, which writing it back would lose; its
// message tells the user what to add by hand.
type manualEditError struct {
	msg string
}

// Error returns the message for the user.
func (e *manualEditError) Error() string { return e.msg }

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

// reportManual prints the line of an install that left a config file for
// the user to edit.
func reportManual(stdout io.Writer, name string) Result {
	fmt.Fprintf(stdout, "  !  %s — manual step needed\n", name)
	return ResultManual
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
// could not be written, ResultManual when the only problem is a file left
// for the user to edit by hand, otherwise ResultOK.
func installFileTarget(t Target, env Env, binaryPath string, stdout, stderr io.Writer) Result {
	paths := t.configPaths(env)
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "[%s] no configuration location found\n", t.Name)
		return reportFailed(stdout, t.Name)
	}
	updated, failed, manual := false, false, false
	for _, path := range paths {
		existed, err := writeEntry(t.Shape, path, binaryPath)
		if manualErr := (*manualEditError)(nil); errors.As(err, &manualErr) {
			fmt.Fprintf(stderr, "[%s] %v\n", t.Name, err)
			manual = true
			continue
		}
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
	if manual {
		return reportManual(stdout, t.Name)
	}
	reportInstalled(stdout, t.Name, updated)
	return ResultOK
}

// writeEntry merges sync82's entry into the shape's object in the config
// file at path, keeping every other key, the file mode and a symlink at
// path. existed reports whether an entry for sync82 was already there. A
// file that already holds the wanted entry is left unchanged. Otherwise, a
// file with comments or trailing commas is left unchanged and the error,
// a *manualEditError, carries the entry to add by hand. It returns an error
// when the file cannot be read or parsed, shape has no Entry, the shape's key
// holds a value that is not a JSON object, or the file cannot be written.
func writeEntry(shape Shape, path, binaryPath string) (existed bool, err error) {
	if shape.Entry == nil {
		return false, fmt.Errorf("%s: no entry format defined", path)
	}
	cfg, strict, raw, err := loadConfig(path)
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
	current, existed := servers[serverName]
	entry := shape.Entry(binaryPath)
	if existed && entryMatches(current, entry) {
		// Already registered as wanted: the file stays as it is, comments
		// included.
		return true, nil
	}
	if !strict {
		snippet, _ := json.MarshalIndent(map[string]any{shape.Key: map[string]any{serverName: entry}}, "", "  ")
		return existed, &manualEditError{fmt.Sprintf("%s contains comments or trailing commas, so it was left unchanged. Add this entry to it by hand:\n%s", path, snippet)}
	}
	value, err := marshalNoEscape(entry)
	if err != nil {
		return existed, err
	}
	return existed, writeServers(path, raw, shape.Key, func(servers []jsonMember) []jsonMember {
		return setMember(servers, serverName, value)
	})
}

// jsonMember is one key of a JSON object and its value as written.
type jsonMember struct {
	key   string
	value json.RawMessage
}

// writeServers rewrites the config file at path, whose content is raw (a
// JSON object, or blank), after passing the members of its key object
// through edit; a missing or null key object counts as empty, and is added
// at the end when missing. Every other key and value stays as in raw, and
// keys keep their order. The file is written indented with two spaces and
// a trailing newline, keeping the mode of an existing file and a symlink at
// path.
func writeServers(path string, raw []byte, key string, edit func([]jsonMember) []jsonMember) error {
	top, err := objectMembers(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var servers []jsonMember
	if v, ok := memberValue(top, key); ok && !bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		if servers, err = objectMembers(v); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	top = setMember(top, key, encodeMembers(edit(servers)))
	var out bytes.Buffer
	if err := json.Indent(&out, encodeMembers(top), "", "  "); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	out.WriteByte('\n')
	return fsutil.AtomicWriteFile(path, out.Bytes(), 0o644)
}

// objectMembers decodes raw, a JSON object, into its members in the order
// they are written, with each value compacted. A key written more than once
// keeps its first position and its last value, the value encoding/json
// decodes. Blank raw has no members. It returns an error when raw is not a
// single JSON object.
func objectMembers(raw []byte) ([]jsonMember, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errUnparsableConfig
	}
	var members []jsonMember
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, errUnparsableConfig
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, value); err != nil {
			return nil, err
		}
		members = setMember(members, key, compact.Bytes())
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return members, nil
}

// memberValue returns the value of key in members, and whether it is there.
func memberValue(members []jsonMember, key string) (json.RawMessage, bool) {
	for _, m := range members {
		if m.key == key {
			return m.value, true
		}
	}
	return nil, false
}

// setMember sets key to value in members: in place when key is there,
// otherwise appended at the end. It returns the updated members.
func setMember(members []jsonMember, key string, value json.RawMessage) []jsonMember {
	for i := range members {
		if members[i].key == key {
			members[i].value = value
			return members
		}
	}
	return append(members, jsonMember{key: key, value: value})
}

// encodeMembers returns members as one compact JSON object, in order. Each
// value is written as it is held.
func encodeMembers(members []jsonMember) json.RawMessage {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := marshalNoEscape(m.key) // a string always encodes
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(m.value)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// marshalNoEscape encodes v as compact JSON, writing characters such as
// "&", "<" and ">" as they are, not escaped, so values the user wrote (URLs,
// commands) don't change form when the file is rewritten.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// fileHasEntry reports whether the config file at path holds an entry
// for sync82 under the shape's key. A missing file holds none. It returns
// the error of a file that exists but can't be read or parsed, so the
// caller reports it instead of taking the file for one without sync82.
func fileHasEntry(path string, shape Shape) (bool, error) {
	if !fileExists(path) {
		return false, nil
	}
	cfg, _, err := readConfig(path)
	if err != nil {
		return false, err
	}
	_, ok := asObject(cfg[shape.Key])[serverName]
	return ok, nil
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
	cfg, strict, _, err = loadConfig(path)
	return cfg, strict, err
}

// loadConfig reads the config file at path as readConfig does, and also
// returns its content, without a UTF-8 byte order mark (nil for a missing
// file), for a rewrite that keeps it as written.
func loadConfig(path string) (cfg map[string]any, strict bool, raw []byte, err error) {
	raw, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, true, nil, nil
	}
	if err != nil {
		return nil, false, nil, err
	}
	// A UTF-8 byte order mark, which some Windows editors write, is not
	// JSON; it is left out, and a rewrite drops it.
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, true, raw, nil
	}
	if cfg, ok := decodeObject(raw); ok {
		return cfg, true, raw, nil
	}
	if cfg, ok := decodeObject(stripJSONC(raw)); ok {
		return cfg, false, raw, nil
	}
	return nil, false, nil, fmt.Errorf("%s: %w", path, errUnparsableConfig)
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

// entryMatches reports whether current, an existing sync82 entry, holds
// every key of want with an equal JSON value. Keys the user added to the
// entry, such as env or cwd, are allowed.
func entryMatches(current any, want map[string]any) bool {
	cur, ok := current.(map[string]any)
	if !ok {
		return false
	}
	for k, v := range want {
		a, errA := json.Marshal(cur[k])
		b, errB := json.Marshal(v)
		if errA != nil || errB != nil || !bytes.Equal(a, b) {
			return false
		}
	}
	return true
}
