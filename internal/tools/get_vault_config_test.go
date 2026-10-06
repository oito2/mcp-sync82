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
	"path/filepath"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestGetVaultConfigTool_NoWorkspaceRoot_OmitsLocalConfigKey verifies that,
// without a workspace root, the report shows the default vault as active, no
// global vault configured, and no "local_config" key at all.
func TestGetVaultConfigTool_NoWorkspaceRoot_OmitsLocalConfigKey(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &GetVaultConfigTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(result.Text), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, result.Text)
	}

	if report["active_vault"] != r.DefaultDBPath {
		t.Errorf("active_vault = %v, want %v", report["active_vault"], r.DefaultDBPath)
	}
	if report["global_vault_configured"] != false {
		t.Errorf("global_vault_configured = %v, want false", report["global_vault_configured"])
	}
	if _, present := report["local_config"]; present {
		t.Errorf("expected \"local_config\" key to be absent entirely when workspace_root isn't given, got %+v", report)
	}
}

// TestGetVaultConfigTool_WorkspaceRootWithoutLocalConfig_ReportsExplicitNull
// verifies that, when a workspace root without a .sync82.json is given, the
// report contains "local_config" as an explicit null.
func TestGetVaultConfigTool_WorkspaceRootWithoutLocalConfig_ReportsExplicitNull(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &GetVaultConfigTool{Resolver: r, Stores: mgr}

	workspace := t.TempDir()
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(result.Text), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, result.Text)
	}

	localConfig, present := report["local_config"]
	if !present {
		t.Fatal("expected \"local_config\" key to be present (as null) when workspace_root is given but no .sync82.json exists")
	}
	if localConfig != nil {
		t.Errorf("local_config = %v, want null", localConfig)
	}
}

// TestGetVaultConfigTool_WorkspaceRootWithLocalConfig_ReportsSubprojects
// verifies that "local_config" reports the project and subproject from
// .sync82.json together with the sorted list of the project's subprojects.
func TestGetVaultConfigTool_WorkspaceRootWithLocalConfig_ReportsSubprojects(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatal(err)
	}

	tool := &GetVaultConfigTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(result.Text), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, result.Text)
	}

	localConfig, ok := report["local_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected local_config to be an object, got %T: %+v", report["local_config"], report["local_config"])
	}
	if localConfig["project"] != "oito2" {
		t.Errorf("project = %v, want oito2", localConfig["project"])
	}
	if localConfig["subproject"] != "sync82" {
		t.Errorf("subproject = %v, want sync82", localConfig["subproject"])
	}
	subs, ok := localConfig["subprojects"].([]any)
	if !ok {
		t.Fatalf("expected subprojects to be an array, got %T", localConfig["subprojects"])
	}
	if len(subs) != 2 || subs[0] != "perci" || subs[1] != "sync82" {
		t.Fatalf("subprojects = %v, want [perci sync82]", subs)
	}
}

// TestGetVaultConfigTool_WorkspaceRootWithLocalConfigPath_ReportsSubprojectsFr
// omThatVault verifies that the subprojects in "local_config" are read from
// the vault named by the "path" field of .sync82.json, not from the default
// vault.
func TestGetVaultConfigTool_WorkspaceRootWithLocalConfigPath_ReportsSubprojectsFromThatVault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	customVault := t.TempDir() + "/custom.db"
	cs, err := mgr.Get(ctx, customVault)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cs.EnsureProject(ctx, "oito2", "onlyincustom"); err != nil {
		t.Fatal(err)
	}
	// A same-named project without subprojects in the default vault, so a
	// query against the wrong vault returns an empty list instead of failing.
	ds, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ds.EnsureProject(ctx, "oito2", ""); err != nil {
		t.Fatal(err)
	}

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2", Path: customVault}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	tool := &GetVaultConfigTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(result.Text), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, result.Text)
	}
	localConfig, ok := report["local_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected local_config to be an object, got %T: %+v", report["local_config"], report["local_config"])
	}
	subs, ok := localConfig["subprojects"].([]any)
	if !ok {
		t.Fatalf("expected subprojects to be an array, got %T", localConfig["subprojects"])
	}
	if len(subs) != 1 || subs[0] != "onlyincustom" {
		t.Fatalf("subprojects = %v, want [onlyincustom] (from the custom vault, not the default one)", subs)
	}
}

// TestGetVaultConfigTool_LocalConfigRelativePath_ResolvesAgainstConfigDir
// verifies that a relative "path" in .sync82.json is resolved against the
// directory holding that file, so the subprojects come from that vault.
func TestGetVaultConfigTool_LocalConfigRelativePath_ResolvesAgainstConfigDir(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	workspace := t.TempDir()
	cs, err := mgr.Get(ctx, filepath.Join(workspace, "vault", "custom.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cs.EnsureProject(ctx, "oito2", "onlyincustom"); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2", Path: filepath.Join("vault", "custom.db")}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	tool := &GetVaultConfigTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(result.Text), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, result.Text)
	}
	localConfig, _ := report["local_config"].(map[string]any)
	subs, _ := localConfig["subprojects"].([]any)
	if len(subs) != 1 || subs[0] != "onlyincustom" {
		t.Fatalf("subprojects = %v, want [onlyincustom] from the vault next to .sync82.json", subs)
	}
}

// TestGetVaultConfigTool_ReflectsGlobalConfig verifies that the report shows
// the global vault as configured and exposes the last project and subproject
// stored in the global config.
func TestGetVaultConfigTool_ReflectsGlobalConfig(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "oito2", LastSubproject: "perci", VaultPath: "HOME/team-vault.db"}); err != nil {
		t.Fatalf("WriteGlobalConfig: %v", err)
	}

	tool := &GetVaultConfigTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal([]byte(result.Text), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, result.Text)
	}
	if report["global_vault_configured"] != true {
		t.Errorf("global_vault_configured = %v, want true", report["global_vault_configured"])
	}
	if report["last_project"] != "oito2" || report["last_subproject"] != "perci" {
		t.Errorf("last_project/last_subproject = %v/%v, want oito2/perci", report["last_project"], report["last_subproject"])
	}
}
