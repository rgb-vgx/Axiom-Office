package agent

import (
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/tools"
)

func planCatalog() *office.CommandCatalog {
	return &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "wpp.listSlides", Kind: "wpp", Agent: true},
		{Name: "wpp.checkLayout", Kind: "wpp", Agent: true},
		{Name: "wpp.addShape", Kind: "wpp", Agent: true},
		{Name: "wpp.addConnector", Kind: "wpp", Agent: true},
		{Name: "wpp.addParagraphs", Kind: "wpp", Agent: true},
		{Name: "wpp.closeAll", Kind: "wpp", Agent: false},
	}}
}

// Luot that 25/10/2026: o lap ke hoach model chi thay lenh doc nen len ke hoach so do bang chu. Ke hoach phai biet
// lenh ghi va tool chi co khi chay (slides_from_brief), nhung khong duoc goi chung luc nay.
func TestPlanCapabilitiesListsWriteActionsAndActTools(t *testing.T) {
	o := &Orchestrator{}
	catalog := planCatalog()
	text := PlanCapabilities(tools.NewOfficeActionTool(catalog, "wpp"), o.actTools("wpp", catalog, &Request{}))
	for _, want := range []string{"WHEN THE USER RUNS THE PLAN", "do not call them now", "wpp.addShape", "wpp.addConnector",
		"Tool slides_from_brief: "} {
		if !strings.Contains(text, want) {
			t.Fatalf("thieu %q: %s", want, text)
		}
	}
	// Lenh doc da co trong tool cua luot; lenh khong cho agent va write_todos khong nhac lai.
	for _, unwanted := range []string{"wpp.listSlides", "wpp.closeAll", tools.TodoToolName} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("khong duoc co %q: %s", unwanted, text)
		}
	}
}

func TestPlanCapabilitiesEmptyWhenNothingToAdd(t *testing.T) {
	reads := &office.CommandCatalog{Commands: []office.Command{{Name: "wpp.listSlides", Kind: "wpp", Agent: true}}}
	if text := PlanCapabilities(tools.NewOfficeActionTool(reads, "wpp"), nil); text != "" {
		t.Fatalf("chi co lenh doc thi khong them gi: %q", text)
	}
}

// actTools giu dieu kien cu: slides_from_brief chi cho PowerPoint co wpp.addParagraphs, doc_design khong mo khi dang
// khoa vung boi den, write_todos luon co.
func TestActToolsConditions(t *testing.T) {
	o := &Orchestrator{}
	names := func(list []tools.Tool) string {
		out := []string{}
		for _, tool := range list {
			out = append(out, tool.Name())
		}
		return strings.Join(out, ",")
	}
	if got := names(o.actTools("wpp", planCatalog(), &Request{})); got != "slides_from_brief,"+tools.TodoToolName {
		t.Fatalf("wpp: %s", got)
	}
	writer := &office.CommandCatalog{Commands: []office.Command{{Name: "writer.insertBlocks", Kind: "wps", Agent: true}}}
	if got := names(o.actTools("wps", writer, &Request{})); !strings.HasPrefix(got, "doc_design,") {
		t.Fatalf("wps: %s", got)
	}
	locked := &Request{Document: &DocumentContext{SelectionLocked: true}}
	if got := names(o.actTools("wps", writer, locked)); got != tools.TodoToolName {
		t.Fatalf("wps dang khoa vung: %s", got)
	}
}
