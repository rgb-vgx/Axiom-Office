package api

import (
	"strings"
	"testing"
)

// Pane WPS dung GET /wps/v1/quick-actions de lay nhan: `actions` = menu "Sua bang AI" (giu nguyen cho khung chat mini va
// menu chuot phai), `documentActions` = lenh ca tai lieu cua nut ribbon - tach rieng de khong lot vao menu boi den.
func TestQuickActionsListsDocumentActionsSeparately(t *testing.T) {
	handler, _ := newSkillsHandler(t)
	reply := call(handler, "GET", "/wps/v1/quick-actions", "", wpsOrigin)
	result, _ := decode(t, reply.Body.String())["result"].(map[string]any)
	actions, _ := result["actions"].([]any)
	documents, _ := result["documentActions"].([]any)
	if reply.Code != 200 || len(actions) != 15 || len(documents) != 7 {
		t.Fatalf("quick-actions: %d %s", reply.Code, reply.Body.String())
	}
	first, _ := documents[0].(map[string]any)
	if first["id"] != "proofread" || first["label"] != "Soát chính tả cả bài" || first["edits"] != true {
		t.Fatalf("lenh ca tai lieu: %v", first)
	}
	// app/input: chi lenh can moi co (nut theo ung dung, lenh cho nguoi dung go yeu cau).
	if _, has := first["app"]; has {
		t.Fatalf("proofread khong mang app: %v", first)
	}
	create, _ := documents[2].(map[string]any)
	slides, _ := documents[3].(map[string]any)
	if create["id"] != "smartCreate" || create["app"] != "wps" || create["input"] == nil || create["input"] == "" ||
		slides["id"] != "slidesFromDoc" || slides["app"] != "wpp" || slides["input"] != nil {
		t.Fatalf("lenh Tao moi: %v / %v", create, slides)
	}
	if strings.Contains(reply.Body.String(), "Soát chính tả cả tài liệu đang mở") {
		t.Fatal("cau lenh (prompt) khong tra ra cho pane")
	}
}

// shownIn: ung dung khac App cung hien nut (pane Writer giu "Tao slide tu tai lieu" trong documentActions cua no).
func TestQuickActionsReportShownIn(t *testing.T) {
	handler, _ := newSkillsHandler(t)
	reply := call(handler, "GET", "/wps/v1/quick-actions", "", wpsOrigin)
	result, _ := decode(t, reply.Body.String())["result"].(map[string]any)
	documents, _ := result["documentActions"].([]any)
	for _, raw := range documents {
		item, _ := raw.(map[string]any)
		shown, has := item["shownIn"].([]any)
		if item["id"] == "slidesFromDoc" {
			if !has || len(shown) != 1 || shown[0] != "wps" || item["app"] != "wpp" {
				t.Fatalf("slidesFromDoc: app wpp, shownIn [wps]; duoc %v", item)
			}
		} else if has {
			t.Fatalf("%v khong co shownIn", item)
		}
	}
}
