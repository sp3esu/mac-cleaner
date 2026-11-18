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
	// Components optionally breaks Size down into the individual paths (or
	// sources) it was summed from. It is always set when Size covers anything
	// other than exactly Path, or when cleanup would not delete Path; some
	// scanners (unused apps) set it on every entry. When set, the component
	// sizes sum to Size. When empty, Size is the size of Path alone and
	// cleanup removes Path.
	Components []SizeComponent `json:"components,omitempty"`
}

// SizeComponent is one path (or non-filesystem source) that contributes to
// a ScanEntry's Size.
type SizeComponent struct {
	// Path is the filesystem path (or pseudo-path such as "docker:Images")
	// this component was measured from.
	Path string `json:"path"`
	// Size is this component's contribution to the entry's Size, in bytes.
	Size int64 `json:"size"`
	// Removed reports whether cleanup deletes this path.
	Removed bool `json:"removed"`
	// Note optionally explains how the size was obtained or why cleanup
	// does not remove the path.
	Note string `json:"note,omitempty"`
}

// RemovedSize returns the number of bytes of this entry that cleanup
// deletes: the sum of removed components, or Size when no breakdown is set.
func (e ScanEntry) RemovedSize() int64 {
	if len(e.Components) == 0 {
		return e.Size
	}
	var n int64
	for _, c := range e.Components {
		if c.Removed {
			n += c.Size
		}
	}
	return n
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
	// TotalSize is the sum of all entry sizes in bytes: what was counted,
	// which can include data cleanup leaves in place and estimates.
	TotalSize int64 `json:"total_size"`
	// RemovableSize is the number of bytes cleanup deletes for this
	// category: the sum of each entry's RemovedSize (pseudo-path entries
	// such as docker: or git-gc: delete no files and add 0). Set by
	// Summarize; paths shared with other categories are counted here too.
	RemovableSize int64 `json:"removable_size"`
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
	// TotalSize is the sum of all category sizes in bytes: what was
	// counted, with paths counted by more than one entry counted each time.
	TotalSize int64 `json:"total_size"`
	// UniqueSize is TotalSize with every path counted once.
	UniqueSize int64 `json:"unique_size"`
	// RemovableSize is the number of bytes cleanup deletes, with every
	// path counted once (see Reclaimable).
	RemovableSize int64 `json:"removable_size"`
	// PermissionIssues records paths that could not be scanned.
	PermissionIssues []PermissionIssue `json:"permission_issues,omitempty"`
}

// Summarize aggregates results into a ScanSummary. It sets each category's
// RemovableSize (the sum of its entries' RemovedSize, counting only real
// filesystem paths) on a copy of results,
// and fills the summary's TotalSize (sum of category TotalSize), UniqueSize
// (TotalSize minus bytes counted more than once), RemovableSize (Reclaimable,
// de-duplicated) and PermissionIssues. The input slice is not modified.
func Summarize(results []CategoryResult) ScanSummary {
	cats := make([]CategoryResult, len(results))
	copy(cats, results)

	var s ScanSummary
	for i := range cats {
		var removable int64
		for _, e := range cats[i].Entries {
			removable += removedPathSize(e)
		}
		cats[i].RemovableSize = removable
		s.TotalSize += cats[i].TotalSize
		s.PermissionIssues = append(s.PermissionIssues, cats[i].PermissionIssues...)
	}
	_, dupCounted, _ := FindOverlaps(cats)
	s.Categories = cats
	s.UniqueSize = s.TotalSize - dupCounted
	s.RemovableSize = Reclaimable(cats)
	return s
}
