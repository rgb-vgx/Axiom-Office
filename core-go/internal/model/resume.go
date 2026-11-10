package model

import (
	"context"
	"sync/atomic"
	"time"
)

// Phat hien may vua thuc day sau khi ngu (connected standby). Dong ho don dieu cua Go tren Windows dung khi may ngu,
// nen watchdog idle 300 s cung dung theo; ket noi TCP toi nha cung cap thi chet trong luc ngu ma khong ai biet. Luot
// 25/10/2026: mot request cho model treo 1 gio 43 phut roi moi bao "empty reply" (docs/plans/19-h0 H2). Dong ho THUC
// (wall clock) thi van chay trong luc ngu: hai lan kiem cach nhau xa hon han chu ky la may vua thuc day.
var (
	resumeCheck = 10 * time.Second
	resumeGap   = 60 * time.Second
	// wallNow: dong ho thuc, bo phan don dieu (Round(0)) de phep tru khong dung dong ho don dieu.
	wallNow = func() time.Time { return time.Now().Round(0) }
)

// resumeWatch huy request khi phat hien may vua thuc day; stop() dung theo doi. Resumed() cho biet lan huy do may ngu.
type resumeWatch struct {
	resumed atomic.Bool
	done    chan struct{}
	// exited dong khi goroutine theo doi da thoat: stop() cho no de khong con ai doc dong ho sau khi stop tra ve.
	exited chan struct{}
}

func watchResume(cancel context.CancelFunc) *resumeWatch {
	watch := &resumeWatch{done: make(chan struct{}), exited: make(chan struct{})}
	check, gap, clock := resumeCheck, resumeGap, wallNow
	go func() {
		defer close(watch.exited)
		ticker := time.NewTicker(check)
		defer ticker.Stop()
		last := clock()
		for {
			select {
			case <-watch.done:
				return
			case <-ticker.C:
				now := clock()
				if now.Sub(last) > check+gap {
					watch.resumed.Store(true)
					cancel()
					return
				}
				last = now
			}
		}
	}()
	return watch
}

// stop dung theo doi va cho goroutine thoat han (test doi dong ho gia sau stop - go test -race bao race neu khong cho).
func (w *resumeWatch) stop() {
	close(w.done)
	<-w.exited
}

func (w *resumeWatch) Resumed() bool { return w.resumed.Load() }

// resumedError: loi tam thoi (IsTransient) de post thu lai ngay tren ket noi moi.
const resumedError = "HttpRequestException: connection lost while the computer was asleep"
