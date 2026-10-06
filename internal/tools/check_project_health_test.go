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
	"strings"
	"testing"
)

// TestCheckProjectHealthTool_Healthy verifies that a project with every
// standard kind populated is reported HEALTHY, with each kind marked OK and a
// non-error result.
func TestCheckProjectHealthTool_Healthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"memory", "architecture", "stack", "next_steps"} {
		if err := s.WriteDocument(ctx, "acme", "", k, "content"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- x"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-01-01", "## 2026-01-01\n- y"); err != nil {
		t.Fatal(err)
	}

	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected IsError=false for a healthy project, got text: %s", result.Text)
	}
	if !strings.Contains(result.Text, "HEALTHY") {
		t.Fatalf("expected HEALTHY status, got: %s", result.Text)
	}
	for _, k := range standardKinds {
		if !strings.Contains(result.Text, k+": OK") {
			t.Errorf("expected %q reported OK, got: %s", k, result.Text)
		}
	}
}

// TestCheckProjectHealthTool_Unhealthy verifies that a project with missing
// kinds is reported UNHEALTHY with the missing kinds listed and a
// recommendation, and that this is returned as an error result rather than a
// Go error.
func TestCheckProjectHealthTool_Unhealthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	// Only memory is written — everything else stays missing.
	if err := s.WriteDocument(ctx, "acme", "", "memory", "content"); err != nil {
		t.Fatal(err)
	}

	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for an unhealthy project — this is deliberate, not a crash")
	}
	if !strings.Contains(result.Text, "UNHEALTHY") {
		t.Fatalf("expected UNHEALTHY status, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "memory: OK") {
		t.Fatalf("expected memory reported OK, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "architecture: MISSING") {
		t.Fatalf("expected architecture reported MISSING, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Recommendation") {
		t.Fatalf("expected a recommendation when unhealthy, got: %s", result.Text)
	}
}

// TestCheckProjectHealthTool_NeedsInput verifies that, with no project
// resolvable, the tool returns NeedsInputMessage as a non-error result.
func TestCheckProjectHealthTool_NeedsInput(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Text != NeedsInputMessage {
		t.Fatalf("expected NeedsInputMessage, got: %s", result.Text)
	}
	if result.IsError {
		t.Fatal("NeedsInputMessage must not be an error result")
	}
}

// TestCheckProjectHealthTool_FullyArchivedLogStaysHealthy verifies that a log
// kind (progress, decisions) whose entries have all been archived still counts
// as present, so the project stays HEALTHY.
func TestCheckProjectHealthTool_FullyArchivedLogStaysHealthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"memory", "architecture", "stack", "next_steps"} {
		if err := s.WriteDocument(ctx, "acme", "", k, "content"); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []string{"progress", "decisions"} {
		if err := s.AppendEntry(ctx, "acme", "", k, "2020-01-01", "## 2020-01-01\n- old"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ArchiveEntries(ctx, "acme", "", k, "2021-01-01"); err != nil {
			t.Fatal(err)
		}
	}

	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError || !strings.Contains(result.Text, "HEALTHY") || strings.Contains(result.Text, "UNHEALTHY") {
		t.Fatalf("expected HEALTHY, got: %s", result.Text)
	}
}
