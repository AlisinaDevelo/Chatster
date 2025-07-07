package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// WSUpgrade limits WebSocket upgrade attempts per client IP.
type WSUpgrade struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

// NewWSUpgrade returns a per-IP limiter. rps is requests per second; burst allows short spikes.
func NewWSUpgrade(rps float64, burst int) *WSUpgrade {
	if burst < 1 {
		burst = 1
	}
	return &WSUpgrade{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(rps),
		burst:    burst,
	}
}

// Allow reports whether clientIP may open a new WebSocket now.
func (w *WSUpgrade) Allow(clientIP string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	lim, ok := w.limiters[clientIP]
	if !ok {
		lim = rate.NewLimiter(w.r, w.burst)
		w.limiters[clientIP] = lim
	}
	return lim.Allow()
}
