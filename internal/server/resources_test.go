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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// TestRecoverAsInternal_TurnsAPanicIntoAnError verifies that a deferred
// recoverAsInternal answers a panicking handler with an internal error.
func TestRecoverAsInternal_TurnsAPanicIntoAnError(t *testing.T) {
	handler := func() (err error) {
		defer recoverAsInternal(slog.New(slog.DiscardHandler), "test/method", &err)
		panic("boom")
	}
	var rpcErr *jsonrpc.Error
	if err := handler(); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInternalError {
		t.Fatalf("err = %v, want a JSON-RPC internal error", err)
	}
}

// TestInternalError_WordsErrorsLikeTools verifies that internalError hides
// vault-open details and keeps the upgrade hint of a too-new schema.
func TestInternalError_WordsErrorsLikeTools(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	open := fmt.Errorf("%w: open vault %s: %w", store.ErrOpenFailed, "/home/u/.sync82/knowledge.db", errors.New("permission denied"))
	if msg := internalError(logger, "resources/read", open).Error(); strings.Contains(msg, "/home/u") {
		t.Errorf("message leaks the vault path: %q", msg)
	}
	tooNew := fmt.Errorf("%w: migrate vault %s: %w", store.ErrOpenFailed, "/home/u/v.db", store.ErrSchemaTooNew)
	if msg := internalError(logger, "resources/read", tooNew).Error(); !strings.Contains(msg, "upgrade sync82") {
		t.Errorf("message lacks the upgrade hint: %q", msg)
	}
}

// TestHasArgument verifies that completion only accepts an argument the
// referenced prompt or resource template actually has.
func TestHasArgument(t *testing.T) {
	opts := Options{
		Prompts: []tools.PromptDefinition{{Name: "start_session", Arguments: []tools.PromptArgument{{Name: "project"}}}},
		ResourceTemplates: []tools.ResourceTemplate{
			{URITemplate: "sync82://projects/{project}/context"},
			{URITemplate: "sync82://projects/{project}/files/{file}"},
		},
	}
	cases := []struct {
		ref  mcp.CompleteReference
		arg  string
		want bool
	}{
		{mcp.CompleteReference{Type: "ref/prompt", Name: "start_session"}, "project", true},
		{mcp.CompleteReference{Type: "ref/prompt", Name: "start_session"}, "file", false},
		{mcp.CompleteReference{Type: "ref/prompt", Name: "other"}, "project", false},
		{mcp.CompleteReference{Type: "ref/resource", URI: "sync82://projects/{project}/files/{file}"}, "file", true},
		{mcp.CompleteReference{Type: "ref/resource", URI: "sync82://projects/{project}/context"}, "file", false},
		{mcp.CompleteReference{Type: "ref/resource", URI: "sync82://projects/{project}/context"}, "project", true},
	}
	for _, c := range cases {
		if got := hasArgument(opts, &c.ref, c.arg); got != c.want {
			t.Errorf("hasArgument(%+v, %q) = %v, want %v", c.ref, c.arg, got, c.want)
		}
	}
}

// TestListResourcesMiddleware_RejectsACursor verifies that resources/list
// with a cursor is an invalid-params error, since the list has one page.
func TestListResourcesMiddleware_RejectsACursor(t *testing.T) {
	mw := listResourcesMiddleware(nil, slog.New(slog.DiscardHandler))
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		t.Fatal("resources/list was passed on to the SDK")
		return nil, nil
	}
	req := &mcp.ListResourcesRequest{Params: &mcp.ListResourcesParams{Cursor: "garbage"}}
	_, err := mw(next)(context.Background(), "resources/list", req)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("err = %v, want an invalid-params error", err)
	}
}

// TestCancelAsInfo verifies that the SDK's logger records an error caused
// by context.Canceled at INFO and every other error at ERROR.
func TestCancelAsInfo(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(cancelAsInfo{slog.NewTextHandler(&buf, nil)}).With("server", "sync82")
	logger.Error("server run cancelled", "error", fmt.Errorf("run: %w", context.Canceled))
	logger.Error("server failed", "error", errors.New("broken pipe"))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "level=INFO") || !strings.Contains(lines[1], "level=ERROR") {
		t.Errorf("logged:\n%s\nwant the cancellation at INFO and the failure at ERROR", buf.String())
	}
}
