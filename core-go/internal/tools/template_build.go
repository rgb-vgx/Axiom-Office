package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"axiomoffice/core/internal/pptx"
	"axiomoffice/core/internal/slidelayout"
)

// SlidesFromTemplateTool: dung lai ca bai trinh chieu theo tep mau, tung lo (docs/architecture/template-design.md muc 6.6).
//
// Luot that 13/10/2026 (mimo, 6 slide): ~4 lan goi tool + ~2,5 phut moi slide, tran 200k token cua pane chi du
// ~15-18 slide. Tu van ChatGPT: model chi KHAI BAO y dinh, may thuc thi; slide may tu quyet duoc (bia, slide anh
// giu tieu de + anh, slide gach dau dong ngan) thi khong can model. Tool nay doc bai nguon mot lan
// (wpp.sourceManifest), dung moi slide tu quyet duoc bang chuoi lenh bridge co dinh, va CHI tra ve cho model nhung
// slide can viet (needsPlan) cung loi theo tung dong ke hoach. Moi slide dich mang tag = so slide nguon nen chay lai
// (lo bi dung giua chung, sua mot dong) thay dung slide do, khong tao trung.
type SlidesFromTemplateTool struct{}

func NewSlidesFromTemplateTool() *SlidesFromTemplateTool { return &SlidesFromTemplateTool{} }

func (t *SlidesFromTemplateTool) Name() string { return "slides_from_template" }

// BatchSize: so slide nguon moi lan goi. Mot slide ~5-8 lenh bridge (chep anh ~0,5-1 s); 25 slide ~1 phut, ket qua
// van ngan de model doc.
const BatchSize = 25

func (t *SlidesFromTemplateTool) Description() string {
	return "Redesign an existing presentation (source, e.g. the old .ppt) into a NEW presentation made from a template " +
		"file, part by part. Default mode \"compose\": call with template + source (first=1) and you get the first " +
		"part of the source (numbered paragraphs and pictures of " + strconv.Itoa(DesignBatch) + " slides), an outline of " +
		"the whole lecture and how to answer: you write the new slides (short points, cards, tables, captions, callouts, " +
		"section / objectives / summary slides), each naming the source paragraphs it is based on - the tool checks " +
		"nothing is lost, puts the original text in the notes, builds, saves and sends the next part. Mode \"design\" " +
		"keeps the original words (only re-layout). Mode \"copy\" (only when the user asks to keep " +
		"every slide as it is): one new slide per source slide, in batches of " + strconv.Itoa(BatchSize) + "; the tool builds " +
		"every slide it can decide alone (cover, slides with pictures keep their title and pictures, short bullet " +
		"slides); slides it cannot decide come back in needsPlan with their text: write one plan row for each and " +
		"call again with the same first/last. Then call with next.first until done. Plan row: {\"src\": source slide " +
		"number, \"title\": text, \"items\": [short bullet texts] (list slides), \"imgs\": [picture numbers of the " +
		"source slide] or \"all\", \"notes\": details for the speaker, \"kind\": template slide number (optional)}. " +
		"Keep each text within the maxChars the result reports. A row for an already built slide rebuilds it. " +
		"Do not use wpp.addSlideFromTemplate / wpp.setShapeText for these slides, and never write labels onto " +
		"medical pictures."
}

func (t *SlidesFromTemplateTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"template":{"type":"string","description":"full path of the template .pptx/.potx"},` +
		`"source":{"type":"string","description":"full path of the presentation to rebuild (the open old deck)"},` +
		`"target":{"type":"string","description":"new .pptx to build; default: '<source name> - mau moi.pptx' next to the source"},` +
		`"first":{"type":"integer","description":"first source slide of this batch (default 1)"},` +
		`"last":{"type":"integer","description":"last source slide of this batch (default first+` + strconv.Itoa(BatchSize-1) + `)"},` +
		`"plan":{"type":"array","items":{"type":"object"},"description":"plan rows for slides listed in needsPlan, or to redo a slide"},` +
		`"closing":{"type":"boolean","description":"copy mode: also add the template's closing slide at the end (on the last batch)"},` +
		`"mode":{"type":"string","enum":["compose","design","copy"],"description":"compose (default): rewrite as a professional lecture part by part (short points, cards, tables, captions; the original text goes to the notes); design: keep the original words, only re-layout; copy: one new slide per source slide"},` +
		`"drop":{"type":"array","items":{"type":"string"},"description":"compose mode: pictures left out on purpose (near-duplicates), e.g. [\"9:i2\"]"},` +
		`"slides":{"type":"array","items":{"type":"object"},"description":"design mode: the new slides of this part, in order: {layout, title, use} or {layout: compare, title, left, right}"}},` +
		`"required":["template","source"]}`)
}

// planRow: mot dong ke hoach cua model (hoac tu dong).
type planRow struct {
	Src   int
	Kind  int
	Title string
	Items []string
	Texts map[string]string
	Imgs  []int
	All   bool
	Notes string
	// Text: doan van giu nguyen (canh anh, hoac ca vung noi dung khi khong co anh).
	Text string
	// Tag: dau cua slide dich (mac dinh = so slide nguon; phan tach "9.2").
	Tag string
	// Paras: cung noi dung voi Text nhung giu phan cap cua slide nguon (cap, dam, so thu tu) - tu dong moi co.
	Paras []sourcePara
}

func (row planRow) tag() string {
	if row.Tag != "" {
		return row.Tag
	}
	return strconv.Itoa(row.Src)
}

type rowError struct {
	Src     int    `json:"src"`
	Field   string `json:"field"`
	Error   string `json:"error"`
	Max     int    `json:"max,omitempty"`
	Actual  int    `json:"actual,omitempty"`
	Allowed string `json:"allowed,omitempty"`
}

// sourceSlide: mot slide cua wpp.sourceManifest.
type sourceSlide struct {
	N        int
	Title    string
	Texts    []string
	Pictures []sourcePicture
	Other    int
	// TitleParas, TextParas: doan van cua khung tieu de va tung khung trong Texts (bridge cu khong co -> rong).
	TitleParas []sourcePara
	TextParas  [][]sourcePara
}

// sourcePara: mot doan cua slide nguon - {t, l, b, i, n, u} cua wpp.sourceManifest / wpp.addParagraphs.
type sourcePara struct {
	T string `json:"t"`
	L int    `json:"l,omitempty"`
	B bool   `json:"b,omitempty"`
	I bool   `json:"i,omitempty"`
	N string `json:"n,omitempty"`
	U bool   `json:"u,omitempty"`
	// A: "center" - can giua (tieu de the, chu thich anh).
	A string `json:"a,omitempty"`
}

// line: doan nhu nguoi xem thay (so tu dong + chu) - dung de do suc chua.
func (p sourcePara) line() string {
	if p.N != "" {
		return p.N + " " + p.T
	}
	return p.T
}

func parseParas(value any) []sourcePara {
	items, _ := value.([]any)
	paras := []sourcePara{}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		para := sourcePara{L: asInt(entry["l"])}
		para.T, _ = entry["t"].(string)
		para.N, _ = entry["n"].(string)
		para.B, _ = entry["b"].(bool)
		para.I, _ = entry["i"].(bool)
		para.U, _ = entry["u"].(bool)
		if para.T = strings.TrimSpace(para.T); para.T != "" {
			paras = append(paras, para)
		}
	}
	return paras
}

// bulletGlyphs: gach dau dong go tay o dau doan ("• ", "- ") - bo di, thay bang gach dau dong that cua khung.
const bulletGlyphs = "•-–*·▪■►✓➢➤◆◦○"

func cleanParas(paras []sourcePara) []sourcePara {
	out := make([]sourcePara, 0, len(paras))
	for _, para := range paras {
		if trimmed := strings.TrimLeft(para.T, bulletGlyphs+" \t"); trimmed != para.T && para.N == "" {
			if trimmed == "" {
				continue
			}
			para.T, para.U = trimmed, true
		}
		out = append(out, para)
	}
	return out
}

// parasFromText: doan van thuong (dong ke hoach cua model) -> doan co phan cap: tab dau dong = cap, gach dau dong
// go tay = gach dau dong that, dong ket thuc bang ":" ("SA:", "Ky thuat:") = de muc in dam.
func parasFromText(text string) []sourcePara {
	paras := []sourcePara{}
	for _, raw := range strings.Split(text, "\n") {
		level := 1 + len(raw) - len(strings.TrimLeft(raw, "\t"))
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		para := sourcePara{T: line, L: min(level, 5)}
		if para.L == 1 {
			para.L = 0
		}
		para.B = strings.HasSuffix(line, ":") && len([]rune(line)) <= 40
		paras = append(paras, para)
	}
	return cleanParas(paras)
}

func parasText(paras []sourcePara) string {
	lines := make([]string, 0, len(paras))
	for _, para := range paras {
		lines = append(lines, para.line())
	}
	return strings.Join(lines, "\n")
}

type sourcePicture struct {
	N     int
	Shape string
	// Overlays: so nhan/mui ten ve DE len anh o slide nguon - chep ca vung (copyPicture overlays) de khong mat nhan.
	Overlays int
	// W, H: kich thuoc tren slide nguon (pt) - de chia luoi theo ti le anh.
	W, H float64
	// Slide: slide nguon chua anh khi khac slide dang dung (che do thiet ke gom anh cua nhieu slide nguon).
	Slide int
	// Region: vung so do / bang ve bang shape tren slide nguon [l, t, w, h] - chep ca vung thanh mot anh.
	Region []float64
}

// copyParams: tham so wpp.copyPicture cho mot anh nguon (anh that, hoac vung so do).
func copyParams(sourcePath string, fromSlide int, picture sourcePicture, slide int, box [4]float64) map[string]any {
	params := map[string]any{"from": sourcePath, "fromSlide": fromSlide, "slide": slide,
		"left": box[0], "top": box[1], "width": box[2], "height": box[3], "format": "jpg"}
	if len(picture.Region) == 4 {
		params["region"] = picture.Region
	} else {
		params["shape"], params["overlays"] = picture.Shape, picture.Overlays > 0
	}
	return params
}

// templateRoles: kieu nao dung cho viec gi (theo use + pictureBox cua ppt_profile).
type templateRoles struct {
	cover, picture, list, closing *pptx.Kind
	byNumber                      map[int]*pptx.Kind
}

func rolesOf(template *pptx.Template) templateRoles {
	roles := templateRoles{byNumber: map[int]*pptx.Kind{}}
	for i := range template.Kinds {
		kind := &template.Kinds[i]
		roles.byNumber[kind.Kind] = kind
		switch {
		case kind.Use == "cover" && roles.cover == nil:
			roles.cover = kind
		case kind.Use == "closing" && roles.closing == nil:
			roles.closing = kind
		}
		if len(kind.Items) > 0 && roles.list == nil {
			roles.list = kind
		}
		if len(kind.PictureBox) == 4 && kind.Use == "content" && roles.picture == nil {
			roles.picture = kind
		}
	}
	// Mau khong co kieu "content" rieng: lay kieu dau tien co vung noi dung (tru bia, ket thuc, chuyen phan).
	for i := range template.Kinds {
		kind := &template.Kinds[i]
		if roles.picture == nil && len(kind.PictureBox) == 4 && kind.Use != "cover" && kind.Use != "closing" && kind.Use != "section" {
			roles.picture = kind
		}
	}
	return roles
}

func (t *SlidesFromTemplateTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	templatePath := stringArg(arguments, "template")
	sourcePath := stringArg(arguments, "source")
	// Lan goi sau trong cung luot: model hay bo template/source (luot that 14/10/2026 mat mot vong) - dung lai lan truoc.
	if templatePath == "" {
		templatePath = run.lastTemplate
	}
	if sourcePath == "" {
		sourcePath = run.lastSource
	}
	if templatePath == "" || sourcePath == "" {
		return Result{JSON: ErrorJSON("'template' and 'source' are required (full paths)")}
	}
	run.lastTemplate, run.lastSource = templatePath, sourcePath
	if stringArg(arguments, "target") == "" && run.lastTarget != "" {
		arguments["target"] = run.lastTarget
	}
	template, err := pptx.ReadTemplate(templatePath)
	if err != nil {
		return Result{JSON: ErrorJSON("cannot read the template: " + err.Error())}
	}
	roles := rolesOf(template)
	if roles.picture == nil && roles.list == nil {
		return Result{JSON: ErrorJSON("the template has no content slide with a title and a picture area")}
	}
	targetPath := stringArg(arguments, "target")
	if targetPath == "" {
		base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		targetPath = filepath.Join(filepath.Dir(sourcePath), base+" - mau moi.pptx")
	}
	run.lastTarget = targetPath
	first := max(intArg(arguments, "first", 1), 1)
	last := intArg(arguments, "last", first+BatchSize-1)

	bridge := func(action string, params map[string]any) (map[string]any, string) {
		// Moi lenh tac dong len bai dich chi DICH DANH bai moi: luot that 13/10/2026 17:04, khung Axiom gan vao cua so
		// bai cu, nguoi dung bam vao do giua lo -> "bai dang hoat dong" thanh bai cu, slide 15-25 trung bai cu.
		if targetActions[action] {
			copied := map[string]any{"presentation": targetPath}
			for key, value := range params {
				copied[key] = value
			}
			params = copied
		}
		result := run.Bridge.Command(ctx, run.Office.Port, action, params)
		if !result.OK {
			return nil, action + ": " + result.Error
		}
		return result.Result, ""
	}

	// Bai dich: dang mo -> dung; co tren dia -> mo (hoac dua len); chua co -> tao tu mau.
	created := false
	if problem := ensureTarget(bridge, templatePath, targetPath, &created); problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}

	// compose (mac dinh): bien soan lai co kiem soat; design: giu nguyen van, chi bo cuc lai; copy: moi slide cu mot slide.
	if mode := strings.ToLower(stringArg(arguments, "mode")); mode != "copy" {
		return t.design(ctx, arguments, run, template, roles, templatePath, sourcePath, targetPath, created, bridge, mode != "design")
	}

	// Doc them toi da 15 slide truoc lo: slide "tiep theo" khong co tieu de lay tieu de cua muc dang trinh bay.
	contextFirst := max(1, first-15)
	manifest, problem := bridge("wpp.sourceManifest", map[string]any{"from": sourcePath, "first": contextFirst, "last": last})
	if problem != "" {
		return Result{JSON: ErrorJSON(problem), Mutating: created}
	}
	total := asInt(manifest["slideCount"])
	if last > total {
		last = total
	}
	slides := parseManifest(manifest)

	done := map[string]bool{}
	if listed, problem := bridge("wpp.listSlides", nil); problem == "" {
		if items, ok := listed["slides"].([]any); ok {
			for _, item := range items {
				if entry, ok := item.(map[string]any); ok {
					if tag, ok := entry["tag"].(string); ok {
						done[tag] = true
					}
				}
			}
		}
	}

	batch := []sourceSlide{}
	for _, slide := range slides {
		if slide.N >= first {
			batch = append(batch, slide)
		}
	}
	plans, errors := parsePlan(arguments["plan"], roles, batch)
	summary := buildSummary{}
	summary.errors = errors
	carry := ""
	for _, slide := range slides {
		if ctx.Err() != nil {
			break
		}
		// Slide truoc lo chi de cap nhat tieu de mang sang.
		rows, reason := autoRows(slide, roles, slide.N == 1, &carry)
		if slide.N < first {
			continue
		}
		row, planned := plans[slide.N]
		if planned {
			if reason == "" && len(rows) > 0 {
				row = mergeRow(rows[0], row)
			}
			if rowErrors := validateRow(row, roles, slide); len(rowErrors) > 0 {
				summary.errors = append(summary.errors, rowErrors...)
				continue
			}
			rows, reason = []planRow{row}, ""
			if strings.TrimSpace(row.Title) != "" {
				carry = row.Title
			}
		} else if done[strconv.Itoa(slide.N)] {
			summary.skipped = append(summary.skipped, slide.N)
			continue
		}
		if reason != "" {
			summary.needsPlan = append(summary.needsPlan, needEntry(slide, reason))
			continue
		}
		failed := false
		for _, row := range rows {
			built, issues, problem := buildSlide(bridge, templatePath, sourcePath, roles, row, slide)
			if problem != "" && ctx.Err() == nil {
				// Loi thoang qua cua PowerPoint (dang ban, slide chen sai): dung lai slide do mot lan - slide mang tag nen
				// lan sau thay dung cho. Luot that 13/10/2026 18:42: bao loi ngay thi model tu viet lai 5 slide va dua chu
				// vao ghi chu (mat chu tren slide).
				built, issues, problem = buildSlide(bridge, templatePath, sourcePath, roles, row, slide)
			}
			if problem != "" {
				summary.errors = append(summary.errors, rowError{Src: slide.N, Field: "build", Error: problem,
					Allowed: "PowerPoint error, not a problem of the plan: call again with the same first/last and NO plan row " +
						"for this slide; never move the slide text into notes because of it"})
				failed = true
				break
			}
			if len(issues) > 0 {
				summary.issues = append(summary.issues, map[string]any{"src": slide.N, "slide": built, "issues": issues})
			}
		}
		if failed {
			continue
		}
		summary.built = append(summary.built, slide.N)
		if !planned {
			summary.auto++
		}
	}

	if arguments["closing"] == true && roles.closing != nil && ctx.Err() == nil {
		if added, problem := bridge("wpp.addSlideFromTemplate", map[string]any{
			"template": templatePath, "kind": roles.closing.Kind, "tag": "9999"}); problem != "" {
			summary.errors = append(summary.errors, rowError{Field: "closing", Error: problem})
		} else {
			summary.closing = true
			_, _ = bridge("wpp.setNotes", map[string]any{"slide": asInt(added["slide"]), "text": ""})
		}
	}
	// Luu bai MOI sau moi lo (tep do tool tao, khong phai tai lieu cua nguoi dung): luot that 13/10/2026 model doc
	// lai bai moi bang ppt_profile/ppt_get_text, thay ban tren dia rong (chua luu) roi dung lai va doc di doc lai
	// ~10 vong. Luu con la diem phuc hoi neu PowerPoint tat giua chung.
	if len(summary.built) > 0 || summary.closing {
		if _, problem := bridge("wpp.save", nil); problem == "" {
			summary.saved = true
		}
	}
	report := summary.report(targetPath, first, last, total, roles)
	run.templateBuilt += len(summary.built)
	result := Result{JSON: OKJSON(report), OK: true, Mutating: created || len(summary.built) > 0 || summary.closing}
	// Xong CA bai (moi slide nguon da co slide dich, khong con slide can model viet, khong loi, da luu): ket thuc luot
	// ngay voi cau tra loi cua Core (tu van ChatGPT 13/10/2026: "xong" la trang thai cua Core).
	covered := map[int]bool{}
	for tag := range done {
		if number, err := strconv.Atoi(strings.SplitN(tag, ".", 2)[0]); err == nil {
			covered[number] = true
		}
	}
	for _, number := range summary.built {
		covered[number] = true
	}
	all := total > 0
	for number := 1; all && number <= total; number++ {
		all = covered[number]
	}
	// Khong dung/khong them gi (moi slide da co tu luot truoc) thi khong can luu; co thay doi thi phai luu duoc.
	unchanged := len(summary.built) == 0 && !summary.closing
	if all && len(summary.needsPlan) == 0 && len(summary.errors) == 0 && (summary.saved || unchanged) &&
		(summary.closing || roles.closing == nil || done["9999"]) {
		result.Final = true
		result.FinalText = summary.finalText(targetPath, total, run.templateBuilt, max(0, len(covered)-run.templateBuilt))
	}
	return result
}

// targetActions: lenh lam viec tren bai dich - tool gan them {"presentation": <bai moi>}.
var targetActions = map[string]bool{
	"wpp.addSlideFromTemplate": true, "wpp.setShapeText": true, "wpp.deleteShapes": true, "wpp.copyPicture": true,
	"wpp.setNotes": true, "wpp.checkLayout": true, "wpp.listSlides": true, "wpp.addText": true, "wpp.save": true,
	"wpp.addParagraphs": true, "wpp.deleteTagged": true,
}

// ensureTarget: bai moi phai dang MO (cac lenh sau dich danh no qua `presentation`, khong can no dang hoat dong).
func ensureTarget(bridge func(string, map[string]any) (map[string]any, string), templatePath, targetPath string, created *bool) string {
	if info, problem := bridge("app.info", nil); problem == "" {
		if active, ok := info["activePresentation"].(map[string]any); ok {
			if name, _ := active["fullName"].(string); strings.EqualFold(filepath.Clean(name), filepath.Clean(targetPath)) {
				return ""
			}
		}
	}
	if _, err := os.Stat(targetPath); err == nil {
		_, problem := bridge("wpp.open", map[string]any{"path": targetPath})
		return problem
	}
	if _, problem := bridge("wpp.newFromTemplate", map[string]any{"template": templatePath, "path": targetPath}); problem != "" {
		return problem
	}
	*created = true
	return ""
}

func parseManifest(manifest map[string]any) []sourceSlide {
	slides := []sourceSlide{}
	items, _ := manifest["slides"].([]any)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		slide := sourceSlide{N: asInt(entry["n"]), Other: asInt(entry["other"])}
		slide.Title, _ = entry["title"].(string)
		if texts, ok := entry["texts"].([]any); ok {
			for _, text := range texts {
				if value, ok := text.(string); ok && strings.TrimSpace(value) != "" {
					slide.Texts = append(slide.Texts, strings.TrimSpace(value))
				}
			}
		}
		if pictures, ok := entry["pictures"].([]any); ok {
			for _, picture := range pictures {
				if value, ok := picture.(map[string]any); ok {
					shape, _ := value["shape"].(string)
					width, _ := value["w"].(float64)
					height, _ := value["h"].(float64)
					picture := sourcePicture{N: asInt(value["n"]), Shape: shape, Overlays: asInt(value["overlays"]), W: width, H: height}
					if region, ok := value["region"].([]any); ok && len(region) == 4 {
						for _, number := range region {
							coordinate, _ := number.(float64)
							picture.Region = append(picture.Region, coordinate)
						}
					}
					slide.Pictures = append(slide.Pictures, picture)
				}
			}
		}
		slide.TitleParas = parseParas(entry["titleParas"])
		if list, ok := entry["textParas"].([]any); ok {
			for _, value := range list {
				slide.TextParas = append(slide.TextParas, parseParas(value))
			}
		}
		slides = append(slides, slide)
	}
	return slides
}

// maxPictures: nhieu anh hon thi chia luoi qua nho de doc anh y khoa - tach slide.
const maxPictures = 4

// heading: tieu de cua slide nguon tu khung chu cao nhat - chi khi no TRONG GIONG tieu de (<= 3 dong ngan). Bai
// giang Y4.4-GAN: khung cao nhat cua nhieu slide la doan mo ta ("SA:\nKho CD do u rat thay doi...") chu khong phai
// tieu de; slide do lay tieu de cua muc dang trinh bay (carry) va giu doan mo ta ben canh anh.
func heading(slide sourceSlide, kind *pptx.Kind) (string, bool) {
	lines := splitLines(slide.Title)
	if len(lines) == 0 || len(lines) > 3 {
		return "", false
	}
	// "SA:", "CLVT:", "GPB:" mo dau doan mo ta; cau dai co dau phay la noi dung, khong phai tieu de.
	if strings.HasSuffix(lines[0], ":") {
		return "", false
	}
	for _, line := range lines {
		length := len([]rune(line))
		if length > 70 || (length > 40 && strings.Contains(line, ", ")) {
			return "", false
		}
	}
	limit := titleLimit(kind)
	if joined := strings.Join(lines, " – "); limit == 0 || len([]rune(joined)) <= limit {
		return joined, true
	}
	// "I. Ton thuong khu tru / 1. U lanh tinh / a. U mau gan": muc sau nhat la cu the nhat.
	if last := lines[len(lines)-1]; len([]rune(last)) <= limit {
		return last, true
	}
	return "", false
}

func titleLimit(kind *pptx.Kind) int {
	if kind == nil {
		return 0
	}
	if slot := kind.Title(); slot != nil {
		return slot.MaxChars
	}
	return 0
}

func splitLines(text string) []string {
	lines := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// bodyFont: co chu cua doan van giu nguyen tu slide cu (canh anh nho hon mot nac).
const (
	bodyFont       = 18.0
	sideFont       = 16.0
	sideTextShare  = 0.38
	pictureGap     = slidelayout.PictureGap
	minTitleLength = 2
)

// textFits: doan van vua khung (slidelayout.TextFits - cong thuc do cua skill).
func textFits(text string, width, height, size float64) bool {
	return slidelayout.TextFits(text, width, height, size)
}

// textArea / pictureArea: vung chu va vung anh trong pictureBox cua kieu (co chu: chu ben trai 38%).
func textArea(box []float64, withPictures bool) [4]float64 {
	if !withPictures {
		return [4]float64{box[0], box[1], box[2], box[3]}
	}
	return [4]float64{box[0], box[1], round1(box[2] * sideTextShare), box[3]}
}

func pictureArea(box []float64, withText bool) []float64 {
	if !withText {
		return box
	}
	left := box[2]*sideTextShare + pictureGap
	return []float64{round1(box[0] + left), box[1], round1(box[2] - left), box[3]}
}

func textFitsKind(text string, kind *pptx.Kind, withPictures bool) bool {
	if len(kind.PictureBox) != 4 {
		return false
	}
	area := textArea(kind.PictureBox, withPictures)
	size := bodyFont
	if withPictures {
		size = sideFont
	}
	return textFits(text, area[2], area[3], size)
}

// autoRows: cac dong ke hoach may tu quyet cho mot slide nguon (slide > 4 anh -> nhieu dong); reason != "" = can
// model viet. carry = tieu de muc dang trinh bay (tu slide truoc), cap nhat khi slide co tieu de rieng.
func autoRows(slide sourceSlide, roles templateRoles, isFirst bool, carry *string) ([]planRow, string) {
	if slide.Other > 0 {
		return nil, "has a table, chart, group or diagram that cannot be rebuilt automatically"
	}
	if isFirst && roles.cover != nil && len(slide.Pictures) == 0 {
		title := oneLine(slide.Title)
		row := planRow{Src: slide.N, Kind: roles.cover.Kind, Title: title, Texts: map[string]string{}}
		if subtitle := coverSubtitle(*roles.cover); subtitle != nil && len(slide.Texts) > 0 {
			text := strings.Join(slide.Texts, "\n")
			if len([]rune(text)) > subtitle.MaxChars {
				return nil, "cover text is too long for the template"
			}
			row.Texts[subtitle.Shape] = text
		}
		*carry = title
		return []planRow{row}, fitTitle(row.Title, roles.cover)
	}
	kind := roles.picture
	if len(slide.Pictures) == 0 && roles.list != nil {
		kind = roles.list
	}
	if kind == nil {
		return nil, "the template has no content slide"
	}
	title, isHeading := heading(slide, kind)
	body := slide.Texts
	if !isHeading && strings.TrimSpace(slide.Title) != "" {
		body = append([]string{slide.Title}, slide.Texts...)
	}
	// Doan van giu phan cap cua slide nguon (bridge moi): cung noi dung voi body, khong cat 400 ky tu.
	var paras []sourcePara
	if len(slide.TextParas) == len(slide.Texts) && len(slide.TextParas)+len(slide.TitleParas) > 0 {
		if !isHeading && strings.TrimSpace(slide.Title) != "" {
			paras = append(paras, slide.TitleParas...)
		}
		for _, list := range slide.TextParas {
			paras = append(paras, list...)
		}
		paras = cleanParas(paras)
	}
	// Mot nhan ngan ("Dai the") noi vao tieu de.
	if isHeading && len(body) == 1 && !strings.Contains(body[0], "\n") {
		label := []rune(body[0])
		if len(label) > minTitleLength && len(label) <= 40 {
			if joined := title + " – " + body[0]; len([]rune(joined)) <= titleLimit(kind) {
				title, body, paras = joined, nil, nil
			}
		}
	}
	if isHeading {
		*carry = title
	} else {
		title = *carry
	}
	if strings.TrimSpace(title) == "" {
		return nil, "no title on this slide or the slides before it: write one"
	}
	text := strings.TrimSpace(strings.Join(body, "\n"))
	if len(paras) > 0 {
		text = parasText(paras)
	}

	if len(slide.Pictures) == 0 {
		lines := []string{}
		for _, line := range splitLines(text) {
			if line = strings.TrimSpace(strings.TrimLeft(line, "•-–*·▪■►✓ ")); line != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			return nil, "slide has a title only: write its content or skip it"
		}
		// The I/II/III cua mau chi cho danh sach phang (khong co muc con).
		flat := true
		if len(paras) > 0 {
			lines = lines[:0]
			for _, para := range paras {
				lines = append(lines, para.T)
				flat = flat && para.L <= 1
			}
		}
		bodies := kind.ItemBodies()
		fitsItems := flat && len(lines) <= len(bodies)
		for i := 0; fitsItems && i < len(lines); i++ {
			fitsItems = len([]rune(lines[i])) <= bodies[i].MaxChars
		}
		if fitsItems {
			return []planRow{{Src: slide.N, Kind: kind.Kind, Title: title, Items: lines}}, fitTitle(title, kind)
		}
		// Giu nguyen van ban trong vung noi dung (model yeu viet lai de sai y chuyen mon).
		layout := roles.picture
		if layout == nil {
			layout = kind
		}
		if !textFitsKind(text, layout, false) {
			return nil, fmt.Sprintf("text slide too long for one slide (%d characters): shorten it or split it", len([]rune(text)))
		}
		return []planRow{{Src: slide.N, Kind: layout.Kind, Title: title, Text: text, Paras: paras}}, fitTitle(title, layout)
	}

	if text != "" && !textFitsKind(text, kind, true) {
		return nil, fmt.Sprintf("the text beside the pictures is too long (%d characters): shorten it, move details to notes", len([]rune(text)))
	}
	// Moi slide toi da 4 anh: slide nhieu anh tach thanh nhieu slide (tag "9", "9.2"...), chu o slide dau.
	rows := []planRow{}
	for start := 0; start < len(slide.Pictures); start += maxPictures {
		end := min(start+maxPictures, len(slide.Pictures))
		row := planRow{Src: slide.N, Kind: kind.Kind, Title: title}
		for _, picture := range slide.Pictures[start:end] {
			row.Imgs = append(row.Imgs, picture.N)
		}
		if start == 0 {
			row.Text, row.Paras = text, paras
		} else {
			row.Tag = fmt.Sprintf("%d.%d", slide.N, start/maxPictures+1)
		}
		rows = append(rows, row)
	}
	return rows, fitTitle(title, kind)
}

func fitTitle(title string, kind *pptx.Kind) string {
	slot := kind.Title()
	if slot == nil || slot.MaxChars == 0 {
		return ""
	}
	if strings.TrimSpace(title) == "" {
		return "no title: write one"
	}
	if length := len([]rune(title)); length > slot.MaxChars {
		return fmt.Sprintf("title has %d characters, the template fits %d: shorten it", length, slot.MaxChars)
	}
	return ""
}

// coverSubtitle: o chu thu hai cua bia (rong nhat sau tieu de) - ten giang vien, don vi.
func coverSubtitle(kind pptx.Kind) *pptx.Slot {
	var best *pptx.Slot
	for i := range kind.Slots {
		slot := &kind.Slots[i]
		if slot.Role == "title" || slot.Item > 0 {
			continue
		}
		if best == nil || slot.MaxChars > best.MaxChars {
			best = slot
		}
	}
	return best
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
}

func needEntry(slide sourceSlide, reason string) map[string]any {
	texts := []string{}
	budget := 600
	for _, text := range slide.Texts {
		if budget <= 0 {
			texts = append(texts, "...")
			break
		}
		runes := []rune(text)
		if len(runes) > budget {
			runes = append(runes[:budget], '…')
		}
		budget -= len(runes)
		texts = append(texts, string(runes))
	}
	entry := map[string]any{"src": slide.N, "reason": reason, "title": slide.Title, "pictures": len(slide.Pictures)}
	if len(texts) > 0 {
		entry["texts"] = texts
	}
	return entry
}

// parsePlan: doc cac dong ke hoach; dong hong bao loi theo tung truong (model sua dung dong do).
func parsePlan(value any, roles templateRoles, slides []sourceSlide) (map[int]planRow, []rowError) {
	rows := map[int]planRow{}
	errors := []rowError{}
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		var parsed any
		if json.Unmarshal([]byte(text), &parsed) != nil {
			return rows, []rowError{{Field: "plan", Error: "plan must be a JSON array of rows"}}
		}
		value = parsed
	}
	items, _ := value.([]any)
	inBatch := map[int]sourceSlide{}
	for _, slide := range slides {
		inBatch[slide.N] = slide
	}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			errors = append(errors, rowError{Field: "plan", Error: "each plan row must be an object"})
			continue
		}
		row := planRow{Src: asInt(entry["src"]), Texts: map[string]string{}}
		if _, ok := inBatch[row.Src]; !ok {
			errors = append(errors, rowError{Src: row.Src, Field: "src", Error: "NOT_IN_BATCH",
				Allowed: fmt.Sprintf("%d-%d (first/last of this call)", firstOf(slides), lastOf(slides))})
			continue
		}
		row.Title, _ = entry["title"].(string)
		row.Title = oneLine(row.Title)
		row.Notes, _ = entry["notes"].(string)
		row.Text, _ = entry["text"].(string)
		row.Text = strings.TrimSpace(row.Text)
		row.Kind = kindNumber(entry["kind"], roles)
		if list, ok := entry["items"].([]any); ok {
			for _, value := range list {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					row.Items = append(row.Items, strings.TrimSpace(text))
				}
			}
		}
		if texts, ok := entry["texts"].(map[string]any); ok {
			for shape, value := range texts {
				if text, ok := value.(string); ok {
					row.Texts[shape] = text
				}
			}
		}
		switch imgs := entry["imgs"].(type) {
		case string:
			row.All = strings.EqualFold(strings.TrimSpace(imgs), "all")
		case []any:
			for _, value := range imgs {
				row.Imgs = append(row.Imgs, asInt(value))
			}
		case float64:
			row.Imgs = []int{int(imgs)}
		}
		if row.Kind == 0 {
			switch {
			case row.Src == 1 && roles.cover != nil && !row.All && len(row.Imgs) == 0 && len(row.Items) == 0:
				row.Kind = roles.cover.Kind
			case (row.All || len(row.Imgs) > 0) && roles.picture != nil:
				row.Kind = roles.picture.Kind
			case roles.list != nil:
				row.Kind = roles.list.Kind
			case roles.picture != nil:
				row.Kind = roles.picture.Kind
			}
		}
		rows[row.Src] = row
	}
	return rows, errors
}

func firstOf(slides []sourceSlide) int {
	if len(slides) == 0 {
		return 0
	}
	return slides[0].N
}

func lastOf(slides []sourceSlide) int {
	if len(slides) == 0 {
		return 0
	}
	return slides[len(slides)-1].N
}

// kindNumber: so slide mau, hoac ten cong dung ("cover", "list", "picture", "closing").
func kindNumber(value any, roles templateRoles) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		pick := map[string]*pptx.Kind{"cover": roles.cover, "list": roles.list, "picture": roles.picture, "closing": roles.closing}
		if kind := pick[strings.ToLower(strings.TrimSpace(typed))]; kind != nil {
			return kind.Kind
		}
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	}
	return 0
}

// mergeRow: dong ke hoach cua model chi GHI DE truong no viet; truong bo trong lay tu quy tac tu dong (cung kieu).
// Luot that 13/10/2026: dong cho bia chi co title + notes -> o giang vien giu nguyen chu cua tep mau (ten nguoi khac).
func mergeRow(auto, plan planRow) planRow {
	if auto.Kind != plan.Kind {
		return plan
	}
	merged := plan
	if strings.TrimSpace(merged.Title) == "" {
		merged.Title = auto.Title
	}
	if len(merged.Texts) == 0 && len(auto.Texts) > 0 {
		merged.Texts = auto.Texts
	}
	if merged.Text == "" && len(merged.Items) == 0 && len(merged.Imgs) == 0 && !merged.All {
		merged.Text, merged.Items, merged.Imgs, merged.All = auto.Text, auto.Items, auto.Imgs, auto.All
		merged.Paras = auto.Paras
	}
	if strings.TrimSpace(merged.Notes) == "" {
		merged.Notes = auto.Notes
	}
	return merged
}

func validateRow(row planRow, roles templateRoles, slide sourceSlide) []rowError {
	errors := []rowError{}
	kind := roles.byNumber[row.Kind]
	if kind == nil {
		numbers := []string{}
		for number := range roles.byNumber {
			numbers = append(numbers, strconv.Itoa(number))
		}
		sort.Strings(numbers)
		return []rowError{{Src: row.Src, Field: "kind", Error: "UNKNOWN_KIND", Allowed: strings.Join(numbers, ", ")}}
	}
	if slot := kind.Title(); slot != nil {
		if strings.TrimSpace(row.Title) == "" {
			errors = append(errors, rowError{Src: row.Src, Field: "title", Error: "MISSING"})
		} else if length := len([]rune(row.Title)); slot.MaxChars > 0 && length > slot.MaxChars {
			errors = append(errors, rowError{Src: row.Src, Field: "title", Error: "TOO_LONG", Max: slot.MaxChars, Actual: length})
		}
	}
	bodies := kind.ItemBodies()
	if len(row.Items) > len(bodies) {
		errors = append(errors, rowError{Src: row.Src, Field: "items", Error: "TOO_MANY", Max: len(bodies), Actual: len(row.Items)})
	}
	for i, item := range row.Items {
		if i < len(bodies) && bodies[i].MaxChars > 0 && len([]rune(item)) > bodies[i].MaxChars {
			errors = append(errors, rowError{Src: row.Src, Field: fmt.Sprintf("items[%d]", i), Error: "TOO_LONG",
				Max: bodies[i].MaxChars, Actual: len([]rune(item))})
		}
	}
	slots := map[string]pptx.Slot{}
	for _, slot := range kind.Slots {
		slots[slot.Shape] = slot
	}
	for shape, text := range row.Texts {
		slot, ok := slots[shape]
		if !ok || slot.Role == "title" || slot.Item > 0 {
			errors = append(errors, rowError{Src: row.Src, Field: "texts." + shape, Error: "UNKNOWN_SHAPE", Allowed: otherSlots(*kind)})
		} else if slot.MaxChars > 0 && len([]rune(text)) > slot.MaxChars {
			errors = append(errors, rowError{Src: row.Src, Field: "texts." + shape, Error: "TOO_LONG", Max: slot.MaxChars, Actual: len([]rune(text))})
		}
	}
	pictures := len(slide.Pictures)
	if row.Text != "" && !textFitsKind(row.Text, kind, row.All || len(row.Imgs) > 0) {
		errors = append(errors, rowError{Src: row.Src, Field: "text", Error: "TOO_LONG", Actual: len([]rune(row.Text))})
	}
	if row.All {
		row.Imgs = nil
	}
	if (row.All || len(row.Imgs) > 0) && len(kind.PictureBox) != 4 {
		errors = append(errors, rowError{Src: row.Src, Field: "imgs", Error: "KIND_HAS_NO_PICTURE_AREA"})
	}
	if len(row.Imgs) > maxPictures || (row.All && pictures > maxPictures) {
		errors = append(errors, rowError{Src: row.Src, Field: "imgs", Error: "TOO_MANY", Max: maxPictures})
	}
	for _, number := range row.Imgs {
		if number < 1 || number > pictures {
			errors = append(errors, rowError{Src: row.Src, Field: "imgs", Error: "UNKNOWN_PICTURE", Allowed: fmt.Sprintf("1-%d", pictures)})
			break
		}
	}
	if (row.All || len(row.Imgs) > 0) && len(row.Items) > 0 {
		errors = append(errors, rowError{Src: row.Src, Field: "items", Error: "ITEMS_WITH_PICTURES",
			Allowed: "pictures take the content area: put the bullet text in notes or use a second row"})
	}
	return errors
}

func otherSlots(kind pptx.Kind) string {
	names := []string{}
	for _, slot := range kind.Slots {
		if slot.Role != "title" && slot.Item == 0 {
			names = append(names, slot.Shape)
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

// buildSlide: chuoi lenh bridge co dinh cho mot dong ke hoach; tra so slide dich + loi bo cuc cua slide do.
func buildSlide(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath string,
	roles templateRoles, row planRow, slide sourceSlide) (int, []any, string) {
	kind := roles.byNumber[row.Kind]
	insert := map[string]any{"template": templatePath, "kind": kind.Kind, "tag": row.tag()}
	if slot := kind.Title(); slot != nil {
		insert["expect"] = slot.Shape
	}
	added, problem := bridge("wpp.addSlideFromTemplate", insert)
	if problem != "" {
		return 0, nil, problem
	}
	index := asInt(added["slide"])
	set := func(shape, text string) string {
		_, problem := bridge("wpp.setShapeText", map[string]any{"slide": index, "shape": shape, "text": text})
		return problem
	}
	if slot := kind.Title(); slot != nil && row.Title != "" {
		if problem := set(slot.Shape, row.Title); problem != "" {
			return index, nil, problem
		}
	}
	for shape, text := range row.Texts {
		if problem := set(shape, text); problem != "" {
			return index, nil, problem
		}
	}
	bodies := kind.ItemBodies()
	pictures := pictureNumbers(row, slide)
	keep := len(row.Items)
	if len(pictures) > 0 || row.Text != "" {
		keep = 0
	}
	for i := 0; i < keep && i < len(bodies); i++ {
		if problem := set(bodies[i].Shape, row.Items[i]); problem != "" {
			return index, nil, problem
		}
	}
	extra := []string{}
	if len(kind.Items) > keep {
		for _, item := range kind.Items[keep:] {
			extra = append(extra, item...)
		}
	}
	// O noi dung (placeholder) khong dien: xoa, khong thi chu mau con nguyen ("Muc tieu thu nhat / thu hai" tren 49/50
	// slide - luot that 13/10/2026 23:10, mau truong Y Ha Noi). Chu cua slide dung trong khung rieng (addParagraphs).
	for _, body := range kind.Bodies() {
		if _, filled := row.Texts[body.Shape]; !filled {
			extra = append(extra, body.Shape)
		}
	}
	// emptyPlaceholders: PowerPoint doi ten o khi chen slide mau ("Content Placeholder 2" -> "4"), xoa theo ten bo sot.
	if _, problem := bridge("wpp.deleteShapes", map[string]any{"slide": index, "shapes": extra, "emptyPlaceholders": true}); problem != "" {
		return index, nil, problem
	}
	sizes := [][2]float64{}
	for _, number := range pictures {
		picture := slide.Pictures[number-1]
		sizes = append(sizes, [2]float64{picture.W, picture.H})
	}
	pictureBox := contentBox(kind)
	var boxes [][4]float64
	if row.Text != "" && len(kind.PictureBox) == 4 {
		params, area, rest := textLayout(kind, row, len(pictures) > 0)
		if rest != nil {
			pictureBox = rest
		}
		boxes = pictureBoxes(pictureBox, sizes)
		if params["anchor"] == "top" && len(boxes) > 0 {
			// De muc tren dau nhom anh: ca khoi (de muc + anh) can giua theo chieu doc, khong de khoang trong giua.
			area, boxes = stackCaption(area, boxes, contentBox(kind))
		}
		params["slide"] = index
		params["left"], params["top"], params["width"], params["height"] = area[0], area[1], area[2], area[3]
		if _, problem := bridge("wpp.addParagraphs", params); problem != "" {
			return index, nil, problem
		}
	} else {
		boxes = pictureBoxes(pictureBox, sizes)
	}
	for i, box := range boxes {
		picture := slide.Pictures[pictures[i]-1]
		fromSlide := slide.N
		if picture.Slide > 0 {
			fromSlide = picture.Slide
		}
		if _, problem := bridge("wpp.copyPicture", copyParams(sourcePath, fromSlide, picture, index, box)); problem != "" {
			return index, nil, problem
		}
	}
	// Luon dat lai ghi chu: InsertFromFile mang theo ghi chu cua slide MAU ("Phan bo thoi gian... cap cuu bung") -
	// luot that 13/10/2026 model phai tu xoa ghi chu 97 slide, het ngan sach token.
	if _, problem := bridge("wpp.setNotes", map[string]any{"slide": index, "text": strings.TrimSpace(row.Notes)}); problem != "" {
		return index, nil, problem
	}
	var issues []any
	if checked, problem := bridge("wpp.checkLayout", map[string]any{"slide": index}); problem == "" {
		if list, ok := checked["slides"].([]any); ok && len(list) > 0 {
			if entry, ok := list[0].(map[string]any); ok {
				issues, _ = entry["issues"].([]any)
			}
		}
	}
	return index, issues, ""
}

func pictureNumbers(row planRow, slide sourceSlide) []int {
	if row.All {
		numbers := []int{}
		for i := range slide.Pictures {
			numbers = append(numbers, i+1)
		}
		return numbers
	}
	return row.Imgs
}

// Chu cua slide dung lai (luot that 13/10/2026 18:10, so voi bai goc): chu phang 16-18 pt don goc tren, mat dam va
// so thu tu, nua duoi slide trong. Nay: giu phan cap (wpp.addParagraphs), co chu lon nhat vua khung trong khoang
// [toi thieu, toi da], khoi chu can giua theo chieu doc; de muc in dam mang mau tieu de cua mau, than chu mau toi.
const (
	maxBodyFont = 24.0
	maxSideFont = 20.0
	bodyColor   = "#262626"
	// captionChars: chu ngan canh anh ("Soi duong mat / SA:") la de muc cua nhom anh -> mot dai tren dau anh,
	// anh lay ca be ngang thay vi dung hai dong chu le loi o cot trai 38%.
	captionChars = 70
)

// textLayout: tham so wpp.addParagraphs (chua co slide/vi tri), vung chu, va vung anh con lai (nil = theo mac dinh).
func textLayout(kind *pptx.Kind, row planRow, withPictures bool) (map[string]any, [4]float64, []float64) {
	paras := append([]sourcePara(nil), row.Paras...)
	if len(paras) == 0 {
		paras = parasFromText(row.Text)
	}
	// Phong ro rang (mac dinh Arial): de trong thi PowerPoint lay phong theme, ma theme Office anh xa chu tieng Viet
	// sang Times New Roman (phien add-in Office.js khac 13/10/2026 gap dung loi nay o slide ket thuc).
	accent, font := "#1F4E79", "Arial"
	if slot := kind.Title(); slot != nil {
		if slot.Color != "" && !strings.EqualFold(slot.Color, "#FFFFFF") {
			accent = slot.Color
		}
		if slot.Font != "" {
			font = slot.Font
		}
	}
	params := map[string]any{"color": bodyColor, "accent": accent, "anchor": "middle",
		"fontSize": maxBodyFont, "minFontSize": bodyFont, "font": font}
	box := contentBox(kind)
	var rest []float64
	area := textArea(box, false)
	switch {
	case !withPictures:
	case isCaption(paras):
		height := round1(7.2 + float64(len(paras))*1.21*maxSideFont + 4)
		area = [4]float64{box[0], box[1], box[2], height}
		rest = []float64{box[0], round1(box[1] + height + pictureGap), box[2], round1(box[3] - height - pictureGap)}
		for i := range paras {
			paras[i].B = true
		}
		params["anchor"], params["fontSize"], params["minFontSize"] = "top", maxSideFont, sideFont
	default:
		area = textArea(box, true)
		rest = pictureArea(box, true)
		params["fontSize"], params["minFontSize"] = maxSideFont, sideFont
	}
	params["paragraphs"] = paras
	return params, area, rest
}

// contentPad: le phai them cho vung noi dung - pictureBox lay theo vung an toan cua mau (cach mep slide 12 pt), anh
// rong het vung thi dinh sat mep phai slide (luot that 13/10/2026 18:40, slide 14, 48).
const contentPad = 24.0

func contentBox(kind *pptx.Kind) []float64 {
	box := kind.PictureBox
	if len(box) != 4 {
		return box
	}
	return []float64{box[0], box[1], round1(box[2] - contentPad), box[3]}
}

// stackCaption: de muc tren dau nhom anh + khoi anh xep sat nhau (cach pictureGap), ca cum can giua theo chieu doc
// trong vung noi dung; khoi anh giu vi tri ngang.
func stackCaption(area [4]float64, boxes [][4]float64, box []float64) ([4]float64, [][4]float64) {
	top, bottom := math.Inf(1), math.Inf(-1)
	for _, b := range boxes {
		top, bottom = math.Min(top, b[1]), math.Max(bottom, b[1]+b[3])
	}
	total := area[3] + pictureGap + (bottom - top)
	start := box[1] + math.Max(0, (box[3]-total)/2)
	area[1] = round1(start)
	shift := start + area[3] + pictureGap - top
	moved := make([][4]float64, len(boxes))
	for i, b := range boxes {
		moved[i] = [4]float64{b[0], round1(b[1] + shift), b[2], b[3]}
	}
	return area, moved
}

func isCaption(paras []sourcePara) bool {
	if len(paras) == 0 || len(paras) > 2 {
		return false
	}
	for _, para := range paras {
		if strings.Contains(para.T, "\n") {
			return false
		}
	}
	return len([]rune(parasText(paras))) <= captionChars
}

// pictureBoxes: xep anh theo hang, giu ti le, lap day be ngang (slidelayout.PictureBoxes).
func pictureBoxes(area []float64, sizes [][2]float64) [][4]float64 {
	return slidelayout.PictureBoxes(area, sizes)
}

func round1(value float64) float64 { return slidelayout.Round1(value) }

type buildSummary struct {
	built     []int
	skipped   []int
	auto      int
	needsPlan []map[string]any
	errors    []rowError
	issues    []map[string]any
	closing   bool
	saved     bool
}

func (s buildSummary) report(target string, first, last, total int, roles templateRoles) map[string]any {
	report := map[string]any{
		"target":   target,
		"batch":    fmt.Sprintf("%d-%d of %d", first, last, total),
		"built":    ranges(s.built),
		"auto":     s.auto,
		"fromPlan": len(s.built) - s.auto,
	}
	if len(s.skipped) > 0 {
		report["alreadyBuilt"] = ranges(s.skipped)
	}
	if len(s.needsPlan) > 0 {
		report["needsPlan"] = s.needsPlan
		limits := map[string]any{}
		if roles.picture != nil {
			if slot := roles.picture.Title(); slot != nil {
				limits["title"] = slot.MaxChars
			}
		}
		if roles.list != nil {
			bodies := roles.list.ItemBodies()
			limits["items"] = len(bodies)
			if len(bodies) > 0 {
				limits["itemChars"] = bodies[0].MaxChars
			}
		}
		limits["imgsPerSlide"] = maxPictures
		report["limits"] = limits
	}
	if len(s.errors) > 0 {
		report["errors"] = s.errors
	}
	if len(s.issues) > 0 {
		report["layoutIssues"] = s.issues
	}
	if s.closing {
		report["closing"] = true
	}
	if s.saved {
		report["saved"] = "the new presentation is open and saved; check it with wpp.listSlides / wpp.checkLayout"
	}
	switch {
	case len(s.needsPlan) > 0 || len(s.errors) > 0:
		report["next"] = fmt.Sprintf("call again with first=%d, last=%d and a plan row for each needsPlan/errors entry", first, last)
	case last < total:
		report["next"] = map[string]any{"first": last + 1, "last": min(last+BatchSize, total)}
	case !s.closing && roles.closing != nil:
		report["next"] = "every source slide is built: call once more with closing=true to add the closing slide, then answer the user"
	default:
		// Luot that 13/10/2026: sau "done" model con doc lai bai nguon 4 lan + checkLayout tung slide ~7 phut.
		report["next"] = "FINISHED: every slide is built, checked (layoutIssues lists every problem found) and saved. " +
			"Do not re-check slides one by one; answer the user now with the counts and any layoutIssues."
	}
	return report
}

// finalText: cau tra loi cho nguoi dung khi da xong ca bai (Core viet, khong goi model).
// builtThisRun: slide dung trong ca luot; before: slide nguon da co slide dich tu luot truoc.
func (s buildSummary) finalText(target string, total, builtThisRun, before int) string {
	var text strings.Builder
	switch {
	case builtThisRun == 0:
		// Luot that 13/10/2026 17:00: tep dich con tu luot truoc, khong dung gi ma van bao "da dung lai xong".
		fmt.Fprintf(&text, "Cả %d slide đã có sẵn trong tệp **%s** từ lần dựng trước, nên lần này không dựng lại slide nào. "+
			"Muốn làm lại từ đầu thì đóng và xoá tệp đó rồi gửi lại yêu cầu.", total, filepath.Base(target))
	case before > 0:
		fmt.Fprintf(&text, "Đã dựng xong toàn bộ %d slide theo mẫu trong tệp **%s** (lần này dựng %d slide, %d slide đã có "+
			"từ trước; đã lưu, bài cũ giữ nguyên).", total, filepath.Base(target), builtThisRun, before)
	default:
		fmt.Fprintf(&text, "Đã dựng lại xong toàn bộ %d slide theo mẫu vào tệp mới **%s** (đã lưu, bài cũ giữ nguyên).",
			total, filepath.Base(target))
	}
	if s.closing {
		text.WriteString(" Đã thêm slide cảm ơn ở cuối.")
	}
	if len(s.issues) == 0 {
		text.WriteString(" Bố cục đã được soát: không có lỗi tràn hay chồng khung.")
	} else {
		slides := []string{}
		for _, issue := range s.issues {
			slides = append(slides, fmt.Sprint(issue["slide"]))
		}
		fmt.Fprintf(&text, " Cần xem lại bố cục ở slide %s.", strings.Join(slides, ", "))
	}
	text.WriteString("\n\nBạn nên lướt lại các tiêu đề và đoạn mô tả bên cạnh ảnh; phần chữ cũ không đặt lên slide được đã " +
		"được chuyển vào ghi chú của từng slide.")
	return text.String()
}

// ranges: [1 2 3 5 7 8] -> "1-3, 5, 7-8".
func ranges(numbers []int) string {
	if len(numbers) == 0 {
		return ""
	}
	sorted := append([]int(nil), numbers...)
	sort.Ints(sorted)
	parts := []string{}
	start, previous := sorted[0], sorted[0]
	flush := func() {
		if start == previous {
			parts = append(parts, strconv.Itoa(start))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", start, previous))
		}
	}
	for _, number := range sorted[1:] {
		if number == previous+1 {
			previous = number
			continue
		}
		flush()
		start, previous = number, number
	}
	flush()
	return strings.Join(parts, ", ")
}

func asInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	}
	return 0
}

func intArg(arguments map[string]any, name string, fallback int) int {
	if value, ok := arguments[name]; ok && value != nil {
		if number := asInt(value); number != 0 {
			return number
		}
	}
	return fallback
}
