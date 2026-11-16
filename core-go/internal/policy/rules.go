package policy

import (
	"context"
	"strings"
)

// Effect: y dinh cua mot quy tac chinh sach khai bao (policy.rules) cho mot lenh.
type Effect string

const (
	// EffectAllow: cho chay, khong hoi. Cung la ket qua khi khong quy tac nao khop.
	EffectAllow Effect = "allow"
	// EffectAsk: hoi nguoi dung truoc khi chay (confirm.required).
	EffectAsk Effect = "ask"
	// EffectDeny: chan han, khong hoi va khong xuong bridge.
	EffectDeny Effect = "deny"
)

// ParseEffect doc chuoi effect tu cau hinh. "true"/"1" khong nhan: quy tac phai noi ro y dinh.
func ParseEffect(value string) (Effect, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow", "permit", "ok":
		return EffectAllow, true
	case "ask", "confirm":
		return EffectAsk, true
	case "deny", "block", "forbid":
		return EffectDeny, true
	}
	return "", false
}

// Rule: mot quy tac chinh sach cho mot lenh (Action) tren mot tai lieu/nguon (Resource).
//
// Wildcard (khop theo DOAN duong dan, khong phai khop chuoi con):
//   - "*"  khop dung MOT doan: "writer.save" khop "writer.save", "*.save" khop "writer.save" nhung
//     khong khop "a.b.save"; "writer.*" khop "writer.save" nhung khong khop "writer.section.save".
//   - "**" khop KHONG hoac NHIEU doan: "**" khop moi thu, "et.**" khop "et.writeRange" va
//     "et.range.format", "**/*.docx" khop "C:/tai-lieu/bao-cao.docx".
//   - Trong mot doan, "*" van khop chuoi con tuy y ("writer.get*" khop "writer.getText").
//
// Action tach doan bang '.', Resource tach doan bang '/' (hoac '\'). So khop khong phan biet hoa/thuong.
// Rule rong (Action/Resource rong) = "*" / "**" tuong ung. Resource rong nghia la "moi tai lieu".
type Rule struct {
	Action   string
	Resource string
	Effect   Effect
	// Default: quy tac do DefaultRules() sinh ra (mo ta nhom lenh hien phai hoi), khong phai nguoi dung
	// tu khai bao. Evaluate (dieu kien thuc te: y dinh trong cau lenh, do dai tai lieu, file da co...)
	// duoc phep go mot quy tac Default; quy tac nguoi dung khai bao thi quyet dinh cuoi cung.
	Default bool
}

// Ruleset: danh sach quy tac theo thu tu ap dung. Quy tac sau thang quy tac truoc (last match wins);
// nguoi dung noi them vao DefaultRules() nen quy tac cua ho nam sau va de len mac dinh.
type Ruleset []Rule

// DefaultRules: cac nhom lenh hien phai hoi nguoi dung (muc 8.6) duoi dang quy tac khai bao. Day la
// ban mo ta "lop lenh rui ro"; Evaluate van la noi quyet dinh tung truong hop cu the (luu co y dinh
// trong cau lenh thi khong hoi, replaceAll tren tai lieu ngan thi khong hoi, file chua ton tai thi
// khong hoi...). Lenh chi doc duoc liet ke tuong minh la allow.
func DefaultRules() Ruleset {
	readOnly := []string{
		"app.info", "app.screenshot",
		"writer.getText", "writer.selection", "writer.checkTables", "writer.outline", "writer.checkLayout",
		"writer.listComments", "writer.listRevisions",
		"et.readRange", "et.readRanges", "et.checkRange", "et.listSheets", "et.whatIf", "et.exportChart",
		"wpp.listSlides", "wpp.checkLayout",
	}
	rules := make(Ruleset, 0, len(readOnly)+12)
	for _, action := range readOnly {
		rules = append(rules, Rule{Action: action, Resource: "**", Effect: EffectAllow, Default: true})
	}
	ask := []string{
		// Luu/xuat file khi nguoi dung khong yeu cau; ghi de file da co.
		"*.save", "*.saveAs", "*.exportPdf", "wpp.newFromTemplate",
		// Xoa vat co san cua nguoi dung.
		"wpp.deleteSlide", "wpp.deleteShapes", "wpp.deleteTagged", "et.deleteSheet",
		// Thay the toan bo tren tai lieu dai.
		"writer.replaceAll", "writer.replaceMany",
		// Cong cu ngoai ghi file/trang thai (server MCP khong tin cay, hoac ghi de file da co).
		"mcp__**",
	}
	for _, action := range ask {
		rules = append(rules, Rule{Action: action, Resource: "**", Effect: EffectAsk, Default: true})
	}
	return rules
}

// Decide: ap Ruleset cho mot lenh. Quy tac khop CUOI CUNG thang; rieng deny thang ask khi ca hai cung
// khop (mot quy tac cam khong bi mot quy tac hoi de len). Khong quy tac nao khop = allow.
func Decide(rules Ruleset, action, resource string) Decision {
	var last, lastDeny *Rule
	for index := range rules {
		rule := &rules[index]
		if !matchAction(rule.Action, action) || !matchResource(rule.Resource, resource) {
			continue
		}
		last = rule
		if rule.Effect == EffectDeny {
			lastDeny = rule
		}
	}
	if last == nil {
		return Allow
	}
	effect, matched := last.Effect, last
	if effect == EffectAsk && lastDeny != nil {
		effect, matched = EffectDeny, lastDeny
	}
	explicit := !matched.Default
	switch effect {
	case EffectDeny:
		return Decision{
			Denied:   true,
			Reason:   "Quy tắc chính sách chặn lệnh này (" + strings.TrimSpace(action) + ")",
			Explicit: explicit,
		}
	case EffectAsk:
		return Decision{
			NeedsConfirmation: true,
			Reason:            "Quy tắc chính sách yêu cầu xác nhận (" + strings.TrimSpace(action) + ")",
			Explicit:          explicit,
		}
	default:
		return Decision{Explicit: explicit}
	}
}

// Check: quyet dinh cho mot lenh cu the. Decide (quy tac khai bao) chay TRUOC, roi Evaluate (dieu kien
// thuc te) - de mot quy tac deny chan ngay ma khong ton mot lan goi bridge (WordLength), va de quy tac
// nguoi dung khai bao ro (allow/ask/deny) la quyet dinh cuoi cung.
//
// Quy tac mac dinh (DefaultRules) chi mo ta lop lenh: Evaluate van quyet dinh tung truong hop vi no biet
// nguoi dung co yeu cau luu khong, tai lieu dai bao nhieu, file da ton tai chua. Nho vay lop lenh moi
// them sau nay chi can them vao Evaluate nhu cu.
func Check(rules Ruleset, ctx context.Context, action, resource string, params map[string]any, prompt string,
	documentLength func(context.Context) *int) Decision {
	declared := Decide(rules, action, resource)
	if declared.Explicit {
		return declared
	}
	if declared.Denied {
		return declared
	}
	return Evaluate(ctx, action, params, prompt, documentLength)
}

// matchAction: action tach doan bang '.'.
func matchAction(pattern, action string) bool {
	return matchPath(pattern, action, ".")
}

// matchResource: duong dan tai lieu; '/' va '\' deu la ranh gioi doan.
func matchResource(pattern, resource string) bool {
	return matchPath(pattern, strings.ReplaceAll(resource, `\`, "/"), "/")
}

func matchPath(pattern, value, separators string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	value = strings.ToLower(strings.TrimSpace(value))
	if pattern == "" || pattern == "**" {
		return true
	}
	return matchSegments(splitPath(pattern, separators), splitPath(value, separators))
}

func splitPath(value, separators string) []string {
	return strings.FieldsFunc(value, func(char rune) bool {
		return strings.ContainsRune(separators, char)
	})
}

// matchSegments: "**" khop khong hoac nhieu doan; doan con lai khop theo matchSegment.
func matchSegments(pattern, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		for skip := 0; skip <= len(value); skip++ {
			if matchSegments(pattern[1:], value[skip:]) {
				return true
			}
		}
		return false
	}
	if len(value) == 0 || !matchSegment(pattern[0], value[0]) {
		return false
	}
	return matchSegments(pattern[1:], value[1:])
}

// matchSegment: glob trong MOT doan - "*" khop chuoi con tuy y (ke ca "**" viet trong doan, vd "mcp__**").
func matchSegment(pattern, value string) bool {
	patternIndex, valueIndex := 0, 0
	star, starValue := -1, 0
	for valueIndex < len(value) {
		switch {
		case patternIndex < len(pattern) && pattern[patternIndex] == value[valueIndex]:
			patternIndex++
			valueIndex++
		case patternIndex < len(pattern) && pattern[patternIndex] == '*':
			star, starValue = patternIndex, valueIndex
			patternIndex++
		case star >= 0:
			// Nha "*" ra them mot ky tu nua roi thu lai tu sau no.
			patternIndex = star + 1
			starValue++
			valueIndex = starValue
		default:
			return false
		}
	}
	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}
