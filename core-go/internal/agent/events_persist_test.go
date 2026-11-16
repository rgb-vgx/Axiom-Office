package agent

import (
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/store"
)

func openAgentTestStores(t *testing.T) *store.Stores {
	t.Helper()
	stores := store.OpenStores(filepath.Join(t.TempDir(), "core.db"))
	if !stores.Available {
		t.Fatalf("stores khong san sang: %s", stores.Error)
	}
	t.Cleanup(func() { _ = stores.DB.Close() })
	return stores
}

// Sink nhan moi su kien sau khi vao buffer; khi buffer bo su kien cu thi phat them stream.lagged.
func TestEventStreamSinkAndLagged(t *testing.T) {
	stream := NewEventStream()
	seen := []Event{}
	stream.SetSink(func(event Event) { seen = append(seen, event) })

	published := eventBufferSize + 5
	for i := 0; i < published; i++ {
		stream.Publish("run.progress", map[string]any{"i": i})
	}

	// Buffer khong vuot qua gioi han (mot su kien lagged duoc chen sau khi cat).
	if got := len(stream.Since(0)); got > eventBufferSize+1 {
		t.Fatalf("buffer = %d su kien, muon <= %d", got, eventBufferSize+1)
	}
	if stream.LastSeq() <= int64(published) {
		t.Fatalf("phai co them seq cho stream.lagged: lastSeq=%d", stream.LastSeq())
	}

	lagged := []Event{}
	for _, event := range seen {
		if event.Type == "stream.lagged" {
			lagged = append(lagged, event)
		}
	}
	if len(lagged) == 0 {
		t.Fatalf("phai phat stream.lagged khi buffer bo su kien: %+v", seen[:1])
	}
	if dropped, _ := lagged[0].Data["dropped"].(int); dropped < 1 {
		t.Fatalf("stream.lagged phai co so su kien bi bo, duoc %v", lagged[0].Data)
	}
	if lastSeq, _ := lagged[0].Data["lastSeq"].(int64); lastSeq < 1 {
		t.Fatalf("stream.lagged phai co seq cuoi cung bi bo, duoc %v", lagged[0].Data)
	}
	// Sink thay moi su kien da phat (ke ca su kien lagged) - khong mat du lieu ben.
	if got := len(seen); got != published+len(lagged) {
		t.Fatalf("sink nhan %d su kien, muon %d", got, published+len(lagged))
	}

	// Su kien lagged nam trong buffer de nguoi dang nghe SSE thay no.
	bufferedLagged := false
	for _, event := range stream.Since(0) {
		if event.Type == "stream.lagged" {
			bufferedLagged = true
		}
	}
	if !bufferedLagged {
		t.Fatal("stream.lagged phai co trong buffer")
	}
}

// Manager gan sink khi tao Run: su kien di vao run_events de replay duoc sau khi Core khoi dong lai.
func TestManagerPersistsRunEvents(t *testing.T) {
	stores := openAgentTestStores(t)
	manager := NewManager()
	manager.SetRunEvents(stores.Runs)

	run, problem := manager.TryStart(47899)
	if run == nil || problem != "" {
		t.Fatalf("TryStart = %+v %q", run, problem)
	}
	run.Events.Publish("run.started", map[string]any{"runId": run.ID, "round": 0})
	run.Events.Publish("run.completed", nil)

	rows := stores.Runs.EventsSince(run.ID, 0)
	if len(rows) != 2 {
		t.Fatalf("phai luu 2 su kien, duoc %+v", rows)
	}
	if rows[0].Type != "run.started" || rows[0].Seq != 1 || rows[1].Type != "run.completed" || rows[1].Seq != 2 {
		t.Fatalf("su kien luu sai thu tu/noi dung: %+v", rows)
	}
	if rows[0].DataJSON != `{"round":0,"runId":"`+run.ID+`"}` {
		t.Fatalf("payload JSON phai giu nguyen: %s", rows[0].DataJSON)
	}
	if rows[1].DataJSON != "null" {
		t.Fatalf("data nil phai luu la null: %s", rows[1].DataJSON)
	}
}

// Luot khong con trong RAM van replay duoc tu run_events (API test day du o internal/api).
func TestRunEventsSurviveManagerCleanup(t *testing.T) {
	stores := openAgentTestStores(t)
	manager := NewManager()
	manager.SetRunEvents(stores.Runs)
	run, _ := manager.TryStart(47898)
	run.Events.Publish("run.started", nil)

	stores.Runs.Insert(run.ID, nil, "m")
	stores.Runs.Finish(run.ID, StatusCompleted, 1, 1, 1, 0, nil, nil)
	old := store.TimeText(time.Now().UTC().Add(-(RetentionMinutes + 1) * time.Minute))
	if _, err := stores.DB.SQL().Exec("UPDATE runs SET finished_at = ? WHERE id = ?", old, run.ID); err != nil {
		t.Fatalf("dat finished_at cu: %v", err)
	}

	// finish() cua luot sau don nhat ky cua luot da qua retention.
	o := &Orchestrator{Runs: stores.Runs}
	o.finish(newRun("r_next", 47897))

	if got := stores.Runs.EventsSince(run.ID, 0); len(got) != 0 {
		t.Fatalf("su kien cua luot qua retention phai bi don: %+v", got)
	}
}
