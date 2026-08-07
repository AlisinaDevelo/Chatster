package main

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
	"github.com/AliSinaDevelo/Chatster/internal/telemetry"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	maxWebSocketReadBytes = 32 * 1024
	outboundQueueSize     = 256
	writeWait             = 10 * time.Second
	shutdownCloseCode     = websocket.CloseServiceRestart
	shutdownCloseReason   = "server shutting down"
	shutdownCloseWait     = 250 * time.Millisecond
	pongWait              = 60 * time.Second
	pingPeriod            = (pongWait * 9) / 10
)

// Client represents a connected client.
type Client struct {
	ID         string
	Conn       *websocket.Conn
	Username   string
	Room       string
	Hub        *Hub
	writeMu    sync.Mutex // gorilla/websocket allows one writer at a time
	usernameMu sync.RWMutex
	msgLimiter *rate.Limiter
	send       chan Message
	done       chan struct{}
	closeOnce  sync.Once
	traceCtx   context.Context
	traceSpan  trace.Span
}

func (c *Client) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return c.Conn.WriteJSON(v)
}

func (c *Client) writeControl(messageType int, data []byte, deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.Conn.WriteControl(messageType, data, deadline)
}

func (c *Client) setUsername(username string) {
	c.usernameMu.Lock()
	defer c.usernameMu.Unlock()
	c.Username = username
}

func (c *Client) username() string {
	c.usernameMu.RLock()
	defer c.usernameMu.RUnlock()
	return c.Username
}

func (c *Client) room() string {
	if c.Room == "" {
		return db.DefaultRoom
	}
	return c.Room
}

func (c *Client) traceContext() context.Context {
	if c.traceCtx == nil {
		return context.Background()
	}
	return c.traceCtx
}

func (c *Client) finishTrace() {
	if c.traceSpan == nil {
		return
	}
	c.traceSpan.SetAttributes(attribute.String("chatster.websocket.lifecycle", "closed"))
	c.traceSpan.End()
	c.traceSpan = nil
}

func (c *Client) enqueue(message Message) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.send <- message:
		return true
	case <-c.done:
		return false
	default:
		return false
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	})
}

func (c *Client) closeForShutdown() {
	if c.Conn != nil {
		// WriteControl is safe alongside the single data writer and has its own short deadline.
		_ = c.Conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(shutdownCloseCode, shutdownCloseReason),
			time.Now().Add(shutdownCloseWait),
		)
	}
	c.close()
}

func (c *Client) allowMessage() bool {
	return c.msgLimiter == nil || c.msgLimiter.Allow()
}

func (c *Client) sendRateLimitNotice() {
	notice := Message{
		Username: "System",
		Content:  "You are sending messages too quickly. Please slow down.",
		Type:     "notification",
		Room:     c.room(),
	}
	if !c.enqueue(notice) {
		slog.Warn("queue rate limit notice")
	}
}

func (c *Client) auditRejectedMessage(reason, content string) {
	ctx, cancel := context.WithTimeout(c.traceContext(), storageOperationTimeout)
	defer cancel()
	ctx, span := telemetry.Start(
		ctx,
		"chatster.storage.moderation_event",
		attribute.String("chatster.moderation.reason", reason),
	)
	defer span.End()
	if _, err := c.Hub.database.SaveModerationEventContext(ctx, c.ID, c.username(), reason, content); err != nil {
		telemetry.MarkError(span)
		slog.Warn("save moderation audit event", "err", err, "reason", reason, "session_id", c.ID)
	}
}

func (c *Client) writeMessages() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		select {
		case <-c.done:
			return
		case message := <-c.send:
			if err := c.writeJSON(message); err != nil {
				metrics.WSOutboundDrops.WithLabelValues("write_error").Inc()
				slog.Warn("write websocket message", "err", err)
				c.close()
				return
			}
		}
	}
}

func (c *Client) startHeartbeat(done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.writeControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				slog.Warn("ping client", "err", err)
				c.close()
				return
			}
		case <-done:
			return
		}
	}
}

func (c *Client) readMessages() {
	done := make(chan struct{})
	go c.startHeartbeat(done)

	defer func() {
		close(done)
		metrics.ConnectedClients.Dec()
		c.Hub.unregisterClient(c)
		c.Hub.notifyClientFinished(c)
		c.close()
		c.finishTrace()
	}()

	c.Conn.SetReadLimit(maxWebSocketReadBytes)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		slog.Warn("set read deadline", "err", err)
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("read message", "err", err)
			}
			break
		}

		if msg.Type == "username" {
			name := strings.TrimSpace(msg.Content)
			if !validUsername(name) {
				metrics.MessagesRejected.WithLabelValues("invalid_username").Inc()
				c.auditRejectedMessage("invalid_username", msg.Content)
				slog.Warn("invalid username rejected", "session_id", c.ID)
				continue
			}
			c.setUsername(name)
			continue
		}

		if msg.Type != "message" {
			msg.Type = "message"
		}
		body := strings.TrimSpace(msg.Content)
		if !validMessageBody(body) {
			metrics.MessagesRejected.WithLabelValues("invalid_body").Inc()
			c.auditRejectedMessage("invalid_body", msg.Content)
			slog.Warn("invalid message rejected", "session_id", c.ID)
			continue
		}
		if !c.allowMessage() {
			metrics.MessagesRejected.WithLabelValues("rate_limited").Inc()
			c.auditRejectedMessage("rate_limited", body)
			slog.Warn("message rate limited", "session_id", c.ID)
			c.sendRateLimitNotice()
			continue
		}
		msg.Content = body
		msg.Username = c.username()
		msg.Room = c.room()

		dbMsg, err := saveMessageObservedInRoomContext(c.traceContext(), c.Hub.database, msg.Room, msg.Username, msg.Content, msg.Type)
		if err != nil {
			slog.Warn("save message", "err", err)
		} else {
			msg.ID = dbMsg.ID
			msg.Timestamp = dbMsg.Timestamp
		}

		metrics.MessagesIngested.Inc()
		if !c.Hub.publishContext(c.traceContext(), msg) {
			return
		}
	}
}

func newMessageLimiter(cfg config.Config) *rate.Limiter {
	if cfg.DisableMessageRateLimit || cfg.MessageRPS <= 0 {
		return nil
	}
	burst := cfg.MessageBurst
	if burst < 1 {
		burst = 1
	}
	return rate.NewLimiter(rate.Limit(cfg.MessageRPS), burst)
}
