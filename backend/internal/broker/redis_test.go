package broker

import (
	"context"
	"testing"
	"time"
)

func TestNewRedisRequiresURL(t *testing.T) {
	_, err := NewRedis(Config{Namespace: "test", InstanceID: "instance-a"})
	if err == nil {
		t.Fatal("expected missing Redis URL to fail")
	}
}

func TestNewRedisRejectsInvalidNamespaceAndInstance(t *testing.T) {
	for name, cfg := range map[string]Config{
		"namespace": {URL: "redis://127.0.0.1:6379/0", Namespace: "test:prod", InstanceID: "instance-a"},
		"instance":  {URL: "redis://127.0.0.1:6379/0", Namespace: "test", InstanceID: "instance:a"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewRedis(cfg)
			if err == nil {
				t.Fatal("expected invalid Redis configuration to fail before dialing")
			}
		})
	}
}

func TestWaitForBackoffHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if waitForBackoff(ctx, time.Hour) {
		t.Fatal("canceled backoff should stop")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("canceled backoff took too long: %s", elapsed)
	}
}
