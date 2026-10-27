//go:build live

package tools

// Tool P3 (doc_from_brief, dashboard_from_data) tren Office THAT. tests/live/test_office_templates.py mo Word/Excel, dat
// bien moi truong roi chay:
//
//	AXIOM_LIVE_PORT=47831 AXIOM_LIVE_TOKEN=<token> AXIOM_LIVE_APP=wps AXIOM_LIVE_OUT=<file.docx> \
//	  go test -tags live -run LiveP3 -v ./internal/tools/
//
// Test dung tai lieu/so, luu ra AXIOM_LIVE_OUT; script Python cham tep bang XML.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"axiomoffice/core/internal/office"
)

func liveP3Run(t *testing.T, app string) *RunContext {
	t.Helper()
	if os.Getenv("AXIOM_LIVE_APP") != app {
		t.Skip("AXIOM_LIVE_APP khong phai " + app)
	}
	port, err := strconv.Atoi(os.Getenv("AXIOM_LIVE_PORT"))
	if err != nil || port == 0 {
		t.Skip("AXIOM_LIVE_PORT chua dat")
	}
	bridge := office.NewBridgeClient(&http.Client{Timeout: 5 * time.Minute}, os.Getenv("AXIOM_LIVE_TOKEN"))
	return &RunContext{Office: &office.Session{App: app, Port: port}, Bridge: bridge}
}

func liveSave(t *testing.T, run *RunContext, action string) {
	t.Helper()
	result := run.Bridge.Command(context.Background(), run.Office.Port, action, map[string]any{"path": os.Getenv("AXIOM_LIVE_OUT")})
	if !result.OK {
		t.Fatalf("%s: %s", action, result.Error)
	}
}

func TestLiveP3DocFromBrief(t *testing.T) {
	run := liveP3Run(t, "wps")
	var arguments map[string]any
	_ = json.Unmarshal([]byte(`{
		"kind": "to-trinh",
		"agency": {"parent": "Sở Giáo dục và Đào tạo Hà Nội", "name": "Trường THPT Nguyễn Du"},
		"number": "12/TTr-THPT", "place": "Hà Nội", "date": "5/2/2026",
		"subject": "mua 5 máy tính xách tay cho phòng Kế toán", "to": ["Giám đốc Sở"],
		"body": [
			{"heading": "1. Lý do", "level": 1},
			{"text": "Máy cũ đã dùng 7 năm, thường xuyên hỏng."},
			{"bullets": ["5 máy tính xách tay", "Tổng kinh phí 125 triệu đồng"]},
			{"table": [["Hạng mục", "Số lượng", "Thành tiền"], ["Máy tính", "5", "125.000.000"]], "caption": "Dự toán kinh phí"},
			{"text": "Kính đề nghị Giám đốc Sở xem xét, phê duyệt."}
		],
		"signer": {"position": "Hiệu trưởng", "onBehalf": ""},
		"recipients": ["Như trên", "Lưu: VT, KT"]
	}`), &arguments)
	result := NewDocFromBriefTool().Invoke(context.Background(), arguments, run)
	t.Log(result.JSON)
	if !result.OK {
		t.Fatalf("doc_from_brief: %s", result.JSON)
	}
	liveSave(t, run, "writer.saveAs")
}

func TestLiveP3Dashboard(t *testing.T) {
	run := liveP3Run(t, "et")
	var arguments map[string]any
	_ = json.Unmarshal([]byte(`{
		"source": "Data!A1:D7", "title": "Doanh thu 6 tháng đầu năm",
		"kpis": [
			{"label": "Tổng doanh thu", "column": "Doanh thu", "agg": "sum", "format": "vnd"},
			{"label": "Chi phí TB", "column": "Chi phí", "agg": "average", "format": "vnd"},
			{"label": "Tỷ suất cao nhất", "column": "Tỷ suất", "agg": "max", "format": "percent"}
		],
		"charts": [
			{"type": "column", "title": "Doanh thu và chi phí", "x": "Tháng", "y": ["Doanh thu", "Chi phí"]},
			{"type": "line", "title": "Tỷ suất", "x": "Tháng", "y": ["Tỷ suất"]}
		]
	}`), &arguments)
	result := NewDashboardFromDataTool().Invoke(context.Background(), arguments, run)
	t.Log(result.JSON)
	if !result.OK {
		t.Fatalf("dashboard_from_data: %s", result.JSON)
	}
	liveSave(t, run, "et.saveAs")
}
