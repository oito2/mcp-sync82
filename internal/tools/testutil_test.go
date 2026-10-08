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
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/oito2/mcp-sync82/internal/store"
)

// newToolTestEnv returns a Resolver (default vault pointing at a fresh,
// already created, empty temp SQLite file) and the Manager backing it,
// with HOME isolated to a temp dir so no test ever touches the real global
// config.
func newToolTestEnv(t *testing.T) (*Resolver, *store.Manager) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() {
		if err := mgr.Close(); err != nil {
			t.Errorf("close store manager: %v", err)
		}
	})

	if _, err := mgr.Get(context.Background(), dbPath); err != nil {
		t.Fatalf("create test vault: %v", err)
	}

	r := NewResolver(dbPath, slog.New(slog.DiscardHandler))
	return r, mgr
}

// mustJSON marshals v for use as a tool's raw json.RawMessage input. It
// fails the test if v cannot be marshaled.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal test args: %v", err)
	}
	return data
}
