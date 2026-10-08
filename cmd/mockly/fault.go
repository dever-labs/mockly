package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/internal/config"
)

func faultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "Control direct HTTP fault injection on a running Mockly instance",
	}
	cmd.AddCommand(faultSetCmd(), faultClearCmd(), faultStatusCmd())
	return cmd
}

func faultSetCmd() *cobra.Command {
	var status int
	var delayStr string
	var delayMinStr string
	var delayMaxStr string
	var body string
	var errorRate float64
	var rateLimit int
	var rateLimitStatus int

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Enable direct HTTP fault injection",
		Long: `Enable direct HTTP fault injection. Examples:

  # Add 500ms latency to every request
  mockly fault set --delay 500ms

  # Add jittery 150ms-400ms latency to every request
  mockly fault set --delay-min 150ms --delay-max 400ms

  # Return 503 for every request
  mockly fault set --status 503 --body '{"error":"service_unavailable"}'

  # Return 429 for 30% of requests
  mockly fault set --status 429 --error-rate 0.3

  # Return 429 once more than 5 requests/sec are received
  mockly fault set --rate-limit 5

  # Combine: 200ms latency + 500 errors 10% of the time
  mockly fault set --delay 200ms --status 500 --error-rate 0.1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)

			fault := config.HTTPFault{
				Status:    status,
				Body:      body,
				ErrorRate: errorRate,
			}
			if delayStr != "" {
				if err := fault.Delay.UnmarshalText([]byte(delayStr)); err != nil {
					return err
				}
			}
			if delayMinStr != "" || delayMaxStr != "" {
				if delayMinStr == "" || delayMaxStr == "" {
					return fmt.Errorf("--delay-min and --delay-max must be set together")
				}
				dr := &config.DelayRange{}
				if err := dr.Min.UnmarshalText([]byte(delayMinStr)); err != nil {
					return err
				}
				if err := dr.Max.UnmarshalText([]byte(delayMaxStr)); err != nil {
					return err
				}
				fault.DelayRange = dr
			}
			if rateLimit > 0 {
				fault.RateLimit = &config.RateLimitFault{
					RequestsPerSecond: rateLimit,
					OverLimitStatus:   rateLimitStatus,
				}
			}
			if err := postJSON(fmt.Sprintf("http://localhost:%d/api/fault/http", cfg.Mockly.API.Port), fault); err != nil {
				return err
			}
			fmt.Println("HTTP fault injection enabled.")
			return nil
		},
	}
	cmd.Flags().IntVar(&status, "status", 0, "HTTP status code to inject (0 = only inject delay)")
	cmd.Flags().StringVar(&delayStr, "delay", "", "Latency to add to every request (e.g. 500ms, 2s)")
	cmd.Flags().StringVar(&delayMinStr, "delay-min", "", "Minimum latency for a random jittered delay (e.g. 150ms); requires --delay-max")
	cmd.Flags().StringVar(&delayMaxStr, "delay-max", "", "Maximum latency for a random jittered delay (e.g. 400ms); requires --delay-min")
	cmd.Flags().StringVar(&body, "body", "", "Response body to return when fault fires")
	cmd.Flags().Float64Var(&errorRate, "error-rate", 0, "Fraction of requests to affect (0.0–1.0; default: all)")
	cmd.Flags().IntVar(&rateLimit, "rate-limit", 0, "Requests/sec allowed before returning --rate-limit-status (0 = disabled)")
	cmd.Flags().IntVar(&rateLimitStatus, "rate-limit-status", 0, "Status code returned once --rate-limit is exceeded (default 429)")
	return cmd
}

func faultClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Disable direct HTTP fault injection",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			req, _ := http.NewRequest(http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/api/fault", cfg.Mockly.API.Port), nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			fmt.Println("HTTP fault injection cleared.")
			return nil
		},
	}
}

func faultStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current global fault configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/fault", cfg.Mockly.API.Port))
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			printResponse(resp)
			return nil
		},
	}
}
