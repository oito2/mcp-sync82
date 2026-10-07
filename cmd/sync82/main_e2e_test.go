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
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// buildBinary compiles the sync82 command into a temporary directory and
// returns the binary's path. The calling test is skipped under -short and
// fails if the build fails. These tests run the real binary as a
// subprocess, the way MCP clients do.
func buildBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the binary; skipped with -short")
	}
	name := "sync82"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// isolatedEnv returns the current environment extended so that HOME,
// USERPROFILE and the default vault path (SYNC82_DB_PATH) point into
// temporary directories.
func isolatedEnv(t *testing.T) []string {
	home := t.TempDir()
	return append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "SYNC82_DB_PATH="+filepath.Join(t.TempDir(), "vault.db"))
}

// TestBinary_ServesMCPOverStdio starts the real binary over stdio, lists
// its tools and round-trips a project through it.
func TestBinary_ServesMCPOverStdio(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin)
	cmd.Env = isolatedEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "v0"}, nil).Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 19 {
		t.Fatalf("binary lists %d tools, want 19", len(res.Tools))
	}

	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"create_project", map[string]any{"project": "acme"}},
		{"write_memory", map[string]any{"project": "acme", "filename": "memory", "content": "Acme builds rockets."}},
	} {
		r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err != nil || r.IsError {
			t.Fatalf("%s: result=%+v err=%v", call.name, r, err)
		}
	}
	r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "read_memory", Arguments: map[string]any{"project": "acme", "filename": "memory"}})
	if err != nil || r.IsError {
		t.Fatalf("read_memory: result=%+v err=%v", r, err)
	}
	if text := r.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "Acme builds rockets.") {
		t.Fatalf("read_memory = %q", text)
	}
}

// TestBinary_UnknownSubcommandExits1 checks that a typo'd subcommand
// fails with the usage text instead of starting the server.
func TestBinary_UnknownSubcommandExits1(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "instal")
	cmd.Env = isolatedEnv(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit code 1", err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stdout=%q stderr=%q, want only the usage on stderr", stdout.String(), stderr.String())
	}
}

// TestBinary_SIGTERMStopsServerCleanly checks that a server sent SIGTERM
// shuts down within 10 seconds and exits with code 0, so its vault is
// closed and its SQLite WAL checkpointed.
func TestBinary_SIGTERMStopsServerCleanly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no SIGTERM on Windows")
	}
	bin := buildBinary(t)
	cmd := exec.Command(bin)
	cmd.Env = isolatedEnv(t)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server exited with %v after SIGTERM, want exit code 0", err)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill() // best effort: the test fails either way
		t.Fatal("server did not stop within 10s of SIGTERM")
	}
}
