package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sp3esu/mac-cleaner/internal/schedule"
)

var (
	flagScheduleAt            string
	flagScheduleWeekdays      string
	flagScheduleTargets       []string
	flagScheduleSimCaches     bool
	flagScheduleEvenIfRunning bool
)

// newScheduleManager returns the launchd manager for the current user.
// It is a package-level variable for test injection.
var newScheduleManager = func() (schedule.Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return schedule.Manager{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return schedule.Manager{Home: home, UID: os.Getuid()}, nil
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "manage automatic scheduled cleanup (launchd)",
	Long: `Install a per-user launchd agent that cleans developer caches on a schedule.

The agent lives in ~/Library/LaunchAgents/com.mac-cleaner.plist and runs as your
user (no sudo). If the Mac is asleep at the scheduled time, the job runs on wake;
if it is powered off, that run is skipped. By default the run is skipped while
Xcode is open.

Examples:
  mac-cleaner schedule install                         Xcode DerivedData daily at 03:00
  mac-cleaner schedule install --at 22:30 --weekdays mon,fri --simulator-caches
  mac-cleaner schedule status
  mac-cleaner schedule run                             run the job now
  mac-cleaner schedule uninstall`,
}

var scheduleInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "install or replace the scheduled cleanup",
	Args:  cobra.NoArgs,
	RunE:  runScheduleInstall,
}

var scheduleStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show the scheduled cleanup",
	Args:  cobra.NoArgs,
	RunE:  runScheduleStatus,
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run",
	Short: "run the scheduled cleanup now",
	Args:  cobra.NoArgs,
	RunE:  runScheduleRun,
}

var scheduleUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "remove the scheduled cleanup",
	Args:  cobra.NoArgs,
	RunE:  runScheduleUninstall,
}

func init() {
	f := scheduleInstallCmd.Flags()
	f.StringVar(&flagScheduleAt, "at", "03:00", "time of day to run (HH:MM, 24-hour)")
	f.StringVar(&flagScheduleWeekdays, "weekdays", "", "comma-separated days to run, e.g. mon,wed,fri (default every day)")
	f.StringSliceVar(&flagScheduleTargets, "targets", []string{"derived-data"}, "scan items to clean (see 'mac-cleaner scan --help')")
	f.BoolVar(&flagScheduleSimCaches, "simulator-caches", false, "also clean iOS Simulator caches")
	f.BoolVar(&flagScheduleEvenIfRunning, "even-if-running", false, "clean even while Xcode is running")

	for _, c := range []*cobra.Command{scheduleInstallCmd, scheduleStatusCmd, scheduleRunCmd, scheduleUninstallCmd} {
		// Execute() prints the error; skip cobra's duplicate message and usage dump.
		c.SilenceErrors = true
		c.SilenceUsage = true
		scheduleCmd.AddCommand(c)
	}
	rootCmd.AddCommand(scheduleCmd)
}

// scheduleTargets validates and de-duplicates the requested targets
// against the targeted scan flags in scanGroups.
func scheduleTargets(requested []string, simCaches bool) ([]string, error) {
	if simCaches {
		requested = append(append([]string(nil), requested...), "simulator-caches")
	}
	valid := map[string]bool{}
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.FlagName != "" && item.ScanFlag != nil {
				valid[item.FlagName] = true
			}
		}
	}
	seen := map[string]bool{}
	var targets []string
	for _, t := range requested {
		t = strings.TrimPrefix(strings.TrimSpace(t), "--")
		if !valid[t] {
			return nil, fmt.Errorf("unknown target %q (see 'mac-cleaner scan --help' for targeted scan flags)", t)
		}
		if !seen[t] {
			seen[t] = true
			targets = append(targets, t)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one target is required")
	}
	return targets, nil
}

func runScheduleInstall(cmd *cobra.Command, _ []string) error {
	hour, minute, err := schedule.ParseTime(flagScheduleAt)
	if err != nil {
		return err
	}
	days, err := schedule.ParseWeekdays(flagScheduleWeekdays)
	if err != nil {
		return err
	}
	targets, err := scheduleTargets(flagScheduleTargets, flagScheduleSimCaches)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	bin, warning := schedule.ResolveBinary(exe, exec.LookPath)

	m, err := newScheduleManager()
	if err != nil {
		return err
	}
	s := schedule.Schedule{
		Hour:               hour,
		Minute:             minute,
		Weekdays:           days,
		Targets:            targets,
		Binary:             bin,
		LogPath:            schedule.LogPath(m.Home),
		EvenIfXcodeRunning: flagScheduleEvenIfRunning,
	}
	w := cmd.OutOrStdout()
	if flagDryRun {
		if err := s.Validate(); err != nil {
			return err
		}
		fmt.Fprintf(w, "Dry run: would install scheduled cleanup (%s).\n", s.Describe())
		printSchedule(w, s, m.Home)
		return nil
	}
	if err := m.Install(s); err != nil {
		return err
	}

	fmt.Fprintf(w, "Scheduled cleanup installed (%s).\n", s.Describe())
	printSchedule(w, s, m.Home)
	if warning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", warning)
	}
	return nil
}

func runScheduleStatus(cmd *cobra.Command, _ []string) error {
	m, err := newScheduleManager()
	if err != nil {
		return err
	}
	st, err := m.Status()
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	if !st.Installed {
		fmt.Fprintln(w, "No scheduled cleanup installed. Use 'mac-cleaner schedule install'.")
		return nil
	}
	loaded := "yes"
	if !st.Loaded {
		loaded = "no (run 'mac-cleaner schedule install' to reload)"
	}
	fmt.Fprintf(w, "Schedule:  %s\n", st.Schedule.Describe())
	fmt.Fprintf(w, "Loaded:    %s\n", loaded)
	printSchedule(w, st.Schedule, m.Home)
	if st.LastExitCode != "" {
		fmt.Fprintf(w, "Last exit: %s\n", st.LastExitCode)
	}
	return nil
}

func runScheduleRun(cmd *cobra.Command, _ []string) error {
	m, err := newScheduleManager()
	if err != nil {
		return err
	}
	if err := m.RunNow(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Scheduled cleanup started. Output: %s\n", shortenHome(schedule.LogPath(m.Home), m.Home))
	return nil
}

func runScheduleUninstall(cmd *cobra.Command, _ []string) error {
	m, err := newScheduleManager()
	if err != nil {
		return err
	}
	removed, err := m.Uninstall()
	if err != nil {
		return err
	}
	if removed {
		fmt.Fprintln(cmd.OutOrStdout(), "Scheduled cleanup removed.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No scheduled cleanup installed.")
	}
	return nil
}

// printSchedule writes the shared detail lines for install and status.
func printSchedule(w io.Writer, s schedule.Schedule, home string) {
	xcode := "skip run while Xcode is open"
	if s.EvenIfXcodeRunning {
		xcode = "run even while Xcode is open"
	}
	fmt.Fprintf(w, "Targets:   %s\n", strings.Join(s.Targets, ", "))
	fmt.Fprintf(w, "Xcode:     %s\n", xcode)
	fmt.Fprintf(w, "Next run:  %s\n", s.NextRun(time.Now()).Format("Mon 2006-01-02 15:04"))
	fmt.Fprintf(w, "Command:   %s\n", strings.Join(s.ProgramArguments(), " "))
	fmt.Fprintf(w, "Log:       %s\n", shortenHome(s.LogPath, home))
	fmt.Fprintf(w, "Agent:     %s\n", shortenHome(schedule.PlistPath(home), home))
}
