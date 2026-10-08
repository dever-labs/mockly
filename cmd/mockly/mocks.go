package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/internal/config"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/mocks/http", cfg.Mockly.API.Port))
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			printResponse(resp)
			return nil
		},
	}
}

func addHTTPCmd() *cobra.Command {
	var method, path, status, body, delayStr, id string
	cmd := &cobra.Command{
		Use:   "add http",
		Short: "Add an HTTP mock at runtime",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			apiAddr := fmt.Sprintf("http://localhost:%d", cfg.Mockly.API.Port)

			statusCode := 200
			if _, err := fmt.Sscan(status, &statusCode); err != nil {
				return fmt.Errorf("invalid status code %q: %w", status, err)
			}

			var delay config.Duration
			if delayStr != "" {
				if err := delay.UnmarshalText([]byte(delayStr)); err != nil {
					return err
				}
			}

			mock := config.HTTPMock{
				ID:       id,
				Request:  config.HTTPRequest{Method: method, Path: path},
				Response: config.HTTPResponse{Status: statusCode, Body: body, Delay: delay},
			}
			if err := postJSON(apiAddr+"/api/mocks/http", mock); err != nil {
				return err
			}
			fmt.Println("Mock added.")
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Mock ID (auto-generated if empty)")
	cmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	cmd.Flags().StringVar(&path, "path", "/", "URL path")
	cmd.Flags().StringVar(&status, "status", "200", "Response status code")
	cmd.Flags().StringVar(&body, "body", "", "Response body")
	cmd.Flags().StringVar(&delayStr, "delay", "", "Artificial delay (e.g. 100ms)")
	return cmd
}

func deleteCmd() *cobra.Command {
	var protocol string
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a mock by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, _ := config.Load(cfgFile)
			apiAddr := fmt.Sprintf("http://localhost:%d/api/mocks/%s/%s", cfg.Mockly.API.Port, protocol, id)
			req, _ := http.NewRequest(http.MethodDelete, apiAddr, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close() //nolint:errcheck
			fmt.Printf("Deleted mock %s.\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "http", "Protocol (http, websocket, grpc)")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of all protocol servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/protocols", cfg.Mockly.API.Port))
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			printResponse(resp)
			return nil
		},
	}
}

func resetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset all state and clear logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			resp, err := http.Post(
				fmt.Sprintf("http://localhost:%d/api/reset", cfg.Mockly.API.Port),
				"application/json", nil,
			)
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			fmt.Println("State and logs reset.")
			return nil
		},
	}
}
