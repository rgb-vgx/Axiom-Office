package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/agent"
)

// Kenh ribbon/menu chuot phai -> khung chat dang mo (docs/architecture/ribbon-ui.md muc 7).
//
// Tren WPS, ribbon (main.js) va khung chat (pane.html) la hai trang CEF rieng va KHONG co kenh nao giua chung:
// Application.CreateTaskPane chi MO trang, khong dua duoc gi vao trang dang mo. Ca hai deu noi duoc voi Core, nen
// Core lam tram trung chuyen:
//
//	main.js  : POST /wps/v1/pane/request {quickAction}   - nguoi dung chon "Trau chuot" trong menu chuot phai
//	pane.html: GET  /wps/v1/pane/events  (SSE)           - khung chat nghe, nhan su kien "pane.quick" roi tu chay
//	                                                       muc do trong CHINH hoi thoai dang mo
//
// WPS co the mo ca ba app, moi app mot khung chat: yeu cau mang client id cua add-in gui no (main.js) va chi giao cho
// khung chat cua CUNG client (pane.html nghe voi ?client=). Yeu cau/khung chat khong co client id (ban cu) thi dung
// chung nhu truoc.
//
// Moi yeu cau chi giao cho MOT khung chat (khung chat nao lay truoc), va duoc GIU paneRequestTTL giay: main.js
// vua tao khung chat moi thi trang con dang nap, chua kip nghe - toi khi no noi vao van con yeu cau de lay. Qua
// han thi bo: nguoi dung bam chuot phai tu lau roi, chay luc nay la chay khi ho khong con cho.
const paneRequestTTL = 20 * time.Second

// panePingEvery: dong ": ping" giu ket noi SSE khong bi proxy/CEF cat vi im lang.
var panePingEvery = 15 * time.Second

type paneRequest struct {
	quickAction string
	at          time.Time
}

type paneRelay struct {
	mu sync.Mutex
	// pending: yeu cau dang cho theo client id ("" = khong ghi client, khung chat nao cung lay duoc).
	pending   map[string]*paneRequest
	wake      chan struct{} // dong (roi thay moi) moi khi co yeu cau moi - danh thuc moi khung chat dang cho
	listeners int
}

func newPaneRelay() *paneRelay {
	return &paneRelay{pending: map[string]*paneRequest{}, wake: make(chan struct{})}
}

// push: ghi de yeu cau dang cho (nguoi dung bam lien hai muc thi muc sau thang) va tra so khung chat dang nghe.
func (r *paneRelay) push(client, quickAction string, now time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending[client] = &paneRequest{quickAction: quickAction, at: now}
	close(r.wake)
	r.wake = make(chan struct{})
	return r.listeners
}

// take: lay VA xoa yeu cau dang cho (con han) danh cho khung chat cua `client`: yeu cau cua chinh client do, roi yeu
// cau khong ghi client; khung chat khong co client id (ban cu) lay duoc moi yeu cau. nil neu khong co. Kenh tra ve
// dong khi co yeu cau tiep theo.
func (r *paneRelay) take(client string, now time.Time) (*paneRequest, <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	keys := []string{client}
	if client != "" {
		keys = append(keys, "")
	} else {
		for key := range r.pending {
			if key != "" {
				keys = append(keys, key)
			}
		}
	}
	for _, key := range keys {
		request := r.pending[key]
		if request == nil {
			continue
		}
		delete(r.pending, key)
		if now.Sub(request.at) <= paneRequestTTL {
			return request, r.wake
		}
	}
	return nil, r.wake
}

func (r *paneRelay) listen(delta int) {
	r.mu.Lock()
	r.listeners += delta
	r.mu.Unlock()
}

func mapWPSPane(mux *http.ServeMux, deps *Deps) {
	relay := newPaneRelay()

	mux.HandleFunc("POST /wps/v1/pane/request", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id := strings.TrimSpace(textOf(body["quickAction"]))
		if _, ok := agent.FindQuickAction(id); !ok {
			Error(w, http.StatusBadRequest, "unknown 'quickAction' \""+id+"\" (see GET /v1/quick-actions)")
			return
		}
		client := strings.TrimSpace(textOf(body["client"]))
		OK(w, map[string]any{"queued": true, "listening": relay.push(client, id, time.Now())})
	})

	mux.HandleFunc("GET /wps/v1/pane/events", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher, canFlush := w.(http.Flusher)
		flush := func() {
			if canFlush {
				flusher.Flush()
			}
		}
		// Dong dau tien de EventSource bao "open" ngay, khong phai doi toi su kien dau.
		if _, err := w.Write([]byte(": ok\n\n")); err != nil {
			return
		}
		flush()

		client := strings.TrimSpace(r.URL.Query().Get("client"))
		relay.listen(1)
		defer relay.listen(-1)
		ping := time.NewTicker(panePingEvery)
		defer ping.Stop()
		for {
			request, wake := relay.take(client, time.Now())
			if request != nil {
				data, _ := json.Marshal(map[string]any{"quickAction": request.quickAction})
				if _, err := w.Write([]byte("event: pane.quick\ndata: " + string(data) + "\n\n")); err != nil {
					return
				}
				flush()
				continue
			}
			select {
			case <-r.Context().Done():
				return
			case <-wake:
			case <-ping.C:
				if _, err := w.Write([]byte(": ping\n\n")); err != nil {
					return
				}
				flush()
			}
		}
	})
}
