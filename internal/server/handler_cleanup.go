package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// CleanupProgress is a progress event streamed during cleanup.
type CleanupProgress struct {
	Event     string `json:"event"` // "category_start", "entry_progress"
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

	// Validate against prior scan results (replay protection).
	lastResults := h.server.lastScan.results.Load()
	if lastResults == nil {
		_ = w.WriteErrorMsg(req.ID, "no prior scan results; run scan first")
		return
	}

	// Filter to requested categories, or use all if none specified.
	var toClean []scan.CategoryResult
	if len(params.Categories) > 0 {
		wanted := make(map[string]bool, len(params.Categories))
		for _, id := range params.Categories {
			wanted[id] = true
		}
		for _, cat := range *lastResults {
			if wanted[cat.Category] {
				toClean = append(toClean, cat)
			}
		}
		if len(toClean) == 0 {
			_ = w.WriteErrorMsg(req.ID, "none of the requested categories match prior scan results")
			return
		}
	} else {
		toClean = *lastResults
	}

	result := cleanup.Execute(toClean, func(categoryDesc, entryPath string, current, total int) {
		// Check for client disconnect — stop streaming if gone.
		if ctx.Err() != nil {
			return
		}

		event := "entry_progress"
		if entryPath == "" {
			event = "category_start"
		}
		_ = w.WriteProgress(req.ID, CleanupProgress{
			Event:     event,
			Category:  categoryDesc,
			EntryPath: entryPath,
			Current:   current,
			Total:     total,
		})
	})

	// If client disconnected during cleanup, skip final result.
	if ctx.Err() != nil {
		return
	}

	// Clear scan results after cleanup (prevents replay).
	h.server.lastScan.results.Store(nil)

	var errs []string
	for _, e := range result.Errors {
		errs = append(errs, e.Error())
	}

	_ = w.WriteResult(req.ID, CleanupResult{
		Removed:    result.Removed,
		Failed:     result.Failed,
		BytesFreed: result.BytesFreed,
		Errors:     errs,
	})
}
