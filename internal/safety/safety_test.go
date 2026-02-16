package safety

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestIsPathBlocked(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantBlocked bool
		wantReason  string
	}{
		// SIP-protected paths
		{name: "System root", path: "/System", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "System Library Caches", path: "/System/Library/Caches", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "System deep subpath", path: "/System/Library/Extensions/foo.kext", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "usr root", path: "/usr", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "usr bin", path: "/usr/bin", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "usr share", path: "/usr/share", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "bin root", path: "/bin", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "bin bash", path: "/bin/bash", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "sbin root", path: "/sbin", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "sbin mount", path: "/sbin/mount", wantBlocked: true, wantReason: "SIP-protected"},

		// Swap/VM paths
		{name: "private var vm", path: "/private/var/vm", wantBlocked: true, wantReason: "swap/VM file"},
		{name: "swapfile0", path: "/private/var/vm/swapfile0", wantBlocked: true, wantReason: "swap/VM file"},
		{name: "sleepimage", path: "/private/var/vm/sleepimage", wantBlocked: true, wantReason: "swap/VM file"},

		// SIP exceptions — /usr/local and subpaths are allowed
		{name: "usr local", path: "/usr/local", wantBlocked: false, wantReason: ""},
		{name: "usr local bin", path: "/usr/local/bin", wantBlocked: false, wantReason: ""},
		{name: "usr local Cellar", path: "/usr/local/Cellar", wantBlocked: false, wantReason: ""},

		// Safe paths — not under any blocked prefix
		{name: "user Library Caches", path: "/Users/test/Library/Caches", wantBlocked: false, wantReason: ""},
		{name: "Library Caches", path: "/Library/Caches", wantBlocked: false, wantReason: ""},
		{name: "tmp", path: "/tmp", wantBlocked: false, wantReason: ""},
		{name: "Applications", path: "/Applications", wantBlocked: false, wantReason: ""},
		{name: "private var folders", path: "/private/var/folders", wantBlocked: false, wantReason: ""},

		// Edge cases — path boundary, must NOT false-positive
		{name: "SystemVolume not System", path: "/SystemVolume", wantBlocked: false, wantReason: ""},
		{name: "usrlocal not usr", path: "/usrlocal", wantBlocked: false, wantReason: ""},
		{name: "sbinaries not sbin", path: "/sbinaries", wantBlocked: false, wantReason: ""},
		{name: "binary not bin", path: "/binary", wantBlocked: false, wantReason: ""},

		// Path traversal — caught by filepath.Clean
		{name: "traversal to System Library", path: "/System/../System/Library", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "traversal usr local to usr bin", path: "/usr/local/../../usr/bin", wantBlocked: true, wantReason: "SIP-protected"},

		// Trailing slash normalization
		{name: "System with trailing slash", path: "/System/", wantBlocked: true, wantReason: "SIP-protected"},
		{name: "usr with trailing slash", path: "/usr/", wantBlocked: true, wantReason: "SIP-protected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked, reason := IsPathBlocked(tt.path)
			if blocked != tt.wantBlocked {
				t.Errorf("IsPathBlocked(%q) blocked = %v, want %v", tt.path, blocked, tt.wantBlocked)
			}
			if reason != tt.wantReason {
				t.Errorf("IsPathBlocked(%q) reason = %q, want %q", tt.path, reason, tt.wantReason)
			}
		})
	}
}

func TestWarnBlocked(t *testing.T) {
	// Capture stderr output
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	WarnBlocked("/System", "SIP-protected")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("failed to read from pipe: %v", err)
	}

	got := buf.String()
	want := "SKIP: /System (SIP-protected)\n"
	if got != want {
		t.Errorf("WarnBlocked output = %q, want %q", got, want)
	}
}

func TestWarnBlockedFormat(t *testing.T) {
	tests := []struct {
		path   string
		reason string
		want   string
	}{
		{"/System", "SIP-protected", "SKIP: /System (SIP-protected)\n"},
		{"/private/var/vm", "swap/VM file", "SKIP: /private/var/vm (swap/VM file)\n"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.path, tt.reason), func(t *testing.T) {
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			os.Stderr = w

			WarnBlocked(tt.path, tt.reason)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			_, err = buf.ReadFrom(r)
			if err != nil {
				t.Fatalf("failed to read from pipe: %v", err)
			}

			if got := buf.String(); got != tt.want {
				t.Errorf("WarnBlocked(%q, %q) = %q, want %q", tt.path, tt.reason, got, tt.want)
			}
		})
	}
}

func TestPathHasPrefix(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   bool
	}{
		{"/System/Library", "/System", true},
		{"/System", "/System", true},
		{"/SystemVolume", "/System", false},
		{"/usr/bin", "/usr", true},
		{"/usrlocal", "/usr", false},
		{"/bin/bash", "/bin", true},
		{"/binary", "/bin", false},
		{"/sbin/mount", "/sbin", true},
		{"/sbinaries", "/sbin", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_under_%s", tt.path, tt.prefix), func(t *testing.T) {
			if got := pathHasPrefix(tt.path, tt.prefix); got != tt.want {
				t.Errorf("pathHasPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
			}
		})
	}
}
