package tools

import "sort"

// Hop dong "tool can lenh bridge nao": tool lon (dung slide, trinh bay van ban, dashboard...) goi mot chuoi lenh bridge
// co dinh. Mot backend thieu lenh (WPS Linux, LibreOffice, ban add-in cu) ma van dua tool cho model thi tool hong
// GIUA CHUNG - da sua mot nua tai lieu roi moi bao loi. Truoc 31/10/2026 Core chi kiem mot hai lenh "dai dien" chon
// tay (doc_design kiem writer.insertBlocks nhung con goi writer.restyle, writer.outline).
//
// Moi tool khai bao CommandNeeds; Core an tool khi catalog cua phien thieu mot lenh Required (WithCommands).
// tools/needs_test.go doc ma nguon cua tung tool va bat buoc moi lenh xuat hien trong do phai nam o Required hoac
// Optional - them lenh moi vao tool ma quen khai bao thi test do.

// CommandNeeds: lenh bridge mot tool goi.
type CommandNeeds struct {
	// Required: duong chinh cua tool - thieu mot lenh thi tool khong lam duoc viec, Core khong dua tool cho model.
	Required []string
	// Optional: chi dung o nhanh phu (slide co bieu do, dung lai dashboard, mo bai dich chua mo...): thieu thi nhanh
	// do bao loi ro rang, phan con lai cua tool van chay.
	Optional []string
}

// NeedsCommands: tool goi lenh bridge truc tiep (khong qua office_action cua model).
type NeedsCommands interface {
	Commands() CommandNeeds
}

// MissingCommands: lenh Required ma catalog khong co (rong = tool dung duoc). Tool khong khai bao thi khong thieu gi.
func MissingCommands(tool Tool, has func(string) bool) []string {
	needs, ok := tool.(NeedsCommands)
	if !ok {
		return nil
	}
	missing := []string{}
	for _, name := range needs.Commands().Required {
		if !has(name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// WithCommands bo cac tool thieu lenh Required; hidden: ten tool -> lenh thieu (de ghi log, giai thich vi sao tool
// khong co tren mot backend).
func WithCommands(list []Tool, has func(string) bool) (kept []Tool, hidden map[string][]string) {
	hidden = map[string][]string{}
	for _, tool := range list {
		if missing := MissingCommands(tool, has); len(missing) > 0 {
			hidden[tool.Name()] = missing
			continue
		}
		kept = append(kept, tool)
	}
	return kept, hidden
}

// ---- Khai bao cua tung tool (doc cung ma nguon cua tool: needs_test.go doi chieu) ----

// Commands: slides_from_template (template_build.go, template_compose.go, template_design.go).
func (t *SlidesFromTemplateTool) Commands() CommandNeeds {
	return CommandNeeds{
		Required: []string{"wpp.sourceManifest", "wpp.listSlides", "wpp.addSlideFromTemplate", "wpp.setShapeText",
			"wpp.deleteShapes", "wpp.addParagraphs", "wpp.copyPicture", "wpp.setNotes", "wpp.checkLayout", "wpp.save",
			"wpp.deleteTagged"},
		// open/newFromTemplate: chi khi bai dich chua mo; addGrid: slide co bang; app.info: doc ten app cho thong bao;
		// addText: nam trong danh sach lenh duoc gan bai dich (targetActions), tool khong tu goi.
		Optional: []string{"wpp.open", "wpp.newFromTemplate", "wpp.addGrid", "wpp.addText", "app.info"},
	}
}

// Commands: slides_from_brief (slides_brief.go).
func (t *SlidesFromBriefTool) Commands() CommandNeeds {
	return CommandNeeds{
		Required: []string{"wpp.addSlide", "wpp.addParagraphs", "wpp.addShape", "wpp.checkLayout"},
		// deleteSlide: dung lai slide cu (replace); setNotes: slide co ghi chu; addChart/addGrid: slide bieu do/bang;
		// addConnector/deleteShapes: so do (noi day, di lai day).
		Optional: []string{"wpp.deleteSlide", "wpp.setNotes", "wpp.addChart", "wpp.addGrid", "wpp.addConnector",
			"wpp.deleteShapes"},
	}
}

// Commands: doc_design (doc_design.go).
func (t *DocDesignTool) Commands() CommandNeeds {
	return CommandNeeds{Required: []string{"writer.getText", "writer.restyle", "writer.outline", "writer.insertBlocks"}}
}

// Commands: doc_from_brief (doc_brief.go).
func (t *DocFromBriefTool) Commands() CommandNeeds {
	return CommandNeeds{
		Required: []string{"writer.getText", "writer.insertLayoutTable", "writer.writeContent", "writer.checkLayout"},
		// insertToc: van ban co muc luc; setHeaderFooter: danh so trang.
		Optional: []string{"writer.insertToc", "writer.setHeaderFooter"},
	}
}

// Commands: dashboard_from_data (dashboard.go).
func (t *DashboardFromDataTool) Commands() CommandNeeds {
	return CommandNeeds{
		Required: []string{"et.listSheets", "et.readRange", "et.addSheet", "et.writeRange", "et.formatRange",
			"et.addChart", "et.activateSheet"},
		// deleteSheet: chi khi dung lai dashboard do chinh luot nay tao (replace=true).
		Optional: []string{"et.deleteSheet"},
	}
}

// Commands: trace_formula (trace_formula.go).
func (t *TraceFormulaTool) Commands() CommandNeeds {
	return CommandNeeds{Required: []string{"et.readRanges", "et.sheetInfo"}}
}

// Commands: look_at_document (visual.go). wpp.renderSlide chi khi xem mot slide theo so; khong co thi chup man hinh.
func (t *VisualTool) Commands() CommandNeeds {
	return CommandNeeds{Required: []string{"app.screenshot"}, Optional: []string{"wpp.renderSlide"}}
}
