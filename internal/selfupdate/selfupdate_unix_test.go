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

//go:build unix

package selfupdate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunVersion_ExecutesBinary checks that runVersion runs a script with
// --version and returns its trimmed output.
func TestRunVersion_ExecutesBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake-sync82")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho v9.9.9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := runVersion(context.Background(), bin)
	if err != nil {
		t.Fatalf("runVersion: %v", err)
	}
	if got != "v9.9.9" {
		t.Fatalf("runVersion = %q, want %q", got, "v9.9.9")
	}
}

// TestRunSelfUpdate_ReadOnlyBinaryDirSuggestsPrivileges checks the
// message shown when the binary's directory is not writable.
func TestRunSelfUpdate_ReadOnlyBinaryDirSuggestsPrivileges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only directory")
	}
	content := []byte("new binary bytes")
	srv := fakeReleaseServer(t, "v2.0.0", content, "sync82_linux_amd64", sha256Hex(content))
	dir := t.TempDir()
	exePath := filepath.Join(dir, "sync82")
	if err := os.WriteFile(exePath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Error(err)
		}
	})

	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return exePath, nil },
		ValidateURL:    acceptAnyURL,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Permission denied") {
		t.Fatalf("stderr = %q, want the permission hint", stderr.String())
	}
}
