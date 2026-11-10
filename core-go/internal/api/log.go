package api

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// GET /v1/log: phan cuoi nhat ky Core, cho nut "Mo nhat ky" tren ribbon WPS (docs/architecture/ribbon-ui.md muc 2).
//
// Ban Windows lam viec nay bang Process.Start(Logger.LogFilePath) - giao cho trinh soan thao cua he
// dieu hanh. Trong CEF khong mo duoc chuong trinh ngoai, nen pane phai DOC noi dung roi tu hien thi.
//
// Duong nay la routeBoth: /v1/log (co token, cho script/MCP) va /wps/v1/log (kenh pane, xac thuc kenh
// add-in). No tra ve dung tep ma ban Windows mo - khong loc, khong che - nen no khong mo them gi moi:
// tep nam trong thu muc du lieu cua chinh nguoi dung, va nguoi nguoi dung da doc duoc no bang trinh
// soan thao. Xem docs/architecture/ribbon-ui.md muc 5.2.
//
// CHI DOC PHAN CUOI, co tran: corelog xoay file o 10MB, doc ca file vao mot response vua lang phi vua
// co the treo pane (CEF phai render tung dong).
const (
	defaultLogLines = 200
	maxLogLines     = 2000
	// maxLogBytes: doc toi da bay nhieu byte tu CUOI file. 2000 dong log dai trung binh ~150 byte van
	// nam trong tran nay; dong dai hon thi bi cat bot va `truncated` noi ro.
	maxLogBytes = 512 * 1024
)

func mapLog(mux *http.ServeMux, deps *Deps) {
	serve := func(w http.ResponseWriter, r *http.Request) {
		limit := clampLogLines(r.URL.Query().Get("lines"))
		payload, err := tailLog(deps.Paths.LogFile, limit, maxLogBytes)
		if err != nil {
			// Loi doc file that (khong phai "chua co file"): noi ro duong dan de con tim duoc.
			Error(w, http.StatusInternalServerError, "cannot read "+deps.Paths.LogFile+": "+err.Error())
			return
		}
		OK(w, payload)
	}

	// /v1/log: duong co token (script, MCP, doctor) - luon co, ke ca khi kenh WPS tat.
	mux.HandleFunc("GET /v1/log", serve)

	// /wps/v1/log: duong cho pane trong WPS. Chi dang ky khi kenh WPS bat, giong mapWPS - khong co
	// bridge thi ca /wps/v1/* deu 404, khong rieng gi mot duong nay song.
	if deps.WPS != nil {
		mux.HandleFunc("GET /wps/v1/log", func(w http.ResponseWriter, r *http.Request) {
			if !checkWPSAuth(w, r, deps) {
				return
			}
			serve(w, r)
		})
	}
}

func clampLogLines(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return defaultLogLines
	}
	if value > maxLogLines {
		return maxLogLines
	}
	return value
}

// tailLog doc `limit` dong cuoi cua tep log, toi da `maxBytes` byte tu cuoi.
//
// Tep CHUA CO khong phai loi: Core moi cai chua ghi dong nao, va do la trang thai binh thuong - tra
// exists:false de pane noi "chua co nhat ky" thay vi hien mot cau loi do.
func tailLog(path string, limit, maxBytes int) (map[string]any, error) {
	payload := map[string]any{"path": path, "lines": []string{}, "truncated": false}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			payload["exists"] = false
			return payload, nil
		}
		return nil, err
	}
	payload["exists"] = true
	payload["size"] = info.Size()
	payload["modified"] = info.ModTime().UTC().Format("2006-01-02T15:04:05.000Z")

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Doc tu cuoi len: cat mot khuc, roi lay cac dong CUOI trong khuc do. Khuc bi cat giua mot dong thi
	// dong dau tien khong tron ven - bo no di, vi mot dong cut doi khi doc ra nhu mot dong that.
	start := info.Size() - int64(maxBytes)
	cut := start > 0
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	chunk, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(chunk), "\r\n", "\n")
	all := strings.Split(text, "\n")
	// Dong cuoi cung cua file ket thuc bang "\n" -> phan tu cuoi la chuoi rong, khong phai mot dong.
	if len(all) > 0 && all[len(all)-1] == "" {
		all = all[:len(all)-1]
	}
	if cut && len(all) > 0 {
		all = all[1:]
	}
	// Dong dai (stack trace, JSON) lam pane vo: cat bot tung dong va noi ro.
	for index, line := range all {
		if len(line) > 2000 {
			all[index] = line[:2000] + " …"
		}
	}
	truncated := cut
	if len(all) > limit {
		all = all[len(all)-limit:]
		truncated = true
	}
	payload["lines"] = all
	payload["truncated"] = truncated
	payload["count"] = len(all)
	return payload, nil
}
