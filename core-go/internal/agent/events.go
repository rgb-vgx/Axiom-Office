package agent

import (
	"context"
	"sync"
	"time"
)

// Event: mot su kien cua luot chay (SSE - New_arch.md muc 7.4). Seq tang dan de client ket noi lai voi ?after=.
type Event struct {
	Seq  int64
	Type string
	Data map[string]any
	At   string
}

const eventBufferSize = 1000

// EventStream: buffer su kien + phat cho nguoi dang nghe. Nhieu subscriber doc cung mot buffer
// (kieu TaskCompletionSource cua ban .NET: moi lan phat thi dong kenh bao hieu hien tai).
type EventStream struct {
	mu     sync.Mutex
	buffer []Event
	seq    int64
	done   bool
	signal chan struct{}
}

func NewEventStream() *EventStream {
	return &EventStream{signal: make(chan struct{})}
}

func (s *EventStream) Publish(eventType string, data map[string]any) Event {
	s.mu.Lock()
	s.seq++
	item := Event{Seq: s.seq, Type: eventType, Data: data, At: nowText()}
	s.buffer = append(s.buffer, item)
	if len(s.buffer) > eventBufferSize {
		s.buffer = append([]Event{}, s.buffer[len(s.buffer)-eventBufferSize:]...)
	}
	s.notify()
	s.mu.Unlock()
	return item
}

func (s *EventStream) Complete() {
	s.mu.Lock()
	s.done = true
	s.notify()
	s.mu.Unlock()
}

func (s *EventStream) Completed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

func (s *EventStream) LastSeq() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seq
}

// Since: cac su kien co Seq > afterSeq.
func (s *EventStream) Since(afterSeq int64) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := []Event{}
	for _, item := range s.buffer {
		if item.Seq > afterSeq {
			pending = append(pending, item)
		}
	}
	return pending
}

// WaitForChange cho co su kien moi (hoac het timeout de caller gui ping). false = luot chay da ket thuc
// hoac context bi huy.
func (s *EventStream) WaitForChange(timeout time.Duration, ctx context.Context) bool {
	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return false
	}
	signal := s.signal
	s.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-signal:
		return true
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// notify: dong kenh bao hieu hien tai de moi nguoi dang cho thuc day, roi tao kenh moi (goi trong lock).
func (s *EventStream) notify() {
	close(s.signal)
	s.signal = make(chan struct{})
}

func nowText() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
