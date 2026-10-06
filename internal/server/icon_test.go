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

package server

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestServerIcon_MatchesPublishedIcon checks that the embedded serverInfo
// icon is the published 64×64 icon, so the two never drift apart.
func TestServerIcon_MatchesPublishedIcon(t *testing.T) {
	published, err := os.ReadFile(filepath.Join("..", "..", "docs", "img", "icons", "icon-sync82-cropped-64.png"))
	if err != nil {
		t.Fatalf("read published icon: %v", err)
	}
	if !bytes.Equal(iconPNG, published) {
		t.Fatal("internal/server/icon.png differs from docs/img/icons/icon-sync82-cropped-64.png")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(iconPNG))
	if err != nil {
		t.Fatalf("decode icon: %v", err)
	}
	if cfg.Width != 64 || cfg.Height != 64 {
		t.Fatalf("icon is %dx%d, want 64x64", cfg.Width, cfg.Height)
	}
}
