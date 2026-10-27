package tools

import (
	"strconv"
	"strings"
)

// Hoan tac chi trong pham vi luot (docs/plans/21-v0 P0). Bridge goi Undo cua app `count` lan, khong biet buoc nao cua
// luot: bo do Word 26/10/2026 (w08) model goi writer.undo {count:30} va xoa sach noi dung co san cua nguoi dung. Excel
// con te hon - app.Undo() chi lui duoc thao tac giao dien CUOI cua nguoi dung (lenh ghi qua COM khong vao ngan undo),
// nen undo khi luot chua sua gi la hoan tac viec cua nguoi dung. Core dem so lenh sua thanh cong cua luot (moi lenh
// writer.*/wpp.* la mot buoc "AI: ..." trong ngan undo) va khong cho lui qua so do.

func isUndo(action string) bool { return strings.HasSuffix(action, ".undo") }

// noUndoStep: lenh khong nam trong noDocumentChange nhung cung KHONG tao buoc hoan tac (doc, chon sheet, tao/mo/dong
// tai lieu, khoa vung chon). Doi chieu voi truong "undo" cua catalog/live-commands.json trong undo_test.go - them lenh
// moi ma quen o day thi test bao.
var noUndoStep = map[string]bool{
	"ai.ask": true, "ui.askpane": true, "ui.setup": true,
	"writer.newDocument": true, "writer.open": true, "writer.closeAll": true,
	"writer.lockSelection": true, "writer.unlockSelection": true,
	"et.newWorkbook": true, "et.open": true, "et.closeAll": true, "et.sheetInfo": true, "et.activateSheet": true,
	"et.listConditionalFormats": true, "et.listCharts": true, "et.deleteSheet": true,
	"wpp.newPresentation": true, "wpp.open": true, "wpp.closeAll": true, "wpp.newFromTemplate": true,
	"wpp.sourceManifest": true, "wpp.renderSlide": true,
}

// makesUndoStep: lenh nay de lai mot buoc trong ngan hoan tac cua app.
func makesUndoStep(action string) bool {
	return ChangesDocument(action) && !noUndoStep[action] && !isUndo(action)
}

func undoApp(action string) string {
	if index := strings.Index(action, "."); index > 0 {
		return action[:index]
	}
	return action
}

// limitUndo: cat count ve so buoc cua luot; luot chua sua gi thi tu choi (tra cau loi cho model).
func limitUndo(run *RunContext, action string, params map[string]any) (map[string]any, string) {
	left := run.undoable[undoApp(action)]
	if left <= 0 {
		return params, "nothing to undo: this request has not changed the document yet - undo only reverts this " +
			"request's own edits, never what the user did before. To fix earlier content, edit it directly"
	}
	count := 1
	if value, ok := params["count"]; ok {
		if parsed := asInt(value); parsed > 0 {
			count = parsed
		} else if text, ok := value.(string); ok {
			if parsed, err := strconv.Atoi(strings.TrimSpace(text)); err == nil && parsed > 0 {
				count = parsed
			}
		}
	}
	if params == nil {
		params = map[string]any{}
	}
	params["count"] = min(count, left)
	return params, ""
}

// countUndoable: lenh sua thanh cong (co de lai buoc hoan tac) -> them mot buoc; undo thanh cong -> tru so buoc da lui.
func countUndoable(run *RunContext, action string, mutating bool, result map[string]any) {
	app := undoApp(action)
	if run.undoable == nil {
		run.undoable = map[string]int{}
	}
	switch {
	case isUndo(action):
		run.undoable[app] = max(run.undoable[app]-asInt(result["undone"]), 0)
	case mutating && makesUndoStep(action):
		run.undoable[app]++
	}
}

// addUndoSteps: tool tu goi bridge (doc_from_brief, dashboard_from_data) - moi lenh sua da chay la mot buoc hoan tac
// cua luot, de writer.undo / et.undo lui duoc phan tool vua dung.
func (run *RunContext) addUndoSteps(action string, steps int) {
	if steps <= 0 {
		return
	}
	if run.undoable == nil {
		run.undoable = map[string]int{}
	}
	run.undoable[undoApp(action)] += steps
}

// UndoSteps: so buoc hoan tac CHAC CHAN cua luot con lai trong app ("writer", "et", "wpp") - da tru cac lan *.undo cua
// chinh luot. Pane dung so nay cho nut "Hoan tac luot nay" thay vi tu dem lenh ghi (tu dem thi khong tru duoc lan agent
// tu undo, va lui vao noi dung co tu truoc luot - audit agent runtime F02).
func (run *RunContext) UndoSteps(app string) int {
	if run == nil {
		return 0
	}
	return run.undoable[app]
}
