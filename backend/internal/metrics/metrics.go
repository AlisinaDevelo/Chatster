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
)
