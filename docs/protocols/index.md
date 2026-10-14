# Protocols

Mockly mocks 20 protocols from a single binary and config file. Each protocol
section below covers its request matching options, response shaping, and any
protocol-specific features (streaming, sequences, faults, etc.).

| Protocol | Typical use case |
|---|---|
| [HTTP](http.md) | REST APIs, webhooks, static/HTML pages, record mode |
| [Outbound Webhooks](webhooks.md) | Server-initiated callbacks fired by an HTTP mock |
| [WebSocket](websocket.md) | Realtime/bidirectional messaging |
| [gRPC](grpc.md) | Unary and streaming RPCs from a `.proto` definition |
| [GraphQL](graphql.md) | Queries, mutations, and subscriptions |
| [TCP](tcp.md) | Raw byte-stream protocols |
| [Redis](redis.md) | RESP-protocol commands |
| [SMTP](smtp.md) | Inbound mail capture |
| [MQTT](mqtt.md) | Pub/sub messaging for IoT |
| [NATS](nats.md) | Pub/sub and JetStream messaging |
| [SNMP](snmp.md) | Network device management (GET/GETNEXT/SET, traps) |
| [DNS](dns.md) | Name resolution records |
| [AMQP](amqp.md) | RabbitMQ-style message queues |
| [Kafka](kafka.md) | Topic-based event streaming |
| [LDAP](ldap.md) | Directory lookups and binds |
| [IMAP](imap.md) | Mailbox retrieval |
| [FTP](ftp.md) | File transfer |
| [Memcached](memcached.md) | Key-value caching |
| [STOMP](stomp.md) | Simple text-oriented messaging |
| [CoAP](coap.md) | Constrained-device (IoT) messaging |
| [SIP](sip.md) | VoIP signaling |

Every protocol shares the same core concepts — matching, templated
responses, state conditions, and scenarios — documented once in
[Configuration](../getting-started/configuration.md); this section covers
what's unique to each protocol.
