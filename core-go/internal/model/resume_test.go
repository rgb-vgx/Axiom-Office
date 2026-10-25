package model

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// May ngu giua request: dong ho thuc nhay 2 gio -> huy request dang treo va thu lai ngay, khong doi watchdog idle
// (docs/plans/19-h0 H2).
func TestRequestRetriesAfterComputerWakesUp(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 1 {
			<-r.Context().Done() // ket noi "chet" trong luc may ngu: khong bao gio tra loi
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"xong"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer server.Close()

	restore := fakeWallClock(t, 2*time.Hour)
	defer restore()

	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	client.RetryDelays = []time.Duration{time.Millisecond}
	started := time.Now()
	text, errText, status := client.post(context.Background(), context.Background(), map[string]any{"messages": []any{}}, false, nil)
	if text == nil || status != 200 {
		t.Fatalf("phai thu lai va thanh cong: err=%q status=%d calls=%d", errText, status, calls.Load())
	}
	if calls.Load() != 2 {
		t.Fatalf("phai goi lai dung mot lan, goi %d lan", calls.Load())
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("phat hien may thuc day qua cham: %s", time.Since(started))
	}
}

func TestResumeWatchIgnoresNormalTicks(t *testing.T) {
	restore := fakeWallClock(t, 0)
	defer restore()
	cancelled := make(chan struct{})
	watch := watchResume(func() { close(cancelled) })
	defer watch.stop()
	select {
	case <-cancelled:
		t.Fatal("dong ho deu thi khong duoc huy")
	case <-time.After(100 * time.Millisecond):
	}
	if watch.Resumed() {
		t.Fatal("khong co lan ngu nao")
	}
}

// fakeWallClock: chu ky kiem 5 ms; lan doc thu hai cua dong ho thuc nhay them `jump` (0 = deu theo nhip).
func fakeWallClock(t *testing.T, jump time.Duration) func() {
	t.Helper()
	oldCheck, oldGap, oldNow := resumeCheck, resumeGap, wallNow
	resumeCheck, resumeGap = 5*time.Millisecond, time.Second
	var mu sync.Mutex
	reads := 0
	base := time.Now().Round(0)
	wallNow = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		reads++
		now := base.Add(time.Duration(reads) * resumeCheck)
		if reads >= 2 {
			now = now.Add(jump)
		}
		return now
	}
	return func() { resumeCheck, resumeGap, wallNow = oldCheck, oldGap, oldNow }
}
