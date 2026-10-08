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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// uninstall runs UninstallTarget for target and returns the result with the
// captured stdout and stderr.
func uninstall(t *testing.T, target Target, env Env) (Result, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	got := UninstallTarget(context.Background(), target, env, &stdout, &stderr)
	return got, stdout.String(), stderr.String()
}

// fakeCLITarget returns a CLI target driven by a fakeCLI, with
// codex-like get/remove commands.
func fakeCLITarget(command string) Target {
	return Target{
		Kind:       KindCLI,
		Name:       "fake",
		DetectCmd:  command,
		Command:    command,
		Args:       func(bin string) []string { return []string{"mcp", "add", "sync82", "--", bin} },
		GetArgs:    []string{"mcp", "get", "sync82"},
		RemoveArgs: func(string) []string { return []string{"mcp", "remove", "sync82"} },
	}
}

// TestUninstallTarget_CLI_NotFound_Skip verifies that a CLI target whose command is
// not on PATH is skipped as not detected.
func TestUninstallTarget_CLI_NotFound_Skip(t *testing.T) {
	target := fakeCLITarget("definitely-not-a-real-command-sync82")
	got, out, _ := uninstall(t, target, testEnv("linux", t.TempDir(), nil))
	if got != ResultSkip || !strings.Contains(out, "Skipped: fake not detected.") {
		t.Fatalf("UninstallTarget() = %q, stdout %q; want a skip", got, out)
	}
}

// TestUninstallTarget_CLI_NotConfigured verifies that a CLI target without a
// registration is skipped as not configured and no removal is attempted.
func TestUninstallTarget_CLI_NotConfigured(t *testing.T) {
	command, dir := fakeCLI(t)
	got, out, _ := uninstall(t, fakeCLITarget(command), testEnv("linux", t.TempDir(), nil))
	if got != ResultSkip || !strings.Contains(out, "not configured, skipping") {
		t.Fatalf("UninstallTarget() = %q, stdout %q; want not configured", got, out)
	}
	if c := calls(t, dir); len(c) != 1 || c[0] != "mcp get sync82" {
		t.Errorf("calls = %q, want only the registration check", c)
	}
}

// TestUninstallTarget_CLI_Removes verifies that a registered CLI target is removed
// and reported as such.
func TestUninstallTarget_CLI_Removes(t *testing.T) {
	command, dir := fakeCLI(t)
	writeFile(t, filepath.Join(dir, "registered"), "", 0o644)
	got, out, _ := uninstall(t, fakeCLITarget(command), testEnv("linux", t.TempDir(), nil))
	if got != ResultOK || !strings.Contains(out, "fake — removed.") {
		t.Fatalf("UninstallTarget() = %q, stdout %q; want removed", got, out)
	}
	want := "mcp get sync82|mcp remove sync82"
	if c := strings.Join(calls(t, dir), "|"); c != want {
		t.Errorf("calls = %q, want %q", c, want)
	}
}

// TestUninstallTarget_OpenCodeCleansConfigFiles checks that opencode's
// config files are cleaned even when its command is not on PATH.
func TestUninstallTarget_OpenCodeCleansConfigFiles(t *testing.T) {
	home := t.TempDir()
	env := testEnv("linux", home, nil)
	env.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	target := mustFind(t, "opencode")
	path := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	writeFile(t, path, `{"$schema":"https://opencode.ai/config.json","mcp":{"sync82":{"type":"local","command":["/opt/sync82"]},"other":{"type":"local"}}}`, 0o644)

	if got, out, errOut := uninstall(t, target, env); got != ResultOK || !strings.Contains(out, "opencode — removed.") {
		t.Fatalf("UninstallTarget() = %q, stdout %q, stderr %q; want removed", got, out, errOut)
	}
	cfg := readJSON(t, path)
	mcp := cfg["mcp"].(map[string]any)
	if _, ok := mcp["sync82"]; ok {
		t.Error("sync82 entry still present")
	}
	if _, ok := mcp["other"]; !ok || cfg["$schema"] == nil {
		t.Errorf("other keys dropped: %v", cfg)
	}
	if got, out, _ := uninstall(t, target, env); got != ResultSkip || !strings.Contains(out, "not configured") {
		t.Fatalf("second UninstallTarget() = %q, stdout %q; want not configured", got, out)
	}
}

// TestUninstallTarget_File_MissingFileOrKey verifies that a missing config file, or
// one without the sync82 key, is skipped as not configured without creating files.
func TestUninstallTarget_File_MissingFileOrKey(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "app", "config.json")
	env := testEnv("linux", home, nil)
	if got, out, _ := uninstall(t, fileTargetAt(path), env); got != ResultSkip || !strings.Contains(out, "not configured, skipping") {
		t.Fatalf("missing file: %q, %q", got, out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("uninstall created %s (stat err = %v)", path, err)
	}
	const withoutKey = `{"mcpServers":{"other":{}}}`
	writeFile(t, path, withoutKey, 0o644)
	if got, out, _ := uninstall(t, fileTargetAt(path), env); got != ResultSkip || !strings.Contains(out, "not configured, skipping") {
		t.Fatalf("missing key: %q, %q", got, out)
	}
	if data, _ := os.ReadFile(path); string(data) != withoutKey {
		t.Errorf("config = %q, want it untouched", data)
	}
}

// TestUninstallTarget_File_LeavesJSONCUnchanged checks that a config with
// comments holding sync82 is not rewritten and the target needs a manual
// step, with instructions.
func TestUninstallTarget_File_LeavesJSONCUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := "{\n  // keep me\n  \"mcpServers\": {\"sync82\": {\"command\": \"/opt/sync82\"}}\n}\n"
	writeFile(t, path, original, 0o644)
	got, _, errOut := uninstall(t, fileTargetAt(path), testEnv("linux", t.TempDir(), nil))
	if got != ResultManual || !strings.Contains(errOut, `Remove the "sync82" entry`) {
		t.Fatalf("UninstallTarget() = %q, stderr %q; want a manual step with instructions", got, errOut)
	}
	if data, _ := os.ReadFile(path); string(data) != original {
		t.Fatalf("config = %q, want it untouched", data)
	}
}

// TestUninstallTarget_File_KeepsModeAndSymlink verifies that removing the entry keeps
// the other keys, the file mode and a symlink at the config path.
func TestUninstallTarget_File_KeepsModeAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits and symlinks")
	}
	home := t.TempDir()
	realPath := filepath.Join(home, "dotfiles", "config.json")
	writeFile(t, realPath, `{"mcpServers":{"sync82":{"command":"/opt/sync82"}},"keep":1}`, 0o600)
	link := filepath.Join(home, "app", "config.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, link); err != nil {
		t.Fatal(err)
	}
	if got, _, errOut := uninstall(t, fileTargetAt(link), testEnv("linux", home, nil)); got != ResultOK {
		t.Fatalf("UninstallTarget() = %q; stderr %q", got, errOut)
	}
	assertModeAndSymlink(t, link, realPath)
	cfg := readJSON(t, realPath)
	if _, ok := cfg["mcpServers"].(map[string]any)["sync82"]; ok || cfg["keep"] == nil {
		t.Errorf("config = %v, want sync82 removed and other keys kept", cfg)
	}
}
