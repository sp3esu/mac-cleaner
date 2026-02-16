package safety

import (
	"path/filepath"
	"strings"
	"testing"
)

func FuzzIsPathBlocked(f *testing.F) {
	// Seed corpus with representative paths.
	f.Add("/System/Library")
	f.Add("/usr/local/bin")
	f.Add("/usr/bin")
	f.Add("/../../../System")
	f.Add("/private/var/vm/swapfile0")
	f.Add("/")
	f.Add("/Users")
	f.Add("/Applications")
	f.Add("")
	f.Add("/Users/test/Library/Caches")

	f.Fuzz(func(t *testing.T, path string) {
		blocked, reason := IsPathBlocked(path)

		// Invariant: resolved SIP paths must always be blocked.
		cleaned := filepath.Clean(path)
		if strings.HasPrefix(cleaned, "/System/") || cleaned == "/System" {
			if !blocked {
				t.Errorf("path %q resolved to SIP area but was not blocked (reason: %q)", path, reason)
			}
		}

		// Invariant: swap paths must always be blocked.
		if strings.HasPrefix(cleaned, "/private/var/vm/") || cleaned == "/private/var/vm" {
			if !blocked {
				t.Errorf("path %q resolved to swap area but was not blocked (reason: %q)", path, reason)
			}
		}

		// Invariant: root must always be blocked.
		if cleaned == "/" {
			if !blocked {
				t.Errorf("root path %q was not blocked (reason: %q)", path, reason)
			}
		}

		// Invariant: if blocked, reason must not be empty.
		if blocked && reason == "" {
			t.Errorf("path %q is blocked but has empty reason", path)
		}

		// Invariant: if not blocked, reason must be empty.
		if !blocked && reason != "" {
			t.Errorf("path %q is not blocked but has reason %q", path, reason)
		}
	})
}
