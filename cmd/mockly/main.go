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
	"github.com/dever-labs/mockly/internal/presets"
	"github.com/dever-labs/mockly/internal/protocols/graphqlserver"
	"github.com/dever-labs/mockly/internal/protocols/grpcserver"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/redisserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/protocols/snmpserver"
	"github.com/dever-labs/mockly/internal/protocols/tcpserver"
	"github.com/dever-labs/mockly/internal/protocols/wsserver"
	"github.com/dever-labs/mockly/internal/scenarios"
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
		presetCmd(),
		scenarioCmd(),
		faultCmd(),
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
	sc := scenarios.New(cfg.Scenarios)
	log := logger.New(500)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 8)

	var httpSrv api.HTTPProtocol
	var wsSrv api.WSProtocol
	var grpcSrv api.GRPCProtocol
	var graphqlSrv api.GraphQLProtocol
	var tcpSrv api.TCPProtocol
	var redisSrv api.RedisProtocol
	var smtpSrv api.SMTPProtocol
	var mqttSrv api.MQTTProtocol
	var snmpSrv api.SNMPProtocol

	if cfg.Protocols.HTTP != nil && cfg.Protocols.HTTP.Enabled {
		srv := httpserver.New(cfg.Protocols.HTTP, store, sc, log)
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

	if cfg.Protocols.GraphQL != nil && cfg.Protocols.GraphQL.Enabled {
		srv := graphqlserver.New(cfg.Protocols.GraphQL, store, sc, log)
		graphqlSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ GraphQL server    on :%d%s\n", cfg.Protocols.GraphQL.Port, cfg.Protocols.GraphQL.Path)
	}

	if cfg.Protocols.TCP != nil && cfg.Protocols.TCP.Enabled {
		srv := tcpserver.New(cfg.Protocols.TCP, store, log)
		tcpSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ TCP server        on :%d\n", cfg.Protocols.TCP.Port)
	}

	if cfg.Protocols.Redis != nil && cfg.Protocols.Redis.Enabled {
		srv := redisserver.New(cfg.Protocols.Redis, store, log)
		redisSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ Redis server      on :%d\n", cfg.Protocols.Redis.Port)
	}

	if cfg.Protocols.SMTP != nil && cfg.Protocols.SMTP.Enabled {
		srv := smtpserver.New(cfg.Protocols.SMTP, log)
		smtpSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ SMTP server       on :%d (%s)\n", cfg.Protocols.SMTP.Port, cfg.Protocols.SMTP.Domain)
	}

	if cfg.Protocols.MQTT != nil && cfg.Protocols.MQTT.Enabled {
		srv := mqttserver.New(cfg.Protocols.MQTT, store, log)
		mqttSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ MQTT broker       on :%d\n", cfg.Protocols.MQTT.Port)
	}

	if cfg.Protocols.SNMP != nil && cfg.Protocols.SNMP.Enabled {
		srv := snmpserver.New(cfg.Protocols.SNMP, store, log)
		snmpSrv = srv
		go func() { errCh <- srv.Start(ctx) }()
		fmt.Printf("→ SNMP agent        on :%d\n", cfg.Protocols.SNMP.Port)
	}

	apiSrv := api.New(cfg, store, sc, log, httpSrv, wsSrv, grpcSrv, graphqlSrv, tcpSrv, redisSrv, smtpSrv, mqttSrv, snmpSrv)

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
			defer resp.Body.Close() //nolint:errcheck
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
			defer resp.Body.Close() //nolint:errcheck
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
			defer resp.Body.Close() //nolint:errcheck
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
			defer resp.Body.Close() //nolint:errcheck
			fmt.Println("State and logs reset.")
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// preset
// ---------------------------------------------------------------------------

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



// ---------------------------------------------------------------------------
// scenario
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// fault
// ---------------------------------------------------------------------------

func faultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "Control global fault injection on a running Mockly instance",
	}
	cmd.AddCommand(faultSetCmd(), faultClearCmd(), faultStatusCmd())
	return cmd
}

func faultSetCmd() *cobra.Command {
	var status int
	var delayStr string
	var body string
	var errorRate float64

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Enable global fault injection",
		Long: `Enable global fault injection. Examples:

  # Add 500ms latency to every request
  mockly fault set --delay 500ms

  # Return 503 for every request
  mockly fault set --status 503 --body '{"error":"service_unavailable"}'

  # Return 429 for 30% of requests
  mockly fault set --status 429 --error-rate 0.3

  # Combine: 200ms latency + 500 errors 10% of the time
  mockly fault set --delay 200ms --status 500 --error-rate 0.1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)

			fault := config.GlobalFault{
				Enabled:        true,
				StatusOverride: status,
				Body:           body,
				ErrorRate:      errorRate,
			}
			if delayStr != "" {
				if err := fault.Delay.UnmarshalText([]byte(delayStr)); err != nil {
					return err
				}
			}
			if err := postJSON(fmt.Sprintf("http://localhost:%d/api/fault", cfg.Mockly.API.Port), fault); err != nil {
				return err
			}
			fmt.Println("Global fault injection enabled.")
			return nil
		},
	}
	cmd.Flags().IntVar(&status, "status", 0, "HTTP status code to inject (0 = only inject delay)")
	cmd.Flags().StringVar(&delayStr, "delay", "", "Latency to add to every request (e.g. 500ms, 2s)")
	cmd.Flags().StringVar(&body, "body", "", "Response body to return when fault fires")
	cmd.Flags().Float64Var(&errorRate, "error-rate", 0, "Fraction of requests to affect (0.0–1.0; default: all)")
	return cmd
}

func faultClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Disable global fault injection",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(cfgFile)
			req, _ := http.NewRequest(http.MethodDelete,
				fmt.Sprintf("http://localhost:%d/api/fault", cfg.Mockly.API.Port), nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("could not reach Mockly API (is it running?): %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck
			fmt.Println("Global fault injection cleared.")
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

func postJSON(url string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(data))) // #nosec G107 -- URL from trusted config
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
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
