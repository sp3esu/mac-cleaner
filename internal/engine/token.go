package engine

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// ScanToken is an opaque identifier linking a cleanup to a prior scan.
type ScanToken string

// tokenEntry stores scan results for a single token.
type tokenEntry struct {
	results []scan.CategoryResult
	created time.Time
}

// storeResults saves results under a new token, invalidating any previous
// token (single-token store policy). Returns the new token.
func (e *Engine) storeResults(results []scan.CategoryResult) ScanToken {
	b := make([]byte, 16)
	// crypto/rand.Read never returns an error for small reads on supported platforms.
	_, _ = rand.Read(b)
	token := ScanToken(hex.EncodeToString(b))

	e.mu.Lock()
	e.lastToken.token = token
	e.lastToken.entry = &tokenEntry{
		results: results,
		created: time.Now(),
	}
	e.mu.Unlock()

	return token
}

// validateToken checks that the given token matches the stored token.
// If valid, returns a copy of the stored results and clears the token
// (one-time use / replay protection). If invalid, returns a TokenError.
func (e *Engine) validateToken(token ScanToken) ([]scan.CategoryResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.lastToken.entry == nil || e.lastToken.token != token {
		return nil, &TokenError{Token: token, Reason: "unknown or expired"}
	}

	// Copy results to prevent caller from mutating the stored slice.
	src := e.lastToken.entry.results
	results := make([]scan.CategoryResult, len(src))
	copy(results, src)

	// Clear the token (consumed).
	e.lastToken.token = ""
	e.lastToken.entry = nil

	return results, nil
}
