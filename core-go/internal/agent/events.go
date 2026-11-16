package agent

import (
	"context"
	"sync"
	"time"
)

// Event: mot su kien cua luot chay (SSE - docs/architecture/core.md muc 7.4). Seq tang dan de client ket noi lai voi ?after=.
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
	// lagged: su kien stream.lagged gan nhat (buffer da bo su kien cu). Nam ngoai buffer nen khong bi
	// tinh vao so su kien bi bo lan sau.
	lagged *Event
	// sink: noi nhan tung su kien de luu ben (Agent Run gan sink ghi core.db run_events). nil = chi RAM.
	sink func(Event)
}

func NewEventStream() *EventStream {
	return &EventStream{signal: make(chan struct{})}
}

// SetSink: gan noi nhan su kien ben ngoai buffer (luu ben, do luong...). Goi truoc khi phat su kien
// dau tien. Sink chay ngoai khoa cua stream nen khong duoc goi nguoc vao Publish cung stream.
func (s *EventStream) SetSink(sink func(Event)) {
	s.mu.Lock()
	s.sink = sink
	s.mu.Unlock()
}

func (s *EventStream) Publish(eventType string, data map[string]any) Event {
	s.mu.Lock()
	s.seq++
	item := Event{Seq: s.seq, Type: eventType, Data: data, At: nowText()}
	s.buffer = append(s.buffer, item)
	// Buffer chi giu eventBufferSize su kien moi nhat. Su kien bi bo duoc bao cho nguoi nghe bang mot
	// su kien stream.lagged (kem so luong bi bo va seq cuoi cung bi bo) de pane biet no da mat du lieu.
	var lagged *Event
	if len(s.buffer) > eventBufferSize {
		dropped := len(s.buffer) - eventBufferSize
		lastDroppedSeq := s.buffer[dropped-1].Seq
		s.buffer = append([]Event{}, s.buffer[dropped:]...)
		s.seq++
		laggedItem := Event{
			Seq: s.seq, Type: "stream.lagged", At: nowText(),
			Data: map[string]any{"dropped": dropped, "lastSeq": lastDroppedSeq},
		}
		s.lagged = &laggedItem
		lagged = &laggedItem
	}
	s.notify()
	sink := s.sink
	s.mu.Unlock()
	// Ghi ben ngoai khoa: mot lan ghi cham (SQLite) khong chan nguoi dang nghe doc buffer.
	if sink != nil {
		sink(item)
		if lagged != nil {
			sink(*lagged)
		}
	}
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
	if s.lagged != nil && s.lagged.Seq > afterSeq {
		pending = append(pending, *s.lagged)
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
