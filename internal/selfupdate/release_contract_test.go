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

package selfupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oito2/mcp-sync82/internal/fsutil"
)

// TestReleaseContract_DistMatchesSelfUpdate checks that a release directory
// produced by scripts/release matches what self-update expects. When
// SYNC82_DIST_DIR names such a directory, it must hold one asset per
// supported platform, each named as AssetName produces and listed in
// checksums.txt with its real SHA-256, and checksums.txt must also cover the
// sync82.mcpb bundle. The test is skipped when the variable is unset.
func TestReleaseContract_DistMatchesSelfUpdate(t *testing.T) {
	dist := os.Getenv("SYNC82_DIST_DIR")
	if dist == "" {
		t.Skip("SYNC82_DIST_DIR not set")
	}
	known := map[string]bool{}
	for _, p := range ReleasePlatforms() {
		known[AssetName(p[0], p[1])] = true
	}

	assets, err := filepath.Glob(filepath.Join(dist, "sync82_*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != len(known) {
		t.Errorf("found %d sync82_* assets in %s, want one per platform (%d)", len(assets), dist, len(known))
	}
	checked := append(assets, filepath.Join(dist, "sync82.mcpb"))
	for _, asset := range checked {
		name := filepath.Base(asset)
		if name != "sync82.mcpb" && !known[name] {
			t.Errorf("%s is not a name AssetName produces", name)
			continue
		}
		want, err := findChecksum(filepath.Join(dist, "checksums.txt"), name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got, err := fsutil.SHA256File(asset); err != nil || got != want {
			t.Errorf("%s: sha256 = %s (err %v), checksums.txt says %s", name, got, err, want)
		}
	}
}
