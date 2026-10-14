// Package schedule manages a per-user launchd LaunchAgent that runs
// mac-cleaner cleanups on a recurring calendar schedule. It renders and
// parses the agent's property list and drives launchctl to load, unload,
// inspect, and trigger the job.
package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// Label is the launchd job label and the plist file's base name.
const Label = "com.mac-cleaner"

// Hidden scan flags appended to the scheduled command line.
const (
	// FlagScheduled makes the scan command print a timestamped log header.
	FlagScheduled = "--scheduled"
	// FlagSkipIfXcodeRunning makes the scan command exit early while Xcode is open.
	FlagSkipIfXcodeRunning = "--skip-if-xcode-running"
)

// commandTimeout bounds every external command (launchctl, plutil, pgrep).
const commandTimeout = 30 * time.Second

// targetPattern restricts target names to scan flag syntax (e.g. "derived-data").
var targetPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Schedule describes a scheduled cleanup job.
type Schedule struct {
	// Hour and Minute are the local time of day the job runs.
	Hour, Minute int
	// Weekdays limits the job to specific days. Empty means every day.
	Weekdays []time.Weekday
	// Targets are scan item flag names without the "--" prefix (e.g. "derived-data").
	Targets []string
	// Binary is the absolute path of the mac-cleaner executable to run.
	Binary string
	// LogPath receives the job's stdout and stderr.
	LogPath string
	// EvenIfXcodeRunning disables the "skip while Xcode is open" guard.
	EvenIfXcodeRunning bool
}

// Runner executes an external command and returns its combined output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner runs the command via exec.CommandContext (no shell).
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() // #nosec G204 -- fixed binaries (launchctl, plutil, pgrep) with validated args
}

// PlistPath returns the LaunchAgent plist location for the given home directory.
func PlistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist")
}

// LogPath returns the scheduled-run log location for the given home directory.
func LogPath(home string) string {
	return filepath.Join(home, "Library", "Logs", "mac-cleaner", "scheduled.log")
}

// ParseTime parses a 24-hour "HH:MM" time of day.
func ParseTime(s string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time %q (expected HH:MM, 24-hour)", s)
	}
	return t.Hour(), t.Minute(), nil
}

var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseWeekdays parses a comma-separated list of three-letter weekday names
// (e.g. "mon,wed,fri"). An empty string means every day and returns nil.
// Duplicates are removed and the result is ordered Sunday..Saturday.
func ParseWeekdays(s string) ([]time.Weekday, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	seen := map[time.Weekday]bool{}
	for _, part := range strings.Split(s, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		d, ok := weekdayNames[name]
		if !ok {
			return nil, fmt.Errorf("invalid weekday %q (use sun, mon, tue, wed, thu, fri, sat)", part)
		}
		seen[d] = true
	}
	var days []time.Weekday
	for d := time.Sunday; d <= time.Saturday; d++ {
		if seen[d] {
			days = append(days, d)
		}
	}
	return days, nil
}

// Validate checks that the schedule is well-formed.
func (s Schedule) Validate() error {
	if s.Hour < 0 || s.Hour > 23 || s.Minute < 0 || s.Minute > 59 {
		return fmt.Errorf("invalid time %02d:%02d", s.Hour, s.Minute)
	}
	for _, d := range s.Weekdays {
		if d < time.Sunday || d > time.Saturday {
			return fmt.Errorf("invalid weekday %d", d)
		}
	}
	if len(s.Targets) == 0 {
		return errors.New("at least one target is required")
	}
	for _, t := range s.Targets {
		if !targetPattern.MatchString(t) {
			return fmt.Errorf("invalid target %q", t)
		}
	}
	if !filepath.IsAbs(s.Binary) {
		return fmt.Errorf("binary path must be absolute: %q", s.Binary)
	}
	if !filepath.IsAbs(s.LogPath) {
		return fmt.Errorf("log path must be absolute: %q", s.LogPath)
	}
	return nil
}

// ProgramArguments returns the command line launchd runs for this schedule.
func (s Schedule) ProgramArguments() []string {
	args := []string{s.Binary, "scan"}
	for _, t := range s.Targets {
		args = append(args, "--"+t)
	}
	args = append(args, "--force", FlagScheduled)
	if !s.EvenIfXcodeRunning {
		args = append(args, FlagSkipIfXcodeRunning)
	}
	return args
}

// Describe returns a human-readable summary such as "daily at 03:00"
// or "Mon, Fri at 22:30".
func (s Schedule) Describe() string {
	at := fmt.Sprintf("%02d:%02d", s.Hour, s.Minute)
	if len(s.Weekdays) == 0 {
		return "daily at " + at
	}
	names := make([]string, len(s.Weekdays))
	for i, d := range s.Weekdays {
		names[i] = d.String()[:3]
	}
	return strings.Join(names, ", ") + " at " + at
}

// NextRun returns the first scheduled time strictly after now.
func (s Schedule) NextRun(now time.Time) time.Time {
	for i := 0; i <= 7; i++ {
		day := now.AddDate(0, 0, i)
		t := time.Date(day.Year(), day.Month(), day.Day(), s.Hour, s.Minute, 0, 0, now.Location())
		if !t.After(now) || !s.runsOn(t.Weekday()) {
			continue
		}
		return t
	}
	return time.Time{} // unreachable for a valid schedule
}

func (s Schedule) runsOn(d time.Weekday) bool {
	if len(s.Weekdays) == 0 {
		return true
	}
	for _, w := range s.Weekdays {
		if w == d {
			return true
		}
	}
	return false
}

var plistTemplate = template.Must(template.New("plist").Funcs(template.FuncMap{"x": xmlEscape}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{x .Label}}</string>
	<key>ProgramArguments</key>
	<array>
{{- range .Args}}
		<string>{{x .}}</string>
{{- end}}
	</array>
	<key>StartCalendarInterval</key>
	<array>
{{- range .Intervals}}
		<dict>
{{- if ge .Weekday 0}}
			<key>Weekday</key>
			<integer>{{.Weekday}}</integer>
{{- end}}
			<key>Hour</key>
			<integer>{{.Hour}}</integer>
			<key>Minute</key>
			<integer>{{.Minute}}</integer>
		</dict>
{{- end}}
	</array>
	<key>StandardOutPath</key>
	<string>{{x .LogPath}}</string>
	<key>StandardErrorPath</key>
	<string>{{x .LogPath}}</string>
	<key>ProcessType</key>
	<string>Background</string>
	<key>LowPriorityIO</key>
	<true/>
	<key>Nice</key>
	<integer>10</integer>
</dict>
</plist>
`))

type interval struct{ Weekday, Hour, Minute int }

func xmlEscape(s string) (string, error) {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return "", err
	}
	return b.String(), nil
}

// RenderPlist renders the LaunchAgent property list for the schedule.
func RenderPlist(s Schedule) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	var intervals []interval
	if len(s.Weekdays) == 0 {
		intervals = []interval{{Weekday: -1, Hour: s.Hour, Minute: s.Minute}}
	}
	for _, d := range s.Weekdays {
		intervals = append(intervals, interval{Weekday: int(d), Hour: s.Hour, Minute: s.Minute})
	}
	data := struct {
		Label     string
		Args      []string
		Intervals []interval
		LogPath   string
	}{Label, s.ProgramArguments(), intervals, s.LogPath}

	var b bytes.Buffer
	if err := plistTemplate.Execute(&b, data); err != nil {
		return nil, fmt.Errorf("render plist: %w", err)
	}
	return b.Bytes(), nil
}

// plistJSON is the subset of the agent plist read back by ParsePlistJSON.
type plistJSON struct {
	ProgramArguments      []string        `json:"ProgramArguments"`
	StandardOutPath       string          `json:"StandardOutPath"`
	StartCalendarInterval json.RawMessage `json:"StartCalendarInterval"`
}

type calendarInterval struct {
	Weekday *int `json:"Weekday"`
	Hour    int  `json:"Hour"`
	Minute  int  `json:"Minute"`
}

// ParsePlistJSON reconstructs a Schedule from the agent plist converted to
// JSON (plutil -convert json). It accepts StartCalendarInterval as a single
// dict or an array of dicts, and Weekday 7 as Sunday, as launchd does.
func ParsePlistJSON(data []byte) (Schedule, error) {
	var p plistJSON
	if err := json.Unmarshal(data, &p); err != nil {
		return Schedule{}, fmt.Errorf("parse plist: %w", err)
	}
	args := p.ProgramArguments
	if len(args) < 2 || args[1] != "scan" {
		return Schedule{}, errors.New("parse plist: unexpected ProgramArguments")
	}

	s := Schedule{Binary: args[0], LogPath: p.StandardOutPath, EvenIfXcodeRunning: true}
	for _, a := range args[2:] {
		switch a {
		case "--force", FlagScheduled:
		case FlagSkipIfXcodeRunning:
			s.EvenIfXcodeRunning = false
		default:
			s.Targets = append(s.Targets, strings.TrimPrefix(a, "--"))
		}
	}

	var intervals []calendarInterval
	if err := json.Unmarshal(p.StartCalendarInterval, &intervals); err != nil {
		var single calendarInterval
		if err := json.Unmarshal(p.StartCalendarInterval, &single); err != nil {
			return Schedule{}, errors.New("parse plist: missing or invalid StartCalendarInterval")
		}
		intervals = []calendarInterval{single}
	}
	if len(intervals) == 0 {
		return Schedule{}, errors.New("parse plist: empty StartCalendarInterval")
	}
	s.Hour, s.Minute = intervals[0].Hour, intervals[0].Minute
	for _, iv := range intervals {
		if iv.Weekday != nil {
			s.Weekdays = append(s.Weekdays, time.Weekday(*iv.Weekday%7))
		}
	}
	return s, nil
}

// Manager installs and controls the LaunchAgent for one user.
type Manager struct {
	// Home is the user's home directory.
	Home string
	// UID is the user's numeric ID, used for the gui/<uid> launchd domain.
	UID int
	// Run executes external commands. Nil uses exec.CommandContext.
	Run Runner
}

func (m Manager) run(name string, args ...string) ([]byte, error) {
	r := m.Run
	if r == nil {
		r = defaultRunner
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	return r(ctx, name, args...)
}

func (m Manager) domain() string { return "gui/" + strconv.Itoa(m.UID) }

func (m Manager) service() string { return m.domain() + "/" + Label }

// Install writes the plist and (re)loads the job, replacing any existing one.
func (m Manager) Install(s Schedule) error {
	data, err := RenderPlist(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.LogPath), 0o750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	path := PlistPath(m.Home)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	m.unload()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if out, err := m.run("launchctl", "bootstrap", m.domain(), path); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// unload removes the job from launchd. Errors are ignored because the job
// is commonly not loaded at all.
func (m Manager) unload() {
	_, _ = m.run("launchctl", "bootout", m.service())
}

// Uninstall unloads the job and deletes the plist. It reports whether a
// plist was present.
func (m Manager) Uninstall() (bool, error) {
	m.unload()
	err := os.Remove(PlistPath(m.Home))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("remove plist: %w", err)
	}
	return true, nil
}

// Status describes the installed job.
type Status struct {
	// Installed is true when the plist exists.
	Installed bool
	// Loaded is true when launchd knows about the job.
	Loaded bool
	// Schedule is the parsed plist (valid only when Installed).
	Schedule Schedule
	// LastExitCode is launchd's last exit status, or "" if unknown / never run.
	LastExitCode string
}

var lastExitPattern = regexp.MustCompile(`(?m)^\s*last exit code = (.+)$`)

// Status reads the plist and queries launchd for the job's state.
func (m Manager) Status() (Status, error) {
	path := PlistPath(m.Home)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return Status{}, nil
	}
	data, err := m.run("plutil", "-convert", "json", "-o", "-", path)
	if err != nil {
		return Status{}, fmt.Errorf("read plist: %w: %s", err, strings.TrimSpace(string(data)))
	}
	s, err := ParsePlistJSON(data)
	if err != nil {
		return Status{}, err
	}
	st := Status{Installed: true, Schedule: s}
	out, err := m.run("launchctl", "print", m.service())
	if err == nil {
		st.Loaded = true
		if match := lastExitPattern.FindSubmatch(out); match != nil {
			st.LastExitCode = strings.TrimSpace(string(match[1]))
		}
	}
	return st, nil
}

// RunNow asks launchd to start the job immediately.
func (m Manager) RunNow() error {
	if out, err := m.run("launchctl", "kickstart", m.service()); err != nil {
		return fmt.Errorf("launchctl kickstart: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// XcodeRunning reports whether an Xcode process is running. It is a
// package-level variable for test injection.
var XcodeRunning = func() bool {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	// pgrep exits 0 when at least one process matches.
	_, err := defaultRunner(ctx, "pgrep", "-x", "Xcode")
	return err == nil
}

// ResolveBinary picks the executable path to store in the plist. Homebrew
// installs live under a versioned Cellar directory that disappears on
// upgrade, so when exe is inside a Cellar the stable PATH entry (e.g.
// /opt/homebrew/bin/mac-cleaner) is preferred. The returned warning is
// non-empty when only an unstable path is available.
func ResolveBinary(exe string, lookPath func(string) (string, error)) (path, warning string) {
	if !strings.Contains(exe, "/Cellar/") {
		return exe, ""
	}
	if lp, err := lookPath("mac-cleaner"); err == nil && filepath.IsAbs(lp) && !strings.Contains(lp, "/Cellar/") {
		return lp, ""
	}
	return exe, fmt.Sprintf("binary path %s is version-specific; re-run 'mac-cleaner schedule install' after upgrading", exe)
}

// PrintHeader writes the timestamped header used to separate scheduled runs in the log.
func PrintHeader(w io.Writer, now time.Time) {
	fmt.Fprintf(w, "=== mac-cleaner scheduled run %s ===\n", now.Format(time.RFC3339))
}
