package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/pptx"
)

// writeTemplate: mau 16:9 hai slide giong mau truong (bia: tieu de + o giang vien; noi dung: tieu de + 3 muc
// lap, moi muc = o mau + o chu), du de ReadTemplate ra kinds co tieu de, muc lap va o anh.
func writeTemplate(t *testing.T) string {
	t.Helper()
	const pml = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	box := func(id int, name string, x, y, w, h int, text string, size int) string {
		body := ""
		if text != "" {
			body = fmt.Sprintf(`<p:txBody><a:bodyPr/><a:p><a:r><a:rPr sz="%d" b="1"/><a:t>%s</a:t></a:r></a:p></p:txBody>`, size*100, text)
		}
		return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"/></p:spPr>%s</p:sp>`,
			id, name, x*12700, y*12700, w*12700, h*12700, body)
	}
	slide := func(shapes string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><p:sld ` + pml + `><p:cSld><p:spTree>` + shapes + `</p:spTree></p:cSld></p:sld>`
	}
	cover := box(2, "TextBox 3", 45, 192, 870, 56, "TEN BAI", 40) + box(3, "TextBox 6", 536, 364, 331, 104, "Giang vien", 16)
	content := box(2, "TextBox 2", 292, 40, 594, 47, "MUC TIEU", 28)
	for i, top := range []int{150, 250, 350} {
		content += box(10+i*2, fmt.Sprintf("Rectangle %d", 4+i*5), 310, top, 583, 59, "", 0) +
			box(11+i*2, fmt.Sprintf("TextBox %d", 7+i*5), 340, top+5, 529, 48, "Muc", 17)
	}
	files := map[string]string{
		"[Content_Types].xml":             `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<?xml version="1.0" encoding="UTF-8"?><p:presentation ` + pml + `><p:sldIdLst><p:sldId id="256" r:id="rId2"/><p:sldId id="257" r:id="rId3"/></p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           slide(cover),
		"ppt/slides/slide2.xml":           slide(content),
	}
	path := filepath.Join(t.TempDir(), "mau.pptx")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, body := range files {
		writer, _ := archive.Create(name)
		_, _ = io.WriteString(writer, body)
	}
	_ = archive.Close()
	_ = file.Close()
	return path
}

func testRoles(t *testing.T) (string, templateRoles) {
	path := writeTemplate(t)
	template, err := pptx.ReadTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, rolesOf(template)
}

func TestTemplateRolesFromKinds(t *testing.T) {
	_, roles := testRoles(t)
	if roles.cover == nil || roles.cover.Kind != 1 || roles.list == nil || roles.list.Kind != 2 || roles.picture != roles.list {
		t.Fatalf("roles = %+v", roles)
	}
	if bodies := roles.list.ItemBodies(); len(bodies) != 3 || bodies[1].Shape != "TextBox 12" {
		t.Fatalf("o noi dung muc = %+v", bodies)
	}
}

func TestAutoRowsDecideLectureSlides(t *testing.T) {
	_, roles := testRoles(t)
	carry := ""
	auto := func(slide sourceSlide, first bool) ([]planRow, string) { return autoRows(slide, roles, first, &carry) }

	cover, reason := auto(sourceSlide{N: 1, Title: "CĐHA GAN", Texts: []string{"THS PHAN NHẬT ANH\nBộ môn CĐHA"}}, true)
	if reason != "" || cover[0].Kind != 1 || cover[0].Texts["TextBox 6"] == "" {
		t.Fatalf("bia: %+v %q", cover, reason)
	}
	picture, reason := auto(sourceSlide{N: 2, Title: "ĐẠI CƯƠNG", Texts: []string{"Đại thể"},
		Pictures: []sourcePicture{{N: 1, Shape: "Picture 4"}}}, false)
	if reason != "" || picture[0].Kind != 2 || picture[0].Title != "ĐẠI CƯƠNG – Đại thể" || len(picture[0].Imgs) != 1 || picture[0].Text != "" {
		t.Fatalf("slide anh: %+v %q", picture, reason)
	}
	// Slide anh tiep theo khong co tieu de: lay tieu de cua muc dang trinh bay.
	next, reason := auto(sourceSlide{N: 3, Pictures: []sourcePicture{{N: 1}, {N: 2}}}, false)
	if reason != "" || next[0].Title != "ĐẠI CƯƠNG – Đại thể" || len(next[0].Imgs) != 2 {
		t.Fatalf("slide tiep theo: %+v %q", next, reason)
	}
	// Khung cao nhat la doan mo ta (khong phai tieu de): giu nguyen ben canh anh, tieu de mang sang.
	described, reason := auto(sourceSlide{N: 4, Title: "SA:\nKhó chẩn đoán do u rất thay đổi, thường tăng âm",
		Pictures: []sourcePicture{{N: 1}}}, false)
	if reason != "" || described[0].Title != "ĐẠI CƯƠNG – Đại thể" || !strings.HasPrefix(described[0].Text, "SA:") {
		t.Fatalf("slide mo ta + anh: %+v %q", described, reason)
	}
	// 6 anh: hai slide, tag 5 va 5.2, chu o slide dau.
	many, reason := auto(sourceSlide{N: 5, Title: "U máu gan", Pictures: []sourcePicture{{N: 1}, {N: 2}, {N: 3}, {N: 4}, {N: 5}, {N: 6}}}, false)
	if reason != "" || len(many) != 2 || many[1].tag() != "5.2" || len(many[1].Imgs) != 2 || many[1].Imgs[0] != 5 {
		t.Fatalf("6 anh: %+v %q", many, reason)
	}
	bullets, reason := auto(sourceSlide{N: 6, Title: "Mục tiêu", Texts: []string{"• Một\n• Hai"}}, false)
	if reason != "" || len(bullets[0].Items) != 2 || bullets[0].Items[0] != "Một" {
		t.Fatalf("slide gach dau dong: %+v %q", bullets, reason)
	}
	// 5 dong: qua so muc lap -> giu nguyen van trong vung noi dung.
	long, reason := auto(sourceSlide{N: 7, Title: "Phân loại", Texts: []string{"1\n2\n3\n4\n5"}}, false)
	if reason != "" || long[0].Text != "1\n2\n3\n4\n5" || len(long[0].Items) != 0 {
		t.Fatalf("slide chu dai: %+v %q", long, reason)
	}
	if _, reason := auto(sourceSlide{N: 8, Title: "Bảng", Other: 1}, false); reason == "" {
		t.Fatal("slide co bang phai can model")
	}
	carry = ""
	if _, reason := auto(sourceSlide{N: 9, Pictures: []sourcePicture{{N: 1}}}, false); !strings.Contains(reason, "no title") {
		t.Fatalf("khong co tieu de nao: %q", reason)
	}
}

// Slide 2 cua Y4.5 (bai goc): de muc in dam, muc con cap 2, so tu dong "1." khong nam trong chu - dung lai phai
// giu ca ba (luot that 13/10/2026 18:10 ra mot khoi chu phang, mat so cua 4 muc con).
func TestAutoRowsKeepSourceHierarchy(t *testing.T) {
	_, roles := testRoles(t)
	carry := ""
	paras := []sourcePara{{T: "Chụp bụng không chuẩn bị", N: "1.", B: true}, {T: "2. Siêu âm:", B: true},
		{T: "SA gan mật", L: 2, N: "1."}, {T: "SA trong mổ", L: 2, N: "2."}, {T: "• Siêu âm-nội soi", L: 2},
		{T: "3. Chụp cắt lớp vi tính", B: true}, {T: "4. Chụp đường mật cản quang:", B: true}}
	slide := sourceSlide{N: 2, Title: "Các kỹ thuật thăm khám đường mật", Texts: []string{"…"},
		TitleParas: []sourcePara{{T: "Các kỹ thuật thăm khám đường mật"}}, TextParas: [][]sourcePara{paras}}
	rows, reason := autoRows(slide, roles, false, &carry)
	if reason != "" || len(rows) != 1 || len(rows[0].Paras) != 7 {
		t.Fatalf("slide chu nhieu cap: %+v %q", rows, reason)
	}
	row := rows[0]
	if !strings.HasPrefix(row.Text, "1. Chụp bụng") || row.Paras[4].T != "Siêu âm-nội soi" || !row.Paras[4].U || row.Paras[4].L != 2 {
		t.Fatalf("doan van: %q %+v", row.Text, row.Paras)
	}
	params, area, rest := textLayout(roles.picture, row, false)
	if params["anchor"] != "middle" || params["fontSize"] != maxBodyFont || rest != nil || area[2] != contentBox(roles.picture)[2] || params["font"] != "Arial" {
		t.Fatalf("bo cuc chu: %v %v %v", params, area, rest)
	}

	// Chu ngan canh anh = de muc cua nhom anh: dai in dam tren dau, anh lay ca be ngang ben duoi.
	label := planRow{Text: "Sỏi đường mật\nSA:", Imgs: []int{1, 2}}
	params, area, rest = textLayout(roles.picture, label, true)
	box := contentBox(roles.picture)
	if params["anchor"] != "top" || area[2] != box[2] || rest == nil || rest[2] != box[2] || rest[1] <= box[1] {
		t.Fatalf("de muc nhom anh: %v %v %v", params, area, rest)
	}
	if got := params["paragraphs"].([]sourcePara); !got[0].B || !got[1].B {
		t.Fatalf("de muc phai in dam: %+v", got)
	}
	// Cum de muc + anh can giua theo chieu doc, anh ngay duoi de muc (khong de khoang trong o giua).
	caption, pictures := stackCaption([4]float64{0, 100, 600, 40}, [][4]float64{{0, 200, 300, 150}, {312, 200, 288, 150}}, []float64{0, 100, 600, 400})
	if caption[1] != 199 || pictures[0][1] != 251 || pictures[1][1] != 251 {
		t.Fatalf("cum de muc + anh: %v %v", caption, pictures)
	}
	// Doan dai canh anh: cot trai, anh ben phai.
	long := planRow{Text: strings.Repeat("Sỏi nằm trong đường mật, đậm âm kèm bóng cản. ", 3), Imgs: []int{1}}
	if _, area, rest = textLayout(roles.picture, long, true); rest == nil || area[2] >= box[2]/2 || rest[0] <= area[0]+area[2] {
		t.Fatalf("doan dai canh anh: %v %v", area, rest)
	}
	// Dong ke hoach cua model (chu thuong): tab = cap, "•" = gach dau dong, dong ket thuc ":" = de muc.
	parsed := parasFromText("Kỹ thuật:\n\t• Chọc kim qua gan\nBiến chứng")
	if len(parsed) != 3 || !parsed[0].B || parsed[1].L != 2 || !parsed[1].U || parsed[1].T != "Chọc kim qua gan" || parsed[2].B {
		t.Fatalf("doan tu chu thuong: %+v", parsed)
	}
}

func TestPlanRowsReportErrorsPerField(t *testing.T) {
	_, roles := testRoles(t)
	slides := []sourceSlide{{N: 7, Title: "x", Pictures: []sourcePicture{{N: 1}, {N: 2}}}, {N: 8, Title: "y"}}
	plan := []any{
		map[string]any{"src": float64(7), "title": strings.Repeat("dài ", 40), "imgs": []any{float64(3)}},
		map[string]any{"src": float64(8), "title": "ổn", "items": []any{"a", "b", "c", "d"}},
		map[string]any{"src": float64(30), "title": "ngoài lô"},
	}
	rows, errors := parsePlan(plan, roles, slides)
	if len(errors) != 1 || errors[0].Error != "NOT_IN_BATCH" {
		t.Fatalf("src ngoai lo: %+v", errors)
	}
	got := map[string]bool{}
	for _, src := range []int{7, 8} {
		for _, problem := range validateRow(rows[src], roles, slides[src-7]) {
			got[fmt.Sprintf("%d.%s.%s", problem.Src, problem.Field, problem.Error)] = true
		}
	}
	for _, want := range []string{"7.title.TOO_LONG", "7.imgs.UNKNOWN_PICTURE", "8.items.TOO_MANY"} {
		if !got[want] {
			t.Fatalf("thieu loi %s, co %v", want, got)
		}
	}
	if rows[7].Kind != 2 {
		t.Fatalf("dong co anh tu chon kieu anh: %+v", rows[7])
	}
}

func TestPlanRowKeepsAutoFieldsItDoesNotWrite(t *testing.T) {
	auto := planRow{Src: 1, Kind: 1, Title: "CĐHA GAN", Texts: map[string]string{"TextBox 6": "THS PHAN NHẬT ANH"}}
	merged := mergeRow(auto, planRow{Src: 1, Kind: 1, Title: "CĐHA GAN MẬT", Texts: map[string]string{}, Notes: "mở đầu"})
	if merged.Title != "CĐHA GAN MẬT" || merged.Texts["TextBox 6"] != "THS PHAN NHẬT ANH" || merged.Notes != "mở đầu" {
		t.Fatalf("gop dong ke hoach: %+v", merged)
	}
	picture := mergeRow(planRow{Src: 5, Kind: 2, Title: "U máu", Imgs: []int{1, 2}, Text: "SA: tăng âm"},
		planRow{Src: 5, Kind: 2, Title: "U máu gan", Texts: map[string]string{}})
	if len(picture.Imgs) != 2 || picture.Text != "SA: tăng âm" {
		t.Fatalf("dong khong ghi anh phai giu anh tu dong: %+v", picture)
	}
}

func TestPictureBoxesSplitTheArea(t *testing.T) {
	boxes := pictureBoxes([]float64{222, 122.6, 726, 405.4}, [][2]float64{{400, 300}, {400, 300}})
	if len(boxes) != 2 || boxes[0][2] != 357 || boxes[1][0] != 591 {
		t.Fatalf("2 anh = %v", boxes)
	}
	// 4 anh khong ro co (coi la 4:3) trong o 212 x 112: 2 hang x 2, anh giu 4:3, khoi can giua be ngang.
	if grid := pictureBoxes([]float64{0, 0, 212, 112}, make([][2]float64, 4)); grid[3] != [4]float64{112, 62, 66.7, 50} || grid[0][0] != 33.3 {
		t.Fatalf("luoi 2x2 = %v", grid)
	}
	// Anh khac ti le cung hang: cao bang nhau, thang mep tren (slide 14 Y4.5: lech hang khi chia o bang nhau).
	mixed := pictureBoxes([]float64{0, 0, 700, 400}, [][2]float64{{400, 300}, {300, 300}, {400, 300}, {200, 300}})
	for i := range mixed {
		for j := range mixed {
			if mixed[i][1] == mixed[j][1] && mixed[i][3] != mixed[j][3] {
				t.Fatalf("anh cung hang phai cao bang nhau: %v", mixed)
			}
		}
	}
	// Anh nho tren slide nguon (100 x 75) khong phong qua 2 lan; khoi anh can giua vung.
	small := pictureBoxes([]float64{0, 0, 700, 400}, [][2]float64{{100, 75}})
	if small[0] != [4]float64{250, 125, 200, 150} {
		t.Fatalf("anh nho = %v", small)
	}
	// 3 anh chup vuong trong o rong 726 x 405: 2 cot x 2 hang (anh ~197 pt) to hon 3 cot mot hang (~234 pt
	// chieu rong nhung gioi han boi chieu cao)? - chon theo tong dien tich.
	square := pictureBoxes([]float64{0, 0, 726, 405}, [][2]float64{{300, 300}, {300, 300}, {300, 300}})
	if len(square) != 3 || square[0][2] < 230 {
		t.Fatalf("3 anh vuong = %v", square)
	}
	if ranges([]int{5, 1, 2, 3, 7, 8}) != "1-3, 5, 7-8" {
		t.Fatal(ranges([]int{5, 1, 2, 3, 7, 8}))
	}
}

// Bridge gia: bai nguon 4 slide (bia, anh, gach dau dong, bang), bai dich da co slide nguon 1 (tag).
func TestSlidesFromTemplateBuildsAndReportsNeedsPlan(t *testing.T) {
	template, _ := testRoles(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "bai cu.ppt")
	target := filepath.Join(dir, "bai cu - mau moi.pptx")
	calls := []string{}
	nextSlide := 1
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
			result = `{"slideCount":4,"slides":[` +
				`{"n":1,"title":"CĐHA GAN","texts":["THS PHAN NHẬT ANH"],"pictures":[]},` +
				`{"n":2,"title":"ĐẠI CƯƠNG","texts":["Đại thể"],"pictures":[{"n":1,"shape":"Picture 4"}]},` +
				`{"n":3,"title":"Mục tiêu","texts":["• Một\n• Hai"],"pictures":[]},` +
				`{"n":4,"title":"So sánh","texts":[],"pictures":[],"other":1}]}`
		case "wpp.listSlides":
			result = `{"slides":[{"index":1,"tag":"1"}]}`
		case "wpp.addSlideFromTemplate":
			nextSlide++
			result = `{"slide":` + strconv.Itoa(nextSlide) + `}`
		case "wpp.save":
			result = `{"saved":true}`
		case "wpp.checkLayout":
			result = `{"slides":[{"issues":[]}]}`
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":` + result + `}`))
	}))
	defer server.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	run := &RunContext{Office: &office.Session{App: "wpp", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "")}

	result := NewSlidesFromTemplateTool().Invoke(context.Background(), map[string]any{
		"template": template, "source": source}, run)
	if !result.OK || !result.Mutating {
		t.Fatalf("ket qua: %s", result.JSON)
	}
	var decoded struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal([]byte(result.JSON), &decoded)
	report := decoded.Result
	if report["built"] != "2-3" || report["alreadyBuilt"] != "1" || report["auto"] != float64(2) {
		t.Fatalf("bao cao: %s", result.JSON)
	}
	needs, _ := report["needsPlan"].([]any)
	if len(needs) != 1 || !strings.Contains(fmt.Sprint(needs[0]), "src:4") {
		t.Fatalf("needsPlan: %v", needs)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		`wpp.copyPicture {"format":"jpg","from":` + strconv.Quote(source) + `,"fromSlide":2`,
		`"shape":"Picture 4"`,
		`"shape":"TextBox 2","slide":2,"text":"ĐẠI CƯƠNG – Đại thể"}`,
		`"shape":"TextBox 7","slide":3,"text":"Một"}`,
		`"tag":"3"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("thieu lenh %s trong:\n%s", want, joined)
		}
	}
	if !strings.Contains(joined, "wpp.save") || report["saved"] == nil {
		t.Fatalf("bai moi phai duoc luu sau lo: %s", result.JSON)
	}
	if strings.Contains(joined, "wpp.newFromTemplate") {
		t.Fatal("bai dich dang mo thi khong tao lai")
	}
	// Lenh len bai dich phai dich danh bai moi (khong dua vao bai dang hoat dong).
	for _, line := range calls {
		if strings.HasPrefix(line, "wpp.setShapeText") && !strings.Contains(line, `"presentation":`+strconv.Quote(target)) {
			t.Fatalf("lenh khong dich danh bai moi: %s", line)
		}
	}
	if result.Final {
		t.Fatal("con slide 4 can model viet: chua duoc ket thuc luot")
	}

	// Model viet dong cho slide 4 -> moi slide nguon da co slide dich, da luu: tool bao xong, luot ket thuc.
	final := NewSlidesFromTemplateTool().Invoke(context.Background(), map[string]any{
		"template": template, "source": source,
		"plan": []any{map[string]any{"src": float64(4), "title": "So sánh", "text": "Siêu âm: rẻ\nCLVT: nhanh"}}}, run)
	if !final.Final || !strings.Contains(final.FinalText, "toàn bộ 4 slide") || !strings.Contains(final.FinalText, "bai cu - mau moi.pptx") {
		t.Fatalf("phai bao xong ca bai: final=%t %q\n%s", final.Final, final.FinalText, final.JSON)
	}
	// Chu cua dong ke hoach di qua khung nhieu doan: "Siêu âm: rẻ" giu nguyen, dich danh bai moi.
	written := calls[len(calls)-1]
	for _, line := range calls {
		if strings.HasPrefix(line, "wpp.addParagraphs") {
			written = line
		}
	}
	if !strings.Contains(written, `"t":"Siêu âm: rẻ"`) || !strings.Contains(written, `"presentation":`+strconv.Quote(target)) {
		t.Fatalf("khung chu cua dong ke hoach: %s", written)
	}
}

func TestCallLimitsStopRepeatedReads(t *testing.T) {
	run := &RunContext{}
	for i := 0; i < CallLimits["wpp.sourceManifest"]; i++ {
		if problem := run.CountCall("wpp.sourceManifest"); problem != "" {
			t.Fatalf("lan %d bi chan som: %s", i+1, problem)
		}
	}
	if problem := run.CountCall("wpp.sourceManifest"); !strings.Contains(problem, "limit reached") {
		t.Fatalf("vuot han muc phai bi chan: %q", problem)
	}
	for i := 0; i < 500; i++ {
		if problem := run.CountCall("et.writeRange"); problem != "" {
			t.Fatal("lenh ghi theo khoi khong co han muc")
		}
	}
}
