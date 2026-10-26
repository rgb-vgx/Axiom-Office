package tools

import (
	"strings"

	"axiomoffice/core/internal/office"
)

// Tai lieu dich cua luot (docs/plans/19-h0 H1). Bridge nhan tham so chon tai lieu cho moi lenh writer.*/et.*/wpp.*;
// Core tu gan tai lieu cua luot vao lenh nao model khong truyen, nen model khong phai nho tham so nay va mo ta tool
// khong dai them (an khoi chu ky lenh). Benchmark 25/10/2026: bai moi khong thanh bai active, slide cua luot bi ghi
// noi vao bai cua luot truoc.
var targetParamByKind = map[string]string{"wpp": "presentation", "et": "workbook", "wps": "document"}

func targetParam(command string) string {
	switch {
	case strings.HasPrefix(command, "wpp."):
		return targetParamByKind["wpp"]
	case strings.HasPrefix(command, "et."):
		return targetParamByKind["et"]
	case strings.HasPrefix(command, "writer."):
		return targetParamByKind["wps"]
	}
	return ""
}

// opensDocument: lenh tao/mo tai lieu - thanh cong thi tai lieu do thanh dich cua cac lenh sau trong luot.
func opensDocument(action string) bool {
	return strings.HasSuffix(action, ".open") || strings.HasSuffix(action, ".newPresentation") ||
		strings.HasSuffix(action, ".newDocument") || strings.HasSuffix(action, ".newWorkbook") || action == "wpp.newFromTemplate"
}

// withTarget: gan tai lieu dich vao params neu lenh nhan tham so do ma model khong truyen.
func withTarget(command office.Command, action string, params map[string]any, target string) map[string]any {
	name := targetParam(action)
	if name == "" || strings.TrimSpace(target) == "" || !commandHasParam(command, name) {
		return params
	}
	if params == nil {
		params = map[string]any{}
	}
	if value, ok := params[name].(string); !ok || strings.TrimSpace(value) == "" {
		params[name] = target
	}
	return params
}

// savedDocument: luu tai lieu DICH lam no doi ten (save lan dau cua "Book1" -> "...\Book1.xlsx"; saveAs -> duong dan moi)
// thi tra ten moi de cac lenh sau con nham dung tai lieu. Bo do Excel 26/10/2026 (x06): et.save so moi, moi lenh sau
// bao DOCUMENT_CLOSED "the workbook 'Book1' is not open". Luu tai lieu KHAC (tham so tai lieu khac dich) thi bo qua.
func savedDocument(action string, params, result map[string]any, target string) string {
	if !strings.HasSuffix(action, ".save") && !strings.HasSuffix(action, ".saveAs") {
		return ""
	}
	fullName, _ := result["fullName"].(string)
	fullName = strings.TrimSpace(fullName)
	if fullName == "" || strings.EqualFold(fullName, target) {
		return ""
	}
	if named, _ := params[targetParam(action)].(string); strings.TrimSpace(named) != "" && target != "" &&
		!strings.EqualFold(strings.TrimSpace(named), target) {
		return ""
	}
	return fullName
}

// openedDocument: tai lieu vua tao/mo (duong dan day du neu co, khong thi ten nhu "Presentation2").
func openedDocument(params, result map[string]any) string {
	for _, key := range []string{"fullName", "path", "name", "presentation"} {
		if value, _ := result[key].(string); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if path, _ := params["path"].(string); strings.TrimSpace(path) != "" {
		return strings.TrimSpace(path)
	}
	return ""
}
