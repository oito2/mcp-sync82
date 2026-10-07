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
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/tools"
)

// addResources registers every tools.ResourceTemplate on s, read through
// res, and a receiving middleware that answers resources/list with the
// context resource of every project in the default vault, listed when the
// request arrives so projects created during the session appear. It
// returns the function that tells connected clients the resource list
// changed: it registers the first template again, which makes the SDK send
// notifications/resources/list_changed (debounced), the only way the SDK
// offers to send it.
func addResources(s *mcp.Server, res *tools.Resources, logger *slog.Logger) (listChanged func()) {
	read := readResourceHandler(res, logger)
	templates := make([]*mcp.ResourceTemplate, len(tools.ResourceTemplates))
	for i, t := range tools.ResourceTemplates {
		templates[i] = &mcp.ResourceTemplate{
			URITemplate: t.URITemplate,
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
			MIMEType:    tools.ResourceMIMEType,
		}
		s.AddResourceTemplate(templates[i], read)
	}
	s.AddReceivingMiddleware(listResourcesMiddleware(res, logger))
	return func() { s.AddResourceTemplate(templates[0], read) }
}

// readResourceHandler returns the handler of every sync82 resource
// template: it reads the requested URI through res and returns its text as
// Markdown, marked private (cacheScope "private"): it is the user's own
// memory, which no shared cache may keep. Left unset, the SDK would mark
// it public. An unknown or invalid URI is reported as an MCP resource-not-
// found error; any other failure is logged in full and reported with a
// generic message, since it may hold vault paths.
func readResourceHandler(res *tools.Resources, logger *slog.Logger) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		text, err := res.Read(ctx, uri)
		if errors.Is(err, tools.ErrResourceNotFound) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		if err != nil {
			logger.Error("resource read failed", "uri", uri, "error", err)
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error: could not read the resource"}
		}
		out := &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: tools.ResourceMIMEType, Text: text}}}
		out.CacheScope = "private"
		return out, nil
	}
}

// listResourcesMiddleware answers resources/list with res.List, in one
// page; every other method goes to next. The answer is marked private and
// immediately stale (cacheScope "private", ttlMs 0): it lists the user's
// own vault and changes whenever a project is created. The SDK fills these
// fields only on answers it builds itself, and clients reject an empty
// cacheScope. A failure is logged in full and reported with a generic
// message.
func listResourcesMiddleware(res *tools.Resources, logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "resources/list" {
				return next(ctx, method, req)
			}
			infos, err := res.List(ctx)
			if err != nil {
				logger.Error("resource list failed", "error", err)
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error: could not list the resources"}
			}
			out := &mcp.ListResourcesResult{Resources: []*mcp.Resource{}}
			out.CacheScope, out.TTLMs = "private", 0
			for _, info := range infos {
				out.Resources = append(out.Resources, &mcp.Resource{
					URI:         info.URI,
					Name:        info.Name,
					Title:       info.Title,
					Description: info.Description,
					MIMEType:    tools.ResourceMIMEType,
				})
			}
			return out, nil
		}
	}
}

// addPrompts registers every tools.Prompts definition on s. Getting a
// prompt renders its instruction as one user message; invalid arguments
// are reported as an invalid-params error.
func addPrompts(s *mcp.Server) {
	for _, def := range tools.Prompts {
		args := make([]*mcp.PromptArgument, len(def.Arguments))
		for i, a := range def.Arguments {
			args[i] = &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required}
		}
		render := def.Render
		description := def.Description
		s.AddPrompt(&mcp.Prompt{Name: def.Name, Title: def.Title, Description: def.Description, Arguments: args},
			func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				text, err := render(req.Params.Arguments)
				if err != nil {
					return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
				}
				return &mcp.GetPromptResult{
					Description: description,
					Messages:    []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}},
				}, nil
			})
	}
}

// completionHandler returns the handler of completion/complete: for an
// argument of a sync82 prompt or of a sync82 resource template, the
// matching names from res.Complete; for any other reference, no values. A
// failure is logged in full and reported with a generic message.
func completionHandler(res *tools.Resources, logger *slog.Logger) func(context.Context, *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		out := &mcp.CompleteResult{Completion: mcp.CompletionResultDetails{Values: []string{}}}
		p := req.Params
		if p == nil || p.Ref == nil || !isSync82Reference(p.Ref) {
			return out, nil
		}
		var args map[string]string
		if p.Context != nil {
			args = p.Context.Arguments
		}
		values, total, err := res.Complete(ctx, p.Argument.Name, p.Argument.Value, args)
		if err != nil {
			logger.Error("completion failed", "argument", p.Argument.Name, "error", err)
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error: could not complete the argument"}
		}
		if values != nil {
			out.Completion.Values = values
		}
		out.Completion.Total = total
		out.Completion.HasMore = total > len(out.Completion.Values)
		return out, nil
	}
}

// isSync82Reference reports whether ref names one of tools.Prompts
// ("ref/prompt") or one of tools.ResourceTemplates ("ref/resource").
func isSync82Reference(ref *mcp.CompleteReference) bool {
	switch ref.Type {
	case "ref/prompt":
		for _, p := range tools.Prompts {
			if p.Name == ref.Name {
				return true
			}
		}
	case "ref/resource":
		for _, t := range tools.ResourceTemplates {
			if t.URITemplate == ref.URI {
				return true
			}
		}
	}
	return false
}
