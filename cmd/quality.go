package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jahwag/clem/internal/quality"
	"github.com/spf13/cobra"
)

var qualityCmd = &cobra.Command{
	Use:   "quality",
	Short: "Quality gate metrics and runner-side execution",
	RunE:  runQualitySummary,
}

var qualityRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run quality suite after an agent session (runner-side)",
	Hidden: true,
	RunE:   runQualityRun,
}

var qualityPrePushCmd = &cobra.Command{
	Use:    "pre-push",
	Short:  "Run block-push quality gates (pre-push hook)",
	Hidden: true,
	RunE:   runQualityPrePush,
}

func init() {
	qualityRunCmd.Flags().String("home", "", "agent home directory")
	qualityRunCmd.Flags().String("workdir", "", "agent work directory (project checkout)")
	qualityPrePushCmd.Flags().String("home", "", "agent home directory")
	qualityPrePushCmd.Flags().String("workdir", "", "agent work directory")

	qualityCmd.AddCommand(qualityRunCmd)
	qualityCmd.AddCommand(qualityPrePushCmd)
	rootCmd.AddCommand(qualityCmd)
}

func runQualityRun(cmd *cobra.Command, args []string) error {
	code, err := executeQualityRun(cmd)
	if err != nil {
		return err
	}
	os.Exit(code)
	return nil
}

func executeQualityRun(cmd *cobra.Command) (int, error) {
	home, _ := cmd.Flags().GetString("home")
	workdir, _ := cmd.Flags().GetString("workdir")
	if home == "" || workdir == "" {
		return 1, fmt.Errorf("quality run requires --home and --workdir")
	}
	rc, err := quality.LoadRuntimeConfig(home)
	if err != nil {
		return 1, err
	}
	code, err := quality.RunIteration(home, workdir, rc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return code, nil
}

func runQualityPrePush(cmd *cobra.Command, args []string) error {
	home, _ := cmd.Flags().GetString("home")
	workdir, _ := cmd.Flags().GetString("workdir")
	if home == "" {
		return fmt.Errorf("quality pre-push requires --home")
	}
	rc, err := quality.LoadRuntimeConfig(home)
	if err != nil {
		return err
	}
	if workdir == "" {
		workdir = filepath.Join(home, rc.Project)
	}
	if err := quality.RunPrePush(home, workdir, rc); err != nil {
		return err
	}
	return nil
}

func runQualitySummary(cmd *cobra.Command, args []string) error {
	if cfg.Quality == nil || !cfg.Quality.Enabled {
		fmt.Println("quality gates disabled in config")
		return nil
	}
	keys := make([]string, 0, len(cfg.Agents))
	for k := range cfg.Agents {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("%-10s %-8s %-12s %-10s %s\n", "AGENT", "LAST", "GATES", "ATTEMPT", "PASS RATE")
	fmt.Println(strings.Repeat("-", 60))
	for _, key := range keys {
		home := fmt.Sprintf("/home/%s", cfg.OSUsername(key))
		sum, err := quality.ReadAgentSummary(home, key)
		if err != nil {
			return err
		}
		if sum.GatesTotal == 0 {
			fmt.Printf("%-10s %-8s %-12s %-10d %s\n", key, "-", "-", 0, "-")
			continue
		}
		last := "FAIL"
		if sum.LastPass {
			last = "PASS"
		}
		gates := fmt.Sprintf("%d/%d", sum.GatesPass, sum.GatesTotal)
		rate := fmt.Sprintf("%.0f%%", sum.PassRate*100)
		fmt.Printf("%-10s %-8s %-12s %-10d %s\n", key, last, gates, sum.LastAttempt, rate)
	}

	fmt.Println()
	for _, key := range keys {
		home := fmt.Sprintf("/home/%s", cfg.OSUsername(key))
		stats, err := quality.AggregateGateStats(home, key)
		if err != nil {
			return err
		}
		if len(stats) == 0 {
			continue
		}
		sort.Slice(stats, func(i, j int) bool { return stats[i].Name < stats[j].Name })
		fmt.Printf("Agent %s gate stats:\n", key)
		for _, st := range stats {
			fmt.Printf("  %-12s runs=%-4d pass=%.0f%% avg=%dms\n", st.Name, st.Runs, st.PassRate*100, st.AvgMS)
		}
		fmt.Println()
	}
	return nil
}
