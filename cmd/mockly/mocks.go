package main

import (
	"fmt"
	"net/http"
	"os"

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

// addCmd is the parent "mockly add ..." command group: "add http" adds a
// single ad-hoc HTTP mock, "add page" scaffolds one serving an HTML
// page (or a whole folder of static assets) without hand-escaping markup
// into a YAML string (see cmd/mockly/mocks.go's addPageCmd and issue #202).
func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a mock at runtime",
	}
	cmd.AddCommand(addHTTPCmd(), addPageCmd())
	return cmd
}

func addHTTPCmd() *cobra.Command {
	var method, path, status, body, delayStr, id string
	cmd := &cobra.Command{
		Use:   "http",
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
				Response: config.HTTPResponse{Status: statusCode, Body: config.NewHTTPBodyLiteral(body), Delay: delay},
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

// addPageCmd scaffolds an HTML-page mock without requiring the page's
// markup to be hand-escaped into a YAML/JSON string. "--file" serves one
// page at --path; "--dir" expands a whole folder of static assets
// (HTML/CSS/JS/images/...) into one mock per file, each at --path joined
// with the file's own path, reusing the same config.ExpandHTTPMocks helper
// that config.Load uses for a mockly.yaml "body: {dir: ...}" reference —
// see issue #202's "Directory expansion" section for the full path-mapping
// rules (index.html aliasing, generated mock IDs, etc).
func addPageCmd() *cobra.Command {
	var urlPath, file, dir, id string
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Add an HTML page mock (from a file or a folder of static assets) at runtime",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (file == "") == (dir == "") {
				return fmt.Errorf("exactly one of --file or --dir is required")
			}
			cfg, _ := config.Load(cfgFile)
			apiAddr := fmt.Sprintf("http://localhost:%d", cfg.Mockly.API.Port)

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("determining working directory: %w", err)
			}

			if file != "" {
				mock := config.HTTPMock{
					ID:       id,
					Request:  config.HTTPRequest{Method: "GET", Path: urlPath},
					Response: config.HTTPResponse{Status: 200, Body: config.HTTPBody{File: file}},
				}
				if err := postJSON(apiAddr+"/api/mocks/http", mock); err != nil {
					return err
				}
				fmt.Println("Page mock added.")
				return nil
			}

			parent := config.HTTPMock{
				ID:       id,
				Request:  config.HTTPRequest{Method: "GET", Path: urlPath},
				Response: config.HTTPResponse{Status: 200, Body: config.HTTPBody{Dir: dir}},
			}
			mocks, err := config.ExpandHTTPMocks([]config.HTTPMock{parent}, cwd, false)
			if err != nil {
				return err
			}
			for _, m := range mocks {
				if err := postJSON(apiAddr+"/api/mocks/http", m); err != nil {
					return fmt.Errorf("adding mock %q: %w", m.ID, err)
				}
			}
			fmt.Printf("Page mock added: %d mock(s) from %q.\n", len(mocks), dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Mock ID / ID prefix (auto-generated if empty)")
	cmd.Flags().StringVar(&urlPath, "path", "/", "URL path the page is served at")
	cmd.Flags().StringVar(&file, "file", "", "Single HTML file to serve at --path")
	cmd.Flags().StringVar(&dir, "dir", "", "Folder of static assets to expand into one mock per file under --path")
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
