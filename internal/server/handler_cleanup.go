package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sp3esu/mac-cleaner/internal/engine"
)

// CleanupProgress is a progress event streamed during cleanup.
type CleanupProgress struct {
	Event     string `json:"event"` // "cleanup_category_start", "cleanup_entry"
	Category  string `json:"category"`
	EntryPath string `json:"entry_path,omitempty"`
	Current   int    `json:"current"`
	Total     int    `json:"total"`
}

// CleanupResult is the final result of a cleanup operation.
type CleanupResult struct {
	Removed    int      `json:"removed"`
	Failed     int      `json:"failed"`
	BytesFreed int64    `json:"bytes_freed"`
	Errors     []string `json:"errors,omitempty"`
}

func (h *Handler) handleCleanup(ctx context.Context, req Request, w *NDJSONWriter) {
	if !h.server.busy.CompareAndSwap(false, true) {
		_ = w.WriteErrorMsg(req.ID, "another operation is in progress")
		return
	}
	defer h.server.busy.Store(false)

	// Check for client disconnect before starting.
	if ctx.Err() != nil {
		return
	}

	var params CleanupParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			_ = w.WriteErrorMsg(req.ID, fmt.Sprintf("invalid params: %v", err))
			return
		}
	}

	// Token is required for cleanup (must come from a prior scan).
	if params.Token == "" {
		_ = w.WriteErrorMsg(req.ID, "token is required; run scan first")
		return
	}

	events, done := h.server.engine.Cleanup(ctx, engine.ScanToken(params.Token), params.Categories)

	// Drain events channel, streaming progress to client.
	for event := range events {
		if ctx.Err() != nil {
			break
		}
		_ = w.WriteProgress(req.ID, CleanupProgress{
			Event:     event.Type,
			Category:  event.Category,
			EntryPath: event.EntryPath,
			Current:   event.Current,
			Total:     event.Total,
		})
	}

	result := <-done

	// If client disconnected during cleanup, skip final result.
	if ctx.Err() != nil {
		return
	}

	if result.Err != nil {
		_ = w.WriteErrorMsg(req.ID, result.Err.Error())
		return
	}

	var errs []string
	for _, e := range result.Result.Errors {
		errs = append(errs, e.Error())
	}

	_ = w.WriteResult(req.ID, CleanupResult{
		Removed:    result.Result.Removed,
		Failed:     result.Result.Failed,
		BytesFreed: result.Result.BytesFreed,
		Errors:     errs,
	})
}
