package schedule

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func validSchedule() Schedule {
	return Schedule{
		Hour:    3,
		Minute:  0,
		Targets: []string{"derived-data"},
		Binary:  "/opt/homebrew/bin/mac-cleaner",
		LogPath: "/Users/test/Library/Logs/mac-cleaner/scheduled.log",
	}
}

// plistToJSON converts plist XML to JSON with plutil, skipping the test
// when plutil is unavailable.
func plistToJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	plutil, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil not available")
	}
	path := filepath.Join(t.TempDir(), "in.plist")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(plutil, "-convert", "json", "-o", "-", path).Output() // #nosec G204 -- test-only
	if err != nil {
		t.Fatalf("plutil -convert json: %v", err)
	}
	return out
}

// --- parsing ---

func TestParseTime(t *testing.T) {
	cases := []struct {
		in        string
		h, m      int
		wantError bool
	}{
		{"03:00", 3, 0, false},
		{"23:59", 23, 59, false},
		{"00:00", 0, 0, false},
		{"3:05", 3, 5, false},
		{"24:00", 0, 0, true},
		{"25:00", 0, 0, true},
		{"12:60", 0, 0, true},
		{"3pm", 0, 0, true},
		{"", 0, 0, true},
	}
	for _, c := range cases {
		h, m, err := ParseTime(c.in)
		if c.wantError {
			if err == nil {
				t.Errorf("ParseTime(%q): expected error", c.in)
			}
			continue
		}
		if err != nil || h != c.h || m != c.m {
			t.Errorf("ParseTime(%q) = %d, %d, %v; want %d, %d", c.in, h, m, err, c.h, c.m)
		}
	}
}

func TestParseWeekdays(t *testing.T) {
	got, err := ParseWeekdays("fri, Mon,mon,sun")
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Weekday{time.Sunday, time.Monday, time.Friday}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, err := ParseWeekdays(""); err != nil || got != nil {
		t.Errorf("empty: got %v, %v; want nil, nil", got, err)
	}
	if _, err := ParseWeekdays("mon,funday"); err == nil {
		t.Error("expected error for unknown weekday")
	}
}

// --- validation ---

func TestValidate(t *testing.T) {
	if err := validSchedule().Validate(); err != nil {
		t.Fatalf("valid schedule rejected: %v", err)
	}

	cases := map[string]func(*Schedule){
		"hour too big":      func(s *Schedule) { s.Hour = 24 },
		"negative minute":   func(s *Schedule) { s.Minute = -1 },
		"bad weekday":       func(s *Schedule) { s.Weekdays = []time.Weekday{9} },
		"no targets":        func(s *Schedule) { s.Targets = nil },
		"target with space": func(s *Schedule) { s.Targets = []string{"derived data"} },
		"target with xml":   func(s *Schedule) { s.Targets = []string{"</string>"} },
		"target with dash":  func(s *Schedule) { s.Targets = []string{"--force"} },
		"relative binary":   func(s *Schedule) { s.Binary = "mac-cleaner" },
		"relative log":      func(s *Schedule) { s.LogPath = "scheduled.log" },
	}
	for name, mutate := range cases {
		s := validSchedule()
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
		if _, err := RenderPlist(s); err == nil {
			t.Errorf("%s: RenderPlist should reject invalid schedule", name)
		}
	}
}

// --- program arguments / description ---

func TestProgramArguments(t *testing.T) {
	s := validSchedule()
	s.Targets = []string{"derived-data", "simulator-caches"}
	want := []string{s.Binary, "scan", "--derived-data", "--simulator-caches", "--force", FlagScheduled, FlagSkipIfXcodeRunning}
	if got := s.ProgramArguments(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	s.EvenIfXcodeRunning = true
	for _, a := range s.ProgramArguments() {
		if a == FlagSkipIfXcodeRunning {
			t.Error("EvenIfXcodeRunning should drop the Xcode guard flag")
		}
	}
}

func TestDescribe(t *testing.T) {
	s := validSchedule()
	if got := s.Describe(); got != "daily at 03:00" {
		t.Errorf("got %q", got)
	}
	s.Hour, s.Minute = 22, 5
	s.Weekdays = []time.Weekday{time.Monday, time.Friday}
	if got := s.Describe(); got != "Mon, Fri at 22:05" {
		t.Errorf("got %q", got)
	}
}

func TestNextRun(t *testing.T) {
	loc := time.UTC
	// 2026-10-13 is a Tuesday.
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, loc)

	s := validSchedule() // daily 03:00
	if got, want := s.NextRun(now), time.Date(2026, 10, 14, 3, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("daily: got %v, want %v", got, want)
	}

	s.Hour = 11 // later today
	if got, want := s.NextRun(now), time.Date(2026, 10, 13, 11, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("later today: got %v, want %v", got, want)
	}

	s.Hour = 10 // exactly now -> next day
	if got, want := s.NextRun(now), time.Date(2026, 10, 14, 10, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("exactly now: got %v, want %v", got, want)
	}

	s.Hour = 9
	s.Weekdays = []time.Weekday{time.Tuesday} // already passed today -> next week
	if got, want := s.NextRun(now), time.Date(2026, 10, 20, 9, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("weekly: got %v, want %v", got, want)
	}
}

// --- plist rendering / parsing ---

func TestRenderParseRoundTrip(t *testing.T) {
	cases := map[string]Schedule{
		"daily": validSchedule(),
		"weekdays and multiple targets": func() Schedule {
			s := validSchedule()
			s.Hour, s.Minute = 22, 30
			s.Weekdays = []time.Weekday{time.Sunday, time.Wednesday, time.Saturday}
			s.Targets = []string{"derived-data", "simulator-caches"}
			s.EvenIfXcodeRunning = true
			return s
		}(),
		"xml special chars in paths": func() Schedule {
			s := validSchedule()
			s.Binary = "/Users/a&b/<bin>/mac-cleaner"
			s.LogPath = "/Users/a&b/\"logs\"/scheduled.log"
			return s
		}(),
	}
	for name, s := range cases {
		data, err := RenderPlist(s)
		if err != nil {
			t.Fatalf("%s: render: %v", name, err)
		}
		got, err := ParsePlistJSON(plistToJSON(t, data))
		if err != nil {
			t.Fatalf("%s: parse: %v\n%s", name, err, data)
		}
		if !reflect.DeepEqual(got, s) {
			t.Errorf("%s: round trip mismatch\n got: %+v\nwant: %+v", name, got, s)
		}
	}
}

func TestRenderPlistContents(t *testing.T) {
	data, err := RenderPlist(validSchedule())
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		"<string>" + Label + "</string>",
		"<key>StartCalendarInterval</key>",
		"<key>StandardOutPath</key>",
		"<string>Background</string>",
		"<key>LowPriorityIO</key>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	if strings.Contains(out, "<key>Weekday</key>") {
		t.Error("daily schedule should not contain Weekday")
	}
}

func TestRenderPlistEscapesXML(t *testing.T) {
	s := validSchedule()
	s.Binary = "/tmp/</string><string>evil"
	data, err := RenderPlist(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "</string><string>evil") {
		t.Error("binary path was not XML-escaped")
	}
}

func TestRenderPlistPassesPlutilLint(t *testing.T) {
	plutil, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil not available")
	}
	s := validSchedule()
	s.Weekdays = []time.Weekday{time.Monday}
	data, err := RenderPlist(s)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.plist")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(plutil, "-lint", path).CombinedOutput(); err != nil { // #nosec G204 -- test-only
		t.Fatalf("plutil -lint failed: %v\n%s", err, out)
	}
}

func TestParsePlistJSONSingleDictIntervalAndSunday7(t *testing.T) {
	data := []byte(`{"ProgramArguments":["/bin/mc","scan","--derived-data","--force"],
		"StartCalendarInterval":{"Weekday":7,"Hour":4,"Minute":15},"RunAtLoad":false}`)
	s, err := ParsePlistJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Hour != 4 || s.Minute != 15 || !reflect.DeepEqual(s.Weekdays, []time.Weekday{time.Sunday}) {
		t.Errorf("unexpected schedule: %+v", s)
	}
	if !s.EvenIfXcodeRunning {
		t.Error("missing guard flag should mean EvenIfXcodeRunning")
	}
}

func TestParsePlistJSONErrors(t *testing.T) {
	cases := map[string]string{
		"not json":       "garbage",
		"not a scan job": `{"ProgramArguments":["/bin/ls"],"StartCalendarInterval":{"Hour":1,"Minute":0}}`,
		"no interval":    `{"ProgramArguments":["/bin/mc","scan"]}`,
		"bad interval":   `{"ProgramArguments":["/bin/mc","scan"],"StartCalendarInterval":"x"}`,
		"empty interval": `{"ProgramArguments":["/bin/mc","scan"],"StartCalendarInterval":[]}`,
	}
	for name, in := range cases {
		if _, err := ParsePlistJSON([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// --- Manager ---

type call struct {
	name string
	args []string
}

type fakeRunner struct {
	calls   []call
	results map[string]struct {
		out []byte
		err error
	}
}

func (f *fakeRunner) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name != "launchctl" {
		return defaultRunner(ctx, name, args...)
	}
	f.calls = append(f.calls, call{name, args})
	if len(args) > 0 {
		if r, ok := f.results[args[0]]; ok {
			return r.out, r.err
		}
	}
	return nil, nil
}

func (f *fakeRunner) set(sub string, out string, err error) {
	if f.results == nil {
		f.results = map[string]struct {
			out []byte
			err error
		}{}
	}
	f.results[sub] = struct {
		out []byte
		err error
	}{[]byte(out), err}
}

func newManager(t *testing.T) (Manager, *fakeRunner) {
	f := &fakeRunner{}
	return Manager{Home: t.TempDir(), UID: 501, Run: f.run}, f
}

func TestInstall(t *testing.T) {
	m, f := newManager(t)
	s := validSchedule()
	s.LogPath = LogPath(m.Home)

	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(PlistPath(m.Home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("plist permissions = %v, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Dir(s.LogPath)); err != nil {
		t.Errorf("log directory not created: %v", err)
	}

	want := []call{
		{"launchctl", []string{"bootout", "gui/501/" + Label}},
		{"launchctl", []string{"bootstrap", "gui/501", PlistPath(m.Home)}},
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

func TestInstallReplacesExisting(t *testing.T) {
	m, _ := newManager(t)
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	s.Hour = 5
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	st, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Schedule.Hour != 5 {
		t.Errorf("reinstall did not replace schedule: %+v", st.Schedule)
	}
}

func TestInstallIgnoresBootoutError(t *testing.T) {
	m, f := newManager(t)
	f.set("bootout", "Boot-out failed: 3: No such process", errors.New("exit status 3"))
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err != nil {
		t.Fatalf("bootout failure should be ignored: %v", err)
	}
}

func TestInstallBootstrapError(t *testing.T) {
	m, f := newManager(t)
	f.set("bootstrap", "Bootstrap failed: 5: Input/output error", errors.New("exit status 5"))
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	err := m.Install(s)
	if err == nil || !strings.Contains(err.Error(), "Input/output error") {
		t.Fatalf("expected bootstrap error with output, got %v", err)
	}
}

func TestInstallInvalidScheduleWritesNothing(t *testing.T) {
	m, f := newManager(t)
	s := validSchedule()
	s.Targets = nil
	if err := m.Install(s); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(PlistPath(m.Home)); !errors.Is(err, os.ErrNotExist) {
		t.Error("plist should not be written for invalid schedule")
	}
	if len(f.calls) != 0 {
		t.Errorf("no launchctl calls expected, got %v", f.calls)
	}
}

func TestInstallUnwritableHome(t *testing.T) {
	m, _ := newManager(t)
	// A file where the LaunchAgents directory should be.
	if err := os.MkdirAll(filepath.Join(m.Home, "Library"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Home, "Library", "LaunchAgents"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err == nil {
		t.Fatal("expected error when LaunchAgents is not a directory")
	}

	m2, _ := newManager(t)
	if err := os.WriteFile(filepath.Join(m2.Home, "Library"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s.LogPath = LogPath(m2.Home)
	if err := m2.Install(s); err == nil {
		t.Fatal("expected error when log directory cannot be created")
	}
}

func TestUninstall(t *testing.T) {
	m, f := newManager(t)
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	f.calls = nil

	removed, err := m.Uninstall()
	if err != nil || !removed {
		t.Fatalf("Uninstall = %v, %v; want true, nil", removed, err)
	}
	if _, err := os.Stat(PlistPath(m.Home)); !errors.Is(err, os.ErrNotExist) {
		t.Error("plist still exists")
	}
	if len(f.calls) != 1 || f.calls[0].args[0] != "bootout" {
		t.Errorf("expected a bootout call, got %v", f.calls)
	}

	removed, err = m.Uninstall()
	if err != nil || removed {
		t.Errorf("second Uninstall = %v, %v; want false, nil", removed, err)
	}
}

func TestUninstallRemoveError(t *testing.T) {
	m, _ := newManager(t)
	// A non-empty directory at the plist path cannot be removed by os.Remove.
	if err := os.MkdirAll(filepath.Join(PlistPath(m.Home), "x"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Uninstall(); err == nil {
		t.Fatal("expected error")
	}
}

func TestStatusNotInstalled(t *testing.T) {
	m, f := newManager(t)
	st, err := m.Status()
	if err != nil || st.Installed {
		t.Fatalf("Status = %+v, %v; want not installed", st, err)
	}
	if len(f.calls) != 0 {
		t.Errorf("no launchctl calls expected, got %v", f.calls)
	}
}

func TestStatusLoaded(t *testing.T) {
	m, f := newManager(t)
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	f.set("print", "gui/501/com.mac-cleaner = {\n\tstate = not running\n\tlast exit code = 0\n}\n", nil)

	st, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || !st.Loaded || st.LastExitCode != "0" {
		t.Errorf("unexpected status: %+v", st)
	}
	if !reflect.DeepEqual(st.Schedule, s) {
		t.Errorf("schedule = %+v, want %+v", st.Schedule, s)
	}
}

func TestStatusNeverRun(t *testing.T) {
	m, f := newManager(t)
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	f.set("print", "gui/501/com.mac-cleaner = {\n\tstate = not running\n}\n", nil)
	st, err := m.Status()
	if err != nil || !st.Loaded || st.LastExitCode != "" {
		t.Errorf("Status = %+v, %v", st, err)
	}
}

func TestStatusNotLoaded(t *testing.T) {
	m, f := newManager(t)
	s := validSchedule()
	s.LogPath = LogPath(m.Home)
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	f.set("print", "Could not find service", errors.New("exit status 113"))
	st, err := m.Status()
	if err != nil || !st.Installed || st.Loaded {
		t.Errorf("Status = %+v, %v; want installed but not loaded", st, err)
	}
}

func TestStatusCorruptPlist(t *testing.T) {
	m, _ := newManager(t)
	path := PlistPath(m.Home)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Status(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestStatusUnreadablePlist(t *testing.T) {
	m, _ := newManager(t)
	// A directory at the plist path makes plutil fail.
	if err := os.MkdirAll(PlistPath(m.Home), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Status(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestRunNow(t *testing.T) {
	m, f := newManager(t)
	if err := m.RunNow(); err != nil {
		t.Fatal(err)
	}
	want := []call{{"launchctl", []string{"kickstart", "gui/501/" + Label}}}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}

	f.set("kickstart", "Could not find service", errors.New("exit status 113"))
	if err := m.RunNow(); err == nil || !strings.Contains(err.Error(), "Could not find service") {
		t.Errorf("expected kickstart error, got %v", err)
	}
}

func TestManagerDefaultRunner(t *testing.T) {
	// With Run nil the real exec path is used; "true" ignores its args and succeeds.
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	m := Manager{}
	if _, err := m.run("true", "x"); err != nil {
		t.Errorf("default runner failed: %v", err)
	}
}

func TestXcodeRunningDoesNotPanic(t *testing.T) {
	_ = XcodeRunning()
}

// --- binary resolution / header ---

func TestResolveBinary(t *testing.T) {
	lookOK := func(string) (string, error) { return "/opt/homebrew/bin/mac-cleaner", nil }
	lookFail := func(string) (string, error) { return "", errors.New("not found") }
	lookCellar := func(string) (string, error) { return "/opt/homebrew/Cellar/mac-cleaner/1.0/bin/mac-cleaner", nil }
	cellar := "/opt/homebrew/Cellar/mac-cleaner/1.7.0/bin/mac-cleaner"

	if p, w := ResolveBinary("/usr/local/bin/mac-cleaner", lookFail); p != "/usr/local/bin/mac-cleaner" || w != "" {
		t.Errorf("non-cellar: got %q, %q", p, w)
	}
	if p, w := ResolveBinary(cellar, lookOK); p != "/opt/homebrew/bin/mac-cleaner" || w != "" {
		t.Errorf("cellar with PATH entry: got %q, %q", p, w)
	}
	if p, w := ResolveBinary(cellar, lookFail); p != cellar || w == "" {
		t.Errorf("cellar without PATH entry: got %q, %q", p, w)
	}
	if p, w := ResolveBinary(cellar, lookCellar); p != cellar || w == "" {
		t.Errorf("cellar with Cellar PATH entry: got %q, %q", p, w)
	}
}

func TestPrintHeader(t *testing.T) {
	var b bytes.Buffer
	PrintHeader(&b, time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC))
	if got := b.String(); got != "=== mac-cleaner scheduled run 2026-10-13T03:00:00Z ===\n" {
		t.Errorf("got %q", got)
	}
}

func TestPaths(t *testing.T) {
	if got := PlistPath("/Users/x"); got != "/Users/x/Library/LaunchAgents/com.mac-cleaner.plist" {
		t.Errorf("PlistPath = %q", got)
	}
	if got := LogPath("/Users/x"); got != "/Users/x/Library/Logs/mac-cleaner/scheduled.log" {
		t.Errorf("LogPath = %q", got)
	}
}
