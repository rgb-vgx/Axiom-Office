package memory

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Spec (e2e "khoi phuc memory da xoa" chap chon, 08-11/10/2026): ghi memory qua API (Delete/Restore) va trich
// xuat nen (AddBatch) chay dong thoi tren cung core.db. Transaction mo bang BEGIN DEFERRED doc truoc ghi sau:
// trong WAL, khi ket noi khac vua ghi, viec nang khoa len ghi tra SQLITE_BUSY NGAY (khong cho busy_timeout) -
// va loi cua tx.Exec bi bo qua nen Restore van tra true ma khong ghi gi. Phai: transaction ghi lay khoa ghi
// ngay tu dau (BEGIN IMMEDIATE, cho busy_timeout) va khong bao gio bao thanh cong khi lenh ghi loi.
func TestConcurrentWritesNeverReportSuccessWithoutWriting(t *testing.T) {
	store := newTestStore(t)
	const count = 30
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		results := store.AddBatch([]NewMemory{{Scope: "user", Text: fmt.Sprintf("Su that so %d de xoa roi khoi phuc", i)}},
			ActorUser, "", nil, "")
		if len(results) != 1 || results[0].Item == nil {
			t.Fatalf("khong tao duoc memory %d: %+v", i, results)
		}
		ids = append(ids, results[0].Item.ID)
	}
	for _, id := range ids {
		if !store.Delete(id, "test") {
			t.Fatalf("khong xoa duoc %s", id)
		}
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for writer := 0; writer < 4; writer++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				store.AddBatch([]NewMemory{{Scope: "user", Text: fmt.Sprintf("Ghi nen %d-%d", writer, n)}}, ActorExtract, "", nil, "")
				// Ghi nen that (trich xuat sau luot chay) co nghi giua cac lan; vong ghi khong nghi lam doi khoa.
				time.Sleep(2 * time.Millisecond)
			}
		}(writer)
	}
	restored := make([]bool, count)
	var restorers sync.WaitGroup
	for i, id := range ids {
		restorers.Add(1)
		go func(i int, id string) {
			defer restorers.Done()
			restored[i] = store.Restore(id)
		}(i, id)
	}
	restorers.Wait()
	close(stop)
	wg.Wait()

	for i, id := range ids {
		item := store.Get(id)
		if item == nil {
			t.Fatalf("mat memory %s", id)
		}
		if restored[i] && item.DeletedAt != nil {
			t.Errorf("Restore(%s) bao thanh cong nhung memory van bi xoa (loi ghi bi bo qua)", id)
		}
		if !restored[i] {
			t.Errorf("Restore(%s) that bai khi co ghi dong thoi - transaction ghi phai cho khoa (BEGIN IMMEDIATE + busy_timeout)", id)
		}
	}
}
