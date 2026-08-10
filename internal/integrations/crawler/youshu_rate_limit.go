package crawler

import (
	"context"
	"sync"
	"time"
)

type YouShuClock interface{ Now() time.Time }
type realYouShuClock struct{}

func (realYouShuClock) Now() time.Time { return time.Now() }

// YouShuGate enforces an intentionally conservative local policy. It is safe
// for concurrent use; the defaults are one request at a time and five per sec.
type YouShuGate struct {
	Clock             YouShuClock
	MaxConcurrent     int
	RequestsPerSecond int
	Cooldown          time.Duration
	mu                sync.Mutex
	slots             chan struct{}
	starts            []time.Time
	coolingUntil      time.Time
	probe             bool
}

func (g *YouShuGate) setup() {
	if g.Clock == nil {
		g.Clock = realYouShuClock{}
	}
	if g.MaxConcurrent < 1 {
		g.MaxConcurrent = 1
	}
	if g.MaxConcurrent > 2 {
		g.MaxConcurrent = 2
	}
	if g.RequestsPerSecond < 1 {
		g.RequestsPerSecond = 5
	}
	if g.RequestsPerSecond > 8 {
		g.RequestsPerSecond = 8
	}
	if g.Cooldown <= 0 {
		g.Cooldown = 5 * time.Minute
	}
	if g.slots == nil {
		g.slots = make(chan struct{}, g.MaxConcurrent)
	}
}

func (g *YouShuGate) Acquire(ctx context.Context) error {
	g.mu.Lock()
	g.setup()
	now := g.Clock.Now()
	if now.Before(g.coolingUntil) || (!g.coolingUntil.IsZero() && !now.Before(g.coolingUntil) && g.probe) {
		g.mu.Unlock()
		return &YouShuError{Kind: YouShuRateLimited, Strategy: YouShuRetryLater, Source: "local_cooldown"}
	}
	if !g.coolingUntil.IsZero() && !now.Before(g.coolingUntil) {
		g.probe = true
	}
	cutoff := now.Add(-time.Second)
	kept := g.starts[:0]
	for _, at := range g.starts {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	g.starts = kept
	if len(g.starts) >= g.RequestsPerSecond {
		g.mu.Unlock()
		return &YouShuError{Kind: YouShuThrottled, Strategy: YouShuRetryLater, Source: "local_rate"}
	}
	g.mu.Unlock()
	select {
	case g.slots <- struct{}{}:
	case <-ctx.Done():
		return contextError(ctx)
	}
	g.mu.Lock()
	g.starts = append(g.starts, g.Clock.Now())
	g.mu.Unlock()
	return nil
}
func (g *YouShuGate) Done(success bool, rateLimited bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.setup()
	if rateLimited || (g.probe && !success) {
		g.coolingUntil = g.Clock.Now().Add(g.Cooldown)
	}
	if g.probe {
		g.probe = false
		if success {
			g.coolingUntil = time.Time{}
		}
	}
	<-g.slots
}
func contextError(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return &YouShuError{Kind: YouShuTimeout, Strategy: YouShuRetry, Source: "context"}
	}
	return &YouShuError{Kind: YouShuTransport, Strategy: YouShuRetry, Source: "context"}
}
