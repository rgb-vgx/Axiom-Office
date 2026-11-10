package tools

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Golden chuoi lenh bridge cua buildComposed (docs/plans/18-s0 M1): ghim DUNG lenh va tham so che do bien soan phat ra
// truoc khi tach solver sang package rieng (M2, M3) - doi bat cu so nao trong day la doi hanh vi cua duong mau dang
// chay that. Cap nhat co chu dich: go test ./internal/tools -run ComposeBridgeGolden -update-golden
var updateGolden = flag.Bool("update-golden", false, "ghi lai testdata/compose_bridge_golden.json")

func TestComposeBridgeGolden(t *testing.T) {
	templatePath, roles := testRoles(t)
	content, cover := roles.picture, roles.cover
	points := func(texts ...string) []sourcePara {
		list := []sourcePara{}
		for _, text := range texts {
			list = append(list, sourcePara{T: text})
		}
		return list
	}
	card := func(title string, texts ...string) composeCard {
		return composeCard{Title: title, Points: points(texts...)}
	}
	plans := map[string]composePlan{
		"cover": {kind: cover, notes: "ghi chu bia",
			row: composeRow{Layout: "cover", Title: "CĐHA CẤP CỨU BỤNG", Subtitle: "ThS. Phan Nhật Anh"}},
		"content-text": {kind: content, notes: "ghi chu", points: points("**Đứng**, bụng áp phim", "Tia trung tâm ở đường giữa"),
			row: composeRow{Layout: "content", Title: "XQUANG BKCB – TƯ THẾ ĐỨNG"}},
		"content-pictures": {kind: content, notes: "ghi chu", points: points("Liềm hơi dưới hoành"),
			pictures: []sourcePicture{{N: 1, Slide: 7, Shape: "Picture 4", W: 400, H: 300}, {N: 2, Slide: 7, Shape: "Picture 5", W: 300, H: 400}},
			row:      composeRow{Layout: "content", Title: "THỦNG TẠNG RỖNG", Caption: "Trái: đứng • Phải: nằm", Callout: "⚠ Bắt buộc lấy vòm hoành"}},
		"cards-3": {kind: content, notes: "ghi chu",
			row: composeRow{Layout: "cards", Title: "VAI TRÒ XQUANG", Cards: []composeCard{card("Ưu điểm", "Rẻ, nhanh"),
				card("Hạn chế", "Độ nhạy thấp"), card("Chỉ định", "Nghi thủng", "Nghi tắc ruột")}}},
		"cards-4": {kind: content, notes: "ghi chu",
			row: composeRow{Layout: "cards", Title: "BỐN NHIỆM VỤ", Cards: []composeCard{card("A", "a"), card("B", "b"),
				card("C", "c"), card("D", "d")}}},
		"table": {kind: content, notes: "ghi chu",
			row: composeRow{Layout: "table", Title: "SO SÁNH KỸ THUẬT", Table: [][]string{{"Bệnh cảnh", "Xquang", "CT"},
				{"Thủng tạng rỗng", "+", "++"}, {"Tắc ruột", "+", "++"}}}},
	}

	recorded := map[string][]map[string]any{}
	for name, plan := range plans {
		calls := []map[string]any{}
		bridge := func(action string, params map[string]any) (map[string]any, string) {
			calls = append(calls, map[string]any{"action": action, "params": params})
			switch action {
			case "wpp.addSlideFromTemplate":
				return map[string]any{"slide": 5.0}, ""
			case "wpp.checkLayout":
				return map[string]any{"slides": []any{map[string]any{"issues": []any{}}}}, ""
			}
			return map[string]any{}, ""
		}
		plan.points = append([]sourcePara(nil), plan.points...)
		if _, _, problem := buildComposed(bridge, templatePath, `C:\nguon\bai-cu.pptx`, plan, "src:3-4"); problem != "" {
			t.Fatalf("%s: %s", name, problem)
		}
		recorded[name] = calls
	}
	data, err := json.MarshalIndent(recorded, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Duong dan mau la thu muc tam cua test: thay bang ten co dinh.
	escaped, _ := json.Marshal(templatePath)
	got := strings.ReplaceAll(string(data), strings.Trim(string(escaped), `"`), "<template>") + "\n"

	golden := filepath.Join("testdata", "compose_bridge_golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("thieu %s - chay voi -update-golden: %v", golden, err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
		t.Fatalf("chuoi lenh bridge cua buildComposed da doi so voi %s; neu co chu dich thi chay lai voi -update-golden va "+
			"giai thich trong commit", golden)
	}
}
