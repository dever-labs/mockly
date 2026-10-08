package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/internal/config"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Work with Mockly config files",
	}
	cmd.AddCommand(configValidateCmd())
	return cmd
}

func configValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate a config file without starting any servers",
		Long: `Parses and structurally validates a Mockly config file with no side
effects (no ports bound, no servers started). Reports YAML parse errors,
duplicate mock IDs within a protocol, and invalid regular expressions
(path_regex/uri_regex fields and any "re:..." matcher value).

Any inline schema reference (openapi:/asyncapi:/proto:) is also resolved,
so a broken or stale spec is caught here too, same as it would be at
"mockly start".

Exits non-zero if the file is missing or invalid, so this can be wired into
a CI step or pre-commit hook.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := cfgFile
			if len(args) == 1 {
				path = args[0]
			}
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("config %q: %w", path, err)
			}
			cfg, err := loadResolvedConfig(path)
			if err != nil {
				return fmt.Errorf("config %q: %w", path, err)
			}
			errs := config.Validate(cfg)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, "error:", e)
				}
				return fmt.Errorf("%s: %d validation error(s) found", path, len(errs))
			}
			fmt.Printf("%s: valid\n", path)
			return nil
		},
	}
}
