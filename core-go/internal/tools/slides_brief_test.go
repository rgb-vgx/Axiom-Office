package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

type briefCall struct {
	Action string
	Params map[string]any
}

// Bridge gia PowerPoint 960 x 540: addSlide dem slide, checkLayout tra co slide va khong loi.
func briefBridge(t *testing.T) (*RunContext, *[]briefCall) {
	t.Helper()
	calls := []briefCall{}
	slides, shapes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &request)
		calls = append(calls, briefCall{request.Action, request.Params})
		switch request.Action {
		case "wpp.addSlide":
			slides++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"slide":` + strconv.Itoa(slides) + `}}`))
		case "wpp.addShape", "wpp.addConnector":
			shapes++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"slide":1,"shape":"Shape ` + strconv.Itoa(shapes) + `"}}`))
		case "wpp.checkLayout":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"slideWidth":960,"slideHeight":540,"slides":[{"issues":[]}]}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}
	}))
	t.Cleanup(server.Close)
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	run := &RunContext{Office: &office.Session{App: "wpp", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t"),
		TargetDocument: "Presentation3"}
	return run, &calls
}

func brief(t *testing.T, slides string) map[string]any {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(slides), &value); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"slides": value}
}

func TestSlidesFromBriefBuildsOnTheGrid(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"content","title":"Giải quyết đúng hạn đạt 98,5% trong quý III","points":["Tiếp nhận 1.240 hồ sơ","- Số hóa 3.600 trang"],
		 "source":"Nguồn: báo cáo quý III","notes":"chi tiết"},
		{"layout":"figures","title":"Ba chỉ số nổi bật năm 2026","figures":[{"value":"128 tỷ","label":"doanh thu, +12%"},
		 {"value":"4.500","label":"khách hàng mới"},{"value":"92%","label":"mức hài lòng"}]}
	]`), run)
	if !result.OK || !result.Mutating || !strings.Contains(result.JSON, `"built":2`) {
		t.Fatalf("phai dung 2 slide: %s", result.JSON)
	}
	var title, points, source, values []map[string]any
	for _, call := range *calls {
		if call.Params["presentation"] != "Presentation3" {
			t.Fatalf("moi lenh phai nham bai cua luot: %s %+v", call.Action, call.Params)
		}
		if call.Action != "wpp.addParagraphs" {
			continue
		}
		switch {
		case call.Params["fontSize"] == 28.0:
			title = append(title, call.Params)
		case call.Params["fontSize"] == 20.0 && call.Params["top"] == 130.0:
			points = append(points, call.Params)
		case call.Params["fontSize"] == 14.0:
			source = append(source, call.Params)
		case call.Params["fontSize"] == 40.0:
			values = append(values, call.Params)
		}
	}
	// Luoi skill muc 1: tieu de 40,30,880,80; noi dung 40,130,880,370; nguon 40,504; khong thu chu (min = co).
	if len(title) != 2 || title[0]["left"] != 40.0 || title[0]["top"] != 30.0 || title[0]["width"] != 880.0 {
		t.Fatalf("tieu de = %+v", title)
	}
	if len(points) != 1 || points[0]["height"] != 370.0 || points[0]["minFontSize"] != 20.0 {
		t.Fatalf("hop y = %+v", points)
	}
	if len(source) != 1 || source[0]["top"] != 504.0 {
		t.Fatalf("dong nguon = %+v", source)
	}
	// Ba con so: cot 280, left 40 / 340 / 640 (bo-cuc-slide.md muc 3).
	if len(values) != 3 || values[1]["left"] != 340.0 || values[2]["left"] != 640.0 || values[0]["width"] != 280.0 {
		t.Fatalf("con so = %+v", values)
	}
}

func TestSlidesFromBriefRejectsBeforeBuilding(t *testing.T) {
	run, calls := briefBridge(t)
	long := strings.Repeat("Một ý rất dài cần rút gọn lại. ", 8)
	point := `"` + strings.Repeat("x", 200) + `"`
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"content","title":"Hợp lệ","points":["`+long+`"]},
		{"layout":"content","title":"Năm ý dài","points":[`+strings.Repeat(point+",", 4)+point+`],"callout":"Lưu ý"},
		{"layout":"cards","title":"Một thẻ","cards":[{"title":"A","points":["a"]}]},
		{"layout":"radar","title":"Kiểu lạ"}
	]`), run)
	for _, want := range []string{`"src":1,"field":"points","error":"POINT_TOO_LONG"`, `"src":2,"field":"points","error":"TOO_LONG"`,
		`"src":3,"field":"cards","error":"NEED_2_TO_4_CARDS"`, `"src":4,"field":"layout","error":"UNKNOWN_LAYOUT"`} {
		if !strings.Contains(result.JSON, want) {
			t.Fatalf("thieu %s trong %s", want, result.JSON)
		}
	}
	for _, call := range *calls {
		if call.Action != "wpp.checkLayout" {
			t.Fatalf("co loi thi khong duoc dung gi, da goi %s", call.Action)
		}
	}
}

func TestSlidesFromBriefDrawsProcessAndDiagramWithConnectors(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"process","title":"Năm bước tiếp nhận hồ sơ một cửa","steps":[{"title":"Tiếp nhận","text":"kiểm tra danh mục"},
		 {"title":"Kiểm tra"},{"title":"Phê duyệt"}]},
		{"layout":"diagram","title":"Bộ định tuyến quyết định mô hình và công cụ","nodes":[{"id":"g","text":"API Gateway","layer":1},
		 {"id":"r","text":"Bộ định tuyến","layer":2},{"id":"a","text":"Mô hình A","layer":3},{"id":"b","text":"Mô hình B","layer":3}],
		 "edges":[{"from":"g","to":"r"},{"from":"r","to":"a"},{"from":"r","to":"b"}]}
	]`), run)
	if !result.OK {
		t.Fatalf("phai dung duoc: %s", result.JSON)
	}
	shapes, connectors, directions := map[string]bool{}, [][2]string{}, []string{}
	for _, call := range *calls {
		switch call.Action {
		case "wpp.addShape":
			shapes[call.Params["text"].(string)] = true
			if call.Params["boldFirst"] != true || call.Params["shape"] != "rounded" {
				t.Fatalf("hinh buoc/nut = %+v", call.Params)
			}
		case "wpp.addConnector":
			connectors = append(connectors, [2]string{call.Params["from"].(string), call.Params["to"].(string)})
			directions = append(directions, call.Params["direction"].(string))
		}
	}
	if !shapes["Tiếp nhận\nkiểm tra danh mục"] || !shapes["Bộ định tuyến"] || len(shapes) != 7 {
		t.Fatalf("hinh = %v", shapes)
	}
	// Quy trinh: Shape 1 -> 2 -> 3; so do: gateway (Shape 6) -> router (7) -> A (8), B (9) (ten do bridge gia dat theo thu tu).
	want := [][2]string{{"Shape 1", "Shape 2"}, {"Shape 2", "Shape 3"}, {"Shape 6", "Shape 7"}, {"Shape 7", "Shape 8"}, {"Shape 7", "Shape 9"}}
	if len(connectors) != len(want) {
		t.Fatalf("duong noi = %v", connectors)
	}
	// Huong noi tuong minh (khong de PowerPoint chon diem noi): quy trinh sang phai, so do tu day cha xuong dinh con.
	wantDirections := []string{"right", "right", "down", "down", "down"}
	for i := range want {
		if connectors[i] != want[i] || directions[i] != wantDirections[i] {
			t.Fatalf("duong noi %d = %v %s, muon %v %s", i, connectors[i], directions[i], want[i], wantDirections[i])
		}
	}
	// Slide va hinh do tool dung la cua luot: xoa slide 2 khong phai hoi.
	if !run.ownedObjects().covers("wpp.deleteSlide", map[string]any{"slide": 2.0, "presentation": "Presentation3"}) {
		t.Fatal("slide do tool dung phai ghi vao so cua luot")
	}
}

func TestDiagramDirection(t *testing.T) {
	cases := []struct {
		fromLayer, fromPlace, toLayer, toPlace int
		want                                   string
	}{{0, 0, 1, 1, "down"}, {2, 0, 1, 0, "up"}, {1, 0, 1, 2, "right"}, {1, 2, 1, 0, "left"}}
	for _, c := range cases {
		if got := diagramDirection(c.fromLayer, c.fromPlace, c.toLayer, c.toPlace); got != c.want {
			t.Fatalf("%+v -> %s", c, got)
		}
	}
}

func TestSlidesFromBriefDrawsNativeChart(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"chart","title":"Doanh thu tăng đều qua bốn quý năm 2026","source":"Nguồn: phòng Kế toán",
		 "chart":{"type":"column","categories":["Q1","Q2","Q3","Q4"],"series":[{"name":"Doanh thu","values":[28,"31","33,5",36]}],"unit":" tỷ"}}
	]`), run)
	if !result.OK {
		t.Fatalf("phai dung duoc: %s", result.JSON)
	}
	var chart map[string]any
	for _, call := range *calls {
		if call.Action == "wpp.addChart" {
			chart = call.Params
		}
	}
	if chart == nil {
		t.Fatalf("khong goi wpp.addChart: %v", *calls)
	}
	series := chart["series"].([]any)[0].(map[string]any)
	values, _ := json.Marshal(series["values"])
	// Chuoi so ("31", "33,5") doc thanh so; vung bieu do la vung noi dung cua luoi (duoi tieu de, tren dong nguon).
	if string(values) != "[28,31,33.5,36]" || series["name"] != "Doanh thu" || chart["type"] != "column" || chart["unit"] != " tỷ" ||
		chart["top"].(float64) < 110 || chart["presentation"] != "Presentation3" {
		t.Fatalf("addChart = %+v", chart)
	}
	if colors := chart["colors"].([]any); len(colors) < 4 || colors[0] != "#1F4E79" {
		t.Fatalf("mau = %v", colors)
	}
}

func TestSlidesFromBriefRejectsBadChart(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"chart","title":"Cơ cấu chi phí","chart":{"type":"pie","categories":["Nhân sự","Khác"],
		 "series":[{"name":"A","values":["55%",45]},{"name":"B","values":[1,2]}]}},
		{"layout":"chart","title":"Thiếu dữ liệu"},
		{"layout":"chart","title":"Kiểu lạ","chart":{"type":"radar","categories":["a","b"],"series":[{"name":"x","values":[1]}]}}
	]`), run)
	for _, want := range []string{`"NEED_1_TO_1_SERIES"`, `"NOT_A_NUMBER"`, `"chart.series[0].values[0]"`, `"MISSING"`,
		`"UNKNOWN_CHART_TYPE"`, `"ONE_VALUE_PER_CATEGORY"`} {
		if !strings.Contains(result.JSON, want) {
			t.Fatalf("thieu %s: %s", want, result.JSON)
		}
	}
	for _, call := range *calls {
		if call.Action != "wpp.checkLayout" {
			t.Fatalf("co loi thi khong duoc dung gi, da goi %s", call.Action)
		}
	}
}
