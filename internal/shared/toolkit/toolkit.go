// Package toolkit holds cross-feature helpers for MCP handlers and services:
// structured tool results, error formatting, and an idempotency wrapper that
// ties together hashing, the operation log, and dry-run.
package toolkit

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/tergeoo/asc-mcp/internal/shared/asc"
	"github.com/tergeoo/asc-mcp/internal/shared/mcperr"
	"github.com/tergeoo/asc-mcp/internal/shared/store"
	"github.com/tergeoo/asc-mcp/internal/shared/validate"
)

// Deps are the shared dependencies injected into every feature service.
type Deps struct {
	ASC    *asc.Facade
	Store  store.Store
	DryRun bool
}

// Result wraps any structured payload into an MCP tool result. The payload is
// returned both as structured content and as a JSON text fallback.
func Result(payload any) (*mcp.CallToolResult, error) {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to encode result: " + err.Error()), nil
	}
	return mcp.NewToolResultStructured(payload, string(raw)), nil
}

// Fail converts any error into a structured MCP error result. ASC and
// validation errors are surfaced with their full structured detail; tools never
// swallow errors silently.
func Fail(err error) (*mcp.CallToolResult, error) {
	switch e := err.(type) {
	case *mcperr.Error:
		raw, _ := json.MarshalIndent(map[string]any{"error": e}, "", "  ")
		return mcp.NewToolResultError(string(raw)), nil
	case *validate.Error:
		raw, _ := json.MarshalIndent(map[string]any{"error": map[string]any{
			"kind":   "validation",
			"fields": e.Fields,
		}}, "", "  ")
		return mcp.NewToolResultError(string(raw)), nil
	default:
		return mcp.NewToolResultError(err.Error()), nil
	}
}

// IdemKey identifies an operation for idempotency + audit.
type IdemKey struct {
	Tool      string
	AppID     string
	VersionID string
	Input     any
}

// Outcome is the result of an idempotent operation.
type Outcome struct {
	// Result is the payload produced (or the cached payload on a hit).
	Result any
	// Cached is true when a prior identical successful operation was found.
	Cached bool
	// DryRun is true when the operation was validated but not executed.
	DryRun bool
}

// RunIdempotent executes fn unless an identical successful operation already
// exists (idempotency) or dryRun is set. It records the outcome in the audit
// log. fn must perform the actual ASC write and return the resulting payload.
func RunIdempotent(
	ctx context.Context,
	st store.Store,
	dryRun bool,
	key IdemKey,
	fn func() (any, error),
) (Outcome, error) {
	hash := store.HashInput(key.Tool, key.Input)

	if st.Enabled() {
		if op, found, err := st.LookupOperation(ctx, hash); err == nil && found {
			var cached any
			if op.Result != "" {
				_ = json.Unmarshal([]byte(op.Result), &cached)
			}
			return Outcome{Result: cached, Cached: true}, nil
		}
	}

	if dryRun {
		_ = st.SaveOperation(ctx, store.Operation{
			AppID: key.AppID, VersionID: key.VersionID, Tool: key.Tool,
			InputHash: hash, Status: store.StatusDryRun,
		})
		return Outcome{Result: key.Input, DryRun: true}, nil
	}

	result, err := fn()
	if err != nil {
		_ = st.SaveOperation(ctx, store.Operation{
			AppID: key.AppID, VersionID: key.VersionID, Tool: key.Tool,
			InputHash: hash, Status: store.StatusError, Error: err.Error(),
		})
		return Outcome{}, err
	}

	resultJSON, _ := json.Marshal(result)
	_ = st.SaveOperation(ctx, store.Operation{
		AppID: key.AppID, VersionID: key.VersionID, Tool: key.Tool,
		InputHash: hash, Status: store.StatusSuccess, Result: string(resultJSON),
	})
	return Outcome{Result: result}, nil
}
