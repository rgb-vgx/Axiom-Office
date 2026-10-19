package api

import (
	"testing"
	"time"
)

// Tram trung chuyen menu chuot phai -> khung chat (wpspane.go).
func TestPaneRelay(t *testing.T) {
	relay := newPaneRelay()
	now := time.Now()

	if request, _ := relay.take(now); request != nil {
		t.Fatalf("chua co gi ma da co yeu cau: %+v", request)
	}

	// Khung chat dang cho thi duoc danh thuc khi co yeu cau.
	_, wake := relay.take(now)
	if listening := relay.push("polish", now); listening != 0 {
		t.Fatalf("chua khung chat nao nghe, duoc %d", listening)
	}
	select {
	case <-wake:
	default:
		t.Fatal("push phai danh thuc khung chat dang cho")
	}

	// Giao dung MOT lan: khung chat thu hai khong chay lai cung muc do.
	if request, _ := relay.take(now); request == nil || request.quickAction != "polish" {
		t.Fatalf("phai lay duoc polish, duoc %+v", request)
	}
	if request, _ := relay.take(now); request != nil {
		t.Fatal("yeu cau da giao thi khong giao lan nua")
	}

	// Bam lien hai muc: muc sau thang (chi chay viec nguoi dung chon CUOI).
	relay.push("polish", now)
	relay.push("shorten", now)
	if request, _ := relay.take(now); request == nil || request.quickAction != "shorten" {
		t.Fatalf("muc sau phai thang, duoc %+v", request)
	}

	// Khung chat moi nap trong vai giay van lay duoc; qua han thi bo - nguoi dung da khong con cho.
	relay.push("fix", now)
	if request, _ := relay.take(now.Add(paneRequestTTL - time.Second)); request == nil {
		t.Fatal("con han phai lay duoc")
	}
	relay.push("fix", now)
	if request, _ := relay.take(now.Add(paneRequestTTL + time.Second)); request != nil {
		t.Fatal("qua han phai bo")
	}
	if request, _ := relay.take(now); request != nil {
		t.Fatal("yeu cau qua han phai bi xoa, khong nam cho")
	}

	relay.listen(1)
	relay.listen(1)
	relay.listen(-1)
	if listening := relay.push("fix", now); listening != 1 {
		t.Fatalf("dem khung chat dang nghe sai: %d", listening)
	}
}
