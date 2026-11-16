package agent

import (
	"context"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/tools"
)

// Confirmations: cho xac nhan cua nguoi dung trong mot run: confirm.required ->
// POST /v1/runs/{id}/confirm; het gio = tu choi. Cau tra loi la "yes"/"no" hoac id mot lua chon (Choose).
type Confirmations struct {
	mu      sync.Mutex
	pending map[string]*pendingConfirmation
}

// pendingConfirmation: kenh cho cau tra loi + action/resource cua yeu cau. Action/resource de
// POST /v1/runs/{id}/confirm luu phe duyet khi nguoi dung dong y kem always=true (schema v5).
type pendingConfirmation struct {
	answer   chan answer
	action   string
	resource string
}

// answer: lua chon ("yes"/"no"/id) va chu nguoi dung go (lua chon Input).
type answer struct{ choice, text string }

func newConfirmations() *Confirmations {
	return &Confirmations{pending: map[string]*pendingConfirmation{}}
}

func (c *Confirmations) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// Request: phat confirm.required roi cho tra loi (hoac het gio / bi huy).
func (c *Confirmations) Request(ctx context.Context, events *EventStream, action, reason, paramsPreview string,
	timeout time.Duration) bool {
	return c.RequestFor(ctx, events, action, "", reason, paramsPreview, timeout)
}

// RequestFor: nhu Request nhung kem resource (tai lieu) cua lenh - de luu phe duyet "luon dong y".
func (c *Confirmations) RequestFor(ctx context.Context, events *EventStream, action, resource, reason,
	paramsPreview string, timeout time.Duration) bool {
	return c.ask(ctx, events, action, resource, reason, paramsPreview, nil, timeout).choice == "yes"
}

// Choose: the nhieu lua chon (nguoi dung 15/10/2026: "AI hoi luon user, cho multiple choice ngay tu dau"). Tra id lua
// chon va chu nguoi dung go (lua chon Input); "" = khong tra loi (het gio, huy, tu choi). Khung chat cu chi biet
// Dong y/Tu choi: dong y = lua chon dau.
func (c *Confirmations) Choose(ctx context.Context, events *EventStream, action, question string, choices []tools.Choice,
	timeout time.Duration) (string, string) {
	reply := c.ask(ctx, events, action, "", question, "", choices, timeout)
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

// PendingAction: action/resource cua mot yeu cau dang cho - POST /v1/runs/{id}/confirm doc truoc khi
// Resolve de luu phe duyet khi always=true. Cau tra loi da xu ly xong thi khong con (false).
func (c *Confirmations) PendingAction(confirmationID string) (string, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pending, found := c.pending[confirmationID]
	if !found {
		return "", "", false
	}
	return pending.action, pending.resource, true
}

func (c *Confirmations) ask(ctx context.Context, events *EventStream, action, resource, reason, paramsPreview string,
	choices []tools.Choice, timeout time.Duration) answer {
	id := store.NewID("cf_")[:15] // "cf_" + 12 ky tu
	waiter := make(chan answer, 1)
	pending := &pendingConfirmation{answer: waiter, action: action, resource: resource}

	c.mu.Lock()
	c.pending[id] = pending
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
	pending, found := c.pending[confirmationID]
	c.mu.Unlock()
	if !found {
		return false
	}
	select {
	case pending.answer <- answer{choice: choice, text: text}:
		return true
	default:
		return false
	}
}

// requestConfirmation: cong xac nhan duy nhat cua luot - TRUOC khi hoi nguoi dung, xem da co phe duyet
// luu trong core.db chua (POST /v1/approvals, hoac always=true o POST /v1/runs/{id}/confirm). Co thi
// bo qua cau hoi va ghi audit "approved-by-policy"; khong thi hoi nhu cu (het gio = tu choi).
func (o *Orchestrator) requestConfirmation(ctx context.Context, run *Run, action, resource, reason, preview string,
	timeout time.Duration) bool {
	if o.Approvals != nil {
		if approval := o.Approvals.ActiveApproval(action, resource); approval != nil {
			corelog.Info("run %s: %s on %q approved by saved approval %s (scope %s)", run.ID, action, resource,
				approval.ID, approval.Scope)
			run.Events.Publish("confirm.resolved", map[string]any{
				"confirmationId": nil, "approved": true, "by": "policy",
				"action": action, "approvalId": approval.ID,
			})
			// Audit: mot dong tool_calls danh dau cau hoi da duoc bo qua theo chinh sach (GET /v1/audit).
			if o.Runs != nil {
				o.Runs.AddToolCall(run.ID, run.transcriptLen(), "policy.approval", optional(action), nil, true, nil, 0, 0)
			}
			return true
		}
	}
	return run.Confirmations.RequestFor(ctx, run.Events, action, resource, reason, preview, timeout)
}
