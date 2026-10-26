package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
			at := slides
			if index, ok := request.Params["index"].(float64); ok && int(index) >= 1 && int(index) <= slides {
				at = int(index)
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"slide":` + strconv.Itoa(at) + `}}`))
		case "wpp.deleteSlide":
			slides--
			deleted, _ := request.Params["slide"].(float64)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"deleted":` + strconv.Itoa(int(deleted)) + `,"slideCount":` + strconv.Itoa(slides) + `}}`))
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

// briefBridgeBrokenConnectors: bridge gia biet checkLayout bao connector-crossing cho nhung duong noi ma `broken` chon.
// addConnector tra ten "Shape N" nhu briefBridge; deleteShapes go ten khoi slide de lan checkLayout sau khong con loi.
func briefBridgeBrokenConnectors(t *testing.T, broken func(params map[string]any) bool) (*RunContext, *[]briefCall) {
	t.Helper()
	calls := []briefCall{}
	slides, shapes := 0, 0
	connectors := map[string]map[string]any{}
	order := []string{}
	reply := func(w http.ResponseWriter, result map[string]any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}
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
			reply(w, map[string]any{"slide": slides})
		case "wpp.addShape", "wpp.addConnector":
			shapes++
			name := fmt.Sprintf("Shape %d", shapes)
			if request.Action == "wpp.addConnector" {
				connectors[name] = request.Params
				order = append(order, name)
			}
			reply(w, map[string]any{"slide": 1, "shape": name})
		case "wpp.deleteShapes":
			names, _ := request.Params["shapes"].([]any)
			for _, value := range names {
				delete(connectors, textOf(value))
			}
			reply(w, map[string]any{"deleted": names})
		case "wpp.checkLayout":
			issues := []any{}
			for _, name := range order {
				params, ok := connectors[name]
				if !ok || !broken(params) {
					continue
				}
				issues = append(issues, map[string]any{"type": "connector-crossing", "detail": "crossing",
					"shapes": []any{name, "Shape 1"}})
			}
			reply(w, map[string]any{"slideWidth": 960, "slideHeight": 540,
				"slides": []any{map[string]any{"issues": issues}}})
		default:
			reply(w, map[string]any{})
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

// Luot that 26/10/2026: "source":"Slide 1 — Cover" hien tren bia, model sua qua lai 3 lan.
func TestSlidesFromBriefRejectsSlideLabelAsSource(t *testing.T) {
	run, _ := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"cover","title":"Nền tảng AI Agent","source":"Slide 1 — Cover"},
		{"layout":"content","title":"Vấn đề","points":["a","b"],"source":"Slide 2 — Problem"},
		{"layout":"content","title":"Số liệu","points":["a","b"],"source":"Nguồn: báo cáo nội bộ 2025"}
	]`), run)
	for _, want := range []string{`"src":1,"field":"source","error":"NO_SOURCE_ON_COVER"`,
		`"src":2,"field":"source","error":"SOURCE_IS_SLIDE_LABEL"`} {
		if !strings.Contains(result.JSON, want) {
			t.Fatalf("thieu %s trong %s", want, result.JSON)
		}
	}
	if strings.Contains(result.JSON, `"src":3,"field":"source"`) {
		t.Fatalf("nguon du lieu that khong duoc bi tu choi: %s", result.JSON)
	}
	for _, label := range []string{"Slide 10", "slide 3: Roadmap", "Trang 4", "Cover"} {
		if !slideLabel.MatchString(label) {
			t.Errorf("%q phai la nhan slide", label)
		}
	}
	for _, source := range []string{"Nguồn: Slideshare 2024", "Slides.com", "Cover letter dataset"} {
		if slideLabel.MatchString(source) {
			t.Errorf("%q khong phai nhan slide", source)
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
			// Quy trinh: direction; so do: canh ra/vao do RouteDiagram chon.
			if direction, ok := call.Params["direction"].(string); ok {
				directions = append(directions, direction)
			} else {
				directions = append(directions, call.Params["fromSide"].(string)+">"+call.Params["toSide"].(string))
			}
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
	wantDirections := []string{"right", "right", "bottom>top", "bottom>top", "bottom>top"}
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

// PowerPoint ve lech mo hinh (checkLayout bao connector-crossing du RouteDiagram da chon duong "sach"): tool tu xoa
// duong hong, ve lai bang ung vien ke tiep va kiem lai - model khong phai ton vong de sua loi di day.
func TestSlidesFromBriefReroutesConnectorPowerPointDrawsAcrossBox(t *testing.T) {
	run, calls := briefBridgeBrokenConnectors(t, func(params map[string]any) bool {
		return params["fromSide"] == "bottom" && params["toSide"] == "top"
	})
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"diagram","title":"Hai tầng xử lý","nodes":[{"id":"a","text":"Gateway","layer":1},
		 {"id":"b","text":"Orchestrator","layer":2}],"edges":[{"from":"a","to":"b"}]}
	]`), run)
	if !result.OK || !strings.Contains(result.JSON, `"connectorsRerouted":1`) || !strings.Contains(result.JSON, `"issueCount":0`) {
		t.Fatalf("phai tu di lai va het loi: %s", result.JSON)
	}
	var deleted, redrawn bool
	for _, call := range *calls {
		switch call.Action {
		case "wpp.addConnector":
			if call.Params["fromSide"] == "right" && call.Params["toSide"] == "right" {
				redrawn = true
			}
		case "wpp.deleteShapes":
			names, _ := call.Params["shapes"].([]any)
			for _, name := range names {
				if textOf(name) == "Shape 3" {
					deleted = true
				}
			}
		}
	}
	if !deleted || !redrawn {
		t.Fatalf("phai xoa duong hong Shape 3 va ve lai canh phai: %+v", *calls)
	}
}

// Khong ung vien nao sach (PowerPoint van bao loi): toi da 2 lan moi duong, tra loi that de model biet ma sua mo hinh.
func TestSlidesFromBriefStopsReroutingAfterTwoAttempts(t *testing.T) {
	run, calls := briefBridgeBrokenConnectors(t, func(map[string]any) bool { return true })
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"diagram","title":"Hai tầng xử lý","nodes":[{"id":"a","text":"Gateway","layer":1},
		 {"id":"b","text":"Orchestrator","layer":2}],"edges":[{"from":"a","to":"b"}]}
	]`), run)
	adds := 0
	for _, call := range *calls {
		if call.Action == "wpp.addConnector" {
			adds++
		}
	}
	if adds != 3 {
		t.Fatalf("1 lan ve dau + toi da 2 lan di lai, co %d lan addConnector", adds)
	}
	if !strings.Contains(result.JSON, `"connectorsRerouted":2`) || !strings.Contains(result.JSON, `"issueCount":1`) {
		t.Fatalf("phai dung sau 2 lan va giu loi that: %s", result.JSON)
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

// Nhom mau (luot that 25/10/2026: slide vong doi yeu cau phai phan biet buoc cua AI voi buoc cua he thong nghiep vu):
// moi nhom mot mau, mot dong chu thich o dau vung noi dung; so do va quy trinh day xuong duoi chu thich.
func TestSlidesFromBriefGroupsColourStepsAndAddLegend(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"process","title":"AI đề xuất, hệ thống nghiệp vụ xác minh","steps":[
		 {"title":"Phân loại intent","group":"AI"},{"title":"Kiểm tra quyền","group":"Nghiệp vụ"},
		 {"title":"Soạn trả lời","group":"AI"},{"title":"Ghi audit","group":"Nghiệp vụ"}]}
	]`), run)
	if !result.OK || !strings.Contains(result.JSON, `"built":1`) {
		t.Fatalf("phai dung duoc: %s", result.JSON)
	}
	fills := map[string]string{}
	labels, swatches, stepTop := []string{}, 0, 0.0
	for _, call := range *calls {
		switch call.Action {
		case "wpp.addShape":
			if text, ok := call.Params["text"].(string); ok {
				fills[text] = call.Params["fill"].(string)
				stepTop = call.Params["top"].(float64)
			} else if call.Params["shape"] == "rect" {
				swatches++
			}
		case "wpp.addParagraphs":
			paras := call.Params["paragraphs"].([]any)
			if text := paras[0].(map[string]any)["t"].(string); text == "AI" || text == "Nghiệp vụ" {
				labels = append(labels, text)
			}
		}
	}
	if fills["Phân loại intent"] != fills["Soạn trả lời"] || fills["Phân loại intent"] == fills["Kiểm tra quyền"] {
		t.Fatalf("cung nhom cung mau, khac nhom khac mau: %v", fills)
	}
	if swatches != 2 || strings.Join(labels, ",") != "AI,Nghiệp vụ" {
		t.Fatalf("chu thich = %d o mau, nhan %v", swatches, labels)
	}
	if stepTop < 130+legendHeight {
		t.Fatalf("buoc phai nam duoi dong chu thich, top = %v", stepTop)
	}
}

func TestSlidesFromBriefRejectsTooManyGroups(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"process","title":"Bốn nhóm thì quá nhiều màu","steps":[{"title":"A","group":"1"},{"title":"B","group":"2"},
		 {"title":"C","group":"3"},{"title":"D","group":"4"}]}
	]`), run)
	if !strings.Contains(result.JSON, "MAX_3_GROUPS") {
		t.Fatalf("phai tu choi: %s", result.JSON)
	}
	for _, call := range *calls {
		if call.Action != "wpp.checkLayout" {
			t.Fatalf("co loi thi khong duoc dung gi, da goi %s", call.Action)
		}
	}
}

// Bia: phu de can giua nhu tieu de (truoc day can trai).
func TestSlidesFromBriefCoverSubtitleCentered(t *testing.T) {
	run, calls := briefBridge(t)
	NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"cover","title":"AI Agent Platform for Digital Banking","subtitle":"Secure, Governed and Observable"}
	]`), run)
	aligned := []string{}
	for _, call := range *calls {
		if call.Action == "wpp.addParagraphs" {
			para := call.Params["paragraphs"].([]any)[0].(map[string]any)
			aligned = append(aligned, textOf(para["a"]))
		}
	}
	if strings.Join(aligned, ",") != "center,center" {
		t.Fatalf("tieu de va phu de phai can giua: %v", aligned)
	}
}

// Luot that 25/10/2026: model viet {"layout":"diagram","diagram":{"nodes":[...],"edges":[...]}} (chart thi dung la long
// trong khoa "chart"), tool thay 0 nut, model gui lai y het 3 lan va luot hong. Nay chap nhan dang long.
func TestSlidesFromBriefAcceptsNestedLayoutFields(t *testing.T) {
	run, calls := briefBridge(t)
	result := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[
		{"layout":"diagram","title":"Ba tầng xử lý yêu cầu","diagram":{"nodes":[{"id":"a","text":"Gateway","layer":1},
		 {"id":"b","text":"Orchestrator","layer":2}],"edges":[{"from":"a","to":"b"}]}},
		{"layout":"process","title":"Ba bước tiếp nhận","process":{"steps":[{"title":"Nhận"},{"title":"Kiểm"},{"title":"Trả"}]}}
	]`), run)
	if !result.OK || !strings.Contains(result.JSON, `"built":2`) {
		t.Fatalf("dang long phai dung duoc: %s", result.JSON)
	}
	connectors := 0
	for _, call := range *calls {
		if call.Action == "wpp.addConnector" {
			connectors++
		}
	}
	if connectors != 3 {
		t.Fatalf("1 duong noi so do + 2 mui ten quy trinh, co %d", connectors)
	}
	// Thieu nut that su: loi chi ro cach viet dung.
	bad := NewSlidesFromBriefTool(nil).Invoke(context.Background(), brief(t, `[{"layout":"diagram","title":"Trống","nodes":[]}]`), run)
	if !strings.Contains(bad.JSON, "NEED_2_TO_10_NODES") || !strings.Contains(bad.JSON, "directly on the slide") {
		t.Fatalf("loi phai chi cach viet dung: %s", bad.JSON)
	}
}

// Sua dung slide loi (luot that 25/10/2026: checkLayout bao 3 loi, model dung lai 10 slide roi xoa 10 slide cu): gui lai
// chi slide do voi replace - xoa slide cu cua luot, dung slide moi DUNG cho do. Slide khong do luot dung thi tu choi.
func TestSlidesFromBriefReplacesOwnSlideInPlace(t *testing.T) {
	run, calls := briefBridge(t)
	tool := NewSlidesFromBriefTool(nil)
	first := tool.Invoke(context.Background(), brief(t, `[
		{"layout":"section","title":"Phần một"},{"layout":"section","title":"Phần hai"},{"layout":"section","title":"Phần ba"}
	]`), run)
	if !strings.Contains(first.JSON, `"built":3`) {
		t.Fatalf("dung 3 slide: %s", first.JSON)
	}
	*calls = nil
	fixed := tool.Invoke(context.Background(), brief(t, `[{"layout":"section","title":"Phần hai (sửa)","replace":2}]`), run)
	if !fixed.OK || !strings.Contains(fixed.JSON, `"replaced":2`) || !strings.Contains(fixed.JSON, `"slide":2`) {
		t.Fatalf("phai thay slide 2 tai cho: %s", fixed.JSON)
	}
	var deleted, inserted bool
	for _, call := range *calls {
		if call.Action == "wpp.deleteSlide" && call.Params["slide"] == 2.0 {
			deleted = true
		}
		if call.Action == "wpp.addSlide" && call.Params["index"] == 2.0 {
			inserted = deleted // chen SAU khi da xoa slide cu
		}
	}
	if !deleted || !inserted {
		t.Fatalf("xoa slide 2 roi chen vao vi tri 2: %+v", *calls)
	}
	// Slide moi o vi tri 2 van la cua luot (thay lai lan nua duoc); slide 9 khong ton tai / khong cua luot -> tu choi.
	if !run.ownedObjects().covers("wpp.deleteSlide", map[string]any{"slide": 2.0, "presentation": "Presentation3"}) {
		t.Fatal("slide thay tai cho van la cua luot")
	}
	*calls = nil
	refused := tool.Invoke(context.Background(), brief(t, `[{"layout":"section","title":"Không phải của lượt","replace":9}]`), run)
	if !strings.Contains(refused.JSON, "NOT_BUILT_IN_THIS_REQUEST") {
		t.Fatalf("phai tu choi: %s", refused.JSON)
	}
	for _, call := range *calls {
		if call.Action != "wpp.checkLayout" {
			t.Fatalf("tu choi thi khong duoc dung/xoa gi, da goi %s", call.Action)
		}
	}
}

// Cung mot mo ta slide da dung ra loi bo cuc thi tu choi (luot that 25/10/2026: ~17 lan gui lai slide so do y het qua
// replace, 42 vong); mo ta da dung SACH thi gui lai van duoc (vd thay tai cho sau khi sua slide khac).
func TestSlidesFromBriefRefusesSameSpecWithIssues(t *testing.T) {
	run := &RunContext{}
	specs := briefSpecs([]any{map[string]any{"layout": "section", "title": "A", "replace": 2.0},
		map[string]any{"layout": "section", "title": "B"}})
	run.rememberBriefSpec(briefSpecs([]any{map[string]any{"layout": "section", "title": "A"}})[0], 2)
	run.rememberBriefSpec(specs[1], 0)
	errors := checkSameSpec(specs, run)
	if len(errors) != 1 || errors[0].Src != 1 || errors[0].Error != "SAME_SPEC_SAME_ISSUES" {
		t.Fatalf("chi slide 1 (cung mo ta, co loi; replace khong tinh) bi tu choi: %+v", errors)
	}
}
