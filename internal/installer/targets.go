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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// serverName is the key sync82 is registered under in every client.
const serverName = "sync82"

// Kind distinguishes the two installer strategies: a "cli" target is
// installed by invoking the client's own MCP subcommands; a "file" target
// is installed by read-merge-write of a JSON config file.
type Kind string

// Kind values: KindCLI for targets managed through the client's own
// subcommands, KindFile for targets managed by editing JSON files.
const (
	KindCLI  Kind = "cli"
	KindFile Kind = "file"
)

// Env is the host environment the targets resolve their paths and
// detection against: the operating system, the home directory, an
// environment variable lookup, and a PATH lookup.
type Env struct {
	GOOS     string
	HomeDir  string
	Getenv   func(key string) string
	LookPath func(file string) (string, error)
}

// HostEnv returns the Env of the running process with homeDir as its home
// directory.
func HostEnv(homeDir string) Env {
	return Env{GOOS: runtime.GOOS, HomeDir: homeDir, Getenv: os.Getenv, LookPath: exec.LookPath}
}

// getenv returns the environment variable key, or "" when env has no
// lookup function.
func (env Env) getenv(key string) string {
	if env.Getenv == nil {
		return ""
	}
	return env.Getenv(key)
}

// hasCommand reports whether command resolves on PATH.
func (env Env) hasCommand(command string) bool {
	lookPath := env.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	_, err := lookPath(command)
	return err == nil
}

// Shape describes where a client's JSON config keeps its MCP servers: Key
// is the top-level object holding one entry per server, and Entry builds
// sync82's entry for binaryPath. Entry is nil for a shape that is only
// ever read or cleaned, never written.
type Shape struct {
	Key   string
	Entry func(binaryPath string) map[string]any
}

var (
	// shapeMCPServers is {"mcpServers": {"sync82": {command, args}}}.
	shapeMCPServers = Shape{Key: "mcpServers", Entry: func(bin string) map[string]any {
		return map[string]any{"command": bin, "args": []string{}}
	}}
	// shapeCursor is {"mcpServers": {"sync82": {type: "stdio", command, args}}}.
	shapeCursor = Shape{Key: "mcpServers", Entry: func(bin string) map[string]any {
		return map[string]any{"type": "stdio", "command": bin, "args": []string{}}
	}}
	// shapeZed is {"context_servers": {"sync82": {command, args}}}.
	shapeZed = Shape{Key: "context_servers", Entry: func(bin string) map[string]any {
		return map[string]any{"command": bin, "args": []string{}}
	}}
	// shapeOpenCode is {"mcp": {"sync82": {type: "local", command: [bin], enabled: true}}}.
	shapeOpenCode = Shape{Key: "mcp", Entry: func(bin string) map[string]any {
		return map[string]any{"type": "local", "command": []string{bin}, "enabled": true}
	}}
)

// Target describes one MCP client install target.
type Target struct {
	Name string
	Kind Kind

	// Unsupported, when set and returning a non-empty reason, marks the
	// target as unavailable on env.GOOS; it is then never detected, and
	// install and uninstall skip it.
	Unsupported func(env Env) string
	// A target is detected when DetectCmd is on PATH or any directory
	// returned by DetectDirs exists. Either may be empty.
	DetectCmd  string
	DetectDirs func(env Env) []string

	// CLI targets only. Command is the client's executable. Args returns
	// its MCP-add arguments for binaryPath. GetArgs exits with status 0
	// only while sync82 is registered. RemoveArgs returns the arguments
	// removing sync82 from one scope; a client without scopes receives ""
	// and ignores it. Scope, when set, marks a client keeping
	// registrations in several scopes and extracts from the GetArgs output
	// the scope of the registration it reports, or "" when none is named.
	Command    string
	Args       func(binaryPath string) []string
	GetArgs    []string
	RemoveArgs func(scope string) []string
	Scope      func(getOutput []byte) string

	// File targets only. Shape is the layout of the target's JSON config
	// files. Install merges sync82's entry into every ConfigPaths file.
	// Uninstall cleans every RemovePaths file (ConfigPaths when nil).
	Shape       Shape
	ConfigPaths func(env Env) []string
	RemovePaths func(env Env) []string
}

// configPaths returns the files install writes.
func (t Target) configPaths(env Env) []string {
	if t.ConfigPaths == nil {
		return nil
	}
	return t.ConfigPaths(env)
}

// removePaths returns the files uninstall cleans.
func (t Target) removePaths(env Env) []string {
	if t.RemovePaths != nil {
		return t.RemovePaths(env)
	}
	return t.configPaths(env)
}

// unsupported returns why the target cannot be used on env.GOOS, or "".
func (t Target) unsupported(env Env) string {
	if t.Unsupported == nil {
		return ""
	}
	return t.Unsupported(env)
}

// Detected reports whether the client is installed on env: the target is
// supported on env.GOOS and either DetectCmd resolves on PATH or one of
// the DetectDirs directories exists.
func (t Target) Detected(env Env) bool {
	if t.unsupported(env) != "" {
		return false
	}
	if t.DetectCmd != "" && env.hasCommand(t.DetectCmd) {
		return true
	}
	if t.DetectDirs != nil {
		for _, dir := range t.DetectDirs(env) {
			if dirExists(dir) {
				return true
			}
		}
	}
	return false
}

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists reports whether path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// fixedPaths returns a path function yielding the single path built by
// fn, or no path when fn returns "".
func fixedPaths(fn func(env Env) string) func(env Env) []string {
	return func(env Env) []string {
		if p := fn(env); p != "" {
			return []string{p}
		}
		return nil
	}
}

// appDataDir returns %APPDATA%, falling back to <home>/AppData/Roaming
// when the variable is unset.
func appDataDir(env Env) string {
	if v := env.getenv("APPDATA"); v != "" {
		return v
	}
	return filepath.Join(env.HomeDir, "AppData", "Roaming")
}

// xdgConfigHome returns $XDG_CONFIG_HOME when it is an absolute path,
// otherwise <home>/.config.
func xdgConfigHome(env Env) string {
	if v := env.getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(env.HomeDir, ".config")
}

// claudeDesktopDir returns Claude Desktop's configuration directory:
// ~/Library/Application Support/Claude on macOS, %APPDATA%\Claude on
// Windows and $XDG_CONFIG_HOME/Claude (default ~/.config/Claude) on
// Linux. It returns "" on other systems.
func claudeDesktopDir(env Env) string {
	switch env.GOOS {
	case "darwin":
		return filepath.Join(env.HomeDir, "Library", "Application Support", "Claude")
	case "windows":
		return filepath.Join(appDataDir(env), "Claude")
	case "linux":
		return filepath.Join(xdgConfigHome(env), "Claude")
	}
	return ""
}

// claudeDesktopMSIXDirs returns the Claude directories of the MSIX package
// of Claude Desktop on Windows, the form the claude.ai download and the
// Microsoft Store install: the app's AppData is virtualized under
// %LOCALAPPDATA%\Packages\Claude_<id>\LocalCache\Roaming\Claude, and that
// copy of claude_desktop_config.json is the one the app reads. It returns
// nil on other systems or when no such package exists.
func claudeDesktopMSIXDirs(env Env) []string {
	if env.GOOS != "windows" {
		return nil
	}
	local := env.getenv("LOCALAPPDATA")
	if local == "" {
		local = filepath.Join(env.HomeDir, "AppData", "Local")
	}
	packages := filepath.Join(local, "Packages")
	entries, err := os.ReadDir(packages)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "Claude_") {
			dirs = append(dirs, filepath.Join(packages, e.Name(), "LocalCache", "Roaming", "Claude"))
		}
	}
	return dirs
}

// claudeDesktopDirs returns every Claude Desktop configuration directory to
// look at: claudeDesktopDir, then those of claudeDesktopMSIXDirs.
func claudeDesktopDirs(env Env) []string {
	var dirs []string
	if dir := claudeDesktopDir(env); dir != "" {
		dirs = append(dirs, dir)
	}
	return append(dirs, claudeDesktopMSIXDirs(env)...)
}

// claudeDesktopConfigPaths returns the claude_desktop_config.json of every
// directory of claudeDesktopDirs that exists, so an MSIX install gets the
// file it reads; with none, the file of claudeDesktopDir, as for a classic
// install.
func claudeDesktopConfigPaths(env Env) []string {
	var paths []string
	for _, dir := range claudeDesktopDirs(env) {
		if dirExists(dir) {
			paths = append(paths, filepath.Join(dir, "claude_desktop_config.json"))
		}
	}
	if len(paths) == 0 {
		if p := claudeDesktopConfigPath(env); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// claudeDesktopConfigPath returns Claude Desktop's
// claude_desktop_config.json, or "" on systems without Claude Desktop.
func claudeDesktopConfigPath(env Env) string {
	dir := claudeDesktopDir(env)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "claude_desktop_config.json")
}

// antigravityConfigDir returns ~/.gemini/config, the directory of the
// mcp_config.json shared by the Antigravity IDE and CLI.
func antigravityConfigDir(env Env) string {
	return filepath.Join(env.HomeDir, ".gemini", "config")
}

// antigravityLegacyDir returns ~/.gemini/antigravity, the legacy
// Antigravity directory, which holds its own mcp_config.json.
func antigravityLegacyDir(env Env) string {
	return filepath.Join(env.HomeDir, ".gemini", "antigravity")
}

// antigravityGlobalConfigPath returns ~/.gemini/config/mcp_config.json.
func antigravityGlobalConfigPath(env Env) string {
	return filepath.Join(antigravityConfigDir(env), "mcp_config.json")
}

// opencodeDir returns $XDG_CONFIG_HOME/opencode (default
// ~/.config/opencode).
func opencodeDir(env Env) string {
	return filepath.Join(xdgConfigHome(env), "opencode")
}

// opencodeInstallPath returns the OpenCode config file install writes:
// opencode.jsonc when only that file exists, otherwise opencode.json.
func opencodeInstallPath(env Env) string {
	dir := opencodeDir(env)
	jsonPath, jsoncPath := filepath.Join(dir, "opencode.json"), filepath.Join(dir, "opencode.jsonc")
	if fileExists(jsoncPath) && !fileExists(jsonPath) {
		return jsoncPath
	}
	return jsonPath
}

// opencodeConfigPaths returns every global OpenCode config file:
// opencode.json, opencode.jsonc and the legacy config.json.
func opencodeConfigPaths(env Env) []string {
	dir := opencodeDir(env)
	return []string{
		filepath.Join(dir, "opencode.json"),
		filepath.Join(dir, "opencode.jsonc"),
		filepath.Join(dir, "config.json"),
	}
}

// cursorDir returns ~/.cursor.
func cursorDir(env Env) string {
	return filepath.Join(env.HomeDir, ".cursor")
}

// zedDir returns Zed's user configuration directory: %APPDATA%\Zed on
// Windows, ~/.config/zed on macOS, and $XDG_CONFIG_HOME/zed (default
// ~/.config/zed) elsewhere.
func zedDir(env Env) string {
	switch env.GOOS {
	case "windows":
		return filepath.Join(appDataDir(env), "Zed")
	case "darwin":
		return filepath.Join(env.HomeDir, ".config", "zed")
	}
	return filepath.Join(xdgConfigHome(env), "zed")
}

// clineExtensionDir returns the globalStorage directory of the Cline VS
// Code extension (saoudrizwan.claude-dev) for stable VS Code.
func clineExtensionDir(env Env) string {
	var userDir string
	switch env.GOOS {
	case "darwin":
		userDir = filepath.Join(env.HomeDir, "Library", "Application Support", "Code", "User")
	case "windows":
		userDir = filepath.Join(appDataDir(env), "Code", "User")
	default:
		userDir = filepath.Join(xdgConfigHome(env), "Code", "User")
	}
	return filepath.Join(userDir, "globalStorage", "saoudrizwan.claude-dev")
}

// clineCLIDirs returns the directory marking a Cline CLI install and the
// directory holding its data: ~/.cline and ~/.cline/data — with
// $CLINE_DIR, when it is an absolute path, in place of ~/.cline — or both
// set to $CLINE_DATA_DIR when it is set.
func clineCLIDirs(env Env) (marker, data string) {
	if v := env.getenv("CLINE_DATA_DIR"); v != "" {
		return v, v
	}
	marker = filepath.Join(env.HomeDir, ".cline")
	if v := env.getenv("CLINE_DIR"); filepath.IsAbs(v) {
		marker = v
	}
	return marker, filepath.Join(marker, "data")
}

// clineSettingsPath returns the cline_mcp_settings.json inside a Cline
// data directory.
func clineSettingsPath(dataDir string) string {
	return filepath.Join(dataDir, "settings", "cline_mcp_settings.json")
}

// clineSettingsOverride returns $CLINE_MCP_SETTINGS_PATH, the file the
// Cline CLI reads its MCP servers from instead of the one in its data
// directory, or "" when it is unset or not an absolute path.
func clineSettingsOverride(env Env) string {
	if v := env.getenv("CLINE_MCP_SETTINGS_PATH"); filepath.IsAbs(v) {
		return v
	}
	return ""
}

// clineCLISettingsPath returns the Cline CLI's cline_mcp_settings.json:
// clineSettingsOverride when set, otherwise the one in the data directory
// from clineCLIDirs.
func clineCLISettingsPath(env Env) string {
	if override := clineSettingsOverride(env); override != "" {
		return override
	}
	_, data := clineCLIDirs(env)
	return clineSettingsPath(data)
}

// Targets is the fixed set of install targets, in the order "install" and
// "uninstall" process them.
//
// claude and codex are registered through their own CLIs, with the
// command after "--", after removing any existing registration: claude
// per scope (local and user; a project registration is reported and left
// unchanged), codex through its single configuration. Every other target
// is a JSON config file edited in place.
var Targets = []Target{
	{
		Kind:      KindCLI,
		Name:      "claude",
		DetectCmd: "claude",
		Command:   "claude",
		Args:      func(bin string) []string { return []string{"mcp", "add", "--scope", "user", serverName, "--", bin} },
		GetArgs:   []string{"mcp", "get", serverName},
		RemoveArgs: func(scope string) []string {
			return []string{"mcp", "remove", "--scope", scope, serverName}
		},
		Scope: claudeScope,
	},
	{
		Kind:  KindFile,
		Name:  "claude-desktop",
		Shape: shapeMCPServers,
		Unsupported: func(env Env) string {
			if claudeDesktopDir(env) == "" {
				return "Claude Desktop is only available for macOS, Windows and Linux"
			}
			return ""
		},
		DetectDirs:  claudeDesktopDirs,
		ConfigPaths: claudeDesktopConfigPaths,
		RemovePaths: func(env Env) []string {
			var paths []string
			for _, dir := range claudeDesktopDirs(env) {
				paths = append(paths, filepath.Join(dir, "claude_desktop_config.json"))
			}
			return paths
		},
	},
	{
		Kind:      KindFile,
		Name:      "antigravity",
		Shape:     shapeMCPServers,
		DetectCmd: "agy",
		DetectDirs: func(env Env) []string {
			return []string{antigravityConfigDir(env), antigravityLegacyDir(env)}
		},
		ConfigPaths: fixedPaths(antigravityGlobalConfigPath),
		RemovePaths: func(env Env) []string {
			return []string{
				antigravityGlobalConfigPath(env),
				filepath.Join(antigravityLegacyDir(env), "mcp_config.json"),
				filepath.Join(env.HomeDir, ".gemini", "antigravity-ide", "mcp_config.json"),
			}
		},
	},
	{
		Kind:       KindCLI,
		Name:       "codex",
		DetectCmd:  "codex",
		Command:    "codex",
		Args:       func(bin string) []string { return []string{"mcp", "add", serverName, "--", bin} },
		GetArgs:    []string{"mcp", "get", serverName},
		RemoveArgs: func(string) []string { return []string{"mcp", "remove", serverName} },
	},
	{
		Kind:        KindFile,
		Name:        "opencode",
		Shape:       shapeOpenCode,
		DetectCmd:   "opencode",
		ConfigPaths: fixedPaths(opencodeInstallPath),
		RemovePaths: opencodeConfigPaths,
	},
	{
		Kind:        KindFile,
		Name:        "cursor",
		Shape:       shapeCursor,
		DetectDirs:  fixedPaths(cursorDir),
		ConfigPaths: fixedPaths(func(env Env) string { return filepath.Join(cursorDir(env), "mcp.json") }),
	},
	{
		Kind:        KindFile,
		Name:        "zed",
		Shape:       shapeZed,
		DetectDirs:  fixedPaths(zedDir),
		ConfigPaths: fixedPaths(func(env Env) string { return filepath.Join(zedDir(env), "settings.json") }),
	},
	{
		Kind:  KindFile,
		Name:  "cline",
		Shape: shapeMCPServers,
		DetectDirs: func(env Env) []string {
			marker, _ := clineCLIDirs(env)
			dirs := []string{clineExtensionDir(env), marker}
			if override := clineSettingsOverride(env); override != "" {
				dirs = append(dirs, filepath.Dir(override))
			}
			return dirs
		},
		ConfigPaths: func(env Env) []string {
			var paths []string
			if ext := clineExtensionDir(env); dirExists(ext) {
				paths = append(paths, clineSettingsPath(ext))
			}
			marker, _ := clineCLIDirs(env)
			if override := clineSettingsOverride(env); dirExists(marker) || (override != "" && dirExists(filepath.Dir(override))) {
				paths = append(paths, clineCLISettingsPath(env))
			}
			return paths
		},
		RemovePaths: func(env Env) []string {
			_, data := clineCLIDirs(env)
			paths := []string{clineSettingsPath(clineExtensionDir(env)), clineSettingsPath(data)}
			if override := clineSettingsOverride(env); override != "" && override != paths[1] {
				paths = append(paths, override)
			}
			return paths
		},
	},
}

// TargetNames returns the names of every install target, in Targets order.
func TargetNames() []string {
	return TargetNamesIn(Targets)
}

// TargetNamesIn returns the names of every target in targets, in order.
func TargetNamesIn(targets []Target) []string {
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.Name
	}
	return names
}

// Find returns the target named name in Targets, if any.
func Find(name string) (Target, bool) {
	return FindIn(Targets, name)
}

// FindIn returns the target named name in targets, if any.
func FindIn(targets []Target, name string) (Target, bool) {
	for _, t := range targets {
		if t.Name == name {
			return t, true
		}
	}
	return Target{}, false
}

// DetectedIn returns the targets of targets detected on env, in order.
func DetectedIn(targets []Target, env Env) []Target {
	var detected []Target
	for _, t := range targets {
		if t.Detected(env) {
			detected = append(detected, t)
		}
	}
	return detected
}

// UninstallCandidatesIn returns the targets of targets an uninstall with no
// target acts on, in targets order: every detected one, plus every file
// target that isn't detected any more but whose config files still hold a
// sync82 entry (its client was removed, or moved off PATH), so the entry
// isn't left behind.
func UninstallCandidatesIn(targets []Target, env Env) []Target {
	var out []Target
	for _, t := range targets {
		if t.Detected(env) || (t.Kind == KindFile && t.unsupported(env) == "" && t.holdsEntry(env)) {
			out = append(out, t)
		}
	}
	return out
}

// holdsEntry reports whether one of the file target t's remove paths holds
// a sync82 entry. A file that can't be read counts as holding one, so the
// uninstall reports it.
func (t Target) holdsEntry(env Env) bool {
	for _, path := range t.removePaths(env) {
		if has, err := fileHasEntry(path, t.Shape); has || err != nil {
			return true
		}
	}
	return false
}
