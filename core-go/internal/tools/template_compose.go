package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"axiomoffice/core/internal/pptx"
	"axiomoffice/core/internal/slidelayout"
)

// Che do bien soan (mode "compose", mac dinh tu 14/10/2026). Nguoi dung so ban thiet ke nguyen van voi mot bo slide
// "AI xin" lam tu cung bai (chu bien soan lai, the Uu diem/Han che, bang lua chon ky thuat, chu thich duoi anh, o luu y,
// muc tieu/tong ket, cap nhat co nguon) va chon: bien soan lai co kiem soat + cap nhat bang Tavily. Nen:
//   - model VIET chu tren slide (y ngan, **tu khoa**), nhung moi slide noi ro dua tren doan nao cua bai goc ("from");
//     Core kiem moi doan cua phan co mat trong "from" cua mot slide nao do, va chep NGUYEN VAN cac doan do xuong ghi
//     chu cua slide - khong mat chu goc, nguoi soan doi chieu duoc;
//   - anh: moi anh cua phan hoac duoc dat len slide, hoac duoc bo co chu dich ("drop": anh gan trung) - liet ke trong
//     bao cao;
//   - bo cuc giau hon: the (cards), bang (table), chu thich anh (caption), o luu y (callout); muc tieu / dan bai /
//     tong ket rut tu bai (khong can "from");
//   - thong tin moi chi tren slide "update" voi link tavily_search that (checkUpdate).

// composeRow: mot slide moi do model bien soan.
type composeRow struct {
	Layout   string
	Title    string
	Subtitle string
	Points   []sourcePara
	From     []string
	Imgs     []string
	Caption  string
	Callout  string
	Cards    []composeCard
	Table    [][]string
	Left     composeSide
	Right    composeSide
	Notes    string
	Sources  []string
}

type composeCard struct {
	Title  string
	Points []sourcePara
}

type composeSide struct {
	Title  string
	Points []sourcePara
	Imgs   []string
}

const (
	maxCards      = 4
	maxTableRows  = 9
	maxTableCols  = 5
	maxPointChars = 220
	composeFont   = 16.0
	cardFont      = 14.0
	calloutFont   = slidelayout.CalloutFont
	captionFont   = 12.0
)

// composeLayouts: kieu model chon -> cong dung kieu cua mau.
var composeLayouts = []string{"cover", "agenda", "section", "content", "cards", "table", "compare", "update"}

func composeKind(layout string, byUse map[string]*pptx.Kind) *pptx.Kind {
	switch layout {
	case "cards", "table", "update", "content":
		return byUse["content"]
	case "agenda":
		if byUse["agenda"] != nil {
			return byUse["agenda"]
		}
		return byUse["content"]
	}
	return byUse[layout]
}

// parsePoints: ["y", "- y con", {"t": "...", "l": 2, "b": true}] -> doan (cap 1 co gach dau dong, "- " / "  " = cap 2).
func parsePoints(value any) []sourcePara {
	list := []sourcePara{}
	add := func(text string, level int, bold bool) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		list = append(list, sourcePara{T: text, L: level, B: bold, U: true})
	}
	switch typed := value.(type) {
	case string:
		for _, line := range strings.Split(typed, "\n") {
			level := 1
			if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "- ") {
				level = 2
			}
			add(strings.TrimPrefix(strings.TrimSpace(line), "- "), level, false)
		}
	case []any:
		for _, item := range typed {
			switch entry := item.(type) {
			case string:
				level := 1
				if strings.HasPrefix(entry, "  ") || strings.HasPrefix(entry, "\t") || strings.HasPrefix(strings.TrimSpace(entry), "- ") {
					level = 2
				}
				add(strings.TrimPrefix(strings.TrimSpace(entry), "- "), level, false)
			case map[string]any:
				text, _ := entry["t"].(string)
				bold, _ := entry["b"].(bool)
				add(text, min(max(asInt(entry["l"]), 1), 3), bold)
			}
		}
	}
	return list
}

func stringList(value any) []string {
	list := []string{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && item != nil {
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

func textOf(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

// parseCompose: cac dong slide cua model.
func parseCompose(value any) ([]composeRow, []rowError) {
	if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
		var parsed any
		if json.Unmarshal([]byte(text), &parsed) != nil {
			return nil, []rowError{{Field: "slides", Error: "slides must be a JSON array"}}
		}
		value = parsed
	}
	items, _ := value.([]any)
	rows := []composeRow{}
	errors := []rowError{}
	side := func(value any) composeSide {
		entry, _ := value.(map[string]any)
		if entry == nil {
			return composeSide{}
		}
		return composeSide{Title: oneLine(textOf(entry["title"])), Points: parsePoints(entry["points"]), Imgs: stringList(entry["imgs"])}
	}
	for i, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			errors = append(errors, rowError{Src: i + 1, Field: "slides", Error: "each slide must be an object"})
			continue
		}
		row := composeRow{
			Layout:   strings.ToLower(textOf(entry["layout"])),
			Title:    oneLine(textOf(entry["title"])),
			Subtitle: textOf(entry["subtitle"]),
			Points:   parsePoints(entry["points"]),
			From:     stringList(entry["from"]),
			Imgs:     stringList(entry["imgs"]),
			Caption:  oneLine(textOf(entry["caption"])),
			Callout:  oneLine(textOf(entry["callout"])),
			Left:     side(entry["left"]),
			Right:    side(entry["right"]),
			Notes:    textOf(entry["notes"]),
			Sources:  stringList(entry["sources"]),
		}
		if cards, ok := entry["cards"].([]any); ok {
			for _, card := range cards {
				if value, ok := card.(map[string]any); ok {
					row.Cards = append(row.Cards, composeCard{Title: oneLine(textOf(value["title"])), Points: parsePoints(value["points"])})
				}
			}
		}
		if table, ok := entry["table"].([]any); ok {
			for _, line := range table {
				if cells, ok := line.([]any); ok {
					values := []string{}
					for _, cell := range cells {
						values = append(values, oneLine(fmt.Sprint(cell)))
					}
					row.Table = append(row.Table, values)
				}
			}
		}
		rows = append(rows, row)
	}
	return rows, errors
}

// composePlan: mot slide bien soan da kiem.
type composePlan struct {
	row      composeRow
	kind     *pptx.Kind
	pictures []sourcePicture
	left     []sourcePicture
	right    []sourcePicture
	notes    string
	points   []sourcePara
}

func (p composePlan) slideTitle() string { return p.row.Title }

// pointsText: chu cua cac y (bo dau ** khi do suc chua).
func pointsText(paras []sourcePara) string {
	return strings.ReplaceAll(parasText(paras), "**", "")
}

// checkCompose: kiem bo cuc bien soan cua mot phan - kieu, do dai, suc chua tung vung, anh, link nguon; moi doan cua
// phan nam trong "from" cua mot slide (doan chi lap de muc / lap mot dong da co thi khong can); moi anh dat len slide
// hoac nam trong drop.
func checkCompose(rows []composeRow, drop []string, views []sourceView, byUse map[string]*pptx.Kind,
	webSource func(string) bool) ([]composePlan, []rowError, []string) {
	byNumber := map[int]*sourceView{}
	for i := range views {
		byNumber[views[i].slide.N] = &views[i]
	}
	errors := []rowError{}
	plans := []composePlan{}
	cited := map[string]bool{}
	shown := map[string]int{}
	pictures := func(refs []string, slot int, field string) []sourcePicture {
		list := []sourcePicture{}
		for _, ref := range refs {
			number, paraNumbers, pictureNumbers, problem := resolveRef(ref, byNumber)
			if problem != "" {
				errors = append(errors, rowError{Src: slot, Field: field, Error: problem, Allowed: ref})
				continue
			}
			if len(paraNumbers) > 0 && !strings.Contains(ref, ":") {
				// "7" trong imgs = moi anh cua slide 7.
				paraNumbers = nil
			}
			for _, n := range pictureNumbers {
				picture := byNumber[number].slide.Pictures[n-1]
				picture.Slide = number
				list = append(list, picture)
				shown[fmt.Sprintf("%d:i%d", number, n)]++
			}
		}
		return list
	}
	allowed := strings.Join(composeLayouts, ", ")
	for i, row := range rows {
		slot := i + 1
		kind := composeKind(row.Layout, byUse)
		if kind == nil {
			errors = append(errors, rowError{Src: slot, Field: "layout", Error: "UNKNOWN_LAYOUT", Allowed: allowed})
			continue
		}
		plan := composePlan{row: row, kind: kind, points: row.Points}
		title := kind.Title()
		switch {
		case row.Title == "":
			errors = append(errors, rowError{Src: slot, Field: "title", Error: "MISSING"})
		case title != nil && title.MaxChars > 0 && len([]rune(row.Title)) > title.MaxChars:
			errors = append(errors, rowError{Src: slot, Field: "title", Error: "TOO_LONG", Max: title.MaxChars, Actual: len([]rune(row.Title))})
		}
		// Doan goc ma slide dua tren: dem, va chep nguyen van xuong ghi chu.
		verbatim := []string{}
		for _, ref := range row.From {
			number, paraNumbers, _, problem := resolveRef(ref, byNumber)
			if problem != "" {
				errors = append(errors, rowError{Src: slot, Field: "from", Error: problem, Allowed: ref})
				continue
			}
			for _, n := range paraNumbers {
				key := fmt.Sprintf("%d:p%d", number, n)
				if !cited[key] {
					verbatim = append(verbatim, byNumber[number].paras[n-1].line())
				}
				cited[key] = true
			}
		}
		for _, point := range row.Points {
			if length := len([]rune(point.T)); length > maxPointChars {
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "POINT_TOO_LONG", Max: maxPointChars, Actual: length,
					Allowed: "slides carry short points; the full source text goes to the notes automatically"})
				break
			}
		}
		plan.pictures = pictures(row.Imgs, slot, "imgs")
		area := contentArea(kind)
		switch row.Layout {
		case "cover", "section":
			if len(row.Imgs) > 0 || len(row.Points) > 0 {
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "ONLY_TITLE_AND_SUBTITLE"})
			}
		case "agenda":
			if len(row.Points) == 0 || len(row.Points) > 8 {
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "NEED_1_TO_8_POINTS", Actual: len(row.Points)})
			} else if !textFits(pointsText(row.Points), area[2], area[3], composeFont) {
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "TOO_LONG", Allowed: "shorter points"})
			}
		case "content", "update":
			if row.Layout == "update" {
				title, paras, notes := checkUpdate(designRow{Layout: "update", Title: row.Title, Points: pointsLines(row.Points),
					Sources: row.Sources, Notes: row.Notes}, kind, slot, webSource, &errors)
				plan.row.Title, plan.points, plan.notes = title, paras, notes
				if len(plan.pictures) > 0 {
					errors = append(errors, rowError{Src: slot, Field: "imgs", Error: "NO_PICTURES_ON_UPDATE"})
				}
				break
			}
			if len(row.Points) == 0 && len(plan.pictures) == 0 {
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "EMPTY"})
			}
			if len(plan.pictures) > maxPictures {
				errors = append(errors, rowError{Src: slot, Field: "imgs", Error: "TOO_MANY_PICTURES", Max: maxPictures,
					Actual: len(plan.pictures), Allowed: "keep the clearest pictures, put near-duplicates in drop, or use a second slide"})
			}
			text, _, _ := contentRegions(area, row, len(plan.pictures) > 0)
			if len(row.Points) > 0 && !textFits(pointsText(row.Points), text[2], text[3], composeFont) {
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "TOO_LONG", Actual: len([]rune(pointsText(row.Points))),
					Allowed: "fewer or shorter points (move details to notes), or a second slide"})
			}
			if row.Caption != "" && len(plan.pictures) == 0 {
				errors = append(errors, rowError{Src: slot, Field: "caption", Error: "CAPTION_WITHOUT_PICTURES"})
			}
		case "cards":
			if len(row.Cards) < 2 || len(row.Cards) > maxCards {
				errors = append(errors, rowError{Src: slot, Field: "cards", Error: "NEED_2_TO_4_CARDS", Actual: len(row.Cards)})
				break
			}
			_, cardArea, _ := contentRegions(area, row, false)
			for c, box := range cardBoxes(cardArea, len(row.Cards)) {
				card := row.Cards[c]
				paras := append([]sourcePara{{T: card.Title, B: true}}, card.Points...)
				if card.Title == "" || len(card.Points) == 0 {
					errors = append(errors, rowError{Src: slot, Field: fmt.Sprintf("cards[%d]", c), Error: "NEEDS_TITLE_AND_POINTS"})
				} else if !textFits(pointsText(paras), box[2]-20, box[3]-16, cardFont) {
					errors = append(errors, rowError{Src: slot, Field: fmt.Sprintf("cards[%d]", c), Error: "TOO_LONG",
						Allowed: "shorter points in this card"})
				}
			}
			if len(plan.pictures) > 0 {
				errors = append(errors, rowError{Src: slot, Field: "imgs", Error: "NO_PICTURES_ON_CARDS"})
			}
		case "table":
			cols := 0
			for _, line := range row.Table {
				cols = max(cols, len(line))
				for _, cell := range line {
					if len([]rune(cell)) > 90 {
						errors = append(errors, rowError{Src: slot, Field: "table", Error: "CELL_TOO_LONG", Max: 90})
						break
					}
				}
			}
			if len(row.Table) < 2 || len(row.Table) > maxTableRows || cols < 2 || cols > maxTableCols {
				errors = append(errors, rowError{Src: slot, Field: "table", Error: "TABLE_SIZE",
					Allowed: fmt.Sprintf("header row + 1-%d rows, 2-%d columns", maxTableRows-1, maxTableCols)})
			}
		case "compare":
			plan.left = pictures(row.Left.Imgs, slot, "left.imgs")
			plan.right = pictures(row.Right.Imgs, slot, "right.imgs")
			bodies := kind.Bodies()
			for s, side := range []composeSide{row.Left, row.Right} {
				count := []int{len(plan.left), len(plan.right)}[s]
				field := []string{"left", "right"}[s]
				if len(side.Points) == 0 && count == 0 {
					errors = append(errors, rowError{Src: slot, Field: field, Error: "EMPTY"})
				}
				if count > 2 {
					errors = append(errors, rowError{Src: slot, Field: field + ".imgs", Error: "TOO_MANY_PICTURES", Max: 2})
				}
				if s < len(bodies) && len(bodies[s].Box) == 4 && len(side.Points) > 0 {
					height := bodies[s].Box[3]
					if count > 0 {
						height *= compareTextShare
					}
					text := pointsText(append([]sourcePara{{T: side.Title}}, side.Points...))
					if !textFits(text, bodies[s].Box[2], height, sideFont) {
						errors = append(errors, rowError{Src: slot, Field: field, Error: "TOO_LONG", Allowed: "shorter points"})
					}
				}
			}
		default:
			errors = append(errors, rowError{Src: slot, Field: "layout", Error: "UNKNOWN_LAYOUT", Allowed: allowed})
		}
		if plan.notes == "" {
			notes := []string{}
			if strings.TrimSpace(row.Notes) != "" {
				notes = append(notes, strings.TrimSpace(row.Notes))
			}
			if len(verbatim) > 0 {
				notes = append(notes, "Nguyên văn bài gốc:\n"+strings.Join(verbatim, "\n"))
			}
			plan.notes = strings.Join(notes, "\n\n")
		}
		plans = append(plans, plan)
	}

	// Moi doan cua phan co trong "from"; moi anh duoc dat hoac bo co chu dich.
	dropped := []string{}
	for _, ref := range drop {
		number, _, pictureNumbers, problem := resolveRef(ref, byNumber)
		if problem != "" || !strings.Contains(ref, ":i") {
			errors = append(errors, rowError{Field: "drop", Error: "PICTURE_REFERENCE_EXPECTED", Allowed: ref + " (use \"7:i2\")"})
			continue
		}
		for _, n := range pictureNumbers {
			key := fmt.Sprintf("%d:i%d", number, n)
			shown[key] += 0
			dropped = append(dropped, key)
		}
	}
	isDropped := map[string]bool{}
	for _, key := range dropped {
		isDropped[key] = true
	}
	headings := map[string]bool{}
	citedTexts := map[string]bool{}
	for _, view := range views {
		for _, line := range splitLines(view.slide.Title) {
			headings[normText(line)] = true
		}
		for i, para := range view.paras {
			if cited[fmt.Sprintf("%d:p%d", view.slide.N, i+1)] {
				citedTexts[normText(para.T)] = true
			}
		}
	}
	missing, twice := []string{}, []string{}
	for _, view := range views {
		for i, para := range view.paras {
			key := fmt.Sprintf("%d:p%d", view.slide.N, i+1)
			if !cited[key] && !headings[normText(para.T)] && !citedTexts[normText(para.T)] {
				missing = append(missing, key+" \""+shortText(para.T, 50)+"\"")
			}
		}
		for i := range view.slide.Pictures {
			key := fmt.Sprintf("%d:i%d", view.slide.N, i+1)
			switch {
			case shown[key] == 0 && !isDropped[key]:
				missing = append(missing, key)
			case shown[key] > 1:
				twice = append(twice, key)
			}
		}
	}
	if len(missing) > 0 {
		errors = append(errors, rowError{Field: "coverage", Error: "NOT_COVERED", Actual: len(missing),
			Allowed: "put every paragraph in the \"from\" of the slide that covers it and every picture on a slide or in drop: " +
				strings.Join(limitList(missing, 25), "; ")})
	}
	if len(twice) > 0 {
		errors = append(errors, rowError{Field: "coverage", Error: "PICTURE_SHOWN_TWICE", Allowed: strings.Join(limitList(twice, 25), ", ")})
	}
	return plans, errors, dropped
}

func pointsLines(paras []sourcePara) []string {
	lines := []string{}
	for _, para := range paras {
		lines = append(lines, para.T)
	}
	return lines
}

// contentArea: vung noi dung cua kieu (tru le phai) - [l, t, w, h].
func contentArea(kind *pptx.Kind) [4]float64 {
	box := contentBox(kind)
	if len(box) != 4 {
		return [4]float64{48, 100, 840, 400}
	}
	return [4]float64{box[0], box[1], box[2], box[3]}
}

// contentRegions: chia vung noi dung thanh chu / anh / o luu y (slidelayout.ContentRegions).
func contentRegions(area [4]float64, row composeRow, withPictures bool) (text, pictures, callout [4]float64) {
	return slidelayout.ContentRegions(area, row.Callout, calloutFont, len(row.Points) > 0, withPictures)
}

func estimateLines(text string, width, size float64) int {
	return slidelayout.EstimateLines(text, width, size)
}

// cardBoxes: 2-3 the mot hang; 4 the luoi 2x2 (slidelayout.CardBoxes).
func cardBoxes(area [4]float64, count int) [][4]float64 {
	return slidelayout.CardBoxes(area, count)
}

// cardMaxFont: co chu toi da trong the (slidelayout.CardMaxFont).
const cardMaxFont = slidelayout.CardMaxFont

// fitCards: the cao vua noi dung, ca cum can giua theo chieu doc (slidelayout.FitCards; bo dau nhan "**" khi do).
func fitCards(area [4]float64, cards []composeCard) [][4]float64 {
	plain := make([]slidelayout.Card, len(cards))
	for c, card := range cards {
		plain[c].Title = card.Title
		for _, point := range card.Points {
			plain[c].Points = append(plain[c].Points, strings.ReplaceAll(point.T, "**", ""))
		}
	}
	return slidelayout.FitCards(area, plain)
}

// tint: mau nhat cua mau nhan (tron voi trang) cho nen the / o luu y.
func tint(hex string, share float64) string { return slidelayout.Tint(hex, share) }

// accentOf: mau nhan va phong cua mau (theo tieu de cua kieu noi dung).
func accentOf(kind *pptx.Kind) (string, string) {
	accent, font := "#1F4E79", "Arial"
	if slot := kind.Title(); slot != nil {
		if slot.Color != "" && !strings.EqualFold(slot.Color, "#FFFFFF") {
			accent = slot.Color
		}
		if slot.Font != "" {
			font = slot.Font
		}
	}
	return accent, font
}

// composeSpec: phan hinh hoc cua mot slide bien soan cho slidelayout.Solve.
func composeSpec(row composeRow, plan composePlan) slidelayout.Spec {
	spec := slidelayout.Spec{Layout: row.Layout, HasPoints: len(row.Points) > 0, ShowPoints: len(plan.points) > 0,
		Callout: row.Callout, Caption: row.Caption}
	for _, card := range row.Cards {
		plain := slidelayout.Card{Title: card.Title}
		for _, point := range card.Points {
			plain.Points = append(plain.Points, strings.ReplaceAll(point.T, "**", ""))
		}
		spec.Cards = append(spec.Cards, plain)
	}
	for _, picture := range plan.pictures {
		spec.Pictures = append(spec.Pictures, [2]float64{picture.W, picture.H})
	}
	return spec
}

// buildComposed: dung mot slide bien soan.
func buildComposed(bridge func(string, map[string]any) (map[string]any, string), templatePath, sourcePath string,
	plan composePlan, tag string) (int, []any, string) {
	kind := plan.kind
	row := plan.row
	switch row.Layout {
	case "cover", "section":
		texts := map[string]string{}
		if row.Subtitle != "" {
			if body := bodyFor(kind); body != nil {
				texts[body.Shape] = row.Subtitle
			}
		}
		return buildSlide(bridge, templatePath, sourcePath, templateRolesOf(kind),
			planRow{Kind: kind.Kind, Title: row.Title, Tag: tag, Texts: texts, Notes: plan.notes}, sourceSlide{})
	case "compare":
		sides := designPlan{kind: kind, title: row.Title, notes: plan.notes}
		for s, side := range []composeSide{row.Left, row.Right} {
			paras := append([]sourcePara(nil), side.Points...)
			if side.Title != "" {
				paras = append([]sourcePara{{T: side.Title, B: true}}, paras...)
			}
			sides.sides[s].paras = paras
			sides.sides[s].pictures = [][]sourcePicture{plan.left, plan.right}[s]
		}
		return buildCompare(bridge, templatePath, sourcePath, sides, tag)
	}

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
		if _, problem := bridge("wpp.setShapeText", map[string]any{"slide": index, "shape": slot.Shape, "text": row.Title}); problem != "" {
			return index, nil, problem
		}
	}
	clear := []string{}
	for _, item := range kind.Items {
		clear = append(clear, item...)
	}
	for _, body := range kind.Bodies() {
		clear = append(clear, body.Shape)
	}
	if _, problem := bridge("wpp.deleteShapes", map[string]any{"slide": index, "shapes": clear, "emptyPlaceholders": true}); problem != "" {
		return index, nil, problem
	}
	accent, font := accentOf(kind)
	paragraphs := func(paras []sourcePara, box [4]float64, size, minimum float64, extra map[string]any) string {
		params := map[string]any{"slide": index, "paragraphs": paras, "left": box[0], "top": box[1], "width": box[2], "height": box[3],
			"fontSize": size, "minFontSize": minimum, "color": bodyColor, "accent": accent, "font": font, "anchor": "top"}
		for key, value := range extra {
			params[key] = value
		}
		_, problem := bridge("wpp.addParagraphs", params)
		return problem
	}
	// Hinh hoc do slidelayout.Solve giai (docs/plans/18-s0 M3); o day chi ghep noi dung vao tung hop va phat lenh.
	for _, box := range slidelayout.Solve(composeSpec(row, plan), contentArea(kind), slidelayout.TemplateTheme) {
		var problem string
		switch box.Role {
		case "card":
			card := row.Cards[box.Index]
			paras := append([]sourcePara{{T: card.Title, B: true, A: "center"}}, card.Points...)
			problem = paragraphs(paras, box.Rect, box.Size, box.Min, map[string]any{"box": "rounded", "fill": tint(accent, 0.9),
				"line": tint(accent, 0.55), "anchor": box.Anchor})
		case "grid":
			_, problem = bridge("wpp.addGrid", map[string]any{"slide": index, "values": row.Table, "left": box.Rect[0], "top": box.Rect[1],
				"width": box.Rect[2], "height": box.Rect[3], "header": accent, "band": tint(accent, 0.9), "font": font})
		case "points":
			problem = paragraphs(plan.points, box.Rect, box.Size, box.Min, map[string]any{"anchor": box.Anchor})
		case "caption":
			problem = paragraphs([]sourcePara{{T: row.Caption, I: true, A: "center"}}, box.Rect, box.Size, box.Min,
				map[string]any{"color": "#595959", "anchor": box.Anchor})
		case "picture":
			picture := plan.pictures[box.Index]
			_, problem = bridge("wpp.copyPicture", copyParams(sourcePath, picture.Slide, picture, index, box.Rect))
		case "callout":
			problem = paragraphs([]sourcePara{{T: row.Callout}}, box.Rect, box.Size, box.Min, map[string]any{"box": "rounded",
				"fill": tint(accent, 0.88), "line": accent, "anchor": box.Anchor})
		}
		if problem != "" {
			return index, nil, problem
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

// composeHow: cach goi che do bien soan - kem moi lan tra phan bai.
const composeHow = "Compose the NEW slides for the source slides in `source` like a professional lecture designer, then call " +
	"again with the same first/last and `slides` (in order) and optional `drop`. Slide fields: layout (one of `layouts`), " +
	"title (short, meaningful: \"XQUANG BKCB – TƯ THẾ ĐỨNG THẲNG\"), from (references of the source paragraphs this slide is " +
	"based on: \"7\", \"7:p2-4\"), plus by layout: content {points, imgs, caption, callout}; cards {cards: [{title, points}] " +
	"2-4 cards, e.g. Ưu điểm / Hạn chế / Chỉ định}; table {table: [[header...], [row...]...] up to 8 rows x 5 columns, e.g. " +
	"bệnh cảnh × kỹ thuật}; compare {left/right: {title, points, imgs}} for normal vs abnormal or two techniques; section " +
	"{subtitle}; agenda {points} for objectives / outline; update {points, sources} only with urls tavily_search returned; " +
	"cover {subtitle}. Points: short (under 15 words), \"**key term**\" for emphasis, level 2 with a leading \"- \". " +
	"Pictures: imgs \"7:i1\"; 1-4 per slide with a caption (\"Trái: … • Phải: …\"); put near-duplicate pictures in drop " +
	"(\"9:i2\") and keep the clearest. callout: one warning or pitfall (⚠). Rules: every paragraph of the part must be in " +
	"the `from` of some slide (its original text goes to that slide's notes automatically - do not copy long text onto " +
	"slides); use only facts from the source, new facts only on update slides; never write onto medical pictures. " +
	"Structure: first part starts with cover, an agenda slide \"MỤC TIÊU HỌC TẬP\" (3 objectives) and an agenda slide of " +
	"the lecture parts (use `outline`); each part of the lecture opens with a section slide numbered exactly as in that " +
	"agenda slide (PHẦN 1, PHẦN 2, ... in order, no gaps); the last part ends with a summary (cards or content " +
	"\"TỔNG KẾT\"). template and source may be left out after the first call (the tool remembers them)."
