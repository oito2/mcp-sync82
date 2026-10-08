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
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/tools"
)

// addResources registers every template in defs on s, read through
// res, and a receiving middleware that answers resources/list with the
// context resource of every project in the default vault, listed when the
// request arrives so projects created during the session appear. It
// returns the function that tells connected clients the resource list
// changed: it registers the first template again, which makes the SDK send
// notifications/resources/list_changed (debounced), the only way the SDK
// offers to send it. defs must not be empty.
func addResources(s *mcp.Server, defs []tools.ResourceTemplate, res *tools.Resources, logger *slog.Logger) (listChanged func()) {
	read := readResourceHandler(res, logger)
	templates := make([]*mcp.ResourceTemplate, len(defs))
	for i, t := range defs {
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
// found error; any other failure, or a panic, as an internal error worded
// like a tool's (internalError, recoverAsInternal).
func readResourceHandler(res *tools.Resources, logger *slog.Logger) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (_ *mcp.ReadResourceResult, err error) {
		defer recoverAsInternal(logger, "resources/read", &err)
		uri := req.Params.URI
		text, err := res.Read(ctx, uri)
		if errors.Is(err, tools.ErrResourceNotFound) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		if err != nil {
			return nil, internalError(logger, "resources/read", err)
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
// cacheScope. The list is never paginated, so any cursor is invalid and
// answered with an invalid-params error. A failure or a panic is answered
// with an internal error (internalError, recoverAsInternal).
func listResourcesMiddleware(res *tools.Resources, logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (_ mcp.Result, err error) {
			if method != "resources/list" {
				return next(ctx, method, req)
			}
			defer recoverAsInternal(logger, method, &err)
			if p, ok := req.GetParams().(*mcp.ListResourcesParams); ok && p != nil && p.Cursor != "" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid cursor: resources/list returns every resource in one page"}
			}
			infos, err := res.List(ctx)
			if err != nil {
				return nil, internalError(logger, method, err)
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

// addPrompts registers every definition in prompts on s. Getting a
// prompt renders its instruction as one user message; invalid arguments
// are reported as an invalid-params error, and a panic as an internal
// error logged to logger.
func addPrompts(s *mcp.Server, prompts []tools.PromptDefinition, logger *slog.Logger) {
	for _, def := range prompts {
		args := make([]*mcp.PromptArgument, len(def.Arguments))
		for i, a := range def.Arguments {
			args[i] = &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required}
		}
		render := def.Render
		description := def.Description
		s.AddPrompt(&mcp.Prompt{Name: def.Name, Title: def.Title, Description: def.Description, Arguments: args},
			func(_ context.Context, req *mcp.GetPromptRequest) (_ *mcp.GetPromptResult, err error) {
				defer recoverAsInternal(logger, "prompts/get", &err)
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
// argument that one of opts.Prompts or opts.ResourceTemplates actually
// has, the matching names from opts.Resources.Complete; for any other
// reference or argument, no values. A failure or a panic is answered with an internal error
// (internalError, recoverAsInternal).
func completionHandler(opts Options, logger *slog.Logger) func(context.Context, *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (_ *mcp.CompleteResult, err error) {
		defer recoverAsInternal(logger, "completion/complete", &err)
		out := &mcp.CompleteResult{Completion: mcp.CompletionResultDetails{Values: []string{}}}
		p := req.Params
		if p == nil || p.Ref == nil || !hasArgument(opts, p.Ref, p.Argument.Name) {
			return out, nil
		}
		var args map[string]string
		if p.Context != nil {
			args = p.Context.Arguments
		}
		values, total, err := opts.Resources.Complete(ctx, p.Argument.Name, p.Argument.Value, args)
		if err != nil {
			return nil, internalError(logger, "completion/complete", err)
		}
		if values != nil {
			out.Completion.Values = values
		}
		out.Completion.Total = total
		out.Completion.HasMore = total > len(out.Completion.Values)
		return out, nil
	}
}

// hasArgument reports whether ref names one of opts.Prompts
// ("ref/prompt") with an argument called name, or one of
// opts.ResourceTemplates ("ref/resource") with a {name} variable.
func hasArgument(opts Options, ref *mcp.CompleteReference, name string) bool {
	switch ref.Type {
	case "ref/prompt":
		for _, p := range opts.Prompts {
			if p.Name != ref.Name {
				continue
			}
			for _, a := range p.Arguments {
				if a.Name == name {
					return true
				}
			}
		}
	case "ref/resource":
		for _, t := range opts.ResourceTemplates {
			if t.URITemplate == ref.URI && strings.Contains(t.URITemplate, "{"+name+"}") {
				return true
			}
		}
	}
	return false
}
