package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/presets"
)

func presetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Work with bundled mock presets",
	}
	cmd.AddCommand(presetListCmd(), presetUseCmd(), presetShowCmd())
	return cmd
}

func presetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("%-12s  %s\n", "NAME", "DESCRIPTION")
			fmt.Printf("%-12s  %s\n", "----", "-----------")
			for _, p := range presets.All {
				fmt.Printf("%-12s  %s\n", p.Name, p.Description)
			}
			return nil
		},
	}
}

func presetUseCmd() *cobra.Command {
	var httpPort, apiPort int
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Start Mockly with a bundled preset",
		Long: `Start Mockly using a bundled preset configuration.

Examples:
  mockly preset use keycloak
  mockly preset use stripe --http-port 9000
  mockly preset use openai --api-port 9095`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := presets.Read(args[0])
			if err != nil {
				return err
			}

			// Write to a temp file so config.Load can parse it.
			tmpFile, err := os.CreateTemp("", "mockly-preset-*.yaml")
			if err != nil {
				return fmt.Errorf("creating temp file: %w", err)
			}
			defer os.Remove(tmpFile.Name()) //nolint:errcheck

			if _, err := tmpFile.Write(data); err != nil {
				return fmt.Errorf("writing temp file: %w", err)
			}
			_ = tmpFile.Close() // best-effort close before config.Load reads the file

			cfg, err := config.Load(tmpFile.Name())
			if err != nil {
				return err
			}

			if httpPort > 0 && cfg.Protocols.HTTP != nil {
				cfg.Protocols.HTTP.Port = httpPort
			}
			if apiPort > 0 {
				cfg.Mockly.API.Port = apiPort
				cfg.Mockly.UI.Port = apiPort
			}

			fmt.Printf("Starting preset: %s\n", args[0])
			return runServers(cfg)
		},
	}
	cmd.Flags().IntVar(&httpPort, "http-port", 0, "Override the HTTP mock server port")
	cmd.Flags().IntVar(&apiPort, "api-port", 0, "Override the management API/UI port")
	return cmd
}

func presetShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Print the YAML for a bundled preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := presets.Read(args[0])
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
}
