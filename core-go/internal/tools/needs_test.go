package tools

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// bridgeLiteral: ten lenh bridge viet thang trong ma nguon ("writer.getText", "et.addChart"...).
var bridgeLiteral = regexp.MustCompile(`"((?:writer|et|wpp|app)\.[A-Za-z]+)"`)

// toolSources: tool goi lenh bridge truc tiep -> file ma nguon cua no. Them tool moi kieu nay thi them vao day
// (TestEveryDirectCallerDeclaresNeeds nhac).
var toolSources = map[string]struct {
	tool  Tool
	files []string
}{
	"slides_from_template": {NewSlidesFromTemplateTool(), []string{"template_build.go", "template_compose.go", "template_design.go"}},
	"slides_from_brief":    {NewSlidesFromBriefTool(nil), []string{"slides_brief.go"}},
	"doc_design":           {NewDocDesignTool(), []string{"doc_design.go"}},
	"doc_from_brief":       {NewDocFromBriefTool(), []string{"doc_brief.go"}},
	"dashboard_from_data":  {NewDashboardFromDataTool(), []string{"dashboard.go"}},
	"trace_formula":        {NewTraceFormulaTool(), []string{"trace_formula.go"}},
	VisualToolName:         {NewVisualTool(), []string{"visual.go"}},
}

func literalsIn(t *testing.T, files []string) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("doc %s: %v", file, err)
		}
		for _, match := range bridgeLiteral.FindAllStringSubmatch(string(data), -1) {
			seen[match[1]] = true
		}
	}
	out := []string{}
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Moi lenh tool goi (xuat hien trong ma nguon cua no) phai duoc khai bao Required hoac Optional: them lenh vao tool
// ma quen khai bao thi Core khong biet an tool tren backend thieu lenh do.
func TestToolNeedsCoverEveryCommandInSource(t *testing.T) {
	for name, entry := range toolSources {
		if entry.tool.Name() != name {
			t.Fatalf("toolSources[%q] la tool %q", name, entry.tool.Name())
		}
		needs := entry.tool.(NeedsCommands).Commands()
		declared := map[string]string{}
		for _, command := range needs.Required {
			declared[command] = "required"
		}
		for _, command := range needs.Optional {
			if declared[command] != "" {
				t.Errorf("%s: %s vua Required vua Optional", name, command)
			}
			declared[command] = "optional"
		}
		used := literalsIn(t, entry.files)
		for _, command := range used {
			if declared[command] == "" {
				t.Errorf("%s goi %s nhung khong khai bao trong Commands() (tools/needs.go)", name, command)
			}
		}
		// Khai bao thua (lenh tool khong con goi) lam an tool oan tren backend thieu lenh do.
		inSource := map[string]bool{}
		for _, command := range used {
			inSource[command] = true
		}
		for command := range declared {
			if !inSource[command] {
				t.Errorf("%s khai bao %s nhung ma nguon khong con goi", name, command)
			}
		}
	}
}

// Moi lenh khai bao phai co trong catalog chuan (catalog/live-commands.json): sai chinh ta thi tool bi an moi noi.
func TestToolNeedsExistInCatalog(t *testing.T) {
	data, err := os.ReadFile("../../../catalog/live-commands.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, command := range catalog {
		known[command.Name] = true
	}
	for name, entry := range toolSources {
		needs := entry.tool.(NeedsCommands).Commands()
		for _, command := range append(append([]string{}, needs.Required...), needs.Optional...) {
			if !known[command] {
				t.Errorf("%s khai bao %s - khong co trong catalog/live-commands.json", name, command)
			}
		}
	}
}

// Tool co Invoke ma ma nguon goi lenh bridge thang phai co trong toolSources (va khai bao Commands). office_action la
// ngoai le: lenh do model chon, catalog cua phien da gioi han san.
func TestEveryDirectCallerDeclaresNeeds(t *testing.T) {
	invoke := regexp.MustCompile(`func \(t \*([A-Za-z]+)\) Invoke\(`)
	exempt := map[string]bool{"OfficeActionTool": true}
	covered := map[string]bool{}
	for _, entry := range toolSources {
		for _, file := range entry.files {
			covered[file] = true
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		file := entry.Name()
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, _ := os.ReadFile(file)
		owners := invoke.FindAllStringSubmatch(string(data), -1)
		if len(owners) == 0 || !bridgeLiteral.Match(data) {
			continue
		}
		for _, owner := range owners {
			if exempt[owner[1]] || covered[file] {
				continue
			}
			t.Errorf("%s (%s) goi lenh bridge truc tiep: them Commands() o tools/needs.go va dong o toolSources", owner[1], file)
		}
	}
}

func TestWithCommandsHidesToolsMissingRequired(t *testing.T) {
	has := func(names ...string) func(string) bool {
		set := map[string]bool{}
		for _, name := range names {
			set[name] = true
		}
		return func(name string) bool { return set[name] }
	}
	list := []Tool{NewDocDesignTool(), NewDashboardFromDataTool(), NewTodoTool()}
	kept, hidden := WithCommands(list, has("writer.getText", "writer.outline", "writer.insertBlocks",
		"et.listSheets", "et.readRange", "et.addSheet", "et.writeRange", "et.formatRange", "et.addChart", "et.activateSheet"))
	names := []string{}
	for _, tool := range kept {
		names = append(names, tool.Name())
	}
	// dashboard thieu et.deleteSheet (Optional) van giu; doc_design thieu writer.restyle (Required) bi an; tool khong
	// khai bao (write_todos) giu nguyen.
	if strings.Join(names, ",") != "dashboard_from_data,"+TodoToolName {
		t.Fatalf("kept = %v", names)
	}
	if strings.Join(hidden["doc_design"], ",") != "writer.restyle" {
		t.Fatalf("hidden = %v", hidden)
	}
}
