// Package engine provides scan and cleanup orchestration decoupled from the
// CLI layer. It is used by both the cobra CLI commands and the IPC server.
package engine

import (
	"sync"

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
