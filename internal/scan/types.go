// Package scan provides shared types and utilities for filesystem scanning.
package scan

// ScanEntry represents a single scannable item on the filesystem.
type ScanEntry struct {
	// Path is the absolute filesystem path to the item.
	Path string `json:"path"`
	// Description is a human-readable label for the item.
	Description string `json:"description"`
	// Size is the total size in bytes.
	Size int64 `json:"size"`
	// RiskLevel indicates the deletion risk (safe, moderate, risky).
	RiskLevel string `json:"risk_level"`
}

// PermissionIssue records a path that could not be scanned due to
// insufficient filesystem permissions.
type PermissionIssue struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

// CategoryResult groups scan entries under a named category.
type CategoryResult struct {
	// Category is a machine-readable identifier (e.g. "system-caches").
	Category string `json:"category"`
	// Description is a human-readable label (e.g. "User App Caches").
	Description string `json:"description"`
	// Entries lists the individual scannable items in this category.
	Entries []ScanEntry `json:"entries"`
	// TotalSize is the sum of all entry sizes in bytes.
	TotalSize int64 `json:"total_size"`
	// PermissionIssues records paths that could not be scanned.
	PermissionIssues []PermissionIssue `json:"permission_issues,omitempty"`
}

// SetRiskLevels applies a risk level to all entries in this category
// by calling riskFn with the category ID.
func (cr *CategoryResult) SetRiskLevels(riskFn func(string) string) {
	level := riskFn(cr.Category)
	for i := range cr.Entries {
		cr.Entries[i].RiskLevel = level
	}
}

// ScanSummary aggregates results from all scanned categories.
type ScanSummary struct {
	// Categories holds results for each scanned category.
	Categories []CategoryResult `json:"categories"`
	// TotalSize is the sum of all category sizes in bytes.
	TotalSize int64 `json:"total_size"`
	// PermissionIssues records paths that could not be scanned.
	PermissionIssues []PermissionIssue `json:"permission_issues,omitempty"`
}
