package spinner

import (
	"os"
	"testing"
)

// realStderrIsTerminal keeps the production check before TestMain overrides it.
var realStderrIsTerminal = stderrIsTerminal

// TestMain treats stderr as a terminal so enabled-spinner tests behave the
// same whether or not `go test` output is redirected.
func TestMain(m *testing.M) {
	stderrIsTerminal = func() bool { return true }
	os.Exit(m.Run())
}

func TestFramesNonEmpty(t *testing.T) {
	if len(frames) == 0 {
		t.Fatal("frames must not be empty")
	}
}

func TestEnabledCreatesInner(t *testing.T) {
	s := New("Testing...", true)
	if !s.enabled {
		t.Fatal("enabled spinner should have enabled=true")
	}
	if s.inner == nil {
		t.Fatal("enabled spinner should have non-nil inner")
	}
}

func TestEnabledMethodsDoNotPanic(t *testing.T) {
	s := New("Scanning...", true)

	// Start/Stop/UpdateMessage must not panic even in non-terminal env.
	s.Start()
	s.UpdateMessage("Updated...")
	s.Stop()
}

func TestEnabledUpdateMessage(t *testing.T) {
	s := New("Initial...", true)
	s.UpdateMessage("Updated...")
	if s.inner.Suffix != " Updated..." {
		t.Fatalf("expected suffix %q, got %q", " Updated...", s.inner.Suffix)
	}
}

func TestEnabledInitialSuffix(t *testing.T) {
	s := New("Scanning system...", true)
	if s.inner.Suffix != " Scanning system..." {
		t.Fatalf("expected suffix %q, got %q", " Scanning system...", s.inner.Suffix)
	}
}

func TestDisabled(t *testing.T) {
	s := New("Testing...", false)

	if s.enabled {
		t.Fatal("disabled spinner should have enabled=false")
	}
	if s.inner != nil {
		t.Fatal("disabled spinner should have nil inner")
	}
	if s.Active() {
		t.Fatal("disabled spinner should never be active")
	}

	// All methods should be safe no-ops.
	s.Start()
	s.UpdateMessage("Updated...")
	s.Stop()

	if s.Active() {
		t.Fatal("disabled spinner should never be active after Start")
	}
}

func TestNonTerminalDisables(t *testing.T) {
	orig := stderrIsTerminal
	defer func() { stderrIsTerminal = orig }()
	stderrIsTerminal = func() bool { return false }

	s := New("Scanning...", true)
	if s.enabled || s.inner != nil {
		t.Fatal("spinner should be disabled when stderr is not a terminal")
	}
}

func TestStderrIsTerminalDoesNotPanic(t *testing.T) {
	_ = realStderrIsTerminal()
}
