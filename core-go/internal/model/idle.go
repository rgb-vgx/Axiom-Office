package model

import (
	"context"
	"io"
	"sync/atomic"
	"time"
)

// idleReader huy request khi khong nhan duoc byte nao trong mot khoang thoi gian.
//
// Thay cho tran thoi gian tong cua ca request: mot luot agent co the chay lau (model suy luan viet
// rat cham), nhung mot ket noi treo thi phai bi cat. Dong ho chi chay khi KHONG co du lieu, nen
// model viet cham van song - giong chunkTimeout cua goclaw.
type idleReader struct {
	inner   io.Reader
	timeout time.Duration
	cancel  context.CancelFunc
	timer   *time.Timer
	stalled atomic.Bool
}

func newIdleReader(inner io.Reader, timeout time.Duration, cancel context.CancelFunc) *idleReader {
	return &idleReader{inner: inner, timeout: timeout, cancel: cancel}
}

func (r *idleReader) Read(p []byte) (int, error) {
	if r.timeout > 0 && r.timer == nil {
		// Dong ho phai chay tu lan doc dau tien, khong phai tu byte dau tien: ket noi treo ngay tu
		// dau (khong bao gio tra byte nao) cung phai bi cat.
		r.timer = time.AfterFunc(r.timeout, func() {
			r.stalled.Store(true)
			r.cancel()
		})
	}
	n, err := r.inner.Read(p)
	if r.timer != nil {
		if n > 0 {
			r.timer.Reset(r.timeout)
		}
		if err != nil {
			r.timer.Stop()
		}
	}
	return n, err
}

// Stalled cho biet lan doc hong vua roi la do het thoi gian cho du lieu.
func (r *idleReader) Stalled() bool { return r.stalled.Load() }
