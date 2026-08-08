package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AliSinaDevelo/Chatster/internal/events"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
	redis "github.com/redis/go-redis/v9"
)

const (
	redisPingTimeout = 2 * time.Second
	redisMinBackoff  = 100 * time.Millisecond
	redisMaxBackoff  = 5 * time.Second
)

// Fanout is the transport boundary used by the in-process hub.
type Fanout interface {
	Publish(context.Context, events.Envelope) error
	Run(context.Context, func(events.Envelope)) error
	InstanceID() string
	Close() error
}

// Config contains the non-secret Redis fan-out settings. Credentials remain in URL.
type Config struct {
	URL        string
	Namespace  string
	InstanceID string
}

// RedisFanout provides namespaced, live-only room delivery through Redis Pub/Sub.
type RedisFanout struct {
	client     *redis.Client
	namespace  string
	instanceID string
	ready      chan struct{}
	readyOnce  sync.Once
	closeOnce  sync.Once
	closeErr   error
}

// NewRedis validates configuration and the connection before enabling broker mode.
func NewRedis(cfg Config) (*RedisFanout, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("redis URL is required when Redis fan-out is enabled")
	}

	namespace := strings.ToLower(strings.TrimSpace(cfg.Namespace))
	if _, err := events.ChannelPattern(namespace); err != nil {
		return nil, fmt.Errorf("invalid Redis namespace: %w", err)
	}
	instanceID := strings.ToLower(strings.TrimSpace(cfg.InstanceID))
	if err := events.ValidateInstanceID(instanceID); err != nil {
		return nil, fmt.Errorf("invalid Redis instance ID: %w", err)
	}

	options, err := redis.ParseURL(strings.TrimSpace(cfg.URL))
	if err != nil {
		return nil, errors.New("invalid Redis URL")
	}
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), redisPingTimeout)
	err = client.Ping(ctx).Err()
	cancel()
	if err != nil {
		metrics.RedisConnections.WithLabelValues("error").Inc()
		_ = client.Close()
		return nil, fmt.Errorf("redis connection check failed: %w", err)
	}

	metrics.RedisConnections.WithLabelValues("ok").Inc()
	return &RedisFanout{
		client:     client,
		namespace:  namespace,
		instanceID: instanceID,
		ready:      make(chan struct{}),
	}, nil
}

// InstanceID identifies the publishing process for loop prevention.
func (b *RedisFanout) InstanceID() string {
	return b.instanceID
}

// Ready closes after the first successful subscription. It is useful for integration tests.
func (b *RedisFanout) Ready() <-chan struct{} {
	return b.ready
}

// Publish sends a validated event to its room channel.
func (b *RedisFanout) Publish(ctx context.Context, event events.Envelope) error {
	if err := event.Validate(); err != nil {
		metrics.RedisPublishes.WithLabelValues("error").Inc()
		return fmt.Errorf("validate Redis event: %w", err)
	}
	if event.OriginInstance != b.instanceID {
		metrics.RedisPublishes.WithLabelValues("error").Inc()
		return errors.New("redis event origin does not match this instance")
	}
	channel, err := events.Channel(b.namespace, event.Room)
	if err != nil {
		metrics.RedisPublishes.WithLabelValues("error").Inc()
		return fmt.Errorf("build Redis channel: %w", err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		metrics.RedisPublishes.WithLabelValues("error").Inc()
		return fmt.Errorf("encode Redis event: %w", err)
	}
	if err := b.client.Publish(ctx, channel, payload).Err(); err != nil {
		metrics.RedisPublishes.WithLabelValues("error").Inc()
		return fmt.Errorf("publish Redis event: %w", err)
	}
	metrics.RedisPublishes.WithLabelValues("ok").Inc()
	return nil
}

// Run subscribes to every room in the namespace and reconnects with bounded backoff.
func (b *RedisFanout) Run(ctx context.Context, handler func(events.Envelope)) error {
	if handler == nil {
		return errors.New("redis event handler is required")
	}
	pattern, err := events.ChannelPattern(b.namespace)
	if err != nil {
		return fmt.Errorf("build Redis subscription pattern: %w", err)
	}

	backoff := redisMinBackoff
	for {
		err := b.subscribeOnce(ctx, pattern, handler)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			err = errors.New("redis subscription stopped")
		}
		metrics.RedisReconnects.Inc()
		slog.Warn("redis broker reconnecting", "namespace", b.namespace, "err", err)
		if !waitForBackoff(ctx, backoff) {
			return nil
		}
		backoff *= 2
		if backoff > redisMaxBackoff {
			backoff = redisMaxBackoff
		}
	}
}

func (b *RedisFanout) subscribeOnce(ctx context.Context, pattern string, handler func(events.Envelope)) error {
	pubsub := b.client.PSubscribe(ctx, pattern)
	defer func() { _ = pubsub.Close() }()

	if _, err := pubsub.Receive(ctx); err != nil {
		metrics.RedisSubscriptions.WithLabelValues("error").Inc()
		return fmt.Errorf("subscribe Redis pattern: %w", err)
	}
	metrics.RedisSubscriptions.WithLabelValues("ok").Inc()
	b.readyOnce.Do(func() { close(b.ready) })

	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			metrics.RedisSubscriptions.WithLabelValues("error").Inc()
			return fmt.Errorf("receive Redis event: %w", err)
		}

		var event events.Envelope
		if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
			metrics.RedisDecodes.WithLabelValues("error").Inc()
			slog.Warn("decode redis event", "namespace", b.namespace, "err", err)
			continue
		}
		if err := event.Validate(); err != nil {
			metrics.RedisDecodes.WithLabelValues("error").Inc()
			slog.Warn("validate redis event", "namespace", b.namespace, "err", err)
			continue
		}
		channel, err := events.Channel(b.namespace, event.Room)
		if err != nil || channel != message.Channel {
			metrics.RedisDecodes.WithLabelValues("error").Inc()
			slog.Warn("redis event channel mismatch", "namespace", b.namespace, "room", event.Room)
			continue
		}

		metrics.RedisDecodes.WithLabelValues("ok").Inc()
		handler(event)
	}
}

func waitForBackoff(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// Close releases the publisher and subscriber connection pool.
func (b *RedisFanout) Close() error {
	b.closeOnce.Do(func() {
		b.closeErr = b.client.Close()
	})
	return b.closeErr
}
