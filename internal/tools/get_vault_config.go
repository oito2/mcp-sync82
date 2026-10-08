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

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// GetVaultConfigTool implements get_vault_config: report the current
// effective configuration — active vault path, global config, and (if
// workspace_root is given) the local config plus that project's
// subprojects.
type GetVaultConfigTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// getVaultConfigArgs holds the decoded arguments of the get_vault_config
// tool; its JSON tags match the property names declared in InputSchema.
type getVaultConfigArgs struct {
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
	Path             string `json:"path,omitempty"`
}

// Name returns the MCP tool name, "get_vault_config".
func (t *GetVaultConfigTool) Name() string { return "get_vault_config" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *GetVaultConfigTool) Description() string {
	return "Report the current effective vault configuration: active vault path, global config, and (if workspace_root is given) the local .sync82.json config for that workspace."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional properties workspace_root, search_parent_dirs and path.
func (t *GetVaultConfigTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_root": map[string]any{
				"type":        "string",
				"description": "Path to your project folder. If provided, the report also includes the local .sync82.json config for that workspace, if one exists.",
			},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
	}
}

// Validate decodes raw into getVaultConfigArgs. It has no further rules and
// returns an error only when decoding fails.
func (t *GetVaultConfigTool) Validate(raw json.RawMessage) (any, error) {
	var args getVaultConfigArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	return args, nil
}

// Execute returns a JSON report with the active vault path, the global
// config's vault and last-used project with its vault and, when
// workspace_root is given, the local .sync82.json together with the vault
// it resolves to and the subprojects of its project.
// Failure to read a config or the vault is returned as an error.
func (t *GetVaultConfigTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(getVaultConfigArgs)

	globalCfg, err := config.ReadGlobalConfig()
	if err != nil {
		return ToolResult{}, err
	}
	dbPath := t.Resolver.DBPathOrDefault(args.Path)

	report := map[string]any{
		"active_vault":            dbPath,
		"global_vault_configured": globalCfg.VaultPath != "",
		"global_vault_path":       nilIfEmpty(globalCfg.VaultPath),
		"last_project":            nilIfEmpty(globalCfg.LastProject),
		"last_subproject":         nilIfEmpty(globalCfg.LastSubproject),
		"last_vault_path":         nilIfEmpty(globalCfg.LastVaultPath),
	}

	if args.WorkspaceRoot != "" {
		local, err := config.ReadLocalConfig(args.WorkspaceRoot, args.SearchParentDirs)
		if err != nil {
			return ToolResult{}, err
		}
		if local == nil {
			report["local_config"] = nil
		} else {
			// The subprojects reported below come from the vault this
			// workspace resolves to: an explicit "path" argument wins,
			// otherwise the path recorded in .sync82.json applies, relative
			// to the directory holding that file, as in Resolver.Resolve's
			// second tier.
			localDBPath := t.Resolver.vaultPath(args.Path, local.Config.Path, local.ConfigRoot)
			localReport, err := t.localConfigReport(ctx, localDBPath, local)
			if err != nil {
				return ToolResult{}, err
			}
			report["local_config"] = localReport
		}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return ToolResult{}, fmt.Errorf("encode vault config report: %w", err)
	}
	return ToolResult{Text: string(data)}, nil
}

// localConfigReport builds the "local_config" section of the report for
// the .sync82.json found in local: its location, project, subproject and
// path, the vault it resolves to, dbPath, and the names of the project's
// subprojects read from that vault. A vault or project that doesn't exist yields an empty
// subproject list. Store failures are returned as errors.
func (t *GetVaultConfigTool) localConfigReport(ctx context.Context, dbPath string, local *config.LocalConfigResult) (map[string]any, error) {
	subNames := []string{}
	// A vault that doesn't exist yet has no subprojects to list, and is
	// not created just to report that.
	s, err := t.Stores.GetExisting(ctx, dbPath)
	if err != nil && !errors.Is(err, store.ErrVaultNotFound) {
		return nil, err
	}

	var parent *store.Project
	if s != nil {
		if parent, err = s.FindProjectByName(ctx, local.Config.Project, nil); err != nil {
			return nil, err
		}
	}
	if parent != nil {
		subs, err := s.ListSubprojects(ctx, parent.ID)
		if err != nil {
			return nil, err
		}
		for _, sub := range subs {
			subNames = append(subNames, sub.Name)
		}
	}

	return map[string]any{
		"config_root": local.ConfigRoot,
		"project":     local.Config.Project,
		"subproject":  nilIfEmpty(local.Config.Subproject),
		"path":        nilIfEmpty(local.Config.Path),
		"vault":       dbPath,
		"subprojects": subNames,
	}, nil
}

// nilIfEmpty returns s, or nil when s is empty, so that an unset value is
// encoded as JSON null.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
