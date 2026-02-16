package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sp3esu/mac-cleaner/internal/engine"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// ScanProgress is a progress event streamed during scanning.
type ScanProgress struct {
	Event     string `json:"event"` // "scanner_start", "scanner_done", "scanner_error"
	ScannerID string `json:"scanner_id"`
	Label     string `json:"label"`
	Error     string `json:"error,omitempty"`
}

// ScanResult is the final result of a scan operation.
type ScanResult struct {
	Categories []scan.CategoryResult `json:"categories"`
	TotalSize  int64                 `json:"total_size"`
}

// CategoryInfo describes an available scanner group.
type CategoryInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CategoriesResult is the result of a categories request.
type CategoriesResult struct {
	Scanners []CategoryInfo `json:"scanners"`
}

func (h *Handler) handleScan(ctx context.Context, req Request, w *NDJSONWriter) {
	if !h.server.busy.CompareAndSwap(false, true) {
		_ = w.WriteErrorMsg(req.ID, "another operation is in progress")
		return
	}
	defer h.server.busy.Store(false)

	// Check for client disconnect before starting.
	if ctx.Err() != nil {
		return
	}

	var params ScanParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			_ = w.WriteErrorMsg(req.ID, fmt.Sprintf("invalid params: %v", err))
			return
		}
	}

	skip := make(map[string]bool, len(params.Skip))
	for _, id := range params.Skip {
		skip[id] = true
	}

	results := engine.ScanAll(engine.DefaultScanners(), skip, func(e engine.ScanEvent) {
		// Check for client disconnect — stop streaming if gone.
		if ctx.Err() != nil {
			return
		}

		progress := ScanProgress{
			ScannerID: e.ScannerID,
			Label:     e.Label,
		}
		switch e.Type {
		case engine.EventScannerStart:
			progress.Event = "scanner_start"
		case engine.EventScannerDone:
			progress.Event = "scanner_done"
		case engine.EventScannerError:
			progress.Event = "scanner_error"
			if e.Err != nil {
				progress.Error = e.Err.Error()
			}
		}
		_ = w.WriteProgress(req.ID, progress)
	})

	// If client disconnected during scan, don't bother with final result.
	if ctx.Err() != nil {
		return
	}

	// Store results for cleanup validation.
	h.server.lastScan.results.Store(&results)

	var totalSize int64
	for _, cat := range results {
		totalSize += cat.TotalSize
	}

	_ = w.WriteResult(req.ID, ScanResult{
		Categories: results,
		TotalSize:  totalSize,
	})
}

func (h *Handler) handleCategories(req Request, w *NDJSONWriter) {
	scanners := engine.DefaultScanners()
	infos := make([]CategoryInfo, len(scanners))
	for i, s := range scanners {
		infos[i] = CategoryInfo{ID: s.ID, Label: s.Label}
	}
	_ = w.WriteResult(req.ID, CategoriesResult{Scanners: infos})
}
