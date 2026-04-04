package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/assets"
	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/grpcserver"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/protocols/wsserver"
	"github.com/dever-labs/mockly/internal/state"
)

var (
	cfgFile string
	uiPort  int
	apiPort int
)

func main() {
	root := &cobra.Command{
		Use:   "mockly",
		Short: "Mockly — cross-platform multi-protocol mock server",
		Long: `Mockly is a fast, cross-platform mock server that supports HTTP,
WebSocket, and gRPC protocols with a built-in web UI and REST management API.`,
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "mockly.yaml", "Config file path")

	root.AddCommand(
		startCmd(),
		applyCmd(),
		listCmd(),
		addHTTPCmd(),
		deleteCmd(),
		statusCmd(),
		resetCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// start
// ---------------------------------------------------------------------------

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start all configured mock servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			if uiPort > 0 {
				cfg.Mockly.UI.Port = uiPort
			}
			if apiPort > 0 {
				cfg.Mockly.API.Port = apiPort
			}

			return runServers(cfg)
		},
	}
	cmd.Flags().IntVar(&uiPort, "ui-port", 0, "Override UI port")
	cmd.Flags().IntVar(&apiPort, "api-port", 0, "Override API port")
	return cmd
}

func runServers(cfg *config.Config) error {
	store := state.New()
	log := logger.New(500)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 4)

	var httpSrv api.HTTPProtocol
	var wsSrv api.WSProtocol
	var grpcSrv api.GRPCProtocol

	if cfg.Protocols.HTTP != nil && cfg.Protocols.HTTP.Enabled {
		srv := httpserver.New(cfg.Protocols.HTTP, store, log)
		httpSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ HTTP mock server  on :%d\n", cfg.Protocols.HTTP.Port)
	}

	if cfg.Protocols.WebSocket != nil && cfg.Protocols.WebSocket.Enabled {
		srv := wsserver.New(cfg.Protocols.WebSocket, store, log)
		wsSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ WebSocket server  on :%d\n", cfg.Protocols.WebSocket.Port)
	}

	if cfg.Protocols.GRPC != nil && cfg.Protocols.GRPC.Enabled {
		srv := grpcserver.New(cfg.Protocols.GRPC, store, log)
		grpcSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ gRPC server       on :%d\n", cfg.Protocols.GRPC.Port)
	}

	apiSrv := api.New(&cfg.Mockly, store, log, httpSrv, wsSrv, grpcSrv)

	if cfg.Mockly.UI.Enabled {
		apiSrv.AttachUI(assets.DistFS())
	}

	go func() { errCh <- apiSrv.Start(ctx) }()
	fmt.Printf("→ Management API    on http://localhost:%d/api\n", cfg.Mockly.API.Port)
	if cfg.Mockly.UI.Enabled {
		fmt.Printf("→ Web UI            on http://localhost:%d\n", cfg.Mockly.API.Port)
	}

	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down...")
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// apply
// ---------------------------------------------------------------------------

func applyCmd() *cobra.Command {
	var applyFile string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a config file to a running Mockly instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(applyFile)
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

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

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
			defer resp.Body.Close()
			printResponse(resp)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// add http
// ---------------------------------------------------------------------------

func addHTTPCmd() *cobra.Command {
	var method, path, status, body, delayStr, id string
	cmd := &cobra.Command{
		Use:   "add http",
		Short: "Add an HTTP mock at runtime",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			apiAddr := fmt.Sprintf("http://localhost:%d", cfg.Mockly.API.Port)

			statusCode := 200
			fmt.Sscan(status, &statusCode)

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

// ---------------------------------------------------------------------------
// delete
// ---------------------------------------------------------------------------

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
			defer resp.Body.Close()
			fmt.Printf("Deleted mock %s.\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "http", "Protocol (http, websocket, grpc)")
	return cmd
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

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
			defer resp.Body.Close()
			printResponse(resp)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// reset
// ---------------------------------------------------------------------------

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
			defer resp.Body.Close()
			fmt.Println("State and logs reset.")
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func postJSON(url string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d", resp.StatusCode)
	}
	return nil
}

func printResponse(resp *http.Response) {
	buf := make([]byte, 1<<20)
	n, _ := resp.Body.Read(buf)
	fmt.Println(string(buf[:n]))
}

// newYAMLNode is unused after refactor — kept for reference.
