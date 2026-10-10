package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dever-labs/mockly/assets"
	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/metrics"
	"github.com/dever-labs/mockly/internal/protocols/amqpserver"
	"github.com/dever-labs/mockly/internal/protocols/coapserver"
	"github.com/dever-labs/mockly/internal/protocols/dnsserver"
	"github.com/dever-labs/mockly/internal/protocols/ftpserver"
	"github.com/dever-labs/mockly/internal/protocols/graphqlserver"
	"github.com/dever-labs/mockly/internal/protocols/grpcserver"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/protocols/imapserver"
	"github.com/dever-labs/mockly/internal/protocols/kafkaserver"
	"github.com/dever-labs/mockly/internal/protocols/ldapserver"
	"github.com/dever-labs/mockly/internal/protocols/memcachedserver"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/natsserver"
	"github.com/dever-labs/mockly/internal/protocols/redisserver"
	"github.com/dever-labs/mockly/internal/protocols/sipserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/protocols/snmpserver"
	"github.com/dever-labs/mockly/internal/protocols/stompserver"
	"github.com/dever-labs/mockly/internal/protocols/tcpserver"
	"github.com/dever-labs/mockly/internal/protocols/wsserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/webhook"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start all configured mock servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadResolvedConfig(cfgFile)
			if err != nil {
				return err
			}
			if errs := config.Validate(cfg); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, "error:", e)
				}
				return fmt.Errorf("%s: %d validation error(s) found", cfgFile, len(errs))
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

// launch starts a protocol server's Start(ctx) loop on its own goroutine,
// forwarding its terminal error (if any) onto errCh. It exists purely to
// avoid repeating the same two-line goroutine wrapper for every protocol in
// runServers below.
func launch(ctx context.Context, errCh chan<- error, start func(context.Context) error) {
	go func() { errCh <- start(ctx) }()
}

func runServers(cfg *config.Config) error {
	store := state.New()
	sc := scenarios.New(cfg.Scenarios)
	log := logger.New(500)
	wh := webhook.New(500)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 20)

	var httpSrv api.HTTPProtocol
	var wsSrv api.WSProtocol
	var grpcSrv api.GRPCProtocol
	var graphqlSrv api.GraphQLProtocol
	var tcpSrv api.TCPProtocol
	var redisSrv api.RedisProtocol
	var smtpSrv api.SMTPProtocol
	var mqttSrv api.MQTTProtocol
	var natsSrv api.NATSProtocol
	var snmpSrv api.SNMPProtocol
	var dnsSrv api.DNSProtocol
	var amqpSrv api.AMQPProtocol
	var kafkaSrv api.KafkaProtocol
	var ldapSrv api.LDAPProtocol
	var imapSrv api.IMAPProtocol
	var ftpSrv api.FTPProtocol
	var memcachedSrv api.MemcachedProtocol
	var stompSrv api.STOMPProtocol
	var coapSrv api.CoAPProtocol
	var sipSrv api.SIPProtocol

	if cfg.Protocols.HTTP != nil && cfg.Protocols.HTTP.Enabled {
		srv := httpserver.New(cfg.Protocols.HTTP, store, sc, log, wh)
		httpSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ HTTP mock server  on :%d\n", cfg.Protocols.HTTP.Port)
	}

	if cfg.Protocols.WebSocket != nil && cfg.Protocols.WebSocket.Enabled {
		srv := wsserver.New(cfg.Protocols.WebSocket, store, sc, log)
		wsSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ WebSocket server  on :%d\n", cfg.Protocols.WebSocket.Port)
	}

	if cfg.Protocols.GRPC != nil && cfg.Protocols.GRPC.Enabled {
		srv := grpcserver.New(cfg.Protocols.GRPC, store, sc, log)
		grpcSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ gRPC server       on :%d\n", cfg.Protocols.GRPC.Port)
	}

	if cfg.Protocols.GraphQL != nil && cfg.Protocols.GraphQL.Enabled {
		srv := graphqlserver.New(cfg.Protocols.GraphQL, store, sc, log)
		graphqlSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ GraphQL server    on :%d%s\n", cfg.Protocols.GraphQL.Port, cfg.Protocols.GraphQL.Path)
	}

	if cfg.Protocols.TCP != nil && cfg.Protocols.TCP.Enabled {
		srv := tcpserver.New(cfg.Protocols.TCP, store, sc, log)
		tcpSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ TCP server        on :%d\n", cfg.Protocols.TCP.Port)
	}

	if cfg.Protocols.Redis != nil && cfg.Protocols.Redis.Enabled {
		srv := redisserver.New(cfg.Protocols.Redis, store, sc, log)
		redisSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ Redis server      on :%d\n", cfg.Protocols.Redis.Port)
	}

	if cfg.Protocols.SMTP != nil && cfg.Protocols.SMTP.Enabled {
		srv := smtpserver.New(cfg.Protocols.SMTP, sc, log)
		smtpSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ SMTP server       on :%d (%s)\n", cfg.Protocols.SMTP.Port, cfg.Protocols.SMTP.Domain)
	}

	if cfg.Protocols.MQTT != nil && cfg.Protocols.MQTT.Enabled {
		srv := mqttserver.New(cfg.Protocols.MQTT, store, sc, log)
		mqttSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ MQTT broker       on :%d\n", cfg.Protocols.MQTT.Port)
	}

	if cfg.Protocols.NATS != nil && cfg.Protocols.NATS.Enabled {
		srv := natsserver.New(cfg.Protocols.NATS, store, sc, log)
		natsSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ NATS server       on :%d\n", cfg.Protocols.NATS.Port)
	}

	if cfg.Protocols.SNMP != nil && cfg.Protocols.SNMP.Enabled {
		srv := snmpserver.New(cfg.Protocols.SNMP, store, sc, log)
		snmpSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ SNMP agent        on :%d\n", cfg.Protocols.SNMP.Port)
	}

	if cfg.Protocols.DNS != nil && cfg.Protocols.DNS.Enabled {
		srv := dnsserver.New(cfg.Protocols.DNS, store, sc, log)
		dnsSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ DNS server        on :%d\n", cfg.Protocols.DNS.Port)
	}

	if cfg.Protocols.AMQP != nil && cfg.Protocols.AMQP.Enabled {
		srv := amqpserver.New(cfg.Protocols.AMQP, store, sc, log)
		amqpSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ AMQP server       on :%d\n", cfg.Protocols.AMQP.Port)
	}

	if cfg.Protocols.Kafka != nil && cfg.Protocols.Kafka.Enabled {
		srv := kafkaserver.New(cfg.Protocols.Kafka, store, sc, log)
		kafkaSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ Kafka server      on :%d\n", cfg.Protocols.Kafka.Port)
	}

	if cfg.Protocols.LDAP != nil && cfg.Protocols.LDAP.Enabled {
		srv := ldapserver.New(cfg.Protocols.LDAP, store, sc, log)
		ldapSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ LDAP server       on :%d\n", cfg.Protocols.LDAP.Port)
	}

	if cfg.Protocols.IMAP != nil && cfg.Protocols.IMAP.Enabled {
		srv := imapserver.New(cfg.Protocols.IMAP, sc, log)
		imapSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ IMAP server       on :%d\n", cfg.Protocols.IMAP.Port)
	}

	if cfg.Protocols.FTP != nil && cfg.Protocols.FTP.Enabled {
		srv := ftpserver.New(cfg.Protocols.FTP, sc, log)
		ftpSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ FTP server        on :%d\n", cfg.Protocols.FTP.Port)
	}

	if cfg.Protocols.Memcached != nil && cfg.Protocols.Memcached.Enabled {
		srv := memcachedserver.New(cfg.Protocols.Memcached, store, sc, log)
		memcachedSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ Memcached server  on :%d\n", cfg.Protocols.Memcached.Port)
	}

	if cfg.Protocols.STOMP != nil && cfg.Protocols.STOMP.Enabled {
		srv := stompserver.New(cfg.Protocols.STOMP, store, sc, log)
		stompSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ STOMP server      on :%d\n", cfg.Protocols.STOMP.Port)
	}

	if cfg.Protocols.CoAP != nil && cfg.Protocols.CoAP.Enabled {
		srv := coapserver.New(cfg.Protocols.CoAP, store, sc, log)
		coapSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ CoAP server       on :%d\n", cfg.Protocols.CoAP.Port)
	}

	if cfg.Protocols.SIP != nil && cfg.Protocols.SIP.Enabled {
		srv := sipserver.New(cfg.Protocols.SIP, store, sc, log)
		sipSrv = srv
		launch(ctx, errCh, srv.Start)
		fmt.Printf("→ SIP server        on :%d\n", cfg.Protocols.SIP.Port)
	}

	apiSrv := api.New(cfg, store, sc, log, wh, httpSrv, wsSrv, grpcSrv, graphqlSrv, tcpSrv, redisSrv, smtpSrv, mqttSrv, natsSrv, snmpSrv, dnsSrv, amqpSrv, kafkaSrv, ldapSrv, imapSrv, ftpSrv, memcachedSrv, stompSrv, coapSrv, sipSrv)

	if cfg.Mockly.API.Metrics != nil && cfg.Mockly.API.Metrics.Enabled {
		metricsReg := metrics.New(func() float64 {
			if httpSrv == nil {
				return 0
			}
			return float64(len(httpSrv.GetMocks()))
		})
		apiSrv.SetMetrics(metricsReg)

		// Observe every completed HTTP mock request (logged with
		// Protocol == "http") as Prometheus counters/histograms.
		ch, cancelSub := log.Subscribe("metrics")
		go func() {
			defer cancelSub()
			for {
				select {
				case e, ok := <-ch:
					if !ok {
						return
					}
					if e.Protocol == "http" {
						metricsReg.ObserveHTTPRequest(e.MatchedID, e.Method, e.Status, float64(e.Duration)/1000.0)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

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
