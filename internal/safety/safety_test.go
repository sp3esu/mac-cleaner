package safety

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestIsPathBlocked(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

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

		// Paths that are now blocked by home containment or critical-path check
		{name: "user Library Caches", path: home + "/Library/Caches", wantBlocked: false, wantReason: ""},
		{name: "Library Caches", path: "/Library/Caches", wantBlocked: true, wantReason: "outside home directory"},
		{name: "tmp", path: "/tmp", wantBlocked: true, wantReason: "outside home directory"},
		{name: "Applications", path: "/Applications", wantBlocked: true, wantReason: "critical system path"},
		{name: "private var folders", path: "/private/var/folders", wantBlocked: false, wantReason: ""},

		// Edge cases — path boundary, SIP prefix must NOT false-positive
		// (but these are still blocked by home containment)
		{name: "SystemVolume not System", path: "/SystemVolume", wantBlocked: true, wantReason: "outside home directory"},
		{name: "usrlocal not usr", path: "/usrlocal", wantBlocked: true, wantReason: "outside home directory"},
		{name: "sbinaries not sbin", path: "/sbinaries", wantBlocked: true, wantReason: "outside home directory"},
		{name: "binary not bin", path: "/binary", wantBlocked: true, wantReason: "outside home directory"},

		// Critical paths — exact match blocks
		{name: "root path", path: "/", wantBlocked: true, wantReason: "critical system path"},
		{name: "Users root", path: "/Users", wantBlocked: true, wantReason: "critical system path"},
		{name: "Library root", path: "/Library", wantBlocked: true, wantReason: "critical system path"},
		{name: "Applications root", path: "/Applications", wantBlocked: true, wantReason: "critical system path"},
		{name: "private root", path: "/private", wantBlocked: true, wantReason: "critical system path"},
		// /var and /etc are symlinks to /private/var and /private/etc on macOS,
		// so after symlink resolution they no longer match the critical path
		// exact list; they are blocked by home containment instead.
		{name: "var root", path: "/var", wantBlocked: true, wantReason: "outside home directory"},
		{name: "etc root", path: "/etc", wantBlocked: true, wantReason: "outside home directory"},
		{name: "Volumes root", path: "/Volumes", wantBlocked: true, wantReason: "critical system path"},
		{name: "opt root", path: "/opt", wantBlocked: true, wantReason: "critical system path"},
		{name: "cores root", path: "/cores", wantBlocked: true, wantReason: "critical system path"},

		// Home containment — paths outside home dir and /private/var/folders
		{name: "outside home var log", path: "/var/log", wantBlocked: true, wantReason: "outside home directory"},
		{name: "etc hosts", path: "/etc/hosts", wantBlocked: true, wantReason: "outside home directory"},

		// Critical paths as prefixes — not exact match, but still outside home
		// Note: /Applications/Safari.app is a symlink into /System on macOS,
		// so we use a non-existent app to test the home containment path.
		{name: "Applications subpath", path: "/Applications/SomeNonExistentApp.app", wantBlocked: true, wantReason: "outside home directory"},
		{name: "Library subpath", path: "/Library/Caches/something", wantBlocked: true, wantReason: "outside home directory"},

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
