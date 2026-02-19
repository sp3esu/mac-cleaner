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
	Categories []scanResultCategory `json:"categories"`
	TotalSize  int64                `json:"total_size"`
	Token      string               `json:"token"`
}

// scanResultCategory mirrors scan.CategoryResult for JSON serialization.
// We reuse the scan package types directly via the engine results.
type scanResultCategory = interface{}

// CategoryInfo describes an available scanner group with enriched sub-category data.
type CategoryInfo struct {
	ID          string        `json:"id"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	SkipFlag    string        `json:"skip_flag"`
	Categories  []SubCategory `json:"categories"`
}

// SubCategory describes a sub-category within a scanner group.
type SubCategory struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	SkipFlag  string `json:"skip_flag"`
	RiskLevel string `json:"risk_level"`
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

	events, done := h.server.engine.ScanAll(ctx, skip)

	// Drain events channel, streaming progress to client.
	for event := range events {
		if ctx.Err() != nil {
			break
		}
		progress := ScanProgress{ScannerID: event.ScannerID, Label: event.Label}
		switch event.Type {
		case engine.EventScannerStart:
			progress.Event = "scanner_start"
		case engine.EventScannerDone:
			progress.Event = "scanner_done"
		case engine.EventScannerError:
			progress.Event = "scanner_error"
			if event.Err != nil {
				progress.Error = event.Err.Error()
			}
		}
		_ = w.WriteProgress(req.ID, progress)
	}

	result := <-done

	// If client disconnected during scan, don't bother with final result.
	if ctx.Err() != nil {
		return
	}

	// Normalize nil slices to empty arrays so JSON encodes [] not null.
	for i := range result.Results {
		if result.Results[i].Entries == nil {
			result.Results[i].Entries = []scan.ScanEntry{}
		}
	}

	var totalSize int64
	for _, cat := range result.Results {
		totalSize += cat.TotalSize
	}

	_ = w.WriteResult(req.ID, struct {
		Categories interface{} `json:"categories"`
		TotalSize  int64       `json:"total_size"`
		Token      string      `json:"token"`
	}{
		Categories: result.Results,
		TotalSize:  totalSize,
		Token:      string(result.Token),
	})
}

func (h *Handler) handleCategories(req Request, w *NDJSONWriter) {
	infos := h.server.engine.Categories()
	cats := make([]CategoryInfo, len(infos))
	for i, info := range infos {
		subs := make([]SubCategory, len(info.SubCategories))
		for j, sub := range info.SubCategories {
			subs[j] = SubCategory{
				ID:        sub.ID,
				Label:     sub.Label,
				SkipFlag:  sub.SkipFlag,
				RiskLevel: sub.RiskLevel,
			}
		}
		cats[i] = CategoryInfo{
			ID:          info.ID,
			Label:       info.Name,
			Description: info.Description,
			SkipFlag:    info.SkipFlag,
			Categories:  subs,
		}
	}
	_ = w.WriteResult(req.ID, CategoriesResult{Scanners: cats})
}
