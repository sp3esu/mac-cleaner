package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sp3esu/mac-cleaner/internal/schedule"
)

// fakeLaunchctl records launchctl invocations and returns canned results
// keyed by subcommand (e.g. "print").
type fakeLaunchctl struct {
	calls [][]string
	fail  map[string]error
	out   map[string]string
}

func (f *fakeLaunchctl) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name != "launchctl" { // plutil runs for real
		out, err := exec.CommandContext(ctx, name, args...).Output() // #nosec G204 -- test-only
		return out, err
	}
	f.calls = append(f.calls, append([]string{name}, args...))
	return []byte(f.out[args[0]]), f.fail[args[0]]
}

// setupSchedule points the schedule commands at a temp home with a fake
// launchctl and resets install flags to their defaults.
func setupSchedule(t *testing.T) (schedule.Manager, *fakeLaunchctl) {
	t.Helper()
	f := &fakeLaunchctl{fail: map[string]error{}, out: map[string]string{}}
	m := schedule.Manager{Home: t.TempDir(), UID: 501, Run: f.run}

	origMgr := newScheduleManager
	newScheduleManager = func() (schedule.Manager, error) { return m, nil }
	t.Cleanup(func() {
		newScheduleManager = origMgr
		flagScheduleAt = "03:00"
		flagScheduleWeekdays = ""
		flagScheduleTargets = []string{"derived-data"}
		flagScheduleSimCaches = false
		flagScheduleEvenIfRunning = false
		flagDryRun = false
	})
	flagScheduleAt = "03:00"
	flagScheduleWeekdays = ""
	flagScheduleTargets = []string{"derived-data"}
	flagScheduleSimCaches = false
	flagScheduleEvenIfRunning = false
	flagDryRun = false
	return m, f
}

func runSchedule(t *testing.T, fn func(*cobra.Command, []string) error) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetErr(&errOut)
	err := fn(c, nil)
	return out.String(), errOut.String(), err
}

func TestScheduleCmd_Registered(t *testing.T) {
	var names []string
	for _, c := range scheduleCmd.Commands() {
		names = append(names, c.Name())
	}
	want := []string{"install", "run", "status", "uninstall"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("subcommands = %v, want %v", names, want)
	}
	found := false
	for _, c := range rootCmd.Commands() {
		if c == scheduleCmd {
			found = true
		}
	}
	if !found {
		t.Error("schedule command not registered on root")
	}
	for _, c := range scheduleCmd.Commands() {
		if !c.SilenceErrors || !c.SilenceUsage {
			t.Errorf("%s: expected SilenceErrors and SilenceUsage", c.Name())
		}
	}
	for _, flag := range []string{"at", "weekdays", "targets", "simulator-caches", "even-if-running"} {
		if scheduleInstallCmd.Flags().Lookup(flag) == nil {
			t.Errorf("install missing --%s", flag)
		}
	}
}

func TestScheduleTargets(t *testing.T) {
	got, err := scheduleTargets([]string{"derived-data"}, false)
	if err != nil || !reflect.DeepEqual(got, []string{"derived-data"}) {
		t.Errorf("default: got %v, %v", got, err)
	}

	got, err = scheduleTargets([]string{"derived-data"}, true)
	if err != nil || !reflect.DeepEqual(got, []string{"derived-data", "simulator-caches"}) {
		t.Errorf("sim caches: got %v, %v", got, err)
	}

	got, err = scheduleTargets([]string{"--derived-data", " simulator-caches", "derived-data"}, true)
	if err != nil || !reflect.DeepEqual(got, []string{"derived-data", "simulator-caches"}) {
		t.Errorf("dedupe/prefix: got %v, %v", got, err)
	}

	for _, bad := range [][]string{{"bogus"}, {"dev-caches"}, {"system-caches"}, {""}, {"force"}} {
		if _, err := scheduleTargets(bad, false); err == nil {
			t.Errorf("%v: expected error", bad)
		}
	}
	if _, err := scheduleTargets(nil, false); err == nil {
		t.Error("empty: expected error")
	}
}

func TestScheduleTargets_DoesNotMutateInput(t *testing.T) {
	in := make([]string, 1, 4)
	in[0] = "derived-data"
	if _, err := scheduleTargets(in, true); err != nil {
		t.Fatal(err)
	}
	if got := in[:cap(in)][1]; got != "" {
		t.Errorf("input backing array mutated: %q", got)
	}
}

func TestScheduleInstall(t *testing.T) {
	m, f := setupSchedule(t)
	flagScheduleAt = "22:30"
	flagScheduleWeekdays = "mon,fri"
	flagScheduleSimCaches = true

	out, _, err := runSchedule(t, runScheduleInstall)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "installed (Mon, Fri at 22:30)") || !strings.Contains(out, "derived-data, simulator-caches") {
		t.Errorf("unexpected output:\n%s", out)
	}

	last := f.calls[len(f.calls)-1]
	if last[1] != "bootstrap" {
		t.Errorf("expected bootstrap as last call, got %v", last)
	}

	st, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	s := st.Schedule
	if s.Hour != 22 || s.Minute != 30 || len(s.Weekdays) != 2 || s.EvenIfXcodeRunning {
		t.Errorf("unexpected installed schedule: %+v", s)
	}
	if s.LogPath != schedule.LogPath(m.Home) {
		t.Errorf("log path = %q", s.LogPath)
	}
}

func TestScheduleInstall_EvenIfRunning(t *testing.T) {
	m, _ := setupSchedule(t)
	flagScheduleEvenIfRunning = true
	out, _, err := runSchedule(t, runScheduleInstall)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "run even while Xcode is open") {
		t.Errorf("unexpected output:\n%s", out)
	}
	st, _ := m.Status()
	if !st.Schedule.EvenIfXcodeRunning {
		t.Error("expected EvenIfXcodeRunning in installed plist")
	}
}

func TestScheduleInstall_DryRun(t *testing.T) {
	m, f := setupSchedule(t)
	flagDryRun = true
	out, _, err := runSchedule(t, runScheduleInstall)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dry run: would install") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if _, err := os.Stat(schedule.PlistPath(m.Home)); !errors.Is(err, os.ErrNotExist) {
		t.Error("dry run must not write the plist")
	}
	if len(f.calls) != 0 {
		t.Errorf("dry run must not call launchctl, got %v", f.calls)
	}
}

func TestScheduleInstall_InvalidInput(t *testing.T) {
	cases := map[string]func(){
		"time":     func() { flagScheduleAt = "25:00" },
		"weekdays": func() { flagScheduleWeekdays = "funday" },
		"target":   func() { flagScheduleTargets = []string{"bogus"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m, f := setupSchedule(t)
			mutate()
			if _, _, err := runSchedule(t, runScheduleInstall); err == nil {
				t.Fatal("expected error")
			}
			if _, err := os.Stat(schedule.PlistPath(m.Home)); !errors.Is(err, os.ErrNotExist) {
				t.Error("plist must not be written on invalid input")
			}
			if len(f.calls) != 0 {
				t.Errorf("no launchctl calls expected, got %v", f.calls)
			}
		})
	}
}

func TestScheduleInstall_BootstrapFails(t *testing.T) {
	_, f := setupSchedule(t)
	f.fail["bootstrap"] = errors.New("exit status 5")
	if _, _, err := runSchedule(t, runScheduleInstall); err == nil {
		t.Fatal("expected error")
	}
}

func TestScheduleCommands_ManagerError(t *testing.T) {
	setupSchedule(t)
	newScheduleManager = func() (schedule.Manager, error) { return schedule.Manager{}, errors.New("no home") }
	for name, fn := range map[string]func(*cobra.Command, []string) error{
		"install":   runScheduleInstall,
		"status":    runScheduleStatus,
		"run":       runScheduleRun,
		"uninstall": runScheduleUninstall,
	} {
		if _, _, err := runSchedule(t, fn); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestScheduleStatus_NotInstalled(t *testing.T) {
	setupSchedule(t)
	out, _, err := runSchedule(t, runScheduleStatus)
	if err != nil || !strings.Contains(out, "No scheduled cleanup installed") {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestScheduleStatus_Installed(t *testing.T) {
	_, f := setupSchedule(t)
	if _, _, err := runSchedule(t, runScheduleInstall); err != nil {
		t.Fatal(err)
	}
	f.out["print"] = "\tlast exit code = 0\n"
	out, _, err := runSchedule(t, runScheduleStatus)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"daily at 03:00", "Loaded:    yes", "Last exit: 0", "derived-data", "skip run while Xcode is open", "~/Library/Logs/mac-cleaner/scheduled.log"} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q:\n%s", want, out)
		}
	}
}

func TestScheduleStatus_NotLoaded(t *testing.T) {
	_, f := setupSchedule(t)
	if _, _, err := runSchedule(t, runScheduleInstall); err != nil {
		t.Fatal(err)
	}
	f.fail["print"] = errors.New("exit status 113")
	out, _, err := runSchedule(t, runScheduleStatus)
	if err != nil || !strings.Contains(out, "Loaded:    no") || strings.Contains(out, "Last exit") {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestScheduleStatus_CorruptPlist(t *testing.T) {
	m, _ := setupSchedule(t)
	path := schedule.PlistPath(m.Home)
	if err := os.MkdirAll(strings.TrimSuffix(path, "/com.mac-cleaner.plist"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSchedule(t, runScheduleStatus); err == nil {
		t.Fatal("expected error")
	}
}

func TestScheduleRun(t *testing.T) {
	_, f := setupSchedule(t)
	out, _, err := runSchedule(t, runScheduleRun)
	if err != nil || !strings.Contains(out, "started") {
		t.Errorf("got %q, %v", out, err)
	}
	if f.calls[0][1] != "kickstart" {
		t.Errorf("expected kickstart, got %v", f.calls)
	}

	f.fail["kickstart"] = errors.New("exit status 113")
	if _, _, err := runSchedule(t, runScheduleRun); err == nil {
		t.Error("expected error when kickstart fails")
	}
}

func TestScheduleUninstall(t *testing.T) {
	setupSchedule(t)
	out, _, err := runSchedule(t, runScheduleUninstall)
	if err != nil || !strings.Contains(out, "No scheduled cleanup installed") {
		t.Errorf("not installed: got %q, %v", out, err)
	}

	if _, _, err := runSchedule(t, runScheduleInstall); err != nil {
		t.Fatal(err)
	}
	out, _, err = runSchedule(t, runScheduleUninstall)
	if err != nil || !strings.Contains(out, "Scheduled cleanup removed") {
		t.Errorf("installed: got %q, %v", out, err)
	}
}

func TestScheduleUninstall_RemoveError(t *testing.T) {
	m, _ := setupSchedule(t)
	if err := os.MkdirAll(schedule.PlistPath(m.Home)+"/x", 0o750); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSchedule(t, runScheduleUninstall); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewScheduleManager_Default(t *testing.T) {
	m, err := newScheduleManager()
	if err != nil {
		t.Fatal(err)
	}
	if m.Home == "" || m.UID != os.Getuid() || m.Run != nil {
		t.Errorf("unexpected manager: %+v", m)
	}
}

// --- hidden scan flags used by the scheduled job ---

func TestScanCmd_HiddenScheduledFlags(t *testing.T) {
	for _, name := range []string{"scheduled", "skip-if-xcode-running"} {
		f := scanCmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("missing --%s", name)
			continue
		}
		if !f.Hidden {
			t.Errorf("--%s should be hidden", name)
		}
	}
}

func TestScanCmd_SkipsWhileXcodeRunning(t *testing.T) {
	origXcode := schedule.XcodeRunning
	origEng := eng
	t.Cleanup(func() {
		schedule.XcodeRunning = origXcode
		eng = origEng
		flagScanDerivedData = false
		flagScheduled = false
		flagSkipIfXcodeRunning = false
	})
	schedule.XcodeRunning = func() bool { return true }
	eng = nil // any scan attempt would panic
	flagScanDerivedData = true
	flagScheduled = true
	flagSkipIfXcodeRunning = true

	out := captureStdout(t, func() { scanCmd.Run(scanCmd, nil) })
	if !strings.Contains(out, "=== mac-cleaner scheduled run") {
		t.Errorf("missing scheduled header:\n%s", out)
	}
	if !strings.Contains(out, "Xcode is running; skipping cleanup.") {
		t.Errorf("missing skip message:\n%s", out)
	}
}
