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
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/server"
)

// TestMaxFrameBytes_Boundary serves MCP over a transport capped at
// maxFrameBytes, as runServe's stdio transport is, and sends a frame just
// under the cap, which is answered, then one just over it. The SDK can't
// answer a frame it never finished reading: an oversize frame ends the
// session without a JSON-RPC error, and the client sees the connection
// close.
func TestMaxFrameBytes_Boundary(t *testing.T) {
	if testing.Short() {
		t.Skip("sends two 64 MiB frames")
	}
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	s := server.New("sync82-test", "v0", slog.New(slog.DiscardHandler), server.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, &mcp.IOTransport{Reader: serverIn, Writer: serverOut, MaxLineLength: maxFrameBytes})
	}()
	responses := bufio.NewReaderSize(clientIn, 1<<16)

	send := func(frame string) {
		go func() { _, _ = io.WriteString(clientOut, frame+"\n") }()
	}
	frame := func(id int, size int) string {
		head := `{"jsonrpc":"2.0","id":` + strconv.Itoa(id) + `,"method":"ping","params":{"_meta":{"pad":"`
		tail := `"}}}`
		return head + strings.Repeat("x", size-len(head)-len(tail)) + tail
	}
	readResponse := func() (map[string]any, error) {
		line, err := responses.ReadString('\n')
		if err != nil {
			return nil, err
		}
		var msg map[string]any
		return msg, json.Unmarshal([]byte(line), &msg)
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`)
	if _, err := readResponse(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	send(frame(2, maxFrameBytes-64*1024))
	msg, err := readResponse()
	if err != nil || msg["id"] != float64(2) || msg["error"] != nil {
		t.Fatalf("a frame under maxFrameBytes: response %v, %v; want a result", msg, err)
	}

	send(frame(3, maxFrameBytes+64*1024))
	if msg, err := readResponse(); err == nil {
		t.Fatalf("a frame over maxFrameBytes was answered: %v", msg)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the session did not end after an oversize frame")
	}
}
