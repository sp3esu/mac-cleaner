package engine

import "fmt"

// ScanError wraps a scanner-level error with the scanner ID.
// It supports errors.As() for typed error handling by the server.
type ScanError struct {
	ScannerID string
	Err       error
}

func (e *ScanError) Error() string { return fmt.Sprintf("scanner %s: %v", e.ScannerID, e.Err) }
func (e *ScanError) Unwrap() error { return e.Err }

// CancelledError indicates the operation was cancelled via context.
type CancelledError struct {
	Operation string // "scan" or "cleanup"
}

func (e *CancelledError) Error() string { return fmt.Sprintf("%s cancelled", e.Operation) }

// TokenError indicates an invalid or expired scan token.
type TokenError struct {
	Token  ScanToken
	Reason string
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("invalid token %s: %s", e.Token, e.Reason)
}
