package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"axiomoffice/core/internal/pptx"
)

// Che do thiet ke cua slides_from_template (13/10/2026). Nguoi dung xem ban dung 1-1: "AI dang copy het tu ban goc
// qua template; minh muon thiet ke lai slide moi dua tren template va noi dung file goc" - va chon: GIU NGUYEN VAN,
// duoc gop/tach/them slide, giu du anh. Nen: model len bo cuc cho tung phan bai (slide moi nao, kieu gi - chuyen
// phan, muc tieu, noi dung, hai cot - lay doan/anh nao), nhung KHONG viet lai chu: no tro toi doan cua bai goc
// ("7:p2-4"), Core dat dung chu goc va kiem moi doan, moi anh cua phan do co mat dung mot lan truoc khi dung.

// DesignBatch: so slide nguon moi phan - danh sach doan danh so cua 15 slide van ngan voi model yeu.
const DesignBatch = 15

// designRow: mot slide moi do model thiet ke.
type designRow struct {
	Layout string
	Title  string
	Use    []string
	Left   []string
	Right  []string
	// Notes: ghi chu thuyet trinh model viet them (vd thong tin cap nhat kem nguon) - khong len slide.
	Notes string
	// Points, Sources: slide "update" - y model viet tu ket qua tavily_search va link nguon (phai do tavily_search tra).
	Points  []string
	Sources []string
}

// sourceView: slide nguon nhu model thay - doan noi dung danh so tu 1 (tieu de dang de muc khong tinh), anh danh so.
type sourceView struct {
	slide   sourceSlide
	paras   []sourcePara
	heading string
}

// designLayouts: kieu bo cuc model chon -> cong dung kieu cua mau (pptx.Kind.Use).
var designLayouts = []string{"cover", "section", "agenda", "content", "compare", "update"}

func kindsByUse(template *pptx.Template) map[string]*pptx.Kind {
	byUse := map[string]*pptx.Kind{}
	for i := range template.Kinds {
		kind := &template.Kinds[i]
		if _, taken := byUse[kind.Use]; !taken {
			byUse[kind.Use] = kind
		}
	}
	// Mau thieu kieu: muc tieu dung kieu noi dung.
	if byUse["agenda"] == nil && byUse["content"] != nil {
		byUse["agenda"] = byUse["content"]
	}
	return byUse
}

// viewOf: doan noi dung cua slide nguon. Tieu de giong de muc ("1. Xquang thuong qui") la de muc, khong tinh vao doan
// phai dat len slide; tieu de la cau noi dung thi la doan dau.
func viewOf(slide sourceSlide) sourceView {
	view := sourceView{slide: slide}
	title, isHeading := heading(slide, nil)
	if isHeading {
		view.heading = title
	}
	paras := []sourcePara{}
	if len(slide.TextParas) == len(slide.Texts) && len(slide.TextParas)+len(slide.TitleParas) > 0 {
		if !isHeading {
			paras = append(paras, slide.TitleParas...)
		}
		for _, list := range slide.TextParas {
			paras = append(paras, list...)
		}
	} else {
		if !isHeading && strings.TrimSpace(slide.Title) != "" {
			paras = append(paras, parasFromText(slide.Title)...)
		}
		for _, text := range slide.Texts {
			paras = append(paras, parasFromText(text)...)
		}
	}
	view.paras = cleanParas(paras)
	return view
}

// designView: phan bai cho model - moi slide: so, de muc, doan (so + chu, thut theo cap), anh (so + huong).
func designView(views []sourceView) []map[string]any {
	out := []map[string]any{}
	for _, view := range views {
		entry := map[string]any{"n": view.slide.N}
		if view.heading != "" {
			entry["heading"] = view.heading
		}
		if len(view.paras) > 0 {
			lines := []string{}
			for i, para := range view.paras {
				text := []rune(para.line())
				if len(text) > 160 {
					text = append(text[:160], '…')
				}
				lines = append(lines, fmt.Sprintf("%d %s%s", i+1, strings.Repeat("  ", max(para.L-1, 0)), string(text)))
			}
			entry["p"] = lines
		}
		if len(view.slide.Pictures) > 0 {
			pictures := []string{}
			for _, picture := range view.slide.Pictures {
				shape := "landscape"
				if picture.H > picture.W*1.1 {
					shape = "portrait"
				}
				if len(picture.Region) == 4 {
					shape = "diagram (drawn shapes + their labels, copied as one picture)"
				}
				pictures = append(pictures, fmt.Sprintf("%d %s", picture.N, shape))
			}
			entry["i"] = pictures
		}
		if view.slide.Other > 0 {
			entry["note"] = "has a table/chart/diagram that is not copied"
		}
		out = append(out, entry)
	}
	return out
}

// parseDesign: cac dong {layout, title, use | left, right}.
func parseDesign(value any) ([]designRow, []rowError) {
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		var parsed any
		if json.Unmarshal([]byte(text), &parsed) != nil {
			return nil, []rowError{{Field: "slides", Error: "slides must be a JSON array"}}
		}
		value = parsed
	}
	items, _ := value.([]any)
	rows := []designRow{}
	errors := []rowError{}
	refs := func(value any) []string {
		list := []string{}
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
					list = append(list, text)
				}
			}
		case string:
			for _, part := range strings.Split(typed, ";") {
				if part = strings.TrimSpace(part); part != "" {
					list = append(list, part)
				}
			}
		case float64:
			list = append(list, strconv.Itoa(int(typed)))
		}
		return list
	}
	texts := func(value any) []string {
		list := []string{}
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					list = append(list, oneLine(text))
				}
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				list = append(list, oneLine(typed))
			}
		}
		return list
	}
	for i, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			errors = append(errors, rowError{Src: i + 1, Field: "slides", Error: "each slide must be an object"})
			continue
		}
		row := designRow{Use: refs(entry["use"]), Left: refs(entry["left"]), Right: refs(entry["right"])}
		row.Layout = strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["layout"])))
		row.Title, _ = entry["title"].(string)
		row.Title = oneLine(row.Title)
		row.Notes, _ = entry["notes"].(string)
		row.Notes = strings.TrimSpace(row.Notes)
		row.Points, row.Sources = texts(entry["points"]), texts(entry["sources"])
		rows = append(rows, row)
	}
	return rows, errors
}

var refPattern = regexp.MustCompile(`^(\d+)(?::([pi])([\d,\- ]*))?$`)

// resolveRef: "7" (moi doan + moi anh cua slide 7), "7:p" / "7:p2-4" / "7:p1,3", "7:i" / "7:i2" -> khoa doan/anh.
func resolveRef(ref string, views map[int]*sourceView) (int, []int, []int, string) {
	match := refPattern.FindStringSubmatch(strings.ReplaceAll(strings.TrimSpace(ref), " ", ""))
	if match == nil {
		return 0, nil, nil, "bad reference (use \"7\", \"7:p2-4\", \"7:i1,3\")"
	}
	number, _ := strconv.Atoi(match[1])
	view := views[number]
	if view == nil {
		return number, nil, nil, "slide is not in this part"
	}
	all := func(count int) []int {
		list := make([]int, count)
		for i := range list {
			list[i] = i + 1
		}
		return list
	}
	numbers := func(spec string, count int) ([]int, string) {
		if spec == "" {
			return all(count), ""
		}
		list := []int{}
		for _, part := range strings.Split(spec, ",") {
			bounds := strings.SplitN(part, "-", 2)
			from, err1 := strconv.Atoi(bounds[0])
			to, err2 := from, error(nil)
			if len(bounds) == 2 {
				to, err2 = strconv.Atoi(bounds[1])
			}
			if err1 != nil || err2 != nil || from < 1 || to < from || to > count {
				return nil, fmt.Sprintf("numbers must be within 1-%d", count)
			}
			for n := from; n <= to; n++ {
				list = append(list, n)
			}
		}
		return list, ""
	}
	switch match[2] {
	case "":
		return number, all(len(view.paras)), all(len(view.slide.Pictures)), ""
	case "p":
		list, problem := numbers(match[3], len(view.paras))
		return number, list, nil, problem
	default:
		list, problem := numbers(match[3], len(view.slide.Pictures))
		return number, nil, list, problem
	}
}

// designContent: doan va anh ma mot danh sach tham chieu tro toi (theo thu tu), kem khoa "7:p3"/"7:i1" de dem.
func designContent(refs []string, views map[int]*sourceView, slot int, field string) ([]sourcePara, []sourcePicture, []string, []rowError) {
	paras := []sourcePara{}
	pictures := []sourcePicture{}
	keys := []string{}
	errors := []rowError{}
	for _, ref := range refs {
		number, paraNumbers, pictureNumbers, problem := resolveRef(ref, views)
		if problem != "" {
			errors = append(errors, rowError{Src: slot, Field: field, Error: problem, Allowed: ref})
			continue
		}
		view := views[number]
		for _, n := range paraNumbers {
			paras = append(paras, view.paras[n-1])
			keys = append(keys, fmt.Sprintf("%d:p%d", number, n))
		}
		for _, n := range pictureNumbers {
			picture := view.slide.Pictures[n-1]
			picture.Slide = number
			pictures = append(pictures, picture)
			keys = append(keys, fmt.Sprintf("%d:i%d", number, n))
		}
	}
	return paras, pictures, keys, errors
}

var continuation = regexp.MustCompile(`(?i)\s*\((ti[eế]p( theo)?|tt|cont\.?|continued)\)\s*$`)

func normText(text string) string {
	text = strings.ToLower(oneLine(text))
	return strings.Trim(text, " .:;-–•")
}

// sourceTitle: tieu de model viet phai lay tu bai goc (giu nguyen van) - trung mot de muc / doan cua bai goc (tra
// ve dung chu goc), hoac la mot phan lien tuc cua no ("Xquang thuong qui" trong "1. Xquang thuong qui").
func sourceTitle(title string, candidates []string) (string, bool) {
	suffix := continuation.FindString(title)
	core := strings.TrimSpace(strings.TrimSuffix(title, suffix))
	key := normText(core)
	if key == "" {
		return "", false
	}
	tidy := func(text string) string { return strings.TrimRight(strings.TrimSpace(oneLine(text)), " :：") }
	for _, candidate := range candidates {
		if normText(candidate) == key {
			return tidy(candidate) + suffix, true
		}
	}
	if len([]rune(key)) >= 4 {
		for _, candidate := range candidates {
			if strings.Contains(normText(candidate), key) {
				return tidy(core) + suffix, true
			}
		}
	}
	return "", false
}

// dedupeParas: doan cua MOT slide moi gop tu nhieu slide nguon - bo dong de muc lap lai (trung tieu de slide, hoac trung
// mot dong ngan da co phia tren). Luot thu 13/10/2026: gop 3 slide "1.1. Bung khong chuan bi / Tu the dung thang" thi
// hai dong de muc nay lap 3 lan tren cung slide. Chu van nguyen van, chi bo phan lap.
func dedupeParas(paras []sourcePara, title string) []sourcePara {
	seen := map[string]bool{}
	if key := normText(continuation.ReplaceAllString(title, "")); key != "" {
		seen[key] = true
	}
	kept := make([]sourcePara, 0, len(paras))
	for _, para := range paras {
		key, line := normText(para.T), normText(para.line())
		if seen[key] || seen[line] {
			continue
		}
		if len([]rune(para.T)) <= 80 {
			seen[key], seen[line] = true, true
		}
		kept = append(kept, para)
	}
	return kept
}

// designPlan: mot slide moi da kiem, san sang dung.
type designPlan struct {
	row      designRow
	kind     *pptx.Kind
	title    string
	notes    string
	paras    []sourcePara
	pictures []sourcePicture
	sides    [2]struct {
		paras    []sourcePara
		pictures []sourcePicture
	}
}

// checkDesign: kiem bo cuc cua mot phan bai - kieu, tieu de lay tu bai goc, chu vua khung, so anh, va MOI doan/anh cua
// phan do co mat dung mot lan (doan chi lap lai de muc da lam tieu de thi duoc bo).
func checkDesign(rows []designRow, views []sourceView, byUse map[string]*pptx.Kind, candidates []string,
	webSource func(string) bool) ([]designPlan, []rowError) {
	byNumber := map[int]*sourceView{}
	for i := range views {
		byNumber[views[i].slide.N] = &views[i]
	}
	errors := []rowError{}
	plans := []designPlan{}
	used := map[string]int{}
	titles := map[string]bool{}
	allowed := strings.Join(designLayouts, ", ")
	for i, row := range rows {
		slot := i + 1
		kind := byUse[row.Layout]
		if row.Layout == "agenda" && kind == nil {
			kind = byUse["content"]
		}
		if kind == nil {
			errors = append(errors, rowError{Src: slot, Field: "layout", Error: "UNKNOWN_LAYOUT", Allowed: availableLayouts(byUse, allowed)})
			continue
		}
		plan := designPlan{row: row, kind: kind, notes: row.Notes}
		if row.Layout == "update" {
			plan.title, plan.paras, plan.notes = checkUpdate(row, kind, slot, webSource, &errors)
			plans = append(plans, plan)
			continue
		}
		if len(row.Points) > 0 || len(row.Sources) > 0 {
			errors = append(errors, rowError{Src: slot, Field: "points", Error: "ONLY_FOR_UPDATE",
				Allowed: "source slides keep their own words: put new information on an \"update\" slide or in notes"})
		}
		title, ok := sourceTitle(row.Title, candidates)
		switch {
		case strings.TrimSpace(row.Title) == "":
			errors = append(errors, rowError{Src: slot, Field: "title", Error: "MISSING",
				Allowed: "a heading or line copied from the source slides"})
		case !ok:
			errors = append(errors, rowError{Src: slot, Field: "title", Error: "NOT_FROM_SOURCE",
				Allowed: "copy a heading or a line of the source word for word (\"(tiếp theo)\" may be added), e.g. " + sampleTitles(candidates)})
		default:
			plan.title = title
			titles[normText(continuation.ReplaceAllString(title, ""))] = true
			if slotInfo := kind.Title(); slotInfo != nil && slotInfo.MaxChars > 0 && len([]rune(title)) > slotInfo.MaxChars {
				errors = append(errors, rowError{Src: slot, Field: "title", Error: "TOO_LONG", Max: slotInfo.MaxChars,
					Actual: len([]rune(title)), Allowed: "use a shorter heading of the source"})
			}
		}
		count := func(keys []string) {
			for _, key := range keys {
				used[key]++
			}
		}
		if row.Layout == "compare" {
			bodies := kind.Bodies()
			for side, refs := range [][]string{row.Left, row.Right} {
				field := []string{"left", "right"}[side]
				paras, pictures, keys, refErrors := designContent(refs, byNumber, slot, field)
				errors = append(errors, refErrors...)
				count(keys)
				paras = dedupeParas(paras, plan.title)
				plan.sides[side].paras, plan.sides[side].pictures = paras, pictures
				if len(refs) == 0 {
					errors = append(errors, rowError{Src: slot, Field: field, Error: "MISSING"})
				}
				if len(pictures) > 2 {
					errors = append(errors, rowError{Src: slot, Field: field, Error: "TOO_MANY_PICTURES", Max: 2, Actual: len(pictures)})
				}
				if side < len(bodies) && len(bodies[side].Box) == 4 && len(paras) > 0 {
					box := bodies[side].Box
					height := box[3]
					if len(pictures) > 0 {
						height *= compareTextShare
					}
					if !textFits(parasText(paras), box[2], height, sideFont) {
						errors = append(errors, rowError{Src: slot, Field: field, Error: "TOO_LONG", Actual: len([]rune(parasText(paras))),
							Allowed: "move part of this text to another slide"})
					}
				}
			}
			plans = append(plans, plan)
			continue
		}
		if len(row.Left) > 0 || len(row.Right) > 0 {
			errors = append(errors, rowError{Src: slot, Field: "left/right", Error: "ONLY_FOR_COMPARE", Allowed: "use \"use\""})
		}
		paras, pictures, keys, refErrors := designContent(row.Use, byNumber, slot, "use")
		errors = append(errors, refErrors...)
		count(keys)
		if row.Layout != "cover" {
			paras = dedupeParas(paras, plan.title)
		}
		plan.paras, plan.pictures = paras, pictures
		text := parasText(paras)
		switch row.Layout {
		case "cover", "section", "agenda":
			if len(pictures) > 0 {
				errors = append(errors, rowError{Src: slot, Field: "use", Error: "NO_PICTURES_ON_" + strings.ToUpper(row.Layout),
					Allowed: "put the pictures on a content slide"})
			}
			if row.Layout != "agenda" && text != "" {
				if body := bodyFor(kind); body == nil {
					errors = append(errors, rowError{Src: slot, Field: "use", Error: "NO_TEXT_AREA", Allowed: "only a title fits this layout"})
				} else if body.MaxChars > 0 && len([]rune(text)) > body.MaxChars {
					errors = append(errors, rowError{Src: slot, Field: "use", Error: "TOO_LONG", Max: body.MaxChars, Actual: len([]rune(text))})
				}
			}
			if row.Layout == "agenda" && text != "" && !textFitsKind(text, kind, false) {
				errors = append(errors, rowError{Src: slot, Field: "use", Error: "TOO_LONG", Actual: len([]rune(text)),
					Allowed: "split it over two slides"})
			}
		default:
			if len(paras) == 0 && len(pictures) == 0 {
				errors = append(errors, rowError{Src: slot, Field: "use", Error: "EMPTY"})
			}
			if len(pictures) > maxPictures {
				errors = append(errors, rowError{Src: slot, Field: "use", Error: "TOO_MANY_PICTURES", Max: maxPictures,
					Actual: len(pictures), Allowed: "put the other pictures on the next slide (same title + \" (tiếp theo)\")"})
			}
			if text != "" && !textFitsKind(text, kind, len(pictures) > 0) {
				errors = append(errors, rowError{Src: slot, Field: "use", Error: "TOO_LONG", Actual: len([]rune(text)),
					Allowed: "split: put part of the paragraphs on the next slide (same title + \" (tiếp theo)\")"})
			}
		}
		plans = append(plans, plan)
	}

	// Moi doan, moi anh cua phan nay dung dung mot lan.
	headings := map[string]bool{}
	for key := range titles {
		headings[key] = true
	}
	for _, view := range views {
		for _, line := range splitLines(view.slide.Title) {
			headings[normText(line)] = true
		}
	}
	// Dong chu da dung o bat ky dau thi moi ban lap cua no cung tinh la da dung (luot that 14/10/2026: "1.1. Bung khong
	// chuan bi" lap tren nhieu slide nguon, model bo ban lap -> bi bao thieu 19 muc, mat mot vong sua; ban lap van bi
	// dedupeParas bo khi dung).
	usedTexts := map[string]bool{}
	for _, view := range views {
		for i, para := range view.paras {
			if used[fmt.Sprintf("%d:p%d", view.slide.N, i+1)] > 0 {
				usedTexts[normText(para.T)] = true
			}
		}
	}
	missing, twice := []string{}, []string{}
	for _, view := range views {
		for i, para := range view.paras {
			key := fmt.Sprintf("%d:p%d", view.slide.N, i+1)
			switch {
			case used[key] > 1:
				twice = append(twice, key)
			case used[key] == 0 && !headings[normText(para.T)] && !headings[normText(para.line())] && !usedTexts[normText(para.T)]:
				missing = append(missing, key+" \""+shortText(para.T, 50)+"\"")
			}
		}
		for i := range view.slide.Pictures {
			key := fmt.Sprintf("%d:i%d", view.slide.N, i+1)
			switch used[key] {
			case 0:
				missing = append(missing, key)
			case 1:
			default:
				twice = append(twice, key)
			}
		}
	}
	if len(missing) > 0 {
		errors = append(errors, rowError{Field: "coverage", Error: "MISSING_FROM_SOURCE", Actual: len(missing),
			Allowed: "every paragraph and picture of this part must be on a slide (word for word): " + strings.Join(limitList(missing, 25), "; ")})
	}
	if len(twice) > 0 {
		errors = append(errors, rowError{Field: "coverage", Error: "USED_TWICE", Actual: len(twice),
			Allowed: "use each paragraph and picture once: " + strings.Join(limitList(twice, 25), ", ")})
	}
	return plans, errors
}

// updateMarker: slide do model viet phai ghi ro la phan cap nhat (nguoi dung chon giu nguyen van bai goc).
var updateMarker = regexp.MustCompile(`(?i)c[aậ]p nh[aậ]t|b[oổ] sung|update`)

// checkUpdate: slide "update" - y moi tu tavily_search (model viet), moi link nguon phai la link tavily_search da tra trong
// luot (khong bia nguon); tieu de luon co chu "Cap nhat"; cuoi slide ghi ten trang nguon, ghi chu ghi du link.
func checkUpdate(row designRow, kind *pptx.Kind, slot int, webSource func(string) bool, errors *[]rowError) (string, []sourcePara, string) {
	title := strings.TrimSpace(row.Title)
	if title == "" {
		*errors = append(*errors, rowError{Src: slot, Field: "title", Error: "MISSING"})
	} else if !updateMarker.MatchString(title) {
		title = "Cập nhật: " + title
	}
	if slotInfo := kind.Title(); slotInfo != nil && slotInfo.MaxChars > 0 && len([]rune(title)) > slotInfo.MaxChars {
		*errors = append(*errors, rowError{Src: slot, Field: "title", Error: "TOO_LONG", Max: slotInfo.MaxChars, Actual: len([]rune(title))})
	}
	if len(row.Use) > 0 || len(row.Left) > 0 || len(row.Right) > 0 {
		*errors = append(*errors, rowError{Src: slot, Field: "use", Error: "NOT_FOR_UPDATE",
			Allowed: "an update slide holds only new points; source text goes on the other slides"})
	}
	switch {
	case len(row.Points) == 0:
		*errors = append(*errors, rowError{Src: slot, Field: "points", Error: "MISSING"})
	case len(row.Points) > maxUpdatePoints:
		*errors = append(*errors, rowError{Src: slot, Field: "points", Error: "TOO_MANY", Max: maxUpdatePoints, Actual: len(row.Points)})
	}
	if len(row.Sources) == 0 {
		*errors = append(*errors, rowError{Src: slot, Field: "sources", Error: "MISSING",
			Allowed: "the url of each tavily_search result the points come from"})
	}
	domains := []string{}
	for _, url := range row.Sources {
		if webSource == nil || !webSource(url) {
			*errors = append(*errors, rowError{Src: slot, Field: "sources", Error: "NOT_FROM_WEB_SEARCH", Allowed: url +
				" was not returned by tavily_search in this request: search first and cite only urls it returned"})
			continue
		}
		domain := strings.SplitN(normalizeURL(url), "/", 2)[0]
		if !slices.Contains(domains, domain) {
			domains = append(domains, domain)
		}
	}
	paras := []sourcePara{}
	for _, point := range row.Points {
		paras = append(paras, sourcePara{T: point, U: true})
	}
	if len(domains) > 0 {
		paras = append(paras, sourcePara{T: "Nguồn: " + strings.Join(domains, "; "), I: true})
	}
	if text := parasText(paras); !textFitsKind(text, kind, false) {
		*errors = append(*errors, rowError{Src: slot, Field: "points", Error: "TOO_LONG", Actual: len([]rune(text)),
			Allowed: "fewer or shorter points, or two update slides"})
	}
	notes := strings.TrimSpace(row.Notes)
	if len(row.Sources) > 0 {
		notes = strings.TrimSpace(notes + "\n\nNguồn (tìm trên web, cần kiểm lại):\n" + strings.Join(row.Sources, "\n"))
	}
	return title, paras, notes
}

const maxUpdatePoints = 6

// compareTextShare: cot co ca chu va anh - chu o phan tren, toi da 40% chieu cao cot.
const compareTextShare = 0.4

// bodyFor: o chu thu hai cua kieu (bia: ten giang vien; chuyen phan: dong mo ta) - o rong nhat ngoai tieu de.
func bodyFor(kind *pptx.Kind) *pptx.Slot {
	if bodies := kind.Bodies(); len(bodies) > 0 {
		best := bodies[0]
		for _, body := range bodies[1:] {
			if body.MaxChars > best.MaxChars {
				best = body
			}
		}
		return &best
	}
	return coverSubtitle(*kind)
}

func availableLayouts(byUse map[string]*pptx.Kind, all string) string {
	names := []string{}
	for _, name := range designLayouts {
		if byUse[name] != nil {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return all
	}
	return strings.Join(names, ", ")
}

func sampleTitles(candidates []string) string {
	seen := map[string]bool{}
	list := []string{}
	for _, candidate := range candidates {
		key := normText(candidate)
		if key == "" || seen[key] || len([]rune(candidate)) > 70 {
			continue
		}
		seen[key] = true
		list = append(list, "\""+oneLine(candidate)+"\"")
		if len(list) == 6 {
			break
		}
	}
	return strings.Join(list, ", ")
}

func shortText(text string, limit int) string {
	runes := []rune(oneLine(text))
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return string(runes)
}

func limitList(list []string, limit int) []string {
	if len(list) <= limit {
		return list
	}
	return append(append([]string(nil), list[:limit]...), fmt.Sprintf("... and %d more", len(list)-limit))
}

// titleCandidates: chu duoc phep lam tieu de - de muc cua moi slide da doc (ca slide truoc phan nay) va moi doan cua
// phan nay.
func titleCandidates(all []sourceSlide, views []sourceView) []string {
	list := []string{}
	for _, slide := range all {
		lines := splitLines(slide.Title)
		list = append(list, lines...)
		// Tieu de nhieu dong ("CHAN DOAN HINH ANH" / "CAP CUU BUNG"): model ghep thanh mot dong - van la chu goc.
		if len(lines) > 1 {
			list = append(list, strings.Join(lines, " "), strings.Join(lines, " – "))
		}
	}
	for _, view := range views {
		for _, para := range view.paras {
			if lines := splitLines(para.T); len(lines) > 1 {
				list = append(list, strings.Join(lines, " "))
			}
		}
	}
	for _, view := range views {
		for _, para := range view.paras {
			list = append(list, para.T, para.line())
		}
	}
	return list
}

// designTag: dau cua slide moi - lo nguon first-last, thu tu trong lo (bridge xep theo (dau lo, thu tu)).
func designTag(first, last, index int) string {
	return fmt.Sprintf("%d-%d.%d", first, last, index)
}

// coveredByTags: slide nguon da co slide dich - "9", "9.2" (dung 1-1) va "3-9.2" (thiet ke: ca lo 3-9).
func coveredByTags(tags map[string]bool) map[int]bool {
	covered := map[int]bool{}
	for tag := range tags {
		head := strings.SplitN(tag, ".", 2)[0]
		bounds := strings.SplitN(head, "-", 2)
		first, err := strconv.Atoi(bounds[0])
		if err != nil {
			continue
		}
		last := first
		if len(bounds) == 2 {
			if value, err := strconv.Atoi(bounds[1]); err == nil && value >= first {
				last = value
			}
		}
		for number := first; number <= last; number++ {
			covered[number] = true
		}
	}
	return covered
}

// designHow: cach goi - kem moi lan tra phan bai de model khong phai nho.
const designHow = "Design the NEW slides for the source slides in `source` and call again with the same first/last and " +
	"`slides`: the new slides of this part, in order. Each: {\"layout\": one of `layouts`, \"title\": a heading or line " +
	"copied word for word from the source (\" (tiếp theo)\" may be added), \"use\": references} - compare uses \"left\" " +
	"and \"right\" instead of \"use\". References: \"7\" = all paragraphs and pictures of source slide 7, \"7:p2-4\" = " +
	"paragraphs 2-4, \"7:i1,3\" = pictures 1 and 3. Every paragraph and picture of the part must be used exactly once " +
	"(paragraphs that only repeat a heading you used as a title may be left out); the text is placed word for word. " +
	"Design, do not copy slide by slide: start each new part of the lecture (1., 2., II. ...) with a section slide, " +
	"merge consecutive source slides on the same point, give each slide 1-4 pictures with the text that explains " +
	"them, use compare for two things side by side (normal vs abnormal, two techniques, and pictures labelled with opposite words such as \"Có / Không\", \"Tăng âm / Giảm âm\": each picture with its label in its own column), split a slide that is too long " +
	"into two with the same title + \" (tiếp theo)\". First part: begin with a cover slide (use \"1\"). Optional " +
	"\"notes\" on any slide: speaker notes you write. ONLY if the user asked to update or enrich the content: search " +
	"with tavily_search first, then add {\"layout\": \"update\", \"title\": \"Cập nhật: ...\", \"points\": [short new " +
	"points], \"sources\": [urls tavily_search returned]} after the slides of that topic - new information never " +
	"replaces the source text."

// design: mot luot che do thiet ke - doc phan bai, kiem bo cuc model gui, dung.
func (t *SlidesFromTemplateTool) design(ctx context.Context, arguments map[string]any, run *RunContext,
	template *pptx.Template, roles templateRoles, templatePath, sourcePath, targetPath string, created bool,
	bridge func(string, map[string]any) (map[string]any, string), compose bool) Result {
	byUse := kindsByUse(template)
	how := designHow
	if compose {
		how = composeHow
	}
	if byUse["content"] == nil {
		if roles.picture == nil {
			return Result{JSON: ErrorJSON("the template has no content slide (title + content area)"), Mutating: created}
		}
		byUse["content"] = roles.picture
	}
	byUse["update"] = byUse["content"]
	first := max(intArg(arguments, "first", 1), 1)
	last := intArg(arguments, "last", first+DesignBatch-1)
	manifest, problem := bridge("wpp.sourceManifest", map[string]any{"from": sourcePath, "first": max(1, first-15), "last": last})
	if problem != "" {
		return Result{JSON: ErrorJSON(problem), Mutating: created}
	}
	total := asInt(manifest["slideCount"])
	last = min(last, total)
	slides := parseManifest(manifest)
	views := []sourceView{}
	for _, slide := range slides {
		if slide.N >= first && slide.N <= last {
			views = append(views, viewOf(slide))
		}
	}
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
	layouts := availableLayouts(byUse, "")
	if compose {
		layouts = strings.Join(composeLayouts, ", ")
	}
	report := map[string]any{"target": filepath.Base(targetPath), "part": fmt.Sprintf("%d-%d of %d", first, last, total),
		"layouts": layouts}
	result := Result{OK: true, Mutating: created}
	// Dan bai ca bai cho phan dau (muc tieu, noi dung bai) va phan cuoi (tong ket): model chi thay tung phan 15 slide.
	if compose && (first == 1 || last >= total) {
		report["outline"] = deckOutline(bridge, sourcePath, total)
	}

	var plans []partSlide
	var errors []rowError
	var dropped []string
	if compose {
		rows, parseErrors := parseCompose(arguments["slides"])
		errors = parseErrors
		if len(rows) > 0 {
			composed, checkErrors, drops := checkCompose(rows, stringList(arguments["drop"]), views, byUse, run.WebSource)
			errors, dropped = append(errors, checkErrors...), drops
			for _, plan := range composed {
				plans = append(plans, plan)
			}
		}
	} else {
		rows, parseErrors := parseDesign(arguments["slides"])
		errors = parseErrors
		if len(rows) > 0 {
			designed, checkErrors := checkDesign(rows, views, byUse, titleCandidates(slides, views), run.WebSource)
			errors = append(errors, checkErrors...)
			for _, plan := range designed {
				plans = append(plans, plan)
			}
		}
	}
	if len(plans) == 0 && len(errors) == 0 {
		// Chua co bo cuc: tra phan bai de model thiet ke.
		report["source"] = designView(views)
		report["how"] = how
		if covered := coveredByTags(done); first <= last && covered[first] && covered[last] {
			report["alreadyBuilt"] = "this part is already in the new presentation; sending slides rebuilds it"
		}
		result.JSON = OKJSON(report)
		return result
	}
	if len(errors) > 0 {
		report["errors"] = errors
		report["source"] = designView(views)
		report["next"] = fmt.Sprintf("fix the slides listed in errors (src = slide position in your list) and call again with first=%d, last=%d and the whole list", first, last)
		// Bo cuc sai la lan goi hong: ok:false kem errors (truoc 26/10/2026 tra "ok":true).
		result.OK, result.JSON = false, FailJSON(rowErrorsLine(errors), report)
		return result
	}

	// Dung lai ca phan: bo slide cu cua phan nay (chay lai sau khi sua bo cuc), dung theo thu tu.
	if _, problem := bridge("wpp.deleteTagged", map[string]any{"first": first, "last": last}); problem != "" {
		return Result{JSON: ErrorJSON(problem), Mutating: created}
	}
	issues := []map[string]any{}
	built := 0
	for i, plan := range plans {
		if ctx.Err() != nil {
			break
		}
		tag := designTag(first, last, i+1)
		index, slideIssues, problem := plan.build(bridge, templatePath, sourcePath, tag)
		if problem != "" {
			index, slideIssues, problem = plan.build(bridge, templatePath, sourcePath, tag)
		}
		if problem != "" {
			errors = append(errors, rowError{Src: i + 1, Field: "build", Error: problem,
				Allowed: "PowerPoint error, not a problem of the design: call again with the same first/last and slides"})
			continue
		}
		built++
		if len(slideIssues) > 0 {
			issues = append(issues, map[string]any{"slide": index, "title": plan.slideTitle(), "issues": slideIssues})
		}
	}
	closing := false
	if last >= total && len(errors) == 0 && roles.closing != nil && !done["9999"] && ctx.Err() == nil {
		if added, problem := bridge("wpp.addSlideFromTemplate", map[string]any{"template": templatePath, "kind": roles.closing.Kind,
			"tag": "9999"}); problem == "" {
			closing = true
			_, _ = bridge("wpp.setNotes", map[string]any{"slide": asInt(added["slide"]), "text": ""})
		}
	}
	saved := false
	if built > 0 || closing {
		_, problem := bridge("wpp.save", nil)
		saved = problem == ""
	}
	result.Mutating = created || built > 0 || closing
	run.templateBuilt += built
	report["built"] = built
	if len(dropped) > 0 {
		report["dropped"] = dropped
	}
	run.droppedPictures = append(run.droppedPictures, dropped...)
	if len(errors) > 0 {
		report["errors"] = errors
	}
	if len(issues) > 0 {
		report["layoutIssues"] = issues
	}
	covered := coveredByTags(done)
	if len(errors) == 0 {
		for number := first; number <= last; number++ {
			covered[number] = true
		}
	}
	all := total > 0
	for number := 1; all && number <= total; number++ {
		all = covered[number]
	}
	switch {
	case len(errors) > 0:
		report["next"] = fmt.Sprintf("call again with first=%d, last=%d and the same slides", first, last)
	case last < total:
		nextFirst, nextLast := last+1, min(last+DesignBatch, total)
		report["next"] = map[string]any{"first": nextFirst, "last": nextLast}
		nextViews := []sourceView{}
		if more, problem := bridge("wpp.sourceManifest", map[string]any{"from": sourcePath, "first": nextFirst, "last": nextLast}); problem == "" {
			for _, slide := range parseManifest(more) {
				nextViews = append(nextViews, viewOf(slide))
			}
			// Phan ke tiep di kem luon: model thiet ke ngay, khong ton mot luot goi chi de doc.
			report["source"] = designView(nextViews)
			report["how"] = how
			if compose && nextLast >= total {
				report["outline"] = deckOutline(bridge, sourcePath, total)
			}
		}
	default:
		report["next"] = "FINISHED: every source slide is designed, checked and saved. Answer the user now; do not re-check slide by slide."
	}
	result.JSON = OKJSON(report)
	if len(errors) > 0 {
		// Co slide dung hong (loi PowerPoint): phan con lai da dung va luu (built), nhung lan goi la that bai.
		result.OK, result.JSON = false, FailJSON(rowErrorsLine(errors), report)
	}
	if all && len(errors) == 0 && (saved || built == 0) {
		result.Final = true
		var text strings.Builder
		if compose {
			fmt.Fprintf(&text, "Đã biên soạn lại toàn bộ bài (%d slide gốc) theo mẫu vào tệp mới **%s**: ý chính ngắn gọn, thẻ, bảng, "+
				"chú thích ảnh, mục tiêu và tổng kết. **Nguyên văn bài gốc nằm trong ghi chú của từng slide** để bạn đối chiếu; "+
				"hãy soát lại câu chữ chuyên môn trước khi dùng. Đã lưu, bài cũ giữ nguyên.", total, filepath.Base(targetPath))
			if len(run.droppedPictures) > 0 {
				fmt.Fprintf(&text, " Đã bỏ %d ảnh gần trùng: %s.", len(run.droppedPictures), strings.Join(limitList(run.droppedPictures, 12), ", "))
			}
		} else {
			fmt.Fprintf(&text, "Đã thiết kế lại toàn bộ bài (%d slide gốc) theo mẫu vào tệp mới **%s**: chia phần, gộp các slide cùng ý, "+
				"xếp ảnh theo nhóm; chữ giữ nguyên văn và đủ mọi ảnh của bài gốc. Đã lưu, bài cũ giữ nguyên.", total, filepath.Base(targetPath))
		}
		if closing {
			text.WriteString(" Đã thêm slide cảm ơn ở cuối.")
		}
		if len(issues) > 0 {
			slides := []string{}
			for _, issue := range issues {
				slides = append(slides, fmt.Sprint(issue["slide"]))
			}
			fmt.Fprintf(&text, " Cần xem lại bố cục ở slide %s.", strings.Join(slides, ", "))
		}
		result.FinalText = text.String()
	}
	return result
}

// buildDesigned: dung mot slide da kiem.
func buildDesigned(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath string,
	plan designPlan, tag string) (int, []any, string) {
	kind := plan.kind
	row := planRow{Kind: kind.Kind, Title: plan.title, Tag: tag, Texts: map[string]string{}, Notes: plan.notes}
	switch plan.row.Layout {
	case "compare":
		return buildCompare(bridge, templatePath, sourcePath, plan, tag)
	case "cover", "section":
		if text := parasText(plan.paras); text != "" {
			if body := bodyFor(kind); body != nil {
				row.Texts[body.Shape] = text
			}
		}
		return buildSlide(bridge, templatePath, sourcePath, templateRolesOf(kind), row, sourceSlide{})
	}
	row.Paras = plan.paras
	row.Text = parasText(plan.paras)
	slide := sourceSlide{Pictures: plan.pictures}
	for i := range plan.pictures {
		row.Imgs = append(row.Imgs, i+1)
	}
	return buildSlide(bridge, templatePath, sourcePath, templateRolesOf(kind), row, slide)
}

// templateRolesOf: buildSlide tim kieu theo so - goi cho mot kieu.
func templateRolesOf(kind *pptx.Kind) templateRoles {
	return templateRoles{byNumber: map[int]*pptx.Kind{kind.Kind: kind}}
}

// buildCompare: hai cot - moi cot chu (tren) va anh (duoi) trong khung o noi dung cua mau.
func buildCompare(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath string,
	plan designPlan, tag string) (int, []any, string) {
	kind := plan.kind
	insert := map[string]any{"template": templatePath, "kind": kind.Kind, "tag": tag}
	if slot := kind.Title(); slot != nil {
		insert["expect"] = slot.Shape
	}
	added, problem := bridge("wpp.addSlideFromTemplate", insert)
	if problem != "" {
		return 0, nil, problem
	}
	index := asInt(added["slide"])
	if slot := kind.Title(); slot != nil {
		if _, problem := bridge("wpp.setShapeText", map[string]any{"slide": index, "shape": slot.Shape, "text": plan.title}); problem != "" {
			return index, nil, problem
		}
	}
	bodies := kind.Bodies()
	names := []string{}
	for _, body := range bodies {
		names = append(names, body.Shape)
	}
	if _, problem := bridge("wpp.deleteShapes", map[string]any{"slide": index, "shapes": names, "emptyPlaceholders": true}); problem != "" {
		return index, nil, problem
	}
	for side := 0; side < 2 && side < len(bodies); side++ {
		box := bodies[side].Box
		if len(box) != 4 {
			continue
		}
		paras, pictures := plan.sides[side].paras, plan.sides[side].pictures
		pictureBox := box
		if len(paras) > 0 {
			column := *kind
			column.PictureBox = box
			params, area, _ := textLayout(&column, planRow{Paras: paras}, false)
			area = [4]float64{box[0], box[1], box[2], box[3]}
			params["fontSize"], params["minFontSize"] = maxSideFont, sideFont
			if len(pictures) > 0 {
				area[3] = round1(box[3] * compareTextShare)
				params["anchor"] = "top"
				pictureBox = []float64{box[0], round1(box[1] + area[3] + pictureGap), box[2], round1(box[3] - area[3] - pictureGap)}
			}
			params["slide"] = index
			params["left"], params["top"], params["width"], params["height"] = area[0], area[1], area[2], area[3]
			if _, problem := bridge("wpp.addParagraphs", params); problem != "" {
				return index, nil, problem
			}
		}
		sizes := [][2]float64{}
		for _, picture := range pictures {
			sizes = append(sizes, [2]float64{picture.W, picture.H})
		}
		for i, rect := range pictureBoxes(pictureBox, sizes) {
			picture := pictures[i]
			if _, problem := bridge("wpp.copyPicture", copyParams(sourcePath, picture.Slide, picture, index, rect)); problem != "" {
				return index, nil, problem
			}
		}
	}
	if _, problem := bridge("wpp.setNotes", map[string]any{"slide": index, "text": plan.notes}); problem != "" {
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

// sortedKeys: chi de test/bao cao on dinh.
func sortedKeys(set map[int]bool) []int {
	keys := []int{}
	for key := range set {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

// partSlide: mot slide da kiem cua mot phan (thiet ke nguyen van hoac bien soan).
type partSlide interface {
	slideTitle() string
	build(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath, tag string) (int, []any, string)
}

func (p designPlan) slideTitle() string { return p.title }

func (p designPlan) build(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath, tag string) (int, []any, string) {
	return buildDesigned(bridge, templatePath, sourcePath, p, tag)
}

func (p composePlan) build(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath, tag string) (int, []any, string) {
	return buildComposed(bridge, templatePath, sourcePath, p, tag)
}

// deckOutline: dan bai ca bai - de muc cua tung slide nguon (bo dong lap lien tiep), "n: de muc".
func deckOutline(bridge func(string, map[string]any) (map[string]any, string), sourcePath string, total int) []string {
	outline := []string{}
	manifest, problem := bridge("wpp.sourceManifest", map[string]any{"from": sourcePath, "first": 1, "last": total})
	if problem != "" {
		return outline
	}
	previous := ""
	for _, slide := range parseManifest(manifest) {
		heading := oneLine(slide.Title)
		if runes := []rune(heading); len(runes) > 80 {
			heading = string(runes[:80]) + "…"
		}
		if heading == "" || normText(heading) == normText(previous) {
			continue
		}
		previous = heading
		outline = append(outline, fmt.Sprintf("%d: %s", slide.N, heading))
		if len(outline) >= 160 {
			break
		}
	}
	return outline
}
