package agent

import (
	"context"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/tools"
)

// Confirmations: cho xac nhan cua nguoi dung trong mot run: confirm.required ->
// POST /v1/runs/{id}/confirm; het gio = tu choi. Cau tra loi la "yes"/"no" hoac id mot lua chon (Choose).
type Confirmations struct {
	mu      sync.Mutex
	pending map[string]chan answer
}

// answer: lua chon ("yes"/"no"/id) va chu nguoi dung go (lua chon Input).
type answer struct{ choice, text string }

func newConfirmations() *Confirmations {
	return &Confirmations{pending: map[string]chan answer{}}
}

func (c *Confirmations) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// Request: phat confirm.required roi cho tra loi (hoac het gio / bi huy).
func (c *Confirmations) Request(ctx context.Context, events *EventStream, action, reason, paramsPreview string,
	timeout time.Duration) bool {
	return c.ask(ctx, events, action, reason, paramsPreview, nil, timeout).choice == "yes"
}

// Choose: the nhieu lua chon (nguoi dung 15/10/2026: "AI hoi luon user, cho multiple choice ngay tu dau"). Tra id lua
// chon va chu nguoi dung go (lua chon Input); "" = khong tra loi (het gio, huy, tu choi). Khung chat cu chi biet
// Dong y/Tu choi: dong y = lua chon dau.
func (c *Confirmations) Choose(ctx context.Context, events *EventStream, action, question string, choices []tools.Choice,
	timeout time.Duration) (string, string) {
	reply := c.ask(ctx, events, action, question, "", choices, timeout)
	if reply.choice == "yes" && len(choices) > 0 {
		return choices[0].ID, ""
	}
	for _, choice := range choices {
		if choice.ID != reply.choice {
			continue
		}
		if choice.Input {
			// O tu go ma de trong = chua chon.
			if strings.TrimSpace(reply.text) == "" {
				return "", ""
			}
			return choice.ID, strings.TrimSpace(reply.text)
		}
		return choice.ID, ""
	}
	return "", ""
}

func (c *Confirmations) ask(ctx context.Context, events *EventStream, action, reason, paramsPreview string,
	choices []tools.Choice, timeout time.Duration) answer {
	id := store.NewID("cf_")[:15] // "cf_" + 12 ky tu
	waiter := make(chan answer, 1)

	c.mu.Lock()
	c.pending[id] = waiter
	c.mu.Unlock()

	payload := map[string]any{
		"confirmationId": id,
		"action":         action,
		"reason":         reason,
		"paramsPreview":  nilIfEmpty(paramsPreview),
		"timeoutSeconds": int(timeout.Seconds()),
	}
	if len(choices) > 0 {
		payload["choices"] = choices
	}
	events.Publish("confirm.required", payload)

	reply, how := answer{}, "user"
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply = <-waiter:
	case <-timer.C:
		how = "timeout"
	case <-ctx.Done():
		how = "cancelled"
	}

	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()

	resolved := map[string]any{"confirmationId": id, "approved": reply.choice != "" && reply.choice != "no", "by": how}
	if len(choices) > 0 && reply.choice != "yes" && reply.choice != "no" {
		resolved["choice"] = nilIfEmpty(reply.choice)
		resolved["text"] = nilIfEmpty(reply.text)
	}
	events.Publish("confirm.resolved", resolved)
	return reply
}

// Resolve: tra loi Dong y/Tu choi.
func (c *Confirmations) Resolve(confirmationID string, approved bool) bool {
	if approved {
		return c.ResolveChoice(confirmationID, "yes")
	}
	return c.ResolveChoice(confirmationID, "no")
}

// ResolveChoice: tra loi bang id mot lua chon (hoac "yes"/"no").
func (c *Confirmations) ResolveChoice(confirmationID, choice string) bool {
	return c.ResolveAnswer(confirmationID, choice, "")
}

// ResolveAnswer: tra loi bang id lua chon kem chu nguoi dung go (lua chon Input).
func (c *Confirmations) ResolveAnswer(confirmationID, choice, text string) bool {
	c.mu.Lock()
	waiter, found := c.pending[confirmationID]
	c.mu.Unlock()
	if !found {
		return false
	}
	select {
	case waiter <- answer{choice: choice, text: text}:
		return true
	default:
		return false
	}
}
