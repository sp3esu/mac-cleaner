// Package safety provides path validation to prevent the tool from
// modifying SIP-protected system paths and swap/VM files on macOS.
// All protections are hardcoded and cannot be overridden by configuration.
package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sipProtectedPrefixes lists path prefixes protected by System Integrity
// Protection. Any path equal to or under these prefixes is blocked, unless
// it falls under a sipException.
var sipProtectedPrefixes = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
}

// sipExceptions lists paths under SIP-protected prefixes that are safe to
// access (e.g. /usr/local is user-writable and not SIP-protected).
var sipExceptions = []string{
	"/usr/local",
}

// swapProtectedPrefixes lists path prefixes for swap and virtual memory
// files that must never be touched.
var swapProtectedPrefixes = []string{
	"/private/var/vm",
}

// IsPathBlocked checks whether a filesystem path is protected and should
// not be modified. It returns whether the path is blocked and the reason.
// Paths are normalized with filepath.Clean and resolved with
// filepath.EvalSymlinks before checking against the blocklist.
func IsPathBlocked(path string) (bool, string) {
	cleaned := filepath.Clean(path)

	// Attempt symlink resolution for additional safety.
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		if !os.IsNotExist(err) {
			// Path exists but cannot be resolved — block for safety.
			return true, fmt.Sprintf("cannot resolve path: %v", err)
		}
		// Path does not exist; check the literal cleaned path.
		resolved = cleaned
	}
	resolved = filepath.Clean(resolved)

	// Check swap/VM prefixes first (no exceptions, simplest check).
	for _, prefix := range swapProtectedPrefixes {
		if pathHasPrefix(resolved, prefix) {
			return true, "swap/VM file"
		}
	}

	// Check SIP-protected prefixes, but allow exceptions.
	for _, prefix := range sipProtectedPrefixes {
		if pathHasPrefix(resolved, prefix) {
			// Check whether this path falls under an exception.
			for _, exc := range sipExceptions {
				if pathHasPrefix(resolved, exc) {
					return false, ""
				}
			}
			return true, "SIP-protected"
		}
	}

	return false, ""
}

// WarnBlocked prints a skip warning to stderr for a blocked path.
// Format: SKIP: {path} ({reason})
func WarnBlocked(path, reason string) {
	fmt.Fprintf(os.Stderr, "SKIP: %s (%s)\n", path, reason)
}

// pathHasPrefix reports whether path is equal to prefix or is a child
// of prefix (i.e. prefix followed by a path separator). This avoids
// false positives like /SystemVolume matching /System.
func pathHasPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator))
}
