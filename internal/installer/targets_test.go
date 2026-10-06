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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestTargetNames verifies the names and order of the built-in targets.
func TestTargetNames(t *testing.T) {
	want := []string{"claude", "claude-desktop", "antigravity", "codex", "opencode", "cursor", "zed", "cline"}
	if got := TargetNames(); !slices.Equal(got, want) {
		t.Fatalf("TargetNames() = %v, want %v", got, want)
	}
}

// TestFind verifies that Find returns known targets and rejects unknown names.
func TestFind(t *testing.T) {
	if _, ok := Find("claude"); !ok {
		t.Fatal("expected to find target \"claude\"")
	}
	for _, name := range []string{"nonexistent", "windsurf", "antigravity-ide", "antigravity-global"} {
		if _, ok := Find(name); ok {
			t.Errorf("target %q should not exist", name)
		}
	}
}

// mustFind returns the built-in target named name, failing the test when it does
// not exist.
func mustFind(t *testing.T, name string) Target {
	t.Helper()
	target, ok := Find(name)
	if !ok {
		t.Fatalf("target %q not found", name)
	}
	return target
}

// TestTargets_PathsPerOS pins every file target's config paths for each
// simulated operating system and environment.
func TestTargets_PathsPerOS(t *testing.T) {
	home := filepath.Join("/", "home", "u")
	j := filepath.Join
	cases := []struct {
		target, goos string
		vars         map[string]string
		want         []string
	}{
		{"claude-desktop", "darwin", nil, []string{j(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")}},
		{"claude-desktop", "windows", map[string]string{"APPDATA": j("/", "appdata")}, []string{j("/", "appdata", "Claude", "claude_desktop_config.json")}},
		{"claude-desktop", "windows", nil, []string{j(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json")}},
		{"claude-desktop", "linux", nil, []string{j(home, ".config", "Claude", "claude_desktop_config.json")}},
		{"claude-desktop", "linux", map[string]string{"XDG_CONFIG_HOME": j("/", "xdg")}, []string{j("/", "xdg", "Claude", "claude_desktop_config.json")}},
		{"claude-desktop", "linux", map[string]string{"XDG_CONFIG_HOME": "relative"}, []string{j(home, ".config", "Claude", "claude_desktop_config.json")}},
		{"claude-desktop", "freebsd", nil, nil},
		{"antigravity", "linux", nil, []string{j(home, ".gemini", "config", "mcp_config.json")}},
		{"antigravity", "windows", nil, []string{j(home, ".gemini", "config", "mcp_config.json")}},
		{"opencode", "linux", nil, []string{j(home, ".config", "opencode", "opencode.json")}},
		{"opencode", "linux", map[string]string{"XDG_CONFIG_HOME": j("/", "xdg")}, []string{j("/", "xdg", "opencode", "opencode.json")}},
		{"opencode", "windows", nil, []string{j(home, ".config", "opencode", "opencode.json")}},
		{"cursor", "windows", nil, []string{j(home, ".cursor", "mcp.json")}},
		{"zed", "linux", nil, []string{j(home, ".config", "zed", "settings.json")}},
		{"zed", "linux", map[string]string{"XDG_CONFIG_HOME": j("/", "xdg")}, []string{j("/", "xdg", "zed", "settings.json")}},
		{"zed", "linux", map[string]string{"XDG_CONFIG_HOME": "relative"}, []string{j(home, ".config", "zed", "settings.json")}},
		{"zed", "darwin", map[string]string{"XDG_CONFIG_HOME": j("/", "xdg")}, []string{j(home, ".config", "zed", "settings.json")}},
		{"zed", "windows", map[string]string{"APPDATA": j("/", "appdata")}, []string{j("/", "appdata", "Zed", "settings.json")}},
	}
	for _, c := range cases {
		got := mustFind(t, c.target).configPaths(testEnv(c.goos, home, c.vars))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s on %s %v: configPaths = %q, want %q", c.target, c.goos, c.vars, got, c.want)
		}
	}
}

// TestTargets_RemovePathsPerOS pins the files uninstall cleans for targets with
// extra remove paths, for each simulated operating system and environment.
func TestTargets_RemovePathsPerOS(t *testing.T) {
	home := filepath.Join("/", "home", "u")
	j := filepath.Join
	ext := func(userDir string) string {
		return j(userDir, "globalStorage", "saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json")
	}
	cases := []struct {
		target, goos string
		vars         map[string]string
		want         []string
	}{
		{"antigravity", "linux", nil, []string{
			j(home, ".gemini", "config", "mcp_config.json"),
			j(home, ".gemini", "antigravity", "mcp_config.json"),
			j(home, ".gemini", "antigravity-ide", "mcp_config.json"),
		}},
		{"opencode", "linux", nil, []string{
			j(home, ".config", "opencode", "opencode.json"),
			j(home, ".config", "opencode", "opencode.jsonc"),
			j(home, ".config", "opencode", "config.json"),
		}},
		{"opencode", "linux", map[string]string{"XDG_CONFIG_HOME": j("/", "xdg")}, []string{
			j("/", "xdg", "opencode", "opencode.json"),
			j("/", "xdg", "opencode", "opencode.jsonc"),
			j("/", "xdg", "opencode", "config.json"),
		}},
		{"cline", "linux", nil, []string{
			ext(j(home, ".config", "Code", "User")),
			j(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
		}},
		{"cline", "darwin", map[string]string{"CLINE_DATA_DIR": j("/", "cline")}, []string{
			ext(j(home, "Library", "Application Support", "Code", "User")),
			j("/", "cline", "settings", "cline_mcp_settings.json"),
		}},
		{"cline", "windows", map[string]string{"APPDATA": j("/", "appdata")}, []string{
			ext(j("/", "appdata", "Code", "User")),
			j(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
		}},
		{"cline", "linux", map[string]string{"CLINE_MCP_SETTINGS_PATH": j("/", "custom", "mcp.json")}, []string{
			ext(j(home, ".config", "Code", "User")),
			j(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
			j("/", "custom", "mcp.json"),
		}},
		{"cline", "linux", map[string]string{"CLINE_MCP_SETTINGS_PATH": "relative/mcp.json"}, []string{
			ext(j(home, ".config", "Code", "User")),
			j(home, ".cline", "data", "settings", "cline_mcp_settings.json"),
		}},
	}
	for _, c := range cases {
		got := mustFind(t, c.target).removePaths(testEnv(c.goos, home, c.vars))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s on %s %v: removePaths = %q, want %q", c.target, c.goos, c.vars, got, c.want)
		}
	}
}

// TestTargets_Shapes pins each file target's config key and the entry built for
// a binary path.
func TestTargets_Shapes(t *testing.T) {
	cases := map[string]struct {
		key   string
		entry map[string]any
	}{
		"claude-desktop": {"mcpServers", map[string]any{"command": "/opt/sync82", "args": []string{}}},
		"antigravity":    {"mcpServers", map[string]any{"command": "/opt/sync82", "args": []string{}}},
		"cursor":         {"mcpServers", map[string]any{"type": "stdio", "command": "/opt/sync82", "args": []string{}}},
		"zed":            {"context_servers", map[string]any{"command": "/opt/sync82", "args": []string{}}},
		"cline":          {"mcpServers", map[string]any{"command": "/opt/sync82", "args": []string{}}},
		"opencode":       {"mcp", map[string]any{"type": "local", "command": []string{"/opt/sync82"}, "enabled": true}},
	}
	for name, want := range cases {
		target := mustFind(t, name)
		if target.Shape.Key != want.key {
			t.Errorf("%s: Shape.Key = %q, want %q", name, target.Shape.Key, want.key)
		}
		got := target.Shape.Entry("/opt/sync82")
		if !reflect.DeepEqual(got, want.entry) {
			t.Errorf("%s: entry = %v, want %v", name, got, want.entry)
		}
	}
}

// TestCLITargets_Args pins each CLI target's exact arguments: claude must
// register in user scope and remove per scope, and every client must
// launch the absolute binary path.
func TestCLITargets_Args(t *testing.T) {
	cases := map[string]struct {
		args, get, remove []string
		scoped            bool
	}{
		"claude": {
			[]string{"mcp", "add", "--scope", "user", "sync82", "--", "/opt/sync82"},
			[]string{"mcp", "get", "sync82"},
			[]string{"mcp", "remove", "--scope", "local", "sync82"},
			true,
		},
		"codex": {
			[]string{"mcp", "add", "sync82", "--", "/opt/sync82"},
			[]string{"mcp", "get", "sync82"},
			[]string{"mcp", "remove", "sync82"},
			false,
		},
	}
	for name, want := range cases {
		target := mustFind(t, name)
		if target.Kind != KindCLI || target.Command != name || target.DetectCmd != name {
			t.Errorf("%s: Kind/Command/DetectCmd = %q/%q/%q", name, target.Kind, target.Command, target.DetectCmd)
		}
		if got := target.Args("/opt/sync82"); !slices.Equal(got, want.args) {
			t.Errorf("%s: Args = %q, want %q", name, got, want.args)
		}
		if !slices.Equal(target.GetArgs, want.get) {
			t.Errorf("%s: GetArgs = %q, want %q", name, target.GetArgs, want.get)
		}
		if got := target.RemoveArgs("local"); !slices.Equal(got, want.remove) {
			t.Errorf("%s: RemoveArgs(local) = %q, want %q", name, got, want.remove)
		}
		if (target.Scope != nil) != want.scoped {
			t.Errorf("%s: Scope set = %v, want %v", name, target.Scope != nil, want.scoped)
		}
	}
	for _, name := range []string{"claude-desktop", "antigravity", "opencode", "cursor", "zed", "cline"} {
		if target := mustFind(t, name); target.Kind != KindFile {
			t.Errorf("%s: Kind = %q, want %q", name, target.Kind, KindFile)
		}
	}
}

// TestClaudeDesktop_UnsupportedOS checks that Claude Desktop is skipped,
// and never detected, on systems other than macOS, Windows and Linux.
func TestClaudeDesktop_UnsupportedOS(t *testing.T) {
	home := t.TempDir()
	env := testEnv("freebsd", home, nil)
	target := mustFind(t, "claude-desktop")
	if target.Detected(env) {
		t.Error("claude-desktop detected on freebsd")
	}
	want := "Skipped: claude-desktop (Claude Desktop is only available for macOS, Windows and Linux)."
	var stdout, stderr bytes.Buffer
	if got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr); got != ResultSkip || !strings.Contains(stdout.String(), want) {
		t.Fatalf("install on freebsd = %q, stdout %q", got, stdout.String())
	}
	stdout.Reset()
	if got := UninstallTarget(context.Background(), target, env, &stdout, &stderr); got != ResultSkip || !strings.Contains(stdout.String(), want) {
		t.Fatalf("uninstall on freebsd = %q, stdout %q", got, stdout.String())
	}
}

// TestFileTargets_RoundTrip installs into and uninstalls from every file
// target with a fake home and a simulated OS: the first install reports
// "configured.", the second "updated.", the uninstall removes only the
// sync82 entry, and a second uninstall finds nothing to remove.
func TestFileTargets_RoundTrip(t *testing.T) {
	cases := []struct {
		target, goos string
		vars         func(home string) map[string]string
		presence     func(home string) string
		config       func(home string) string
		command      string
	}{
		{"claude-desktop", "darwin", nil,
			func(h string) string { return filepath.Join(h, "Library", "Application Support", "Claude") },
			func(h string) string {
				return filepath.Join(h, "Library", "Application Support", "Claude", "claude_desktop_config.json")
			}, ""},
		{"claude-desktop", "windows", func(h string) map[string]string { return map[string]string{"APPDATA": filepath.Join(h, "roaming")} },
			func(h string) string { return filepath.Join(h, "roaming", "Claude") },
			func(h string) string { return filepath.Join(h, "roaming", "Claude", "claude_desktop_config.json") }, ""},
		{"claude-desktop", "linux", nil,
			func(h string) string { return filepath.Join(h, ".config", "Claude") },
			func(h string) string { return filepath.Join(h, ".config", "Claude", "claude_desktop_config.json") }, ""},
		{"claude-desktop", "linux", func(h string) map[string]string { return map[string]string{"XDG_CONFIG_HOME": filepath.Join(h, "xdg")} },
			func(h string) string { return filepath.Join(h, "xdg", "Claude") },
			func(h string) string { return filepath.Join(h, "xdg", "Claude", "claude_desktop_config.json") }, ""},
		{"antigravity", "linux", nil,
			func(h string) string { return filepath.Join(h, ".gemini", "config") },
			func(h string) string { return filepath.Join(h, ".gemini", "config", "mcp_config.json") }, ""},
		{"opencode", "linux", func(h string) map[string]string { return map[string]string{"XDG_CONFIG_HOME": filepath.Join(h, "xdg")} },
			func(h string) string { return filepath.Join(h, "xdg", "opencode") },
			func(h string) string { return filepath.Join(h, "xdg", "opencode", "opencode.json") }, "opencode"},
		{"cursor", "darwin", nil,
			func(h string) string { return filepath.Join(h, ".cursor") },
			func(h string) string { return filepath.Join(h, ".cursor", "mcp.json") }, ""},
		{"zed", "linux", nil,
			func(h string) string { return filepath.Join(h, ".config", "zed") },
			func(h string) string { return filepath.Join(h, ".config", "zed", "settings.json") }, ""},
		{"zed", "windows", func(h string) map[string]string { return map[string]string{"APPDATA": filepath.Join(h, "roaming")} },
			func(h string) string { return filepath.Join(h, "roaming", "Zed") },
			func(h string) string { return filepath.Join(h, "roaming", "Zed", "settings.json") }, ""},
		{"cline", "linux", nil,
			func(h string) string {
				return filepath.Join(h, ".config", "Code", "User", "globalStorage", "saoudrizwan.claude-dev")
			},
			func(h string) string {
				return filepath.Join(h, ".config", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json")
			}, ""},
		{"cline", "darwin", nil,
			func(h string) string { return filepath.Join(h, ".cline") },
			func(h string) string {
				return filepath.Join(h, ".cline", "data", "settings", "cline_mcp_settings.json")
			}, ""},
	}
	for _, c := range cases {
		t.Run(c.target+"/"+c.goos, func(t *testing.T) {
			home := t.TempDir()
			var vars map[string]string
			if c.vars != nil {
				vars = c.vars(home)
			}
			env := testEnv(c.goos, home, vars)
			env.LookPath = func(string) (string, error) { return "", errors.New("not found") }
			target := mustFind(t, c.target)
			if c.command != "" {
				// The command alone is what detects the client.
				var stdout, stderr bytes.Buffer
				if got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr); got != ResultSkip {
					t.Fatalf("install without %s on PATH = %q, want skip", c.command, got)
				}
				env.LookPath = func(file string) (string, error) {
					if file == c.command {
						return "/usr/bin/" + file, nil
					}
					return "", errors.New("not found")
				}
			}
			key := target.Shape.Key
			configPath := c.config(home)

			var stdout, stderr bytes.Buffer
			if c.command == "" {
				if got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr); got != ResultSkip {
					t.Fatalf("install without the client = %q, want skip", got)
				}
			}
			if err := os.MkdirAll(c.presence(home), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, configPath, `{"`+key+`":{"other":{"command":"x"}},"theme":"dark"}`, 0o644)

			for i, want := range []string{"configured.", "updated."} {
				stdout.Reset()
				if got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr); got != ResultOK || !strings.Contains(stdout.String(), want) {
					t.Fatalf("install #%d = %q, stdout %q, stderr %q; want %q", i+1, got, stdout.String(), stderr.String(), want)
				}
			}
			entry := readJSON(t, configPath)[key].(map[string]any)["sync82"].(map[string]any)
			if fmt.Sprint(entry["command"]) != "/opt/sync82" && fmt.Sprint(entry["command"]) != "[/opt/sync82]" {
				t.Fatalf("entry = %v", entry)
			}

			stdout.Reset()
			if got := UninstallTarget(context.Background(), target, env, &stdout, &stderr); got != ResultOK || !strings.Contains(stdout.String(), "removed.") {
				t.Fatalf("uninstall = %q, stdout %q, stderr %q; want removed", got, stdout.String(), stderr.String())
			}
			cfg := readJSON(t, configPath)
			servers := cfg[key].(map[string]any)
			if _, ok := servers["sync82"]; ok {
				t.Error("sync82 entry still present after uninstall")
			}
			if _, ok := servers["other"]; !ok || cfg["theme"] != "dark" {
				t.Errorf("uninstall dropped other keys: %v", cfg)
			}

			stdout.Reset()
			if got := UninstallTarget(context.Background(), target, env, &stdout, &stderr); got != ResultSkip || !strings.Contains(stdout.String(), "not configured, skipping") {
				t.Fatalf("second uninstall = %q, stdout %q; want not configured", got, stdout.String())
			}
		})
	}
}

// TestAntigravity_InstallsOnlyGlobalAndCleansLegacy checks that install
// writes only ~/.gemini/config/mcp_config.json, even when legacy files
// exist, and that uninstall cleans the global file and both legacy files.
func TestAntigravity_InstallsOnlyGlobalAndCleansLegacy(t *testing.T) {
	home := t.TempDir()
	env := testEnv("linux", home, nil)
	env.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	target := mustFind(t, "antigravity")
	global := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	legacy := filepath.Join(home, ".gemini", "antigravity", "mcp_config.json")
	legacyIDE := filepath.Join(home, ".gemini", "antigravity-ide", "mcp_config.json")
	writeFile(t, legacy, `{"mcpServers":{"other":{"command":"x"}}}`, 0o644)

	var stdout, stderr bytes.Buffer
	if got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr); got != ResultOK {
		t.Fatalf("install = %q; stdout %q, stderr %q", got, stdout.String(), stderr.String())
	}
	if _, ok := readJSON(t, global)["mcpServers"].(map[string]any)["sync82"]; !ok {
		t.Errorf("%s lacks the sync82 entry", global)
	}
	if _, ok := readJSON(t, legacy)["mcpServers"].(map[string]any)["sync82"]; ok {
		t.Errorf("%s was written although only the global file is installed", legacy)
	}
	if _, err := os.Stat(legacyIDE); !os.IsNotExist(err) {
		t.Errorf("%s was created", legacyIDE)
	}

	writeFile(t, legacy, `{"mcpServers":{"sync82":{"command":"/old"},"other":{"command":"x"}}}`, 0o644)
	writeFile(t, legacyIDE, `{"mcpServers":{"sync82":{"command":"/old"}}}`, 0o644)
	if got := UninstallTarget(context.Background(), target, env, &stdout, &stderr); got != ResultOK {
		t.Fatalf("uninstall = %q; stderr %q", got, stderr.String())
	}
	for _, p := range []string{global, legacy, legacyIDE} {
		if _, ok := readJSON(t, p)["mcpServers"].(map[string]any)["sync82"]; ok {
			t.Errorf("%s still has the sync82 entry", p)
		}
	}
	if _, ok := readJSON(t, legacy)["mcpServers"].(map[string]any)["other"]; !ok {
		t.Error("uninstall dropped another server from the legacy file")
	}
}

// TestAntigravity_Detection checks that antigravity is detected through
// the agy command, ~/.gemini/config or ~/.gemini/antigravity, and not
// through a bare ~/.gemini directory.
func TestAntigravity_Detection(t *testing.T) {
	target := mustFind(t, "antigravity")
	notFound := func(string) (string, error) { return "", errors.New("not found") }

	env := testEnv("linux", t.TempDir(), nil)
	env.LookPath = func(file string) (string, error) {
		if file == "agy" {
			return "/usr/bin/agy", nil
		}
		return "", errors.New("not found")
	}
	if !target.Detected(env) {
		t.Error("antigravity should be detected when agy is on PATH")
	}

	for _, sub := range []string{"", "config", "antigravity"} {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, ".gemini", sub), 0o755); err != nil {
			t.Fatal(err)
		}
		env := testEnv("linux", home, nil)
		env.LookPath = notFound
		if got, want := target.Detected(env), sub != ""; got != want {
			t.Errorf("~/.gemini/%s: Detected = %v, want %v", sub, got, want)
		}
	}
}

// opencodeEnv returns a linux Env with home as the home directory and the
// opencode command on PATH.
func opencodeEnv(home string) Env {
	env := testEnv("linux", home, nil)
	env.LookPath = func(file string) (string, error) {
		if file == "opencode" {
			return "/usr/bin/opencode", nil
		}
		return "", errors.New("not found")
	}
	return env
}

// TestOpenCode_InstallCreatesConfig checks that install creates
// opencode.json with the local entry, then reports "updated.".
func TestOpenCode_InstallCreatesConfig(t *testing.T) {
	home := t.TempDir()
	env := opencodeEnv(home)
	target := mustFind(t, "opencode")
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	for i, want := range []string{"opencode — configured.", "opencode — updated."} {
		got, out, errOut := install(t, target, env)
		if got != ResultOK || !strings.Contains(out, want) {
			t.Fatalf("install #%d = %q, stdout %q, stderr %q; want %q", i+1, got, out, errOut, want)
		}
	}
	entry := readJSON(t, path)["mcp"].(map[string]any)["sync82"]
	want := map[string]any{"type": "local", "command": []any{"/opt/sync82"}, "enabled": true}
	if !reflect.DeepEqual(entry, want) {
		t.Errorf("entry = %v, want %v", entry, want)
	}
}

// TestOpenCode_InstallUsesExistingJSONC checks that an existing
// opencode.jsonc without opencode.json receives the entry and no second
// global file is created, and that a .jsonc with comments is left
// unchanged with the entry printed.
func TestOpenCode_InstallUsesExistingJSONC(t *testing.T) {
	home := t.TempDir()
	env := opencodeEnv(home)
	target := mustFind(t, "opencode")
	dir := filepath.Join(home, ".config", "opencode")
	jsoncPath := filepath.Join(dir, "opencode.jsonc")
	writeFile(t, jsoncPath, `{"$schema":"https://opencode.ai/config.json"}`, 0o644)

	if got, out, errOut := install(t, target, env); got != ResultOK || !strings.Contains(out, "configured.") {
		t.Fatalf("install = %q, stdout %q, stderr %q", got, out, errOut)
	}
	if _, ok := readJSON(t, jsoncPath)["mcp"].(map[string]any)["sync82"]; !ok {
		t.Error("opencode.jsonc lacks the sync82 entry")
	}
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(err) {
		t.Error("opencode.json was created next to opencode.jsonc")
	}

	original := "{\n  // keep\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"
	writeFile(t, jsoncPath, original, 0o644)
	got, _, errOut := install(t, target, env)
	if got != ResultFail || !strings.Contains(errOut, `"mcp"`) || !strings.Contains(errOut, `"enabled": true`) {
		t.Fatalf("install = %q, stderr %q; want a failure with the entry to add", got, errOut)
	}
	if data, _ := os.ReadFile(jsoncPath); string(data) != original {
		t.Errorf("opencode.jsonc = %q, want it untouched", data)
	}
}

// TestOpenCode_UninstallCleansEveryFile checks that uninstall removes the
// entry from opencode.json, opencode.jsonc and the legacy config.json.
func TestOpenCode_UninstallCleansEveryFile(t *testing.T) {
	home := t.TempDir()
	env := opencodeEnv(home)
	dir := filepath.Join(home, ".config", "opencode")
	files := []string{"opencode.json", "opencode.jsonc", "config.json"}
	for _, name := range files {
		writeFile(t, filepath.Join(dir, name), `{"mcp":{"sync82":{"type":"local"},"other":{"type":"local"}}}`, 0o644)
	}
	if got, out, errOut := uninstall(t, mustFind(t, "opencode"), env); got != ResultOK || !strings.Contains(out, "opencode — removed.") {
		t.Fatalf("uninstall = %q, stdout %q, stderr %q", got, out, errOut)
	}
	for _, name := range files {
		mcp := readJSON(t, filepath.Join(dir, name))["mcp"].(map[string]any)
		if _, ok := mcp["sync82"]; ok {
			t.Errorf("%s still has the sync82 entry", name)
		}
		if _, ok := mcp["other"]; !ok {
			t.Errorf("%s lost another server", name)
		}
	}
}

// TestDetectedIn checks that only detected targets are returned, in order.
func TestDetectedIn(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := opencodeEnv(home)
	if got := TargetNamesIn(DetectedIn(Targets, env)); !slices.Equal(got, []string{"opencode", "cursor"}) {
		t.Errorf("DetectedIn = %v, want [opencode cursor]", got)
	}
}

// TestCline_HonorsMCPSettingsPathOverride checks that, with
// CLINE_MCP_SETTINGS_PATH set to an absolute path, install writes the Cline
// CLI's entry to that file instead of the one in the data directory, that
// the override alone makes the target detected, and that uninstall removes
// the entry from it.
func TestCline_HonorsMCPSettingsPathOverride(t *testing.T) {
	home := t.TempDir()
	override := filepath.Join(home, "custom", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(override), 0o755); err != nil {
		t.Fatal(err)
	}
	env := testEnv("linux", home, map[string]string{"CLINE_MCP_SETTINGS_PATH": override})
	target := mustFind(t, "cline")

	var stdout, stderr bytes.Buffer
	if got := InstallTarget(context.Background(), target, env, "/opt/sync82", &stdout, &stderr); got != ResultOK {
		t.Fatalf("install = %q; stdout %q, stderr %q", got, stdout.String(), stderr.String())
	}
	if _, ok := readJSON(t, override)["mcpServers"].(map[string]any)["sync82"]; !ok {
		t.Fatalf("%s lacks the sync82 entry", override)
	}
	if _, err := os.Stat(filepath.Join(home, ".cline", "data", "settings", "cline_mcp_settings.json")); !os.IsNotExist(err) {
		t.Errorf("default CLI settings file written despite the override (stat err = %v)", err)
	}

	stdout.Reset()
	stderr.Reset()
	if got := UninstallTarget(context.Background(), target, env, &stdout, &stderr); got != ResultOK {
		t.Fatalf("uninstall = %q; stdout %q, stderr %q", got, stdout.String(), stderr.String())
	}
	if _, ok := readJSON(t, override)["mcpServers"].(map[string]any)["sync82"]; ok {
		t.Errorf("%s still has the sync82 entry after uninstall", override)
	}
}

// TestCline_InstallsIntoEveryPresentLocation checks that install writes
// both the VS Code extension's and the Cline CLI's settings when both are
// installed, honoring CLINE_DATA_DIR.
func TestCline_InstallsIntoEveryPresentLocation(t *testing.T) {
	home := t.TempDir()
	dataDir := filepath.Join(home, "cline-data")
	env := testEnv("linux", home, map[string]string{"CLINE_DATA_DIR": dataDir})
	ext := filepath.Join(home, ".config", "Code", "User", "globalStorage", "saoudrizwan.claude-dev")
	for _, d := range []string{ext, dataDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if got := InstallTarget(context.Background(), mustFind(t, "cline"), env, "/opt/sync82", &stdout, &stderr); got != ResultOK {
		t.Fatalf("install = %q; stderr %q", got, stderr.String())
	}
	for _, p := range []string{
		filepath.Join(ext, "settings", "cline_mcp_settings.json"),
		filepath.Join(dataDir, "settings", "cline_mcp_settings.json"),
	} {
		if _, ok := readJSON(t, p)["mcpServers"].(map[string]any)["sync82"]; !ok {
			t.Errorf("%s lacks the sync82 entry", p)
		}
	}
}
