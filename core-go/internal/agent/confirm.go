package agent

import (
	"context"
	"sync"
	"time"

	"axiomoffice/core/internal/store"
)

// Confirmations: cho xac nhan cua nguoi dung trong mot run: confirm.required ->
// POST /v1/runs/{id}/confirm; het gio = tu choi.
type Confirmations struct {
	mu      sync.Mutex
	pending map[string]chan bool
}

func newConfirmations() *Confirmations {
	return &Confirmations{pending: map[string]chan bool{}}
}

func (c *Confirmations) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// Request: phat confirm.required roi cho tra loi (hoac het gio / bi huy).
func (c *Confirmations) Request(ctx context.Context, events *EventStream, action, reason, paramsPreview string,
	timeout time.Duration) bool {
	id := store.NewID("cf_")[:15] // "cf_" + 12 ky tu
	waiter := make(chan bool, 1)

	c.mu.Lock()
	c.pending[id] = waiter
	c.mu.Unlock()

	events.Publish("confirm.required", map[string]any{
		"confirmationId": id,
		"action":         action,
		"reason":         reason,
		"paramsPreview":  nilIfEmpty(paramsPreview),
		"timeoutSeconds": int(timeout.Seconds()),
	})

	approved, how := false, "user"
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case approved = <-waiter:
	case <-timer.C:
		how = "timeout"
	case <-ctx.Done():
		how = "cancelled"
	}

	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()

	events.Publish("confirm.resolved", map[string]any{"confirmationId": id, "approved": approved, "by": how})
	return approved
}

func (c *Confirmations) Resolve(confirmationID string, approved bool) bool {
	c.mu.Lock()
	waiter, found := c.pending[confirmationID]
	c.mu.Unlock()
	if !found {
		return false
	}
	select {
	case waiter <- approved:
		return true
	default:
		return false
	}
}
