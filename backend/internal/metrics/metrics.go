package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for Chatster (plus default process collectors on /metrics).
var (
	ConnectedClients = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "chatster",
		Name:      "websocket_clients_connected",
		Help:      "Currently connected WebSocket clients.",
	})

	WSUpgrades = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "websocket_upgrades_total",
		Help:      "WebSocket upgrade attempts by result (ok, denied_origin, rate_limited, upgrade_error, draining).",
	}, []string{"result"})

	WSDrainStarted = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "websocket_drains_started_total",
		Help:      "WebSocket hub drain operations started.",
	})

	WSDrainDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "chatster",
		Name:      "websocket_drain_duration_seconds",
		Help:      "Duration of WebSocket hub drain operations.",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
	})

	WSDrainClientsRemaining = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "chatster",
		Name:      "websocket_drain_clients_remaining",
		Help:      "Active WebSocket clients that have not completed a shutdown drain.",
	})

	WSDrainForcedCloses = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "websocket_drain_forced_closes_total",
		Help:      "WebSocket clients still active when the shutdown drain deadline expired.",
	})

	WSOutboundDrops = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "websocket_outbound_drops_total",
		Help:      "Outbound WebSocket messages dropped by reason (slow_client, write_error).",
	}, []string{"reason"})

	MessagesIngested = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "chat_messages_ingested_total",
		Help:      "Validated chat messages accepted for broadcast.",
	})

	MessagesRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "chat_messages_rejected_total",
		Help:      "Chat messages rejected by reason (invalid_username, invalid_body, rate_limited).",
	}, []string{"reason"})

	MessagesPruned = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "chat_messages_pruned_total",
		Help:      "Persisted chat messages removed by the configured retention policy.",
	})

	ModerationEventsPruned = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "moderation_audit_events_pruned_total",
		Help:      "Moderation audit events removed by the configured retention policy.",
	})

	MessagePersistDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "chatster",
		Name:      "chat_message_persist_duration_seconds",
		Help:      "Duration of SQLite message persistence by result.",
		Buckets:   []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"result"})

	BroadcastFanoutDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "chatster",
		Name:      "websocket_broadcast_fanout_duration_seconds",
		Help:      "Duration of hub fanout enqueue work for one broadcast message.",
		Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
	})

	RedisConnections = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "redis_connections_total",
		Help:      "Redis broker connection attempts by result (ok, error).",
	}, []string{"result"})

	RedisPublishes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "redis_publishes_total",
		Help:      "Redis broker publish attempts by result (ok, error).",
	}, []string{"result"})

	RedisSubscriptions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "redis_subscriptions_total",
		Help:      "Redis broker subscription attempts by result (ok, error).",
	}, []string{"result"})

	RedisDecodes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "redis_decodes_total",
		Help:      "Redis broker payload decodes by result (ok, error).",
	}, []string{"result"})

	RedisDrops = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "redis_drops_total",
		Help:      "Redis broker events dropped by reason.",
	}, []string{"reason"})

	RedisReconnects = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "chatster",
		Name:      "redis_reconnects_total",
		Help:      "Redis broker subscriber reconnect attempts.",
	})
)
