package cmd

import (
	"fmt"

	"github.com/jahwag/clem/internal/agent"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start all agent systemd services",
	RunE:  runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	// Re-arm the watchdog timer on the way out even if an agent fails to
	// start (clem down stops it to keep agents down): a partially started
	// fleet should still get watchdog recovery rather than no protection.
	defer func() {
		timerName := cfg.WatchdogTimerName()
		fmt.Printf("starting %s... ", timerName)
		if err := agent.StartService(timerName); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			return
		}
		fmt.Println("ok")
	}()

	for agentKey, ac := range cfg.Agents {
		svcName := cfg.ServiceName(agentKey)
		fmt.Printf("starting %s (%s)... ", ac.Name, svcName)
		if err := agent.StartService(svcName); err != nil {
			fmt.Println("FAILED")
			return err
		}
		fmt.Println("ok")

		if ac.WebTerminalPort > 0 {
			ttydSvc := cfg.TtydServiceName(agentKey)
			fmt.Printf("starting %s (port %d)... ", ttydSvc, ac.WebTerminalPort)
			if err := agent.StartService(ttydSvc); err != nil {
				fmt.Println("FAILED")
				return err
			}
			fmt.Println("ok")
		}
	}
	return nil
}
