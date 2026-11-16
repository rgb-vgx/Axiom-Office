package policy

import (
	"context"
	"strings"
	"testing"
)

// matchAction/matchResource la phan de sai nhat cua lop chinh sach: kiem tra ro "*" (mot doan) va
// "**" (nhieu doan) o ranh gioi duong dan.
func TestRuleWildcardMatching(t *testing.T) {
	cases := []struct {
		pattern, value string
		want           bool
	}{
		{"writer.save", "writer.save", true},
		{"writer.save", "writer.saveAs", false},
		{"*.save", "writer.save", true},
		{"et.*", "et.writeRange", true},
		{"et.*", "et.range.format", false},
		{"et.**", "et.writeRange", true},
		{"et.**", "et.range.format", true},
		{"et.**", "writer.save", false},
		{"**", "anything.at.all", true},
		{"*", "writer.save", false},
		{"writer.get*", "writer.getText", true},
		{"writer.get*", "writer.setTitle", false},
		{"", "writer.save", true},
	}
	for _, item := range cases {
		if got := matchAction(item.pattern, item.value); got != item.want {
			t.Errorf("action %q ~ %q = %v, muon %v", item.pattern, item.value, got, item.want)
		}
	}

	resources := []struct {
		pattern, value string
		want           bool
	}{
		{"**", `/tmp/bao-cao.docx`, true},
		{"**/*.docx", `/tmp/bao-cao.docx`, true},
		{"**/*.docx", `C:\tai-lieu\bao-cao.docx`, true},
		{"**/*.docx", "/tmp/bao-cao.xlsx", false},
		{"/*/bao-cao.docx", "/tmp/bao-cao.docx", true},
		{"C:/tai-lieu/*.docx", `C:\tai-lieu\bao-cao.docx`, true},
		{"C:/tai-lieu/*.docx", `c:\TAI-LIEU\bao-cao.docx`, true},
		{"C:/tai-lieu/*.docx", `C:\tai-lieu\khac\bao-cao.docx`, false},
		{"", "", true},
	}
	for _, item := range resources {
		if got := matchResource(item.pattern, item.value); got != item.want {
			t.Errorf("resource %q ~ %q = %v, muon %v", item.pattern, item.value, got, item.want)
		}
	}
}

func TestDecideLastMatchWins(t *testing.T) {
	rules := Ruleset{
		{Action: "*.save", Effect: EffectAsk},
		{Action: "writer.save", Effect: EffectAllow},
	}
	if decision := Decide(rules, "writer.save", ""); decision.NeedsConfirmation || decision.Denied {
		t.Fatalf("quy tac sau (allow) phai thang: %+v", decision)
	}
	if decision := Decide(rules, "et.save", ""); !decision.NeedsConfirmation {
		t.Fatalf("et.save chi khop quy tac ask: %+v", decision)
	}

	// Khong quy tac nao khop = allow.
	if decision := Decide(nil, "writer.getText", ""); decision.NeedsConfirmation || decision.Denied {
		t.Fatalf("khong khop phai allow: %+v", decision)
	}
}

// Deny thang ask du ask nam SAU (mot quy tac cam khong bi mot quy tac hoi de len), nhung allow van
// thang deny neu allow la quy tac khop cuoi cung.
func TestDecideDenyBeatsAsk(t *testing.T) {
	rules := Ruleset{
		{Action: "et.deleteSheet", Effect: EffectDeny},
		{Action: "et.*", Effect: EffectAsk},
	}
	decision := Decide(rules, "et.deleteSheet", "")
	if !decision.Denied || decision.NeedsConfirmation {
		t.Fatalf("deny phai thang ask: %+v", decision)
	}

	allow := Ruleset{
		{Action: "et.deleteSheet", Effect: EffectDeny},
		{Action: "et.deleteSheet", Effect: EffectAllow},
	}
	if decision := Decide(allow, "et.deleteSheet", ""); decision.Denied || decision.NeedsConfirmation {
		t.Fatalf("allow khop cuoi cung phai thang: %+v", decision)
	}
}

// DefaultRules mo ta dung cac lop lenh hien phai hoi, va lenh chi doc la allow.
func TestDefaultRulesCoverConfirmationClasses(t *testing.T) {
	rules := DefaultRules()
	for _, action := range []string{
		"writer.save", "et.saveAs", "wpp.exportPdf", "wpp.newFromTemplate",
		"wpp.deleteSlide", "wpp.deleteShapes", "wpp.deleteTagged", "et.deleteSheet",
		"writer.replaceAll", "writer.replaceMany",
		"mcp__office__excel_write", "mcp__github__create_issue",
	} {
		decision := Decide(rules, action, `/tmp/bao-cao.docx`)
		if !decision.NeedsConfirmation || decision.Denied {
			t.Errorf("%s phai la ask trong DefaultRules: %+v", action, decision)
		}
		if decision.Explicit {
			t.Errorf("%s: quy tac mac dinh khong duoc coi la nguoi dung khai bao", action)
		}
	}
	for _, action := range []string{"writer.getText", "et.readRange", "wpp.listSlides", "app.info"} {
		if decision := Decide(rules, action, ""); decision.NeedsConfirmation || decision.Denied {
			t.Errorf("%s la lenh chi doc, phai allow: %+v", action, decision)
		}
	}
}

// Check: quy tac nguoi dung khai bao ro la quyet dinh cuoi cung; quy tac mac dinh de Evaluate quyet
// dinh tung truong hop (giu nguyen hanh vi cu: nguoi dung yeu cau luu thi khong hoi).
func TestCheckEvaluateKeepsExistingBehaviour(t *testing.T) {
	rules := DefaultRules()
	ask := Check(rules, context.Background(), "writer.save", "", nil, "Tạo bảng điểm", nil)
	if !ask.NeedsConfirmation || !strings.Contains(ask.Reason, "lưu") {
		t.Fatalf("save khong co y dinh phai hoi: %+v", ask)
	}
	allow := Check(rules, context.Background(), "writer.save", "", nil, "Lưu file giúp mình", nil)
	if allow.NeedsConfirmation || allow.Denied {
		t.Fatalf("nguoi dung da yeu cau luu thi khong hoi: %+v", allow)
	}

	// Quy tac nguoi dung: deny chan, allow bo qua dieu kien thuc te.
	deny := append(DefaultRules(), Rule{Action: "et.writeRange", Effect: EffectDeny})
	if decision := Check(deny, context.Background(), "et.writeRange", "", nil, "ghi du lieu", nil); !decision.Denied {
		t.Fatalf("quy tac deny phai chan: %+v", decision)
	}
	allowSave := append(DefaultRules(), Rule{Action: "writer.save", Effect: EffectAllow})
	if decision := Check(allowSave, context.Background(), "writer.save", "", nil, "Tạo bảng điểm", nil); decision.NeedsConfirmation {
		t.Fatalf("quy tac allow cua nguoi dung phai bo qua cau hoi: %+v", decision)
	}
	askWrite := append(DefaultRules(), Rule{Action: "et.writeRange", Effect: EffectAsk})
	if decision := Check(askWrite, context.Background(), "et.writeRange", "", nil, "ghi du lieu", nil); !decision.NeedsConfirmation {
		t.Fatalf("quy tac ask cua nguoi dung phai hoi: %+v", decision)
	}
}

func TestParseEffect(t *testing.T) {
	for _, item := range []struct {
		raw  string
		want Effect
		ok   bool
	}{
		{"allow", EffectAllow, true}, {"ASK", EffectAsk, true}, {"deny", EffectDeny, true},
		{" block ", EffectDeny, true}, {"", "", false}, {"maybe", "", false},
	} {
		got, ok := ParseEffect(item.raw)
		if got != item.want || ok != item.ok {
			t.Errorf("ParseEffect(%q) = %q,%v muon %q,%v", item.raw, got, ok, item.want, item.ok)
		}
	}
}

// Chi lop lenh rui ro mac dinh moi duoc luu "luon dong y"; mot lenh ghi chung nhu et.writeRange
// khong duoc phep, neu khong se bo qua guard ghi de/sort cua moi tai lieu.
func TestApprovableOnlyRiskClasses(t *testing.T) {
	for _, action := range []string{"et.saveAs", "writer.save", "wpp.deleteSlide", "et.deleteSheet", "mcp__foo__bar"} {
		if !Approvable(action) {
			t.Errorf("Approvable(%q) = false, want true", action)
		}
	}
	for _, action := range []string{"", "et.writeRange", "writer.typeText", "writer.getText", "unknown.action"} {
		if Approvable(action) {
			t.Errorf("Approvable(%q) = true, want false", action)
		}
	}
}
