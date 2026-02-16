// Package engine provides scan and cleanup orchestration decoupled from the
// CLI layer. It is used by both the cobra CLI commands and the IPC server.
package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/sp3esu/mac-cleaner/internal/cleanup"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// ScanEvent reports progress during a scan operation.
type ScanEvent struct {
	// Type is one of "scanner_start", "scanner_done", "scanner_error".
	Type string
	// ScannerID identifies which scanner group emitted the event.
	ScannerID string
	// Label is the human-readable scanner group name.
	Label string
	// Results is populated on "scanner_done" events.
	Results []scan.CategoryResult
	// Err is populated on "scanner_error" events.
	Err error
}

// Scan event types.
const (
	EventScannerStart = "scanner_start"
	EventScannerDone  = "scanner_done"
	EventScannerError = "scanner_error"
)

// CleanupEvent reports progress during a cleanup operation.
type CleanupEvent struct {
	// Type is one of the EventCleanup* constants.
	Type string
	// Category is the human-readable category description.
	Category string
	// EntryPath is the filesystem path being cleaned (empty for category-start events).
	EntryPath string
	// Current is the 1-based item index across all categories.
	Current int
	// Total is the overall item count.
	Total int
}

// Cleanup event types.
const (
	EventCleanupCategoryStart = "cleanup_category_start"
	EventCleanupEntry         = "cleanup_entry"
	EventCleanupDone          = "cleanup_done"
	EventCleanupError         = "cleanup_error"
)

// ScanResult holds the final aggregated output of ScanAll.
type ScanResult struct {
	Results []scan.CategoryResult
	Token   ScanToken
}

// CleanupDone holds the final outcome of a Cleanup operation.
type CleanupDone struct {
	Result cleanup.CleanupResult
	Err    error
}

// Engine orchestrates scanning and cleanup operations. It holds the
// scanner registry and token store. Safe for concurrent use.
type Engine struct {
	scanners  []Scanner
	mu        sync.Mutex
	lastToken struct {
		token ScanToken
		entry *tokenEntry
	}
}

// New creates an Engine with an empty scanner registry.
func New() *Engine {
	return &Engine{}
}

// ScanAll runs all registered scanners sequentially, streaming events
// through the returned channel. The done channel receives exactly one
// ScanResult when all scanners complete (or context is cancelled).
// The skip set filters category IDs from the final output.
func (e *Engine) ScanAll(ctx context.Context, skip map[string]bool) (<-chan ScanEvent, <-chan ScanResult) {
	events := make(chan ScanEvent)
	done := make(chan ScanResult, 1)

	go func() {
		defer close(events)
		defer close(done)

		var all []scan.CategoryResult
		for _, s := range e.scanners {
			if ctx.Err() != nil {
				return
			}

			info := s.Info()
			select {
			case events <- ScanEvent{Type: EventScannerStart, ScannerID: info.ID, Label: info.Name}:
			case <-ctx.Done():
				return
			}

			results, err := s.Scan()
			if err != nil {
				select {
				case events <- ScanEvent{Type: EventScannerError, ScannerID: info.ID, Label: info.Name, Err: err}:
				case <-ctx.Done():
					return
				}
				continue
			}

			select {
			case events <- ScanEvent{Type: EventScannerDone, ScannerID: info.ID, Label: info.Name, Results: results}:
			case <-ctx.Done():
				return
			}
			all = append(all, results...)
		}

		filtered := FilterSkipped(all, skip)
		token := e.storeResults(filtered)
		done <- ScanResult{Results: filtered, Token: token}
	}()

	return events, done
}

// Run executes a single scanner synchronously and returns its results.
// Returns an error if the scanner ID is not found, the context is
// cancelled, or the scanner itself fails.
func (e *Engine) Run(ctx context.Context, scannerID string) ([]scan.CategoryResult, error) {
	var target Scanner
	for _, s := range e.scanners {
		if s.Info().ID == scannerID {
			target = s
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("scanner %q not found", scannerID)
	}

	if ctx.Err() != nil {
		return nil, &CancelledError{Operation: "scan"}
	}

	results, err := target.Scan()
	if err != nil {
		return nil, &ScanError{ScannerID: scannerID, Err: err}
	}
	return results, nil
}

// Cleanup removes files for the given categories from a prior scan.
// The token must match a prior ScanAll call and is consumed (one-time use).
// If categoryIDs is empty, all categories from the scan are cleaned.
// Returns an events channel for progress and a done channel for the final result.
func (e *Engine) Cleanup(ctx context.Context, token ScanToken, categoryIDs []string) (<-chan CleanupEvent, <-chan CleanupDone) {
	events := make(chan CleanupEvent)
	done := make(chan CleanupDone, 1)

	go func() {
		defer close(events)
		defer close(done)

		results, err := e.validateToken(token)
		if err != nil {
			done <- CleanupDone{Err: err}
			return
		}

		// Filter by selected categories if specified.
		toClean := results
		if len(categoryIDs) > 0 {
			selected := make(map[string]bool, len(categoryIDs))
			for _, id := range categoryIDs {
				selected[id] = true
			}
			var filtered []scan.CategoryResult
			for _, cat := range results {
				if selected[cat.Category] {
					filtered = append(filtered, cat)
				}
			}
			toClean = filtered
		}

		progressFn := func(categoryDesc, entryPath string, current, total int) {
			var evtType string
			if entryPath == "" {
				evtType = EventCleanupCategoryStart
			} else {
				evtType = EventCleanupEntry
			}
			evt := CleanupEvent{
				Type:      evtType,
				Category:  categoryDesc,
				EntryPath: entryPath,
				Current:   current,
				Total:     total,
			}
			select {
			case events <- evt:
			case <-ctx.Done():
			}
		}

		result := cleanup.Execute(toClean, progressFn)
		done <- CleanupDone{Result: result}
	}()

	return events, done
}

// FilterSkipped removes categories matching the skip set from results.
// It returns the input unchanged if skip is empty.
func FilterSkipped(results []scan.CategoryResult, skip map[string]bool) []scan.CategoryResult {
	if len(skip) == 0 {
		return results
	}
	var filtered []scan.CategoryResult
	for _, cat := range results {
		if !skip[cat.Category] {
			filtered = append(filtered, cat)
		}
	}
	return filtered
}
