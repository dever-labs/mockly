package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dever-labs/mockly/internal/asyncapi"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/openapi"
	"github.com/dever-labs/mockly/internal/protoidl"
)

func generateCmd() *cobra.Command {
	var out string
	var httpPort int
	cmd := &cobra.Command{
		Use:   "generate <spec-file>",
		Short: "Generate a Mockly config from an OpenAPI, AsyncAPI, or Protobuf spec",
		Long: `Parses a local OpenAPI 3.x document, AsyncAPI 2.x/3.x document (YAML or
JSON, auto-detected from the file's top-level "openapi"/"asyncapi" field), or
Protobuf (.proto) service definition, and derives a ready-to-run Mockly
config from it, so you can start mocking a system you only have a spec for
in one step:

  mockly generate api.yaml -o mockly.yaml
  mockly start -c mockly.yaml

For an OpenAPI spec, one HTTP mock is generated per operation: the path and
method come straight from the spec, and the response body is taken from the
operation's example/examples when present, or otherwise synthesised from its
JSON schema.

For an AsyncAPI spec, mocks are generated per channel/operation across
whichever of Kafka, MQTT, AMQP, NATS and WebSocket the spec's servers use.
Not every AsyncAPI operation has a Mockly equivalent (e.g. Mockly has no
spontaneous/scheduled publish mechanism for MQTT/AMQP/NATS yet) — those are
skipped with a warning but generation still succeeds for the rest.

For a .proto file, one gRPC mock is generated per unary RPC method, with the
response body synthesised from the method's output message type. Streaming
methods have no Mockly equivalent and are skipped with a warning. Imports are
resolved only from the spec file's own directory plus the standard
google/protobuf/*.proto well-known types.

Operations/channels/methods Mockly couldn't derive a usable mock for are
skipped; a warning is printed for each one but generation still succeeds as
long as at least one mock could be produced.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := args[0]

			isProto, err := protoidl.IsProtoIDL(specPath)
			if err != nil {
				return err
			}
			isAsync := false
			if !isProto {
				isAsync, err = asyncapi.IsAsyncAPI(specPath)
				if err != nil {
					return err
				}
			}

			cfg := config.Defaults()
			var generated int

			switch {
			case isProto:
				res, err := protoidl.Generate(specPath)
				if err != nil {
					return err
				}
				for _, w := range res.Warnings {
					fmt.Fprintln(os.Stderr, "warn:", w)
				}
				if res.Empty() {
					return fmt.Errorf("%s: no RPC methods could be converted into mocks", specPath)
				}
				cfg.Protocols.GRPC = &config.GRPCConfig{
					Enabled:  true,
					Services: []config.GRPCService{{Proto: specPath, Mocks: res.Mocks}},
				}
				generated = len(res.Mocks)
			case isAsync:
				res, err := asyncapi.Generate(specPath)
				if err != nil {
					return err
				}
				for _, w := range res.Warnings {
					fmt.Fprintln(os.Stderr, "warn:", w)
				}
				if res.Empty() {
					return fmt.Errorf("%s: no operations could be converted into mocks", specPath)
				}
				if len(res.Kafka) > 0 {
					cfg.Protocols.Kafka = &config.KafkaConfig{Enabled: true, Mocks: res.Kafka}
					generated += len(res.Kafka)
				}
				if len(res.MQTT) > 0 {
					cfg.Protocols.MQTT = &config.MQTTConfig{Enabled: true, Mocks: res.MQTT}
					generated += len(res.MQTT)
				}
				if len(res.AMQP) > 0 {
					cfg.Protocols.AMQP = &config.AMQPConfig{Enabled: true, Mocks: res.AMQP}
					generated += len(res.AMQP)
				}
				if len(res.NATS) > 0 {
					cfg.Protocols.NATS = &config.NATSConfig{Enabled: true, Mocks: res.NATS}
					generated += len(res.NATS)
				}
				if len(res.WebSocket) > 0 {
					cfg.Protocols.WebSocket = &config.WebSocketConfig{Enabled: true, Mocks: res.WebSocket}
					generated += len(res.WebSocket)
				}
			default:
				res, err := openapi.Generate(specPath)
				if err != nil {
					return err
				}
				for _, w := range res.Warnings {
					fmt.Fprintln(os.Stderr, "warn:", w)
				}
				if len(res.Mocks) == 0 {
					return fmt.Errorf("%s: no operations could be converted into mocks", specPath)
				}
				cfg.Protocols.HTTP = &config.HTTPConfig{Enabled: true, Port: httpPort, Mocks: res.Mocks}
				generated = len(res.Mocks)
			}

			config.ApplyDefaults(&cfg)

			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("encoding generated config: %w", err)
			}
			if err := os.WriteFile(out, data, 0o644); err != nil { //nolint:gosec // generated mock config, not a secret
				return fmt.Errorf("writing %q: %w", out, err)
			}

			fmt.Printf("Generated %d mock(s) from %s -> %s\n", generated, specPath, out)
			fmt.Printf("Run it with: mockly start -c %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "mockly.generated.yaml", "Output config file path")
	cmd.Flags().IntVar(&httpPort, "http-port", 8080, "HTTP port in the generated config (OpenAPI specs only)")
	return cmd
}
