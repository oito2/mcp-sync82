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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testEnv returns an Env for goos with home as the home directory, vars as
// the only environment variables, and the real PATH lookup.
func testEnv(goos, home string, vars map[string]string) Env {
	return Env{
		GOOS:     goos,
		HomeDir:  home,
		Getenv:   func(key string) string { return vars[key] },
		LookPath: exec.LookPath,
	}
}

// fakeCLI writes an executable script standing in for a client CLI. It
// appends every argument list it receives to <dir>/calls.log and keeps the
// registration as the file <dir>/registered: "mcp get" exits 0 only while
// that file exists, "mcp add" creates it and "mcp remove" deletes it,
// exiting 1 when there was nothing to remove. It returns the script path
// and the directory.
func fakeCLI(t *testing.T) (command, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake CLI")
	}
	dir = t.TempDir()
	command = filepath.Join(dir, "fakecli")
	script := `#!/bin/sh
d="$(dirname "$0")"
echo "$*" >> "$d/calls.log"
case "$2" in
get) [ -f "$d/registered" ] ;;
add) touch "$d/registered" ;;
remove) [ -f "$d/registered" ] && rm -f "$d/registered" ;;
esac
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return command, dir
}

// calls returns the argument lists a fakeCLI received, one per line.
func calls(t *testing.T, dir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "calls.log"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// readJSON parses the JSON object in path.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s is not valid JSON: %v\n%s", path, err, raw)
	}
	return m
}

// writeFile creates path's directory and writes content to it.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// fileTargetAt returns a file target writing the standard
// mcpServers.sync82 entry into configPath, detected through configPath's
// directory.
func fileTargetAt(configPath string) Target {
	return Target{
		Kind:        KindFile,
		Name:        "test-file",
		Shape:       shapeMCPServers,
		DetectDirs:  func(Env) []string { return []string{filepath.Dir(configPath)} },
		ConfigPaths: func(Env) []string { return []string{configPath} },
	}
}

// install runs InstallTarget for target with the binary path "/opt/sync82" and returns the
// result with the captured stdout and stderr.
func install(t *testing.T, target Target, env Env) (Result, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr)
	return got, stdout.String(), stderr.String()
}

// TestInstallTarget_CLI_NotFound_Skip verifies that a CLI target whose command is
// not on PATH is skipped.
func TestInstallTarget_CLI_NotFound_Skip(t *testing.T) {
	target := Target{Kind: KindCLI, Name: "ghost-cli", DetectCmd: "definitely-not-a-real-command-sync82", Command: "definitely-not-a-real-command-sync82"}
	got, out, _ := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultSkip || !strings.Contains(out, "Skipped: ghost-cli not detected.") {
		t.Fatalf("InstallTarget() = %q, stdout %q; want a skip", got, out)
	}
}

// TestInstallTarget_CLI_Success verifies that a detected CLI target is invoked with
// its MCP-add arguments and reported as configured.
func TestInstallTarget_CLI_Success(t *testing.T) {
	target := Target{Kind: KindCLI, Name: "ok-cli", DetectCmd: "true", Command: "true"}
	got, out, _ := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultOK || !strings.Contains(out, "ok-cli — configured.") {
		t.Fatalf("InstallTarget() = %q, stdout %q; want configured", got, out)
	}
}

// TestInstallTarget_CLI_Failure verifies that a CLI target whose command exits
// non-zero yields ResultFail.
func TestInstallTarget_CLI_Failure(t *testing.T) {
	target := Target{Kind: KindCLI, Name: "fail-cli", DetectCmd: "false", Command: "false"}
	got, out, _ := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultFail || !strings.Contains(out, "failed") {
		t.Fatalf("InstallTarget() = %q, stdout %q; want a failure", got, out)
	}
}

// TestInstallTarget_CLI_ConfiguredThenUpdated checks that a scope-less
// CLI target is probed with GetArgs, that an existing registration is
// removed before the add, and that the removal turns "configured." into
// "updated.".
func TestInstallTarget_CLI_ConfiguredThenUpdated(t *testing.T) {
	command, dir := fakeCLI(t)
	target := fakeCLITarget(command)
	env := testEnv("linux", t.TempDir(), nil)

	if got, out, _ := install(t, target, env); got != ResultOK || !strings.Contains(out, "fake — configured.") {
		t.Fatalf("first install = %q, stdout %q; want configured", got, out)
	}
	if got, out, _ := install(t, target, env); got != ResultOK || !strings.Contains(out, "fake — updated.") {
		t.Fatalf("second install = %q, stdout %q; want updated", got, out)
	}
	want := []string{
		"mcp get sync82", "mcp add sync82 -- /opt/sync82",
		"mcp get sync82", "mcp remove sync82", "mcp add sync82 -- /opt/sync82",
	}
	if got := calls(t, dir); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

// TestInstallTarget_UnsupportedOS_Skip verifies that a target marked unsupported on
// the simulated OS is skipped.
func TestInstallTarget_UnsupportedOS_Skip(t *testing.T) {
	target := Target{Kind: KindFile, Name: "nope", Unsupported: func(Env) string { return "not available on this OS" }}
	got, out, _ := install(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultSkip || !strings.Contains(out, "Skipped: nope (not available on this OS).") {
		t.Fatalf("InstallTarget() = %q, stdout %q; want the OS skip", got, out)
	}
}

// TestInstallTarget_File_SkipsWhenPresenceDirMissing verifies that a file target is
// skipped, and nothing is written, when its detection directory is missing.
func TestInstallTarget_File_SkipsWhenPresenceDirMissing(t *testing.T) {
	home := t.TempDir()
	got, out, _ := install(t, fileTargetAt(filepath.Join(home, "ghost", "config.json")), testEnv("linux", home, nil))
	if got != ResultSkip || !strings.Contains(out, "Skipped: test-file not detected.") {
		t.Fatalf("InstallTarget() = %q, stdout %q; want a skip", got, out)
	}
}

// TestInstallTarget_File_CreatesNewConfig verifies that a missing config file is
// created with sync82's entry.
func TestInstallTarget_File_CreatesNewConfig(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "app", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	got, out, errOut := install(t, fileTargetAt(configPath), testEnv("linux", home, nil))
	if got != ResultOK || !strings.Contains(out, "configured.") {
		t.Fatalf("InstallTarget() = %q, stdout %q, stderr %q; want configured", got, out, errOut)
	}
	entry := readJSON(t, configPath)["mcpServers"].(map[string]any)["sync82"].(map[string]any)
	if entry["command"] != "/opt/sync82" {
		t.Errorf("entry = %v, want the absolute binary path as command", entry)
	}
	if raw, _ := os.ReadFile(configPath); !bytes.HasSuffix(raw, []byte("\n")) {
		t.Error("config file should end with a trailing newline")
	}
}

// TestInstallTarget_File_MergesAndReportsUpdated verifies that sync82's entry is
// merged into an existing config without dropping other keys or servers, and
// that a second install is reported as updated.
func TestInstallTarget_File_MergesAndReportsUpdated(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "app", "config.json")
	writeFile(t, configPath, `{"mcpServers":{"other":{"command":"other-cmd"}},"unrelated":true}`, 0o644)
	env := testEnv("linux", home, nil)

	if got, out, _ := install(t, fileTargetAt(configPath), env); got != ResultOK || !strings.Contains(out, "configured.") {
		t.Fatalf("first install = %q, stdout %q; want configured", got, out)
	}
	if got, out, _ := install(t, fileTargetAt(configPath), env); got != ResultOK || !strings.Contains(out, "updated.") {
		t.Fatalf("second install = %q, stdout %q; want updated", got, out)
	}
	parsed := readJSON(t, configPath)
	if parsed["unrelated"] != true {
		t.Errorf("unrelated key was dropped: %v", parsed)
	}
	servers := parsed["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("pre-existing \"other\" server was dropped: %v", servers)
	}
}

// TestInstallTarget_File_LeavesJSONCUnchanged guards against a config with
// comments being rewritten, losing them.
func TestInstallTarget_File_LeavesJSONCUnchanged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := "{\n  // a comment\n  \"mcpServers\": {},\n}\n"
	writeFile(t, configPath, original, 0o644)

	got, out, errOut := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil))
	if got != ResultManual {
		t.Fatalf("InstallTarget() = %q, want %q", got, ResultManual)
	}
	if !strings.Contains(out, "manual step needed") {
		t.Errorf("stdout = %q, want the manual-step status line", out)
	}
	if data, _ := os.ReadFile(configPath); string(data) != original {
		t.Fatalf("config = %q, want it untouched", data)
	}
	if !strings.Contains(errOut, `"command": "/opt/sync82"`) || !strings.Contains(errOut, `"mcpServers"`) {
		t.Errorf("stderr = %q, want the entry to add by hand", errOut)
	}
}

// TestInstallTarget_File_BrokenConfigFails verifies that an unparsable config
// makes the install fail and leaves the file untouched.
func TestInstallTarget_File_BrokenConfigFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, configPath, "{not json", 0o644)
	if got, _, _ := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil)); got != ResultFail {
		t.Fatalf("InstallTarget() = %q, want %q", got, ResultFail)
	}
	if data, _ := os.ReadFile(configPath); string(data) != "{not json" {
		t.Fatalf("config = %q, want it untouched", data)
	}
}

// TestInstallTarget_File_BlankConfigIsTreatedAsEmpty verifies that a whitespace-only
// config is treated as an empty object and installs without warnings.
func TestInstallTarget_File_BlankConfigIsTreatedAsEmpty(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, configPath, "  \n", 0o644)
	got, _, errOut := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil))
	if got != ResultOK || errOut != "" {
		t.Fatalf("InstallTarget() = %q, stderr %q; want success without warning", got, errOut)
	}
}

// TestInstallTarget_File_KeepsLargeNumbers verifies that an integer too large for
// float64 survives a config rewrite unchanged.
func TestInstallTarget_File_KeepsLargeNumbers(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, configPath, `{"limit": 12345678901234567890}`, 0o644)
	if got, _, errOut := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil)); got != ResultOK {
		t.Fatalf("InstallTarget() = %q; stderr=%q", got, errOut)
	}
	if data, _ := os.ReadFile(configPath); !strings.Contains(string(data), "12345678901234567890") {
		t.Fatalf("config = %s, want the integer kept exactly", data)
	}
}

// TestInstallTarget_File_DetectDirs verifies that a file target is skipped until one
// of its detection directories exists, and is then installed.
func TestInstallTarget_File_DetectDirs(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "elsewhere", "config.json")
	target := fileTargetAt(configPath)
	target.DetectDirs = func(env Env) []string { return []string{filepath.Join(env.HomeDir, "marker")} }
	env := testEnv("linux", home, nil)

	if got, _, _ := install(t, target, env); got != ResultSkip {
		t.Fatalf("InstallTarget() without marker = %q, want skip", got)
	}
	if err := os.MkdirAll(filepath.Join(home, "marker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _, errOut := install(t, target, env); got != ResultOK {
		t.Fatalf("InstallTarget() = %q; stderr=%q", got, errOut)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config not written to %s: %v", configPath, err)
	}
}

// TestInstallTarget_File_KeepsConfigModeAndSymlink guards against a 0600
// client config being rewritten world-readable, and a symlinked config
// being replaced by a regular file.
func TestInstallTarget_File_KeepsConfigModeAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits and symlinks")
	}
	home := t.TempDir()
	realPath := filepath.Join(home, "dotfiles", "config.json")
	writeFile(t, realPath, `{"mcpServers":{}}`, 0o600)
	link := filepath.Join(home, "app", "config.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, link); err != nil {
		t.Fatal(err)
	}
	if got, _, errOut := install(t, fileTargetAt(link), testEnv("linux", home, nil)); got != ResultOK {
		t.Fatalf("InstallTarget() = %q; stderr=%q", got, errOut)
	}
	assertModeAndSymlink(t, link, realPath)
	if _, ok := readJSON(t, realPath)["mcpServers"].(map[string]any)["sync82"]; !ok {
		t.Error("sync82 entry not written through the symlink")
	}
}

// assertModeAndSymlink checks that link is still a symlink and that realPath
// kept mode 0600.
func assertModeAndSymlink(t *testing.T, link, realPath string) {
	t.Helper()
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%s is no longer a symlink", link)
	}
	if info, err := os.Stat(realPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("%s mode changed, want 0600", realPath)
	}
}

// TestReadConfig verifies readConfig for plain JSON, JSONC, non-object and
// trailing-data content, and for a missing file.
func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, content string
		strict, fails bool
	}{
		{"plain", `{"a": 1}`, true, false},
		{"jsonc", "{\n // c\n \"a\": \"//not a comment\", /* b */\n}", false, false},
		{"array", `[1]`, false, true},
		{"trailing data", `{} {}`, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name+".json")
			writeFile(t, path, c.content, 0o644)
			cfg, strict, err := readConfig(path)
			if (err != nil) != c.fails {
				t.Fatalf("readConfig() error = %v, want failure %v", err, c.fails)
			}
			if !c.fails && strict != c.strict {
				t.Errorf("strict = %v, want %v", strict, c.strict)
			}
			if c.name == "jsonc" && cfg["a"] != "//not a comment" {
				t.Errorf("string content changed: %v", cfg)
			}
		})
	}
	if cfg, strict, err := readConfig(filepath.Join(dir, "missing.json")); err != nil || !strict || len(cfg) != 0 {
		t.Errorf("missing file = %v, %v, %v; want an empty strict object", cfg, strict, err)
	}
}

// TestAsObject verifies that asObject returns an empty map for non-object values
// and the same map for an object.
func TestAsObject(t *testing.T) {
	for _, in := range []any{nil, []any{1}, "oops"} {
		if got := asObject(in); len(got) != 0 {
			t.Errorf("asObject(%v) = %v, want empty", in, got)
		}
	}
	if got := asObject(map[string]any{"a": 1}); len(got) != 1 {
		t.Errorf("asObject(object) = %v, want it unchanged", got)
	}
}

// TestInstallTarget_File_KeepsUserValuesAndRefusesBadShapes verifies that a
// rewrite keeps characters such as "&" and "<" as written, that a
// non-object server key is refused instead of replaced, and that a file
// with a stray closing bracket after the object is not rewritten.
func TestInstallTarget_File_KeepsUserValuesAndRefusesBadShapes(t *testing.T) {
	env := testEnv("linux", t.TempDir(), nil)

	configPath := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, configPath, `{"mcpServers":{"other":{"url":"https://x?a=1&b=<2>"}}}`, 0o644)
	if got, _, _ := install(t, fileTargetAt(configPath), env); got != ResultOK {
		t.Fatalf("install = %q", got)
	}
	if data, _ := os.ReadFile(configPath); !strings.Contains(string(data), `https://x?a=1&b=<2>`) {
		t.Errorf("the user's URL was re-escaped: %s", data)
	}

	for _, original := range []string{`{"mcpServers":[]}`, `{"mcpServers":"x"}`, `{"mcpServers":{}}}`, `{"a":1}]`} {
		path := filepath.Join(t.TempDir(), "config.json")
		writeFile(t, path, original, 0o644)
		if got, _, _ := install(t, fileTargetAt(path), env); got != ResultFail {
			t.Errorf("%s: install = %q, want %q", original, got, ResultFail)
		}
		if data, _ := os.ReadFile(path); string(data) != original {
			t.Errorf("%s: the file was rewritten to %q", original, data)
		}
	}
}

// TestInstallTarget_File_JSONCAlreadyRegistered checks that a config with
// comments that already holds the wanted entry counts as installed,
// unchanged, instead of needing a manual step on every run; a differing
// entry still needs one.
func TestInstallTarget_File_JSONCAlreadyRegistered(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := "{\n  // a comment\n  \"mcpServers\": {\"sync82\": {\"command\": \"/opt/sync82\", \"args\": [], \"env\": {\"X\": \"1\"}}}\n}\n"
	writeFile(t, configPath, original, 0o644)
	got, out, _ := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil))
	if got != ResultOK || !strings.Contains(out, "updated.") {
		t.Fatalf("InstallTarget() = %q, stdout %q; want an unchanged success", got, out)
	}
	if data, _ := os.ReadFile(configPath); string(data) != original {
		t.Fatalf("config = %q, want it untouched", data)
	}

	writeFile(t, configPath, "{\n  // a comment\n  \"mcpServers\": {\"sync82\": {\"command\": \"/old/sync82\", \"args\": []}}\n}\n", 0o644)
	if got, _, _ := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil)); got != ResultManual {
		t.Errorf("a differing entry: InstallTarget() = %q, want %q", got, ResultManual)
	}
}

// TestFileTargets_ByteOrderMark checks that a config file starting with a
// UTF-8 byte order mark is read: install writes the entry (dropping the
// mark), and uninstall finds and removes it.
func TestFileTargets_ByteOrderMark(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mcp.json")
	writeFile(t, configPath, "\xef\xbb\xbf{\"mcpServers\": {\"other\": {\"command\": \"x\"}}}", 0o644)
	if got, _, errOut := install(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil)); got != ResultOK {
		t.Fatalf("install with a BOM = %q, stderr %q", got, errOut)
	}
	if has, err := fileHasEntry(configPath, shapeMCPServers); err != nil || !has {
		t.Fatalf("entry after install: %v, %v", has, err)
	}
	writeFile(t, configPath, "\xef\xbb\xbf{\"mcpServers\": {\"sync82\": {\"command\": \"/opt/sync82\"}}}", 0o644)
	if got, _, _ := uninstall(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil)); got != ResultOK {
		t.Errorf("uninstall with a BOM = %q, want the entry removed", got)
	}
}

// TestUninstallTarget_File_UnreadableConfigFails checks that a config file
// that can't be parsed is reported as a failure, not as a file without
// sync82.
func TestUninstallTarget_File_UnreadableConfigFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mcp.json")
	writeFile(t, configPath, "not json at all", 0o644)
	got, _, errOut := uninstall(t, fileTargetAt(configPath), testEnv("linux", t.TempDir(), nil))
	if got != ResultFail || errOut == "" {
		t.Errorf("uninstall of an unparsable config = %q, stderr %q; want a reported failure", got, errOut)
	}
}

// TestClineCLIDirs_ClineDir checks that an absolute CLINE_DIR replaces
// ~/.cline, that CLINE_DATA_DIR still wins over it, and that a relative
// CLINE_DIR is ignored.
func TestClineCLIDirs_ClineDir(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(t.TempDir(), "cline")
	if marker, data := clineCLIDirs(testEnv("linux", home, map[string]string{"CLINE_DIR": custom})); marker != custom || data != filepath.Join(custom, "data") {
		t.Errorf("CLINE_DIR: %s, %s", marker, data)
	}
	dataDir := filepath.Join(t.TempDir(), "data")
	if marker, data := clineCLIDirs(testEnv("linux", home, map[string]string{"CLINE_DIR": custom, "CLINE_DATA_DIR": dataDir})); marker != dataDir || data != dataDir {
		t.Errorf("CLINE_DATA_DIR with CLINE_DIR: %s, %s", marker, data)
	}
	if marker, _ := clineCLIDirs(testEnv("linux", home, map[string]string{"CLINE_DIR": "relative"})); marker != filepath.Join(home, ".cline") {
		t.Errorf("relative CLINE_DIR: %s, want ~/.cline", marker)
	}
}
