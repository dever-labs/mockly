// Package main is the mockly CLI entrypoint. Each command group lives in
// its own file in this package (start.go, apply.go, config.go, generate.go,
// mocks.go, preset.go, scenario.go, fault.go); shared.go holds small helpers
// used across more than one of them.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
WebSocket, gRPC, GraphQL, TCP, Redis, SMTP, MQTT, NATS, SNMP, DNS, AMQP, Kafka,
LDAP, IMAP, FTP, Memcached, STOMP, CoAP, and SIP protocols in a single
binary with a built-in web UI and REST management API.`,
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "mockly.yaml", "Config file path")

	root.AddCommand(
		startCmd(),
		applyCmd(),
		configCmd(),
		generateCmd(),
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
