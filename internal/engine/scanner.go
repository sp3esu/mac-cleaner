// Package engine provides scan and cleanup orchestration decoupled from the
// CLI layer. It is used by both the cobra CLI commands and the IPC server.
package engine

import "github.com/sp3esu/mac-cleaner/internal/scan"

// SubCategoryInfo holds metadata about a single sub-category within a
// scanner group. It is used by the server's "categories" response to
// provide rich per-category data to clients.
type SubCategoryInfo struct {
	// ID is the category identifier (e.g. "system-caches", "browser-chrome").
	ID string
	// Label is a human-readable display name (e.g. "User App Caches").
	Label string
	// SkipFlag is the individual skip flag name (e.g. "chrome").
	// Empty string if no individual skip flag exists.
	SkipFlag string
	// RiskLevel is the deletion risk ("safe", "moderate", "risky").
	RiskLevel string
}

// ScannerInfo holds metadata about a scanner group. It provides the
// information needed by the server's "categories" method without extra
// mapping.
type ScannerInfo struct {
	// ID is a machine-readable identifier (e.g. "system", "browser").
	ID string
	// Name is a human-readable label (e.g. "System Caches").
	Name string
	// Description explains what this scanner group covers.
	Description string
	// CategoryIDs lists the category identifiers this scanner can produce.
	CategoryIDs []string
	// SkipFlag is the group-level skip flag name (e.g. "system-caches").
	SkipFlag string
	// SubCategories provides rich metadata for each sub-category.
	SubCategories []SubCategoryInfo
	// RiskLevel is the dominant risk level for the group (may be empty
	// when risk is per-category rather than per-group).
	RiskLevel string
}

// Scanner is the interface all scanners implement. It provides both
// scan execution and metadata access.
type Scanner interface {
	// Scan executes the scan and returns category results.
	Scan() ([]scan.CategoryResult, error)
	// Info returns metadata about this scanner.
	Info() ScannerInfo
}

// scannerAdapter wraps a bare Scan function into the Scanner interface.
type scannerAdapter struct {
	info   ScannerInfo
	scanFn func() ([]scan.CategoryResult, error)
}

func (a *scannerAdapter) Scan() ([]scan.CategoryResult, error) { return a.scanFn() }
func (a *scannerAdapter) Info() ScannerInfo                     { return a.info }

// NewScanner creates a Scanner from metadata and a scan function.
// This adapter pattern wraps existing pkg/*/Scan() functions without
// modifying their signatures.
func NewScanner(info ScannerInfo, fn func() ([]scan.CategoryResult, error)) Scanner {
	return &scannerAdapter{info: info, scanFn: fn}
}
