// Package scan provides shared types and utilities for filesystem scanning.
package scan

// ScanEntry represents a single scannable item on the filesystem.
type ScanEntry struct {
	// Path is the absolute filesystem path to the item.
	Path string
	// Description is a human-readable label for the item.
	Description string
	// Size is the total size in bytes.
	Size int64
}

// CategoryResult groups scan entries under a named category.
type CategoryResult struct {
	// Category is a machine-readable identifier (e.g. "system-caches").
	Category string
	// Description is a human-readable label (e.g. "User App Caches").
	Description string
	// Entries lists the individual scannable items in this category.
	Entries []ScanEntry
	// TotalSize is the sum of all entry sizes in bytes.
	TotalSize int64
}

// ScanSummary aggregates results from all scanned categories.
type ScanSummary struct {
	// Categories holds results for each scanned category.
	Categories []CategoryResult
	// TotalSize is the sum of all category sizes in bytes.
	TotalSize int64
}
