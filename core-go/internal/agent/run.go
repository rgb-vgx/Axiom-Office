package agent

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/store"
)

// Trang thai cua mot luot chay.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusTimedOut  = "timedout"
	StatusStopped   = "stopped"
)

// RetentionMinutes: giu ket qua 30 phut sau khi xong de client doc ket qua va SSE ket noi lai.
const RetentionMinutes = 30

// Run: trang thai mot luot chay (New_arch.md muc 7.3, 8.1).
type Run struct {
	ID             string
	Port           int
	ConversationID string
	Status         string
	Started        time.Time
	Finished       *time.Time
	Reply          string
	Error          string
	ErrorKind      string
	Rounds         int
	InputTokens    int
	OutputTokens   int
	Seconds        float64
	ToolCalls      int
	Transcript     []string
	Events         *EventStream
	Confirmations  *Confirmations

	ctx    context.Context
	cancel context.CancelFunc
}

func newRun(id string, port int) *Run {
	ctx, cancel := context.WithCancel(context.Background())
	return &Run{
		ID: id, Port: port, Status: StatusRunning, Started: time.Now().UTC(),
		Transcript: []string{}, Events: NewEventStream(), Confirmations: newConfirmations(),
		ctx: ctx, cancel: cancel,
	}
}

// Context: context cua luot chay (huy khi nguoi dung bam Huy hoac Core dung).
func (r *Run) Context() context.Context { return r.ctx }

func (r *Run) Cancel() { r.cancel() }

func (r *Run) finishedText() any {
	if r.Finished == nil {
		return nil
	}
	return r.Finished.UTC().Format("2006-01-02T15:04:05.000Z")
}

// JSON: hinh dang GET /v1/runs/{id}.
func (r *Run) JSON() map[string]any {
	transcript := make([]string, len(r.Transcript))
	copy(transcript, r.Transcript)
	payload := map[string]any{
		"runId":          r.ID,
		"conversationId": nilIfEmpty(r.ConversationID),
		"port":           r.Port,
		"status":         r.Status,
		"started":        r.Started.Format("2006-01-02T15:04:05.000Z"),
		"finished":       r.finishedText(),
		"reply":          nilIfEmpty(r.Reply),
		"error":          nilIfEmpty(r.Error),
		"errorKind":      nilIfEmpty(r.ErrorKind),
		"rounds":         r.Rounds,
		"inputTokens":    r.InputTokens,
		"outputTokens":   r.OutputTokens,
		"seconds":        round2(r.Seconds),
		"toolCalls":      r.ToolCalls,
		"lastEvent":      r.Events.LastSeq(),
		"transcript":     transcript,
	}
	return payload
}

// Manager: quan ly cac luot chay - mot luot dang chay cho moi office port, huy duoc, giu ket qua 30 phut.
type Manager struct {
	mu   sync.Mutex
	runs map[string]*Run
}

func NewManager() *Manager { return &Manager{runs: map[string]*Run{}} }

func (m *Manager) TryStart(port int) (*Run, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanup()
	for _, existing := range m.runs {
		if existing.Port == port && existing.Status == StatusRunning {
			return nil, "busy: a run is already in progress on port " + strconv.Itoa(port) + " (run " + existing.ID + ")"
		}
	}
	run := newRun(store.NewID("r_"), port)
	m.runs[run.ID] = run
	return run, ""
}

func (m *Manager) Get(id string) *Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[id]
}

func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	run, found := m.runs[id]
	m.mu.Unlock()
	if !found || run.Status != StatusRunning {
		return false
	}
	run.Cancel()
	return true
}

// CancelRunning: huy moi luot dang chay (Core sap dung) va tra ve so luot vua huy.
// Goi lan hai tra 0 vi cac luot do da co context ket thuc - dung cho shutdown idempotent.
func (m *Manager) CancelRunning() int {
	m.mu.Lock()
	list := make([]*Run, 0, len(m.runs))
	for _, run := range m.runs {
		if run.Status == StatusRunning {
			list = append(list, run)
		}
	}
	m.mu.Unlock()

	cancelled := 0
	for _, run := range list {
		if run.ctx.Err() != nil {
			continue
		}
		run.Cancel()
		cancelled++
	}
	return cancelled
}

// Running: so luot con dang chay (Core cho chung dung han truoc khi thoat).
func (m *Manager) Running() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, run := range m.runs {
		if run.Status == StatusRunning {
			count++
		}
	}
	return count
}

func (m *Manager) List() []*Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanup()
	list := make([]*Run, 0, len(m.runs))
	for _, run := range m.runs {
		list = append(list, run)
	}
	return list
}

func (m *Manager) cleanup() {
	cutoff := time.Now().UTC().Add(-RetentionMinutes * time.Minute)
	for id, run := range m.runs {
		if run.Status != StatusRunning && run.Finished != nil && run.Finished.Before(cutoff) {
			delete(m.runs, id)
		}
	}
}

// nilIfEmpty: ban .NET tra null cho chuoi rong (pane phan biet "chua co" voi "rong").
func nilIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
