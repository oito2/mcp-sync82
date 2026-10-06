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

package main

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/server"
	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// bundleName is the file name of the MCPB bundle and description is the
// one-line summary used in the bundle manifest and in server.json.
const (
	bundleName  = binary + ".mcpb"
	description = "Persistent, structured project memory for AI coding agents, stored locally in SQLite."
)

// bundleBinaries holds the paths of the binaries shipped in the bundle.
type bundleBinaries struct {
	Darwin     string // universal (amd64 + arm64) Mach-O
	LinuxAMD64 string // linux/amd64
	LinuxARM64 string // linux/arm64
	Windows    string // windows/amd64
}

// Paths inside the bundle, relative to its root. MCPB selects a command per
// operating system but not per CPU architecture, so on Linux the command is
// a launcher script that execs the binary matching `uname -m`; macOS gets a
// universal binary instead.
const (
	bundleDarwinBin     = "server/sync82-darwin"
	bundleLinuxLauncher = "server/sync82-linux"
	bundleLinuxAMD64Bin = "server/sync82-linux-amd64"
	bundleLinuxARM64Bin = "server/sync82-linux-arm64"
	bundleWindowsBin    = "server/sync82-windows.exe"
)

// linuxLauncher is the POSIX shell script stored at bundleLinuxLauncher.
//
//go:embed sync82-linux.sh
var linuxLauncher []byte

// dbPathDefault is the db_path user_config default. MCPB leaves an unset
// optional variable as a literal "${user_config.db_path}", so the option
// always carries a value; this one is the vault path sync82 uses when
// SYNC82_DB_PATH is unset, and sync82 expands the leading "~".
const dbPathDefault = "~/.sync82/knowledge.db"

// bundleIcon is one icon file copied into the bundle: src is relative to
// the repository root, dst to the bundle root.
type bundleIcon struct {
	src, dst, size string
}

// bundleIcons lists the icons shipped in the bundle. The cropped variant
// (no background tile, mark filling the canvas) is used at 16/32 px; the
// tiled icon is used from 64 px up.
var bundleIcons = []bundleIcon{
	{"docs/img/icons/icon-sync82-cropped-16.png", "icons/icon-16.png", "16x16"},
	{"docs/img/icons/icon-sync82-cropped-32.png", "icons/icon-32.png", "32x32"},
	{"docs/img/icons/icon-sync82-64.png", "icons/icon-64.png", "64x64"},
	{"docs/img/icons/icon-sync82-128.png", "icons/icon-128.png", "128x128"},
	{"docs/img/icons/icon-sync82-256.png", "icons/icon-256.png", "256x256"},
	{"docs/img/icons/icon-sync82-512.png", "icon.png", "512x512"},
}

// mcpbManifest mirrors the subset of the MCPB manifest (manifest_version
// 0.3) sync82 uses.
type mcpbManifest struct {
	ManifestVersion string             `json:"manifest_version"`
	Name            string             `json:"name"`
	DisplayName     string             `json:"display_name"`
	Version         string             `json:"version"`
	Description     string             `json:"description"`
	LongDescription string             `json:"long_description"`
	Author          mcpbAuthor         `json:"author"`
	Repository      mcpbRepository     `json:"repository"`
	Homepage        string             `json:"homepage"`
	Documentation   string             `json:"documentation"`
	Support         string             `json:"support"`
	Icon            string             `json:"icon"`
	Icons           []mcpbIcon         `json:"icons"`
	Keywords        []string           `json:"keywords"`
	License         string             `json:"license"`
	Server          mcpbServer         `json:"server"`
	Tools           []mcpbTool         `json:"tools"`
	ToolsGenerated  bool               `json:"tools_generated"`
	Compatibility   mcpbCompatibility  `json:"compatibility"`
	UserConfig      map[string]mcpbOpt `json:"user_config"`
}

// mcpbAuthor is the manifest's author entry.
type mcpbAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// mcpbRepository is the manifest's source repository entry.
type mcpbRepository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// mcpbIcon is one manifest icon: a bundle-relative path and its
// "WxH" size.
type mcpbIcon struct {
	Src  string `json:"src"`
	Size string `json:"size"`
}

// mcpbServer describes how the host launches the bundled server.
type mcpbServer struct {
	Type       string        `json:"type"`
	EntryPoint string        `json:"entry_point"`
	MCPConfig  mcpbMCPConfig `json:"mcp_config"`
}

// mcpbMCPConfig is the launch command of the server, its arguments and
// environment, with per-platform command overrides.
type mcpbMCPConfig struct {
	Command           string                    `json:"command"`
	Args              []string                  `json:"args"`
	Env               map[string]string         `json:"env"`
	PlatformOverrides map[string]mcpbPlatformOv `json:"platform_overrides"`
}

// mcpbPlatformOv replaces the launch command on one platform.
type mcpbPlatformOv struct {
	Command string `json:"command"`
}

// mcpbTool is a tool name and short description listed in the manifest.
type mcpbTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// mcpbCompatibility lists the platforms the bundle supports.
type mcpbCompatibility struct {
	Platforms []string `json:"platforms"`
}

// mcpbOpt is one user-configurable option the host asks the user for.
type mcpbOpt struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// buildManifest assembles the MCPB manifest for version (a "vX.Y.Z" tag)
// with the given tool list. The manifest version omits the leading "v".
func buildManifest(version string, tools []mcpbTool) mcpbManifest {
	repoURL := fmt.Sprintf("https://github.com/%s/%s", repoOrg, repoName)
	icons := make([]mcpbIcon, 0, len(bundleIcons))
	for _, ic := range bundleIcons {
		icons = append(icons, mcpbIcon{Src: ic.dst, Size: ic.size})
	}
	return mcpbManifest{
		ManifestVersion: "0.3",
		Name:            binary,
		DisplayName:     binary,
		Version:         strings.TrimPrefix(version, "v"),
		Description:     description,
		LongDescription: "sync82 gives an AI coding agent persistent, structured memory of a software " +
			"project across sessions: goals, architecture, tech stack, decisions and progress, stored " +
			"in a local SQLite vault and exposed as MCP tools so the agent can recall and update them " +
			"the next time it opens the project.",
		Author:        mcpbAuthor{Name: authorName, URL: "https://github.com/" + repoOrg},
		Repository:    mcpbRepository{Type: "git", URL: repoURL + ".git"},
		Homepage:      repoURL,
		Documentation: repoURL + "/blob/main/docs/en/index.md",
		Support:       repoURL + "/issues",
		Icon:          "icon.png",
		Icons:         icons,
		Keywords:      []string{"memory", "context", "knowledge-base", "sqlite", "ai-agents"},
		License:       "GPL-3.0-or-later",
		Server: mcpbServer{
			Type:       "binary",
			EntryPoint: bundleLinuxLauncher,
			MCPConfig: mcpbMCPConfig{
				Command: "${__dirname}/" + bundleLinuxLauncher,
				Args:    []string{},
				Env:     map[string]string{"SYNC82_DB_PATH": "${user_config.db_path}"},
				PlatformOverrides: map[string]mcpbPlatformOv{
					"darwin": {Command: "${__dirname}/" + bundleDarwinBin},
					"win32":  {Command: "${__dirname}/" + bundleWindowsBin},
				},
			},
		},
		Tools:          tools,
		ToolsGenerated: false,
		Compatibility:  mcpbCompatibility{Platforms: []string{"darwin", "win32", "linux"}},
		UserConfig: map[string]mcpbOpt{
			"db_path": {
				Type:  "string",
				Title: "Vault database path",
				Description: "Path of the SQLite vault file that stores project memory. A leading ~ " +
					"is expanded to the home directory; the file is created if missing.",
				Required: false,
				Default:  dbPathDefault,
			},
		},
	}
}

// serverTools lists the tools the sync82 server registers, built for
// version, by connecting an in-memory client to a server made from
// tools.Registered, so the manifest always matches the shipped server.
// Listing tools opens no vault; the resolver's default path only points
// inside a temporary directory. It returns an error if the server or client
// cannot start or the listing fails, and gives up after 10 seconds.
func serverTools(version string) ([]mcpbTool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tmp, err := os.MkdirTemp("", "sync82-tools-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	logger := slog.New(slog.DiscardHandler)
	stores := store.NewManager()
	defer stores.Close()
	resolver := tools.NewResolver(filepath.Join(tmp, "knowledge.db"), logger)
	srv := server.New(binary, version, logger, tools.Registered(resolver, stores))

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("start in-memory server: %w", err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "release", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect in-memory client: %w", err)
	}
	defer session.Close()

	var list []mcpbTool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list tools: %w", err)
		}
		list = append(list, mcpbTool{Name: tool.Name, Description: firstSentence(tool.Description)})
	}
	return list, nil
}

// firstSentence returns s up to and including its first sentence end (a
// ". " followed by an upper-case letter), or all of s, with whitespace runs
// collapsed to single spaces, keeping manifest tool descriptions to one
// short line. An abbreviation such as "e.g. " followed by a lower-case word
// is not treated as a sentence end.
func firstSentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for i := 0; ; {
		j := strings.Index(s[i:], ". ")
		if j < 0 {
			return s
		}
		end := i + j + 1
		if r, _ := utf8.DecodeRuneInString(s[end+1:]); unicode.IsUpper(r) {
			return s[:end]
		}
		i = end
	}
}

// zipEntryTime is the fixed modification time stamped on every bundle
// entry, so building the same inputs twice produces a byte-identical
// archive (and thus the same fileSha256).
var zipEntryTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// writeMCPB writes the MCPB bundle (a ZIP archive with manifest.json at
// its root) to out. It contains the manifest, the Linux launcher script,
// the binaries named in bins and the icons found under repoRoot. The
// result is deterministic for identical inputs. It returns an error if any
// input file cannot be read or the archive cannot be written.
func writeMCPB(out, repoRoot string, manifest mcpbManifest, bins bundleBinaries) (err error) {
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(f)

	add := func(name string, mode os.FileMode, r io.Reader) error {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: zipEntryTime}
		hdr.SetMode(mode)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		return err
	}
	addFile := func(name, src string, mode os.FileMode) error {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		return add(name, mode, in)
	}

	if err := add("manifest.json", 0o644, bytes.NewReader(append(manifestJSON, '\n'))); err != nil {
		return err
	}
	if err := add(bundleLinuxLauncher, 0o755, bytes.NewReader(linuxLauncher)); err != nil {
		return fmt.Errorf("add %s: %w", bundleLinuxLauncher, err)
	}
	for _, b := range []struct{ dst, src string }{
		{bundleDarwinBin, bins.Darwin},
		{bundleLinuxAMD64Bin, bins.LinuxAMD64},
		{bundleLinuxARM64Bin, bins.LinuxARM64},
		{bundleWindowsBin, bins.Windows},
	} {
		if err := addFile(b.dst, b.src, 0o755); err != nil {
			return fmt.Errorf("add %s: %w", b.dst, err)
		}
	}
	for _, ic := range bundleIcons {
		if err := addFile(ic.dst, filepath.Join(repoRoot, filepath.FromSlash(ic.src)), 0o644); err != nil {
			return fmt.Errorf("add %s: %w", ic.dst, err)
		}
	}
	return zw.Close()
}
