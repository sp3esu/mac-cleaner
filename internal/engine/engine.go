// Package engine provides scan and cleanup orchestration decoupled from the
// CLI layer. It is used by both the cobra CLI commands and the IPC server.
package engine

import (
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/pkg/appleftovers"
	"github.com/sp3esu/mac-cleaner/pkg/browser"
	"github.com/sp3esu/mac-cleaner/pkg/creative"
	"github.com/sp3esu/mac-cleaner/pkg/developer"
	"github.com/sp3esu/mac-cleaner/pkg/messaging"
	"github.com/sp3esu/mac-cleaner/pkg/system"
)

// Scanner defines a pluggable scanner group.
type Scanner struct {
	// ID is a machine-readable identifier (e.g. "system", "browser").
	ID string
	// Label is a human-readable name (e.g. "System Caches").
	Label string
	// ScanFn executes the scan and returns category results.
	ScanFn func() ([]scan.CategoryResult, error)
}

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

// ScanProgressFunc is called for each scan event.
type ScanProgressFunc func(ScanEvent)

// DefaultScanners returns all built-in scanner groups in execution order.
func DefaultScanners() []Scanner {
	return []Scanner{
		{ID: "system", Label: "System Caches", ScanFn: system.Scan},
		{ID: "browser", Label: "Browser Data", ScanFn: browser.Scan},
		{ID: "developer", Label: "Developer Caches", ScanFn: developer.Scan},
		{ID: "appleftovers", Label: "App Leftovers", ScanFn: appleftovers.Scan},
		{ID: "creative", Label: "Creative App Caches", ScanFn: creative.Scan},
		{ID: "messaging", Label: "Messaging App Caches", ScanFn: messaging.Scan},
	}
}

// ScanAll runs the given scanners sequentially and returns aggregated results.
// If onProgress is non-nil it is called before and after each scanner.
// Scanner errors are reported via progress events; partial results are still
// returned. The skip set filters category IDs from the final output.
func ScanAll(scanners []Scanner, skip map[string]bool, onProgress ScanProgressFunc) []scan.CategoryResult {
	var all []scan.CategoryResult

	for _, s := range scanners {
		if onProgress != nil {
			onProgress(ScanEvent{
				Type:      EventScannerStart,
				ScannerID: s.ID,
				Label:     s.Label,
			})
		}

		results, err := s.ScanFn()
		if err != nil {
			if onProgress != nil {
				onProgress(ScanEvent{
					Type:      EventScannerError,
					ScannerID: s.ID,
					Label:     s.Label,
					Err:       err,
				})
			}
			continue
		}

		if onProgress != nil {
			onProgress(ScanEvent{
				Type:      EventScannerDone,
				ScannerID: s.ID,
				Label:     s.Label,
				Results:   results,
			})
		}

		all = append(all, results...)
	}

	return FilterSkipped(all, skip)
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
