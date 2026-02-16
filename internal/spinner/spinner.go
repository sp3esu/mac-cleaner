// Package spinner provides a themed CLI spinner for visual feedback during scans.
package spinner

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
)

// broomFrames is a custom broom-sweep animation.
var broomFrames = []string{
	"🧹",
	"🧹 ✨",
	"🧹 ✨✨",
	"🧹 ✨✨✨",
	"🧹 ✨✨",
	"🧹 ✨",
}

// Spinner wraps briandowns/spinner with an enable/disable toggle.
// When disabled, all methods are safe no-ops.
type Spinner struct {
	inner   *spinner.Spinner
	enabled bool
}

// New creates a spinner writing to stderr. When enabled is false, all methods
// are no-ops so JSON output is never corrupted.
func New(message string, enabled bool) *Spinner {
	if !enabled {
		return &Spinner{enabled: false}
	}
	s := spinner.New(broomFrames, 120*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = " " + message
	return &Spinner{inner: s, enabled: true}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	if !s.enabled {
		return
	}
	s.inner.Start()
}

// Stop halts the spinner animation and clears the line.
func (s *Spinner) Stop() {
	if !s.enabled {
		return
	}
	s.inner.Stop()
}

// UpdateMessage changes the spinner suffix text.
func (s *Spinner) UpdateMessage(msg string) {
	if !s.enabled {
		return
	}
	s.inner.Suffix = " " + msg
}

// Active returns whether the spinner is currently animating.
func (s *Spinner) Active() bool {
	if !s.enabled {
		return false
	}
	return s.inner.Active()
}
