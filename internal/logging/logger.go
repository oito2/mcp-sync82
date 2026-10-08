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

// Package logging provides a stderr-only logger.
//
// MCP over stdio uses stdout exclusively for JSON-RPC frames, so nothing in
// the process may write to stdout. Log calls go through this package
// instead of the standard log package or fmt.Print*.
package logging

import (
	"io"
	"log/slog"
)

// New returns a structured slog logger that writes text records to w, using
// the default handler options. w is the process's stderr, or a stand-in for
// it in tests; it must never be stdout.
func New(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}
