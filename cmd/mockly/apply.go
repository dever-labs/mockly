package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func applyCmd() *cobra.Command {
	var applyFile string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a config file to a running Mockly instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadResolvedConfig(applyFile)
			if err != nil {
				return err
			}
			apiAddr := fmt.Sprintf("http://localhost:%d", cfg.Mockly.API.Port)

			if cfg.Protocols.HTTP != nil {
				for _, m := range cfg.Protocols.HTTP.Mocks {
					if err := postJSON(apiAddr+"/api/mocks/http", m); err != nil {
						fmt.Fprintf(os.Stderr, "warn: %v\n", err)
					}
				}
			}
			fmt.Println("Config applied.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&applyFile, "config", "f", "mockly.yaml", "Config file to apply")
	return cmd
}
