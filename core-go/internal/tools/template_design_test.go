package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/pptx"
)

func TestSourceTitleKeepsOriginalWording(t *testing.T) {
	candidates := []string{"1. Xquang thường qui", "1.1. Bụng không chuẩn bị", "Tư thế đứng thẳng"}
	cases := map[string]string{
		"1. xquang thường qui":                 "1. Xquang thường qui",
		"Xquang thường qui":                    "Xquang thường qui",
		"1.1. Bụng không chuẩn bị (tiếp theo)": "1.1. Bụng không chuẩn bị (tiếp theo)",
	}
	for title, want := range cases {
		if got, ok := sourceTitle(title, candidates); !ok || got != want {
			t.Errorf("%q -> %q %v (muon %q)", title, got, ok, want)
		}
	}
	if _, ok := sourceTitle("Giới thiệu chung", candidates); ok {
		t.Fatal("tieu de khong co trong bai goc phai bi tu choi")
	}
}

func TestCoveredByTagsReadsDesignRanges(t *testing.T) {
	covered := coveredByTags(map[string]bool{"1": true, "9.2": true, "3-6.1": true, "3-6.4": true, "9999": true, "x": true})
	for _, n := range []int{1, 3, 4, 5, 6, 9} {
		if !covered[n] {
			t.Fatalf("thieu %d: %v", n, sortedKeys(covered))
		}
	}
	if covered[2] || covered[7] {
		t.Fatalf("thua: %v", sortedKeys(covered))
	}
}

// Che do thiet ke voi bridge gia: (1) chua co bo cuc -> tra phan bai danh so; (2) bo cuc thieu anh + tieu de tu
// viet -> bao loi, khong dung gi; (3) bo cuc du -> xoa slide cu cua phan, dung theo thu tu, anh lay dung slide nguon.
func TestDesignModeChecksCoverageThenBuilds(t *testing.T) {
	template, _ := testRoles(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "bai cu.ppt")
	target := filepath.Join(dir, "bai cu - mau moi.pptx")
	calls := []string{}
	nextSlide := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request map[string]any
		_ = json.Unmarshal(body, &request)
		action, _ := request["action"].(string)
		params, _ := request["params"].(map[string]any)
		encoded, _ := json.Marshal(params)
		calls = append(calls, action+" "+string(encoded))
		result := `{}`
		switch action {
		case "app.info":
			result = `{"activePresentation":{"fullName":` + strconv.Quote(target) + `}}`
		case "wpp.sourceManifest":
			result = `{"slideCount":3,"slides":[` +
				`{"n":1,"title":"CĐHA GAN","texts":["THS PHAN NHẬT ANH"],"pictures":[]},` +
				`{"n":2,"title":"1. ĐẠI CƯƠNG","texts":["Đại thể\nVi thể"],"pictures":[{"n":1,"shape":"Picture 4","w":400,"h":300}]},` +
				`{"n":3,"title":"1. ĐẠI CƯƠNG","texts":["U máu: tăng âm"],"pictures":[{"n":1,"shape":"Picture 2","w":300,"h":300},{"n":2,"shape":"Picture 3","w":300,"h":300}]}]}`
		case "wpp.listSlides":
			result = `{"slides":[]}`
		case "wpp.addSlideFromTemplate":
			nextSlide++
			result = `{"slide":` + strconv.Itoa(nextSlide) + `}`
		case "wpp.checkLayout":
			result = `{"slides":[{"issues":[]}]}`
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":` + result + `}`))
	}))
	defer server.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	run := &RunContext{Office: &office.Session{App: "wpp", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "")}
	invoke := func(slides any) Result {
		arguments := map[string]any{"template": template, "source": source, "mode": "design"}
		if slides != nil {
			arguments["slides"] = slides
		}
		return NewSlidesFromTemplateTool().Invoke(context.Background(), arguments, run)
	}
	countActions := func(action string) int {
		n := 0
		for _, call := range calls {
			if strings.HasPrefix(call, action+" ") {
				n++
			}
		}
		return n
	}

	view := invoke(nil)
	if !view.OK || !strings.Contains(view.JSON, `"1 Đại thể"`) || !strings.Contains(view.JSON, `"2 Vi thể"`) ||
		!strings.Contains(view.JSON, `"2 landscape"`) && !strings.Contains(view.JSON, `"1 landscape"`) || view.Final {
		t.Fatalf("phan bai: %s", view.JSON)
	}

	bad := invoke([]any{
		map[string]any{"layout": "cover", "title": "CĐHA GAN", "use": []any{"1"}},
		map[string]any{"layout": "content", "title": "Giới thiệu chung", "use": []any{"2", "3:p", "3:i1"}},
	})
	if !strings.Contains(bad.JSON, "NOT_FROM_SOURCE") || !strings.Contains(bad.JSON, "MISSING_FROM_SOURCE") ||
		!strings.Contains(bad.JSON, "3:i2") || countActions("wpp.addSlideFromTemplate") != 0 {
		t.Fatalf("bo cuc thieu phai bao loi va khong dung: %s", bad.JSON)
	}

	good := invoke([]any{
		map[string]any{"layout": "cover", "title": "CĐHA GAN", "use": []any{"1"}},
		map[string]any{"layout": "content", "title": "1. đại cương", "use": []any{"2", "3"}},
	})
	if !good.OK || !good.Final || !strings.Contains(good.FinalText, "giữ nguyên văn") {
		t.Fatalf("dung xong ca bai: %+v", good)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{`wpp.deleteTagged {"first":1,"last":3`, `"tag":"1-3.1"`, `"tag":"1-3.2"`,
		`"text":"1. ĐẠI CƯƠNG"`, `"fromSlide":3`, `"t":"U máu: tăng âm"`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("thieu %s trong:\n%s", want, joined)
		}
	}
	if countActions("wpp.copyPicture") != 3 {
		t.Fatalf("phai chep du 3 anh: %d", countActions("wpp.copyPicture"))
	}
}

// Slide "update": link nguon phai do web_search tra trong luot; tieu de luon ghi ro "Cap nhat"; nguon len slide + ghi chu.
func TestUpdateSlideNeedsRealWebSources(t *testing.T) {
	_, roles := testRoles(t)
	run := &RunContext{}
	run.noteWebSource("https://www.acr.org/Clinical-Resources/ACR-Appropriateness-Criteria/", "ACR")
	row := designRow{Layout: "update", Title: "CT liều thấp trong đau bụng cấp", Points: []string{"CT liều thấp thay X quang ở người lớn"},
		Sources: []string{"https://acr.org/Clinical-Resources/ACR-Appropriateness-Criteria", "https://bia.example/khong-co"}}
	errors := []rowError{}
	title, paras, notes := checkUpdate(row, roles.picture, 1, run.WebSource, &errors)
	if len(errors) != 1 || errors[0].Error != "NOT_FROM_WEB_SEARCH" || !strings.Contains(errors[0].Allowed, "bia.example") {
		t.Fatalf("link bia phai bi bat (va chi link do): %+v", errors)
	}
	if title != "Cập nhật: CT liều thấp trong đau bụng cấp" || paras[len(paras)-1].T != "Nguồn: acr.org" ||
		!strings.Contains(notes, "acr.org/Clinical-Resources") {
		t.Fatalf("title=%q paras=%+v notes=%q", title, paras, notes)
	}
	errors = nil
	checkUpdate(designRow{Layout: "update", Title: "Bổ sung", Use: []string{"3"}}, roles.picture, 2, run.WebSource, &errors)
	got := map[string]bool{}
	for _, e := range errors {
		got[e.Field+"."+e.Error] = true
	}
	if !got["use.NOT_FOR_UPDATE"] || !got["points.MISSING"] || !got["sources.MISSING"] {
		t.Fatalf("loi slide update: %+v", errors)
	}
}

// Gop nhieu slide nguon: dong de muc lap (trung tieu de, hoac trung dong ngan da co) chi con mot lan; chu van nguyen van.
func TestDedupeParasDropsRepeatedHeadings(t *testing.T) {
	paras := []sourcePara{{T: "1.1. Bụng không chuẩn bị"}, {T: "Tư thế đứng thẳng", I: true}, {T: "Kỹ thuật:"},
		{T: "1.1. Bụng không chuẩn bị"}, {T: "Tư thế đứng thẳng", I: true}, {T: "Tiêu chuẩn:"}}
	kept := dedupeParas(paras, "1.1. Bụng không chuẩn bị (tiếp theo)")
	lines := []string{}
	for _, para := range kept {
		lines = append(lines, para.T)
	}
	if strings.Join(lines, "|") != "Tư thế đứng thẳng|Kỹ thuật:|Tiêu chuẩn:" {
		t.Fatalf("con lai: %v", lines)
	}
	if title, ok := sourceTitle("Bình thường:", []string{"Bình thường:"}); !ok || title != "Bình thường" {
		t.Fatalf("tieu de bo dau hai cham: %q", title)
	}
}

// Vung so do (sourceManifest "region") chep bang region, khong theo ten shape.
func TestCopyParamsForDiagramRegion(t *testing.T) {
	region := copyParams("a.ppt", 26, sourcePicture{Region: []float64{40, 120, 500, 300}}, 7, [4]float64{1, 2, 3, 4})
	if _, hasShape := region["shape"]; hasShape || len(region["region"].([]float64)) != 4 || region["fromSlide"] != 26 {
		t.Fatalf("vung so do: %v", region)
	}
	picture := copyParams("a.ppt", 3, sourcePicture{Shape: "Picture 4", Overlays: 2}, 7, [4]float64{1, 2, 3, 4})
	if picture["shape"] != "Picture 4" || picture["overlays"] != true {
		t.Fatalf("anh: %v", picture)
	}
}

// Tieu de bia hai dong ghep thanh mot dong van la chu goc (luot that 13/10/2026 23:55 bi bao NOT_FROM_SOURCE).
func TestMultiLineTitleJoinedIsFromSource(t *testing.T) {
	candidates := titleCandidates([]sourceSlide{{N: 1, Title: "CHẨN ĐOÁN HÌNH ẢNH\nCẤP CỨU BỤNG"}}, nil)
	if title, ok := sourceTitle("Chẩn đoán hình ảnh cấp cứu bụng", candidates); !ok || title != "CHẨN ĐOÁN HÌNH ẢNH CẤP CỨU BỤNG" {
		t.Fatalf("%q %v", title, ok)
	}
}

// Dong lap tren nhieu slide nguon chi can dung mot ban (cac ban khac tinh la da co).
func TestRepeatedLineNeedsOneUse(t *testing.T) {
	_, roles := testRoles(t)
	views := []sourceView{
		{slide: sourceSlide{N: 3}, paras: []sourcePara{{T: "1.1. Bụng không chuẩn bị"}, {T: "Kỹ thuật: đứng"}}},
		{slide: sourceSlide{N: 4}, paras: []sourcePara{{T: "1.1. Bụng không chuẩn bị"}, {T: "Tiêu chuẩn: thấy vòm hoành"}}},
	}
	byUse := map[string]*pptx.Kind{"content": roles.picture}
	rows := []designRow{{Layout: "content", Title: "Kỹ thuật: đứng", Use: []string{"3:p1-2", "4:p2"}}}
	_, errors := checkDesign(rows, views, byUse, titleCandidates(nil, views), nil)
	for _, e := range errors {
		if e.Field == "coverage" {
			t.Fatalf("ban lap cua dong da dung khong duoc tinh la thieu: %+v", e)
		}
	}
}
