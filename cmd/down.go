package cmd

import (
	"fmt"

	"github.com/jahwag/clem/internal/agent"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop all agent systemd services",
	RunE:  runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	// Stop the watchdog before the agents: the timer fires every 5 minutes and
	// would otherwise restart every service stopped below. Stopping the timer
	// only suppresses future activations, so also stop the oneshot service in
	// case a watchdog run is mid-flight right now.
	for _, wd := range []string{cfg.WatchdogTimerName(), cfg.WatchdogServiceName()} {
		fmt.Printf("stopping %s... ", wd)
		if err := agent.StopService(wd); err != nil {
			fmt.Println("FAILED")
			return err
		}
		fmt.Println("ok")
	}

	for agentKey, ac := range cfg.Agents {
		if ac.WebTerminalPort > 0 {
			ttydSvc := cfg.TtydServiceName(agentKey)
			fmt.Printf("stopping %s... ", ttydSvc)
			if err := agent.StopService(ttydSvc); err != nil {
				fmt.Println("FAILED")
				return err
			}
			fmt.Println("ok")
		}

		svcName := cfg.ServiceName(agentKey)
		fmt.Printf("stopping %s (%s)... ", ac.Name, svcName)
		if err := agent.StopService(svcName); err != nil {
			fmt.Println("FAILED")
			return err
		}
		fmt.Println("ok")
	}
	return nil
}
