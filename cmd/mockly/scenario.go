package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/internal/config"
)

func scenarioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Manage test scenarios on a running Mockly instance",
	}
	cmd.AddCommand(scenarioListCmd(), scenarioActivateCmd(), scenarioDeactivateCmd(), scenarioActiveCmd())
	return cmd
}

func scenarioListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all defined scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/scenarios", cfg.Mockly.API.Port))
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			printResponse(resp)
			return nil
		},
	}
}

func scenarioActiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "active",
		Short: "Show currently active scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/scenarios/active", cfg.Mockly.API.Port))
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			printResponse(resp)
			return nil
		},
	}
}

func scenarioActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <id>",
		Short: "Activate a scenario by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			url := fmt.Sprintf("http://localhost:%d/api/scenarios/%s/activate", cfg.Mockly.API.Port, args[0])
			resp, err := http.Post(url, "application/json", nil) // #nosec G107 -- URL constructed from trusted config
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("scenario %q not found", args[0])
			}
			fmt.Printf("Scenario %q activated.\n", args[0])
			return nil
		},
	}
}

func scenarioDeactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate <id>",
		Short: "Deactivate a scenario by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			url := fmt.Sprintf("http://localhost:%d/api/scenarios/%s/activate", cfg.Mockly.API.Port, args[0])
			req, _ := http.NewRequest(http.MethodDelete, url, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			fmt.Printf("Scenario %q deactivated.\n", args[0])
			return nil
		},
	}
}
