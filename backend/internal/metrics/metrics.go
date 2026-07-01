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
		Help:      "WebSocket upgrade attempts by result (ok, denied_origin, rate_limited, upgrade_error).",
	}, []string{"result"})

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
)
