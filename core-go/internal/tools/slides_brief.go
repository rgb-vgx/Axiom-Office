package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/slidelayout"
)

// SlidesFromBriefTool: dung slide moi tu spec, KHONG can tep mau (docs/plans/18-s0 M4). Model chon kieu va viet chu;
// tool tu dat vi tri, co chu, mau theo luoi cua skill va _design/tokens.json, roi soat bo cuc. Truoc day model tu tinh
// toa do roi goi wpp.addText - chinh no la cho model yeu sai nhieu nhat.
type SlidesFromBriefTool struct {
	index *skills.Index
}

func NewSlidesFromBriefTool(index *skills.Index) *SlidesFromBriefTool {
	return &SlidesFromBriefTool{index: index}
}

func (t *SlidesFromBriefTool) Name() string { return "slides_from_brief" }

func (t *SlidesFromBriefTool) Description() string {
	return "Add NEW slides at the end of the open presentation from a short spec, without a template file (it adds the slides " +
		"itself - do not call wpp.addSlide first). To FIX slides this tool already built in this request (layout issues, " +
		"wording), send only those slides again with replace: <slide number> - each is rebuilt in place; do not rebuild or " +
		"delete the other slides. You choose " +
		"each slide's layout and write the words; the tool places everything on the grid (positions, font sizes, " +
		"colours) and checks the layout. Use it instead of wpp.addText / wpp.addTable for new slides. Layouts: " +
		"cover {title, subtitle}; section {title}; content {title, points, callout}; cards {title, cards: [{title, " +
		"points}] 2-4}; table {title, table: [[header...], [row...]...] up to 7 rows x 5 columns}; figures {title, " +
		"figures: [{value, label}] 2-3 big numbers}; process {title, steps: [{title, text, group}] 3-5 steps joined by arrows}; " +
		"timeline {title, milestones: [{date, text}] 3-6}; diagram {title, nodes: [{id, text, layer, group}] up to 10 (layer 1 = " +
		"top row, up to 4 nodes per layer), edges: [{from, to}] arrows between node ids - arrows are routed around the " +
		"other boxes, so back arrows and arrows that skip a node are fine, and the tool re-routes one itself if PowerPoint " +
		"draws it across a box}; optional group on steps/nodes (2-3 short names, " +
		"e.g. \"AI\" / \"Business system\"): each group gets its own colour and a legend, to show who does which step; " +
		"chart {title, chart: {type column|bar|line|pie, " +
		"categories: [...] 2-12, series: [{name, values: [numbers]}] 1-4 (pie: 1), unit e.g. \"%\" or \" tỷ\"}} - a real, " +
		"editable PowerPoint chart with value labels. Any slide may add notes (speaker notes); content slides may add source (one line PRINTED on the slide: the data source only, never a slide label). " +
		"title: a short conclusion sentence. points: short phrases, \"**word**\" to stress, \"- \" for level 2. Make " +
		"exactly the number of slides the user asked for. Nothing is built until every slide passes the checks: if a " +
		"text is too long you get errors {slide, field, error, max}; shorten it and call again with all slides."
}

func (t *SlidesFromBriefTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"slides":{"type":"array","items":{"type":"object"},` +
		`"description":"new slides in order: {layout, title, subtitle, points, callout, cards, table, figures, notes, source}"}},` +
		`"required":["slides"]}`)
}

// briefLayouts: kieu dung duoc tu brief.
// slideLabel: "Slide 1", "Slide 1 — Cover", "Trang 3", "Cover" - nhan dan y cua model, khong phai nguon du lieu.
var slideLabel = regexp.MustCompile(`(?i)^\s*((slide|trang)\s*\d+\b|(cover|bìa|title slide)\s*$)`)

var briefLayouts = []string{"cover", "section", "content", "cards", "table", "figures", "process", "timeline", "diagram", "chart"}

type briefFigure struct{ Value, Label string }

// briefItem: mot buoc (title, text) hoac mot moc (date, text). Group: nhom mau cua buoc (vd "AI" / "Nghiệp vụ").
type briefItem struct{ Title, Text, Group string }

type briefNode struct {
	ID, Text, Group string
	Layer           int
}

// maxGroups: so nhom mau toi da tren mot slide quy trinh / so do (mau: primary, muted, accent).
const maxGroups = 3

type briefEdge struct{ From, To string }

// briefChart: bieu do goc cua PowerPoint (wpp.addChart) - so lieu nam trong bang tinh nhung, nguoi dung "Edit Data" duoc.
// Values giu chuoi goc de bao loi dung cho; Numbers la gia tri da doc.
type briefChart struct {
	Type       string
	Categories []string
	Series     []briefSeries
	Unit       string
}

type briefSeries struct {
	Name    string
	Values  []any
	Numbers []float64
}

type briefSlide struct {
	composeRow
	Figures    []briefFigure
	Steps      []briefItem
	Milestones []briefItem
	Nodes      []briefNode
	Edges      []briefEdge
	Chart      *briefChart
	Source     string
	// Replace: so slide (do CHINH luot nay dung) duoc thay tai cho bang slide nay - sua dung slide loi thay vi dung lai
	// ca bai (luot that 25/10/2026: checkLayout bao 3 loi, model dung lai 10 slide roi xoa 10 slide cu, ~10 vong).
	Replace int
}

// briefItems: danh sach object {titleKey, textKey} (hoac chuoi: chi co title).
func briefItems(value any, titleKey, textKey string) []briefItem {
	items := []briefItem{}
	list, _ := value.([]any)
	for _, raw := range list {
		switch entry := raw.(type) {
		case string:
			items = append(items, briefItem{Title: strings.TrimSpace(entry)})
		case map[string]any:
			items = append(items, briefItem{Title: strings.TrimSpace(textOf(entry[titleKey])), Text: strings.TrimSpace(textOf(entry[textKey])),
				Group: strings.TrimSpace(textOf(entry["group"]))})
		}
	}
	return items
}

// briefTokens: phan slide/colors cua _design/tokens.json can cho viec dung.
type briefTokens struct {
	Slide struct {
		Margin float64 `json:"margin"`
		Gutter float64 `json:"gutter"`
		Type   struct {
			Hero, Title, Subtitle, Figure, Body, Caption float64
		} `json:"type"`
		MaxBulletsPerSlide int `json:"maxBulletsPerSlide"`
	} `json:"slide"`
	Colors struct {
		Primary, Accent, Text, Muted string
	} `json:"colors"`
}

// defaultBriefTokens: gia tri hien co cua tokens.json - dung khi khong doc duoc tep (to chuc tu dat thu muc _design khac).
func defaultBriefTokens() briefTokens {
	var tokens briefTokens
	tokens.Slide.Margin, tokens.Slide.Gutter = 40, 20
	tokens.Slide.Type.Hero, tokens.Slide.Type.Title, tokens.Slide.Type.Subtitle = 40, 28, 24
	tokens.Slide.Type.Figure, tokens.Slide.Type.Body, tokens.Slide.Type.Caption = 40, 20, 14
	tokens.Slide.MaxBulletsPerSlide = 5
	tokens.Colors.Primary, tokens.Colors.Accent, tokens.Colors.Text, tokens.Colors.Muted = "#1F4E79", "#2E75B6", "#262626", "#595959"
	return tokens
}

func (t *SlidesFromBriefTool) tokens() briefTokens {
	tokens := defaultBriefTokens()
	if t.index == nil {
		return tokens
	}
	dir := t.index.ResourceDirectory("_design")
	if dir == "" {
		return tokens
	}
	data, err := os.ReadFile(filepath.Join(dir, "tokens.json"))
	if err != nil {
		return tokens
	}
	_ = json.Unmarshal(data, &tokens) // truong thieu giu gia tri mac dinh
	return tokens
}

// briefFont: phong do suc chua (he so 0,53 / 1,21 do bang Arial) va phong chu tieng Viet an toan (template_build.go).
const briefFont = "Arial"

// nestedLayouts: kieu ma model yeu hay viet truong cua no LONG trong mot khoa cung ten - {"layout":"diagram",
// "diagram":{"nodes":[...],"edges":[...]}} - vi chart thi dung la long ({"chart":{...}}). Luot that 25/10/2026
// (ocg/beam-free, de 10 slide kien truc): tool doc nodes o cap slide nen thay 0 nut, bao NEED_2_TO_10_NODES, model
// gui lai y het 3 lan va luot hong. Chart khong nam day (khoa "chart" chinh la truong cua no).
var nestedLayouts = map[string]bool{"diagram": true, "process": true, "timeline": true, "cards": true, "figures": true,
	"content": true, "cover": true}

// unnestLayoutFields: chep truong trong khoa cung ten voi layout len cap slide (truong da co o cap slide giu nguyen).
func unnestLayoutFields(list []any) {
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		layout := strings.TrimSpace(textOf(entry["layout"]))
		nested, ok := entry[layout].(map[string]any)
		if !ok || !nestedLayouts[layout] {
			continue
		}
		for key, value := range nested {
			if _, exists := entry[key]; !exists {
				entry[key] = value
			}
		}
		delete(entry, layout)
	}
}

func parseBrief(value any) ([]briefSlide, []rowError) {
	list, _ := value.([]any)
	if len(list) == 0 {
		return nil, []rowError{{Field: "slides", Error: "EMPTY", Allowed: "one or more slides"}}
	}
	unnestLayoutFields(list)
	rows, errors := parseCompose(value)
	slides := []briefSlide{}
	for i, row := range rows {
		slide := briefSlide{composeRow: row}
		if entry, ok := list[i].(map[string]any); ok {
			slide.Source = strings.TrimSpace(textOf(entry["source"]))
			slide.Replace = asInt(entry["replace"])
			slide.Steps = briefItems(entry["steps"], "title", "text")
			slide.Milestones = briefItems(entry["milestones"], "date", "text")
			if nodes, ok := entry["nodes"].([]any); ok {
				for _, item := range nodes {
					if node, ok := item.(map[string]any); ok {
						slide.Nodes = append(slide.Nodes, briefNode{ID: strings.TrimSpace(textOf(node["id"])),
							Text: strings.TrimSpace(textOf(node["text"])), Group: strings.TrimSpace(textOf(node["group"])),
							Layer: asInt(node["layer"])})
					}
				}
			}
			if edges, ok := entry["edges"].([]any); ok {
				for _, item := range edges {
					if edge, ok := item.(map[string]any); ok {
						slide.Edges = append(slide.Edges, briefEdge{From: strings.TrimSpace(textOf(edge["from"])),
							To: strings.TrimSpace(textOf(edge["to"]))})
					}
				}
			}
			if chart, ok := entry["chart"].(map[string]any); ok {
				slide.Chart = parseChart(chart)
			}
			if figures, ok := entry["figures"].([]any); ok {
				for _, item := range figures {
					if figure, ok := item.(map[string]any); ok {
						slide.Figures = append(slide.Figures, briefFigure{Value: strings.TrimSpace(textOf(figure["value"])),
							Label: strings.TrimSpace(textOf(figure["label"]))})
					}
				}
			}
		}
		slides = append(slides, slide)
	}
	return slides, errors
}

func parseChart(entry map[string]any) *briefChart {
	// unit giu nguyen dau cach: " tỷ" -> "28 tỷ", "%" -> "55%".
	unit, _ := entry["unit"].(string)
	chart := &briefChart{Type: strings.ToLower(strings.TrimSpace(textOf(entry["type"]))), Unit: unit}
	if chart.Type == "" {
		chart.Type = "column"
	}
	categories, _ := entry["categories"].([]any)
	for _, category := range categories {
		chart.Categories = append(chart.Categories, strings.TrimSpace(textOf(category)))
	}
	series, _ := entry["series"].([]any)
	for _, raw := range series {
		if item, ok := raw.(map[string]any); ok {
			values, _ := item["values"].([]any)
			chart.Series = append(chart.Series, briefSeries{Name: strings.TrimSpace(textOf(item["name"])), Values: values})
		}
	}
	return chart
}

// chartNumber: so tu brief - so JSON, hoac chuoi so ("28", "28,5"). Chuoi kem don vi ("28 tỷ") la loi: don vi de o unit.
func chartNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case string:
		number, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", "."), 64)
		return number, err == nil
	}
	return 0, false
}

// checkChart: kiem bieu do va dien Numbers. Bieu do khong co chu dai (nhan danh muc do PowerPoint tu xuong dong), nen
// chi gioi han so luong va do dai nhan.
func checkChart(slot int, chart *briefChart) []rowError {
	if chart == nil {
		return []rowError{{Src: slot, Field: "chart", Error: "MISSING", Allowed: "{type, categories, series: [{name, values}], unit}"}}
	}
	errors := []rowError{}
	switch chart.Type {
	case "column", "bar", "line", "pie":
	default:
		errors = append(errors, rowError{Src: slot, Field: "chart.type", Error: "UNKNOWN_CHART_TYPE", Allowed: "column, bar, line, pie"})
	}
	if n := len(chart.Categories); n < 2 || n > 12 {
		errors = append(errors, rowError{Src: slot, Field: "chart.categories", Error: "NEED_2_TO_12_CATEGORIES", Actual: n})
	}
	for c, category := range chart.Categories {
		if category == "" || len([]rune(category)) > 30 {
			errors = append(errors, rowError{Src: slot, Field: fmt.Sprintf("chart.categories[%d]", c), Error: "LABEL_1_TO_30_CHARS",
				Actual: len([]rune(category))})
		}
	}
	if len([]rune(chart.Unit)) > 8 || strings.ContainsAny(chart.Unit, "\"\\") {
		errors = append(errors, rowError{Src: slot, Field: "chart.unit", Error: "BAD_UNIT", Allowed: "short suffix, e.g. \"%\" or \" tỷ\""})
	}
	maxSeries := 4
	if chart.Type == "pie" {
		maxSeries = 1
	}
	if n := len(chart.Series); n < 1 || n > maxSeries {
		errors = append(errors, rowError{Src: slot, Field: "chart.series", Error: fmt.Sprintf("NEED_1_TO_%d_SERIES", maxSeries), Actual: n})
	}
	for i := range chart.Series {
		series := &chart.Series[i]
		field := fmt.Sprintf("chart.series[%d]", i)
		if series.Name == "" || len([]rune(series.Name)) > 40 {
			errors = append(errors, rowError{Src: slot, Field: field + ".name", Error: "NAME_1_TO_40_CHARS"})
		}
		if len(series.Values) != len(chart.Categories) {
			errors = append(errors, rowError{Src: slot, Field: field + ".values", Error: "ONE_VALUE_PER_CATEGORY",
				Actual: len(series.Values), Max: len(chart.Categories)})
			continue
		}
		series.Numbers = series.Numbers[:0]
		for v, value := range series.Values {
			number, ok := chartNumber(value)
			if !ok || (chart.Type == "pie" && number < 0) {
				errors = append(errors, rowError{Src: slot, Field: fmt.Sprintf("%s.values[%d]", field, v), Error: "NOT_A_NUMBER",
					Allowed: "plain numbers (pie: 0 or more); put the unit in chart.unit"})
				continue
			}
			series.Numbers = append(series.Numbers, number)
		}
	}
	return errors
}

// chartColors: mau chuoi/lat banh theo token - dam truoc, nhat sau.
func chartColors(colors struct{ Primary, Accent, Text, Muted string }) []string {
	return []string{colors.Primary, colors.Accent, slidelayout.Tint(colors.Primary, 0.45), slidelayout.Tint(colors.Accent, 0.55),
		colors.Muted, slidelayout.Tint(colors.Muted, 0.55)}
}

func briefTheme(tokens briefTokens) slidelayout.Theme {
	body, caption := tokens.Slide.Type.Body, tokens.Slide.Type.Caption
	// Khong co chu (Min = Size): chu khong vua thi bao loi de model rut gon (skill muc 2-3), khong thu chu.
	return slidelayout.Theme{Points: body, PointsMin: body, Card: body, CardMin: body, Callout: body, CalloutMin: body,
		Caption: caption, CaptionMin: caption}
}

// checkBrief: kiem truoc khi dung - loi nao cung kem truong va nguong de model sua dung cho.
func checkBrief(slides []briefSlide, grid slidelayout.Grid, tokens briefTokens) []rowError {
	errors := []rowError{}
	theme := briefTheme(tokens)
	body := tokens.Slide.Type.Body
	allowed := strings.Join(briefLayouts, ", ")
	// Phuong an thay the khi chu khong vua - KHONG tu tach slide hay doi kieu: baseline 25/10/2026 cho thay loi hay gap
	// nhat la them slide khong duoc xin (docs/plans/18-s0 M5), nen chi goi y, model quyet dinh theo yeu cau nguoi dung.
	fallback := map[string]string{
		"cards":   "shorter points, or layout table (one row per card)",
		"content": "shorter points (details to notes); a second content slide only if the user did not fix the number of slides",
	}
	fits := func(slot int, field, text string, box slidelayout.Rect, size float64) {
		if !slidelayout.TextFits(text, box[2], box[3], size) {
			hint := fallback[slides[slot-1].Layout]
			if hint == "" || strings.HasPrefix(field, "title") || strings.HasPrefix(field, "source") {
				hint = "shorter text (move details to notes)"
			}
			errors = append(errors, rowError{Src: slot, Field: field, Error: "TOO_LONG", Actual: len([]rune(text)), Allowed: hint})
		}
	}
	for i, slide := range slides {
		slot := i + 1
		title := grid.Title()
		titleSize := tokens.Slide.Type.Title
		switch slide.Layout {
		case "cover":
			title, _ = grid.Cover()
			titleSize = tokens.Slide.Type.Hero
		case "section":
			title = grid.Section()
			titleSize = tokens.Slide.Type.Hero
		}
		if slide.Title == "" {
			errors = append(errors, rowError{Src: slot, Field: "title", Error: "MISSING"})
		} else {
			fits(slot, "title", slide.Title, title, titleSize)
		}
		// Dong nguon HIEN tren slide. Luot that 26/10/2026: model ghi "source":"Slide 1 — Cover" (nhan dan y) -> bia
		// hien dong do, model sua qua lai 3 lan roi tra lai nhu cu. Bia/section khong co dong nguon; nhan slide thi tu choi.
		if slide.Source != "" && (slide.Layout == "cover" || slide.Layout == "section") {
			errors = append(errors, rowError{Src: slot, Field: "source", Error: "NO_SOURCE_ON_" + strings.ToUpper(slide.Layout),
				Allowed: "omit source on this slide (it is printed on the slide); put remarks in notes"})
		} else if slideLabel.MatchString(slide.Source) {
			errors = append(errors, rowError{Src: slot, Field: "source", Error: "SOURCE_IS_SLIDE_LABEL",
				Allowed: "source is printed on the slide: only the data source, e.g. \"Nguồn: báo cáo nội bộ 2025\"; omit it if there is none"})
		} else if source := grid.Source(); slide.Source != "" && slidelayout.EstimateLines(slide.Source, source[2], tokens.Slide.Type.Caption) > 1 {
			errors = append(errors, rowError{Src: slot, Field: "source", Error: "TOO_LONG", Actual: len([]rune(slide.Source)),
				Allowed: "one line"})
		}
		area := grid.Content()
		switch slide.Layout {
		case "cover":
			if slide.Subtitle != "" {
				_, subtitle := grid.Cover()
				fits(slot, "subtitle", slide.Subtitle, subtitle, tokens.Slide.Type.Subtitle)
			}
		case "section":
		case "content":
			maxPoints := max(tokens.Slide.MaxBulletsPerSlide, 1)
			switch {
			case len(slide.Points) == 0:
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "EMPTY"})
			case len(slide.Points) > maxPoints:
				errors = append(errors, rowError{Src: slot, Field: "points", Error: "TOO_MANY_POINTS", Max: maxPoints,
					Actual: len(slide.Points), Allowed: "fewer points, or a second slide only if the user asked for more slides"})
			default:
				for _, point := range slide.Points {
					if length := len([]rune(point.T)); length > maxPointChars {
						errors = append(errors, rowError{Src: slot, Field: "points", Error: "POINT_TOO_LONG", Max: maxPointChars,
							Actual: length, Allowed: "short phrases; details go to notes"})
					}
				}
				for _, box := range slidelayout.Solve(slidelayout.Spec{Layout: "content", HasPoints: true, ShowPoints: true,
					Callout: slide.Callout}, area, theme) {
					switch box.Role {
					case "points":
						fits(slot, "points", pointsText(slide.Points), box.Rect, box.Size)
					case "callout":
						fits(slot, "callout", slide.Callout, slidelayout.Rect{0, 0, box.Rect[2] - 24, box.Rect[3] - 12}, box.Size)
					}
				}
			}
		case "cards":
			if len(slide.Cards) < 2 || len(slide.Cards) > maxCards {
				errors = append(errors, rowError{Src: slot, Field: "cards", Error: "NEED_2_TO_4_CARDS", Actual: len(slide.Cards)})
				break
			}
			for c, box := range slidelayout.CardBoxes(area, len(slide.Cards)) {
				card := slide.Cards[c]
				if card.Title == "" || len(card.Points) == 0 {
					errors = append(errors, rowError{Src: slot, Field: fmt.Sprintf("cards[%d]", c), Error: "NEEDS_TITLE_AND_POINTS"})
					continue
				}
				paras := append([]sourcePara{{T: card.Title}}, card.Points...)
				fits(slot, fmt.Sprintf("cards[%d]", c), pointsText(paras), slidelayout.Rect{0, 0, box[2] - 20, box[3] - 16}, body)
			}
		case "table":
			cols := 0
			for _, line := range slide.Table {
				cols = max(cols, len(line))
				for _, cell := range line {
					if len([]rune(cell)) > 60 {
						errors = append(errors, rowError{Src: slot, Field: "table", Error: "CELL_TOO_LONG", Max: 60, Actual: len([]rune(cell))})
						break
					}
				}
			}
			if len(slide.Table) < 2 || len(slide.Table) > 7 || cols < 2 || cols > maxTableCols {
				errors = append(errors, rowError{Src: slot, Field: "table", Error: "TABLE_SIZE",
					Allowed: fmt.Sprintf("header row + 1-6 rows, 2-%d columns", maxTableCols)})
			}
		case "figures":
			if len(slide.Figures) < 2 || len(slide.Figures) > 3 {
				errors = append(errors, rowError{Src: slot, Field: "figures", Error: "NEED_2_TO_3_FIGURES", Actual: len(slide.Figures)})
				break
			}
			values, labels := grid.Figures(len(slide.Figures), tokens.Slide.Gutter)
			for f, figure := range slide.Figures {
				field := fmt.Sprintf("figures[%d]", f)
				if figure.Value == "" || figure.Label == "" {
					errors = append(errors, rowError{Src: slot, Field: field, Error: "NEEDS_VALUE_AND_LABEL"})
					continue
				}
				fits(slot, field+".value", figure.Value, values[f], tokens.Slide.Type.Figure)
				fits(slot, field+".label", figure.Label, labels[f], body)
			}
		case "process":
			if len(slide.Steps) < 3 || len(slide.Steps) > 5 {
				errors = append(errors, rowError{Src: slot, Field: "steps", Error: "NEED_3_TO_5_STEPS", Actual: len(slide.Steps),
					Allowed: `steps directly on the slide: {"layout":"process","title":"...","steps":[{"title":"...","text":"..."}]}`})
				break
			}
			errors = append(errors, checkGroups(slot, slide)...)
			for s, box := range slidelayout.Process(shapesArea(area, slide), len(slide.Steps)) {
				step := slide.Steps[s]
				field := fmt.Sprintf("steps[%d]", s)
				if step.Title == "" {
					errors = append(errors, rowError{Src: slot, Field: field, Error: "NEEDS_TITLE"})
					continue
				}
				fits(slot, field, stepText(step), shapeText(box), body)
			}
		case "timeline":
			if len(slide.Milestones) < 3 || len(slide.Milestones) > 6 {
				errors = append(errors, rowError{Src: slot, Field: "milestones", Error: "NEED_3_TO_6_MILESTONES", Actual: len(slide.Milestones),
					Allowed: `milestones directly on the slide: {"layout":"timeline","title":"...","milestones":[{"date":"...","text":"..."}]}`})
				break
			}
			_, _, dates, texts := slidelayout.Timeline(area, len(slide.Milestones))
			for m, milestone := range slide.Milestones {
				field := fmt.Sprintf("milestones[%d]", m)
				if milestone.Title == "" {
					errors = append(errors, rowError{Src: slot, Field: field, Error: "NEEDS_DATE"})
					continue
				}
				if slidelayout.EstimateLines(milestone.Title, dates[m][2], body) > 1 {
					errors = append(errors, rowError{Src: slot, Field: field + ".date", Error: "TOO_LONG",
						Actual: len([]rune(milestone.Title)), Allowed: "one short line, e.g. 11/2026"})
				}
				if milestone.Text != "" {
					fits(slot, field+".text", milestone.Text, texts[m], body)
				}
			}
		case "diagram":
			layers, problems := diagramLayers(slide)
			for _, problem := range problems {
				problem.Src = slot
				errors = append(errors, problem)
			}
			if len(problems) > 0 {
				break
			}
			errors = append(errors, checkGroups(slot, slide)...)
			rows := slidelayout.Diagram(shapesArea(area, slide), layerCounts(layers))
			for l, layer := range layers {
				for n, node := range layer {
					fits(slot, "nodes["+node.ID+"]", node.Text, shapeText(rows[l][n]), body)
				}
			}
		case "chart":
			errors = append(errors, checkChart(slot, slide.Chart)...)
		default:
			errors = append(errors, rowError{Src: slot, Field: "layout", Error: "UNKNOWN_LAYOUT", Allowed: allowed})
		}
	}
	return errors
}

// stepText: chu trong hinh cua mot buoc - ten buoc (dong dau, dam) roi mo ta.
func stepText(step briefItem) string {
	if step.Text == "" {
		return step.Title
	}
	return step.Title + "\n" + step.Text
}

// shapeText: vung chu trong mot hinh khoi (le trai/phai 6, tren/duoi 4 - wpp.addShape).
func shapeText(box slidelayout.Rect) slidelayout.Rect {
	return slidelayout.Rect{box[0], box[1], box[2] - 12, box[3] - 8}
}

// diagramLayers: nut theo tang (tang 1 tren cung), giu thu tu trong tang; kiem id, tang, canh.
func diagramLayers(slide briefSlide) ([][]briefNode, []rowError) {
	if len(slide.Nodes) < 2 || len(slide.Nodes) > 10 {
		return nil, []rowError{{Field: "nodes", Error: "NEED_2_TO_10_NODES", Actual: len(slide.Nodes),
			Allowed: `nodes and edges directly on the slide: {"layout":"diagram","title":"...","nodes":[{"id":"a","text":"...","layer":1}],"edges":[{"from":"a","to":"b"}]}`}}
	}
	errors := []rowError{}
	byLayer := map[int][]briefNode{}
	ids := map[string]bool{}
	top := 0
	for _, node := range slide.Nodes {
		switch {
		case node.ID == "" || node.Text == "":
			errors = append(errors, rowError{Field: "nodes", Error: "NEEDS_ID_AND_TEXT"})
			continue
		case ids[node.ID]:
			errors = append(errors, rowError{Field: "nodes[" + node.ID + "]", Error: "DUPLICATE_ID"})
			continue
		case node.Layer < 1 || node.Layer > 4:
			errors = append(errors, rowError{Field: "nodes[" + node.ID + "].layer", Error: "LAYER_1_TO_4", Actual: node.Layer})
			continue
		}
		ids[node.ID] = true
		byLayer[node.Layer] = append(byLayer[node.Layer], node)
		top = max(top, node.Layer)
	}
	layers := [][]briefNode{}
	for layer := 1; layer <= top; layer++ {
		if len(byLayer[layer]) > 4 {
			errors = append(errors, rowError{Field: fmt.Sprintf("layer %d", layer), Error: "MAX_4_NODES_PER_LAYER", Actual: len(byLayer[layer])})
		}
		if len(byLayer[layer]) > 0 {
			layers = append(layers, byLayer[layer])
		}
	}
	for _, edge := range slide.Edges {
		if !ids[edge.From] || !ids[edge.To] || edge.From == edge.To {
			errors = append(errors, rowError{Field: "edges", Error: "UNKNOWN_NODE", Allowed: edge.From + " -> " + edge.To})
		}
	}
	return layers, errors
}

func layerCounts(layers [][]briefNode) []int {
	counts := []int{}
	for _, layer := range layers {
		counts = append(counts, len(layer))
	}
	return counts
}

func (t *SlidesFromBriefTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	slides, errors := parseBrief(arguments["slides"])
	if len(errors) > 0 {
		return Result{JSON: OKJSON(map[string]any{"built": 0, "errors": errors})}
	}
	target := strings.TrimSpace(run.TargetDocument)
	bridge := func(action string, params map[string]any) (map[string]any, string) {
		if target != "" {
			params["presentation"] = target
		}
		result := run.Bridge.Command(ctx, run.Office.Port, action, params)
		run.trace("tool.bridge", map[string]any{"tool": "slides_from_brief", "action": action, "params": params,
			"ok": result.OK, "error": result.Error, "result": result.Result})
		if !result.OK {
			return nil, action + ": " + result.Error
		}
		// Slide va hinh do tool dung la cua luot: agent xoa lai khong phai hoi (owned.go).
		run.ownedObjects().record(action, params, result.Result)
		return result.Result, ""
	}
	layout, problem := bridge("wpp.checkLayout", map[string]any{})
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	tokens := t.tokens()
	grid := slidelayout.Grid{Width: floatOf(layout["slideWidth"], 960), Height: floatOf(layout["slideHeight"], 540), Margin: tokens.Slide.Margin}
	errors = checkBrief(slides, grid, tokens)
	errors = append(errors, checkReplace(slides, run, target)...)
	specs := briefSpecs(arguments["slides"])
	errors = append(errors, checkSameSpec(specs, run)...)
	if len(errors) > 0 {
		return Result{JSON: OKJSON(map[string]any{"built": 0, "errors": errors,
			"how": "nothing was built: fix every error (shorten the text named in field) and call again with ALL slides"})}
	}

	built := []map[string]any{}
	issueCount, rerouted := 0, 0
	for s, slide := range slides {
		if slide.Replace > 0 {
			if _, problem := bridge("wpp.deleteSlide", map[string]any{"slide": slide.Replace}); problem != "" {
				return Result{JSON: OKJSON(map[string]any{"built": len(built), "slides": built, "error": problem}), Mutating: len(built) > 0}
			}
		}
		index, issues, reroutedHere, problem := buildBriefSlide(bridge, grid, tokens, slide, slide.Replace, run.Trace)
		if problem != "" {
			return Result{JSON: OKJSON(map[string]any{"built": len(built), "slides": built, "error": problem}), Mutating: true}
		}
		issueCount += len(issues)
		rerouted += reroutedHere
		if reroutedHere > 0 {
			corelog.Info("run %s: slides_from_brief slide %d: rerouted %d connector(s) after checkLayout",
				run.RunID, index, reroutedHere)
		}
		entry := map[string]any{"slide": index, "layout": slide.Layout, "title": slide.Title, "issues": issues}
		if slide.Replace > 0 {
			entry["replaced"] = slide.Replace
		}
		// Tool tự đi lại dây khi PowerPoint vẽ lệch mô hình: báo để model không tưởng còn lỗi (và không dựng lại slide).
		if reroutedHere > 0 {
			entry["connectorsRerouted"] = reroutedHere
		}
		// Bao ro ghi chu da dat: luot that 25/10/2026 model dat notes qua brief roi van goi wpp.setNotes tung slide (10 vong).
		if strings.TrimSpace(slide.Notes) != "" {
			entry["notes"] = "set"
		}
		built = append(built, entry)
		logIssues(run.RunID, "slides_from_brief", index, issues)
		if s < len(specs) {
			run.rememberBriefSpec(specs[s], len(issues))
		}
	}
	// Tool tu kiem sau khi ghi (wpp.checkLayout tung slide): bao ro DA kiem gi va CHUA kiem gi (tu van ChatGPT
	// 25/10/2026: "0 loi bo cuc" khong phai "dung yeu cau") - Verifies cho verifyGuard biet khong can them vong doc lai.
	verified := map[string]any{"checked": "layout of every new slide (wpp.checkLayout: overflow, overlap, off-slide, " +
		"small font, connectors crossing boxes)", "issueCount": issueCount,
		"notChecked": "wording, facts and whether the slides cover everything the user asked - compare with the request yourself"}
	summary := map[string]any{"built": len(built), "slides": built, "issueCount": issueCount, "verified": verified}
	if rerouted > 0 {
		summary["connectorsRerouted"] = rerouted
	}
	// Con loi thi noi ro cach sua: luot that 26/10/2026 (trace r_22ae8aca) chi bao issueCount, model dung lai CA 10 slide
	// (bai thanh 20 slide) roi mat 7 vong xoa tung slide cu.
	if broken := slidesWithIssues(built); len(broken) > 0 {
		numbers := make([]string, len(broken))
		for i, slide := range broken {
			numbers[i] = strconv.Itoa(slide)
		}
		summary["next"] = "the other slides are done - do NOT rebuild the deck. Fix only slide(s) " +
			strings.Join(numbers, ", ") + ": call slides_from_brief again with ONLY those slides, each with " +
			"\"replace\": <its slide number> and a CHANGED spec (shorter text, fewer items; diagrams: fewer nodes per " +
			"layer or fewer edges); " +
			"the same spec gives the same issues"
	}
	return Result{JSON: OKJSON(summary), OK: true, Mutating: true, Verifies: true}
}

// buildBriefSlide: dung mot slide - slide trong (layout 12), tieu de, cac hop cua kieu, dong nguon, ghi chu, roi soat.
// Tra them so duong noi da tu di lai (checkLayout bao connector-*) de bao cho model.
func buildBriefSlide(bridge func(string, map[string]any) (map[string]any, string), grid slidelayout.Grid, tokens briefTokens,
	slide briefSlide, at int, trace func(string, map[string]any)) (int, []any, int, string) {
	addParams := map[string]any{"layout": 12}
	if at > 0 {
		addParams["index"] = at
	}
	added, problem := bridge("wpp.addSlide", addParams)
	if problem != "" {
		return 0, nil, 0, problem
	}
	index := asInt(added["slide"])
	colors := tokens.Colors
	text := func(paras []sourcePara, box slidelayout.Rect, size float64, color string, extra map[string]any) string {
		params := map[string]any{"slide": index, "paragraphs": paras, "left": box[0], "top": box[1], "width": box[2],
			"height": box[3], "fontSize": size, "minFontSize": size, "color": color, "accent": colors.Primary, "font": briefFont,
			"anchor": "top"}
		for key, value := range extra {
			params[key] = value
		}
		_, problem := bridge("wpp.addParagraphs", params)
		return problem
	}
	heading := []sourcePara{{T: slide.Title, B: true}}
	steps := []func() string{}
	var plan *diagramPlan
	switch slide.Layout {
	case "cover":
		title, subtitle := grid.Cover()
		steps = append(steps, func() string {
			return text([]sourcePara{{T: slide.Title, B: true, A: "center"}}, title, tokens.Slide.Type.Hero, colors.Primary,
				map[string]any{"anchor": "middle"})
		})
		if slide.Subtitle != "" {
			steps = append(steps, func() string {
				// Can giua nhu tieu de (truoc day phu de can trai duoi tieu de can giua - luot that 25/10/2026).
				paras := parasFromText(slide.Subtitle)
				for p := range paras {
					paras[p].A = "center"
				}
				return text(paras, subtitle, tokens.Slide.Type.Subtitle, colors.Muted, nil)
			})
		}
	case "section":
		steps = append(steps, func() string {
			return text([]sourcePara{{T: slide.Title, B: true, A: "center"}}, grid.Section(), tokens.Slide.Type.Hero, colors.Primary,
				map[string]any{"anchor": "middle"})
		})
	default:
		steps = append(steps, func() string {
			return text(heading, grid.Title(), tokens.Slide.Type.Title, colors.Primary, map[string]any{"anchor": "middle"})
		})
		body, drawn := briefBody(bridge, index, grid, tokens, slide, text)
		plan = drawn
		steps = append(steps, body...)
	}
	if slide.Source != "" {
		steps = append(steps, func() string {
			return text([]sourcePara{{T: slide.Source}}, grid.Source(), tokens.Slide.Type.Caption, colors.Muted, nil)
		})
	}
	if strings.TrimSpace(slide.Notes) != "" {
		steps = append(steps, func() string {
			_, problem := bridge("wpp.setNotes", map[string]any{"slide": index, "text": strings.TrimSpace(slide.Notes)})
			return problem
		})
	}
	for _, step := range steps {
		if problem := step(); problem != "" {
			return index, nil, 0, problem
		}
	}
	issues := layoutIssues(bridge, index)
	rerouted := 0
	if plan != nil {
		plan.trace = trace
		issues, rerouted = plan.repair(bridge, issues)
	}
	return index, issues, rerouted, ""
}

// layoutIssues: issues cua MOT slide sau khi dung (wpp.checkLayout). Loi goi bridge thi coi nhu khong doc duoc - khong
// che mat loi dung slide (ket qua cuoi van duoc tra trong "issues").
func layoutIssues(bridge func(string, map[string]any) (map[string]any, string), index int) []any {
	checked, problem := bridge("wpp.checkLayout", map[string]any{"slide": index})
	if problem != "" {
		return nil
	}
	list, _ := checked["slides"].([]any)
	if len(list) == 0 {
		return nil
	}
	entry, _ := list[0].(map[string]any)
	issues, _ := entry["issues"].([]any)
	return issues
}

// briefBody: cac hop noi dung cua content / cards / table / figures (hinh hoc tu slidelayout).
func briefBody(bridge func(string, map[string]any) (map[string]any, string), index int, grid slidelayout.Grid, tokens briefTokens,
	slide briefSlide, text func([]sourcePara, slidelayout.Rect, float64, string, map[string]any) string) ([]func() string, *diagramPlan) {
	colors := tokens.Colors
	steps := []func() string{}
	switch slide.Layout {
	case "process", "timeline", "diagram":
		return briefShapes(bridge, index, grid, tokens, slide, text)
	case "chart":
		return append(steps, func() string {
			area, chart := grid.Content(), slide.Chart
			series := []map[string]any{}
			for _, item := range chart.Series {
				series = append(series, map[string]any{"name": item.Name, "values": item.Numbers})
			}
			_, problem := bridge("wpp.addChart", map[string]any{"slide": index, "left": area[0], "top": area[1], "width": area[2],
				"height": area[3], "type": chart.Type, "categories": chart.Categories, "series": series, "unit": chart.Unit,
				"colors": chartColors(colors), "font": briefFont, "fontSize": tokens.Slide.Type.Caption + 2, "color": colors.Text})
			return problem
		}), nil
	}
	if slide.Layout == "figures" {
		values, labels := grid.Figures(len(slide.Figures), tokens.Slide.Gutter)
		for f, figure := range slide.Figures {
			value, label, valueBox, labelBox := figure.Value, figure.Label, values[f], labels[f]
			steps = append(steps, func() string {
				return text([]sourcePara{{T: value, B: true, A: "center"}}, valueBox, tokens.Slide.Type.Figure, colors.Accent,
					map[string]any{"anchor": "middle"})
			}, func() string {
				return text([]sourcePara{{T: label, A: "center"}}, labelBox, tokens.Slide.Type.Body, colors.Text, nil)
			})
		}
		return steps, nil
	}
	spec := slidelayout.Spec{Layout: slide.Layout, HasPoints: len(slide.Points) > 0, ShowPoints: len(slide.Points) > 0,
		Callout: slide.Callout}
	for _, card := range slide.Cards {
		plain := slidelayout.Card{Title: card.Title}
		for _, point := range card.Points {
			plain.Points = append(plain.Points, strings.ReplaceAll(point.T, "**", ""))
		}
		spec.Cards = append(spec.Cards, plain)
	}
	for _, box := range slidelayout.Solve(spec, grid.Content(), briefTheme(tokens)) {
		box := box
		switch box.Role {
		case "points":
			steps = append(steps, func() string {
				return text(slide.Points, box.Rect, box.Size, colors.Text, map[string]any{"anchor": "top"})
			})
		case "card":
			card := slide.Cards[box.Index]
			steps = append(steps, func() string {
				paras := append([]sourcePara{{T: card.Title, B: true, A: "center"}}, card.Points...)
				return text(paras, box.Rect, box.Size, colors.Text, map[string]any{"box": "rounded",
					"fill": slidelayout.Tint(colors.Primary, 0.9), "line": slidelayout.Tint(colors.Primary, 0.55), "anchor": "middle"})
			})
		case "grid":
			steps = append(steps, func() string {
				_, problem := bridge("wpp.addGrid", map[string]any{"slide": index, "values": slide.Table, "left": box.Rect[0],
					"top": box.Rect[1], "width": box.Rect[2], "height": box.Rect[3], "header": colors.Primary,
					"band": slidelayout.Tint(colors.Primary, 0.9), "color": colors.Text, "font": briefFont,
					"fontSize": tokens.Slide.Type.Body, "minFontSize": tokens.Slide.Type.Caption})
				return problem
			})
		case "callout":
			steps = append(steps, func() string {
				return text([]sourcePara{{T: slide.Callout}}, box.Rect, box.Size, colors.Text, map[string]any{"box": "rounded",
					"fill": slidelayout.Tint(colors.Primary, 0.88), "line": colors.Primary, "anchor": "middle"})
			})
		}
	}
	return steps, nil
}

// briefShapes: quy trinh, dong thoi gian, so do - ve bang hinh khoi va duong noi goc (wpp.addShape, wpp.addConnector),
// nguoi dung keo sua duoc; duong noi bam theo hinh. Ten hinh chi biet khi da ve, nen buoc noi doc ten qua con tro.
func briefShapes(bridge func(string, map[string]any) (map[string]any, string), index int, grid slidelayout.Grid, tokens briefTokens,
	slide briefSlide, text func([]sourcePara, slidelayout.Rect, float64, string, map[string]any) string) ([]func() string, *diagramPlan) {
	colors := tokens.Colors
	steps := []func() string{}
	var plan *diagramPlan
	shape := func(box slidelayout.Rect, kind, label string, extra map[string]any) (string, string) {
		params := map[string]any{"slide": index, "left": box[0], "top": box[1], "width": box[2], "height": box[3], "shape": kind,
			"font": briefFont, "fontSize": tokens.Slide.Type.Body, "color": colors.Text}
		if label != "" {
			params["text"] = label
		}
		for key, value := range extra {
			params[key] = value
		}
		result, problem := bridge("wpp.addShape", params)
		name, _ := result["shape"].(string)
		return name, problem
	}
	// direction: di ra canh nao cua hinh dau, vao canh doi dien cua hinh sau (wpp.addConnector) - khong de PowerPoint tu
	// chon diem noi, duong gap khuc co the cat ngang hop ben canh (benchmark s17, docs/plans/18-s0 9.3).
	connect := func(from, to *string, elbow bool, direction string) func() string {
		return func() string {
			_, problem := bridge("wpp.addConnector", map[string]any{"slide": index, "from": *from, "to": *to, "elbow": elbow,
				"direction": direction, "color": colors.Muted})
			return problem
		}
	}
	// drawRoute: so do - canh ra/vao do RouteDiagram chon (tranh hop khac, duong vong co do sau rieng). Ten duong noi
	// duoc giu lai de plan.repair xoa/ve lai dung duong khi checkLayout bao loi sau khi PowerPoint ve.
	drawRoute := func(plan *diagramPlan, e int, from, to *string, route slidelayout.Route) func() string {
		return func() string {
			result, problem := bridge("wpp.addConnector", connectorParams(index, *from, *to, route, colors.Muted))
			if name := textOf(result["shape"]); problem == "" && name != "" {
				plan.connectors[e] = name
			}
			return problem
		}
	}
	groups := groupsOf(slide)
	palette := groupPalette(colors)
	style := func(filled bool, group string) map[string]any {
		values := map[string]any{"line": colors.Primary, "boldFirst": true}
		if filled {
			values["fill"] = slidelayout.Tint(colors.Primary, 0.9)
		}
		// Co nhom: moi nhom mot mau (nen nhat + vien dam), ke ca hop khong nhom dung mau nhom dau.
		if len(groups) > 0 {
			swatch := palette[max(0, indexOf(groups, group))]
			values["fill"], values["line"] = swatch.Fill, swatch.Line
		}
		return values
	}
	area := shapesArea(grid.Content(), slide)
	if len(groups) > 0 {
		steps = append(steps, legendSteps(groups, palette, grid.Content(), func(box slidelayout.Rect, swatch groupSwatch) string {
			_, problem := shape(box, "rect", "", map[string]any{"fill": swatch.Fill, "line": swatch.Line})
			return problem
		}, func(label string, box slidelayout.Rect) string {
			return text([]sourcePara{{T: label}}, box, tokens.Slide.Type.Caption, colors.Text, map[string]any{"anchor": "middle"})
		})...)
	}
	switch slide.Layout {
	case "process":
		names := make([]string, len(slide.Steps))
		for s, rect := range slidelayout.Process(area, len(slide.Steps)) {
			s, rect, step := s, rect, slide.Steps[s]
			steps = append(steps, func() string {
				var problem string
				names[s], problem = shape(rect, "rounded", stepText(step), style(true, step.Group))
				return problem
			})
		}
		for s := 1; s < len(slide.Steps); s++ {
			steps = append(steps, connect(&names[s-1], &names[s], false, "right"))
		}
	case "timeline":
		axis, dots, dates, texts := slidelayout.Timeline(area, len(slide.Milestones))
		steps = append(steps, func() string {
			_, problem := shape(axis, "rect", "", map[string]any{"fill": colors.Muted})
			return problem
		})
		for m, milestone := range slide.Milestones {
			dot, date, label, milestone := dots[m], dates[m], texts[m], milestone
			steps = append(steps, func() string {
				_, problem := shape(dot, "oval", "", map[string]any{"fill": colors.Primary})
				return problem
			}, func() string {
				return text([]sourcePara{{T: milestone.Title, B: true, A: "center"}}, date, tokens.Slide.Type.Body, colors.Accent,
					map[string]any{"anchor": "middle"})
			})
			if milestone.Text != "" {
				steps = append(steps, func() string {
					return text([]sourcePara{{T: milestone.Text, A: "center"}}, label, tokens.Slide.Type.Body, colors.Text, nil)
				})
			}
		}
	case "diagram":
		layers, _ := diagramLayers(slide)
		rows := slidelayout.Diagram(area, layerCounts(layers))
		names := map[string]*string{}
		layerOf, place := map[string]int{}, map[string]int{}
		for l, layer := range layers {
			for n, node := range layer {
				rect, node, name, first := rows[l][n], node, new(string), l == 0
				names[node.ID], layerOf[node.ID], place[node.ID] = name, l, n
				steps = append(steps, func() string {
					var problem string
					*name, problem = shape(rect, "rounded", node.Text, style(first, node.Group))
					return problem
				})
			}
		}
		edges := []slidelayout.Edge{}
		for _, edge := range slide.Edges {
			edges = append(edges, slidelayout.Edge{FromLayer: layerOf[edge.From], FromPlace: place[edge.From],
				ToLayer: layerOf[edge.To], ToPlace: place[edge.To]})
		}
		// Tieu de, dong chu thich va dong nguon cung la vat can cua duong noi (checkLayout tinh ca chung la "hop";
		// luot that 25/10/2026: duong vong duoi hang cuoi cat qua dong nguon).
		obstacles := append([]slidelayout.Rect{grid.Title()}, legendRects(groups, grid.Content())...)
		if slide.Source != "" {
			obstacles = append(obstacles, grid.Source())
		}
		routes := slidelayout.RouteDiagram(rows, edges, obstacles...)
		plan = &diagramPlan{slide: index, rows: rows, edges: edges, obstacles: obstacles, routes: routes,
			connectors: make([]string, len(edges)), fromNames: make([]*string, len(edges)), toNames: make([]*string, len(edges)),
			color: colors.Muted}
		for e, route := range routes {
			plan.fromNames[e], plan.toNames[e] = names[slide.Edges[e].From], names[slide.Edges[e].To]
			steps = append(steps, drawRoute(plan, e, plan.fromNames[e], plan.toNames[e], route))
		}
	}
	return steps, plan
}

// diagramPlan: so do vua ve cua mot slide - giu ten hai dau, ten tung duong noi va duong da chon de tu di lai.
type diagramPlan struct {
	// trace: DebugTrace (RunContext.Trace) - nil khi tat.
	trace      func(string, map[string]any)
	slide      int
	rows       [][]slidelayout.Rect
	edges      []slidelayout.Edge
	obstacles  []slidelayout.Rect
	routes     []slidelayout.Route
	connectors []string
	fromNames  []*string
	toNames    []*string
	color      string
	rerouted   int
}

// connectorParams: tham so wpp.addConnector cua mot Route - dung chung cho lan ve dau va lan di lai.
func connectorParams(slide int, from, to string, route slidelayout.Route, color string) map[string]any {
	params := map[string]any{"slide": slide, "from": from, "to": to, "elbow": route.Elbow,
		"fromSide": route.FromSide, "toSide": route.ToSide, "color": color}
	if route.Offset > 0 {
		params["offset"] = slidelayout.Round1(route.Offset)
	}
	return params
}

// repair: checkLayout bao connector-crossing/connector-overlap tren duong do tool ve thi xoa duong do va ve lai bang
// ung vien khac cua DiagramAlternatives (toi da 2 lan moi duong), roi kiem lai bang checkLayout. PowerPoint la nguon
// dung cuoi cung: mo hinh chi chon ung vien, con ket qua that do checkLayout tra.
func (p *diagramPlan) repair(bridge func(string, map[string]any) (map[string]any, string), issues []any) ([]any, int) {
	attempts := make([]int, len(p.connectors))
	tried := make([][]slidelayout.Route, len(p.connectors))
	for e := range tried {
		if e < len(p.routes) {
			tried[e] = append(tried[e], p.routes[e])
		}
	}
	for {
		e := p.brokenEdge(issues, attempts)
		if e < 0 {
			return issues, p.rerouted
		}
		alternatives := slidelayout.DiagramAlternatives(p.rows, p.edges, p.routes, p.obstacles, e, tried[e]...)
		if p.trace != nil {
			p.trace("reroute", map[string]any{"slide": p.slide, "edge": e, "connector": p.connectors[e], "attempt": attempts[e],
				"current": p.routes[e], "tried": len(tried[e]), "alternatives": alternatives, "issues": issues})
		}
		if len(alternatives) == 0 {
			attempts[e] = 2
			continue
		}
		from, to := p.fromNames[e], p.toNames[e]
		if from == nil || to == nil {
			attempts[e] = 2
			continue
		}
		route := alternatives[0]
		result, problem := bridge("wpp.addConnector", connectorParams(p.slide, *from, *to, route, p.color))
		if problem != "" {
			attempts[e] = 2
			continue
		}
		newName := textOf(result["shape"])
		if newName == "" {
			attempts[e] = 2
			continue
		}
		// Ve duong moi TRUOC khi xoa duong cu: add loi thi van con duong cu, khong mat mui ten.
		if old := p.connectors[e]; old != "" {
			if _, problem := bridge("wpp.deleteShapes", map[string]any{"slide": p.slide, "shapes": []any{old}}); problem != "" {
				issues = append(issues, map[string]any{"type": "connector-reroute-failed",
					"detail": "drew a replacement for connector '" + old + "' but could not remove the old one: " + problem})
				attempts[e] = 2
				continue
			}
		}
		previous := p.routes[e]
		p.connectors[e] = newName
		p.routes[e] = route
		tried[e] = append(tried[e], route)
		p.rerouted++
		attempts[e]++
		next := layoutIssues(bridge, p.slide)
		// Duong moi lam so do TE HON (them loi duong noi): ve lai duong cu. Luot that 26/10/2026 (trace r_22ae8aca):
		// khong so sanh thi 11 lan di lai dua slide tu 3 len 9-12 loi, model phai dung lai slide 9 lan.
		if before, after := connectorIssueCount(issues), connectorIssueCount(next); after > before {
			if restored := p.restore(bridge, e, previous, *from, *to); restored {
				if p.trace != nil {
					p.trace("reroute.reverted", map[string]any{"slide": p.slide, "edge": e, "before": before, "after": after})
				}
				next = layoutIssues(bridge, p.slide)
			}
		}
		issues = next
	}
}

// slidesWithIssues: so slide (theo thu tu dung) con loi bo cuc sau khi dung.
func slidesWithIssues(built []map[string]any) []int {
	broken := []int{}
	for _, entry := range built {
		if issues, _ := entry["issues"].([]any); len(issues) > 0 {
			broken = append(broken, asInt(entry["slide"]))
		}
	}
	return broken
}

// restore: ve lai duong noi e theo tuyen cu roi xoa duong vua thu; loi giua chung thi giu nguyen duong vua thu.
func (p *diagramPlan) restore(bridge func(string, map[string]any) (map[string]any, string), e int, route slidelayout.Route,
	from, to string) bool {
	result, problem := bridge("wpp.addConnector", connectorParams(p.slide, from, to, route, p.color))
	name := textOf(result["shape"])
	if problem != "" || name == "" {
		return false
	}
	if _, problem := bridge("wpp.deleteShapes", map[string]any{"slide": p.slide, "shapes": []any{p.connectors[e]}}); problem != "" {
		_, _ = bridge("wpp.deleteShapes", map[string]any{"slide": p.slide, "shapes": []any{name}})
		return false
	}
	p.connectors[e], p.routes[e] = name, route
	p.rerouted--
	return true
}

// connectorIssueCount: so loi duong noi (connector-*) trong ket qua checkLayout cua mot slide.
func connectorIssueCount(issues []any) int {
	count := 0
	for _, item := range issues {
		if issue, _ := item.(map[string]any); strings.HasPrefix(textOf(issue["type"]), "connector-") {
			count++
		}
	}
	return count
}

// brokenEdge: duong noi dau tien bi checkLayout bao loi va con luot di lai (toi da 2 lan moi duong). Connector-overlap
// tra hai ten - sua duong dau tien con thuoc so do cua tool.
func (p *diagramPlan) brokenEdge(issues []any, attempts []int) int {
	for _, item := range issues {
		issue, _ := item.(map[string]any)
		if !strings.HasPrefix(textOf(issue["type"]), "connector-") {
			continue
		}
		shapes, _ := issue["shapes"].([]any)
		for _, value := range shapes {
			name := textOf(value)
			for e, connector := range p.connectors {
				if connector != "" && connector == name && e < len(attempts) && attempts[e] < 2 {
					return e
				}
			}
		}
	}
	return -1
}

// Nhom mau cua quy trinh / so do (vd buoc cua AI va buoc xac minh cua he thong nghiep vu): moi nhom mot mau theo
// token (primary, muted, accent) va mot dong chu thich o day vung noi dung.

type groupSwatch struct{ Fill, Line string }

// legendHeight: dong chu thich (o mau 14 pt + nhan, cao 24) cong mot lan trong cho duong vong phia tren tang 1 (vong
// cach dinh hop 16 pt) - 34 thi duong vong do chay xuyen qua nhan chu thich (luot that 25/10/2026).
const legendHeight = 50.0

func groupPalette(colors struct{ Primary, Accent, Text, Muted string }) []groupSwatch {
	// Nhom 3 dam hon han (Tint 0,55): Tint(Accent, 0,82) gan nhu trung mau nhom 1 (luot that 25/10/2026: chu thich "Kenh"
	// va "Du lieu" cung mot mau xanh nhat, khong phan biet duoc).
	return []groupSwatch{{slidelayout.Tint(colors.Primary, 0.9), colors.Primary}, {slidelayout.Tint(colors.Muted, 0.85), colors.Muted},
		{slidelayout.Tint(colors.Accent, 0.55), colors.Accent}}
}

// groupsOf: ten nhom theo thu tu xuat hien (bo trong = khong nhom).
func groupsOf(slide briefSlide) []string {
	names := []string{}
	add := func(group string) {
		if group != "" && indexOf(names, group) < 0 {
			names = append(names, group)
		}
	}
	switch slide.Layout {
	case "process":
		for _, step := range slide.Steps {
			add(step.Group)
		}
	case "diagram":
		for _, node := range slide.Nodes {
			add(node.Group)
		}
	}
	// Mot nhom thi khong co gi de phan biet: khong to mau, khong chu thich (luot that 25/10/2026: chu thich chi co "Ops").
	if len(names) < 2 {
		return nil
	}
	return names
}

func indexOf(list []string, value string) int {
	for i, item := range list {
		if item == value {
			return i
		}
	}
	return -1
}

// shapesArea: vung ve hinh cua quy trinh / so do - co nhom thi chua dong chu thich o DAU vung (o day, duong vong duoi
// hang cuoi cua so do se cat qua chu thich).
func shapesArea(area slidelayout.Rect, slide briefSlide) slidelayout.Rect {
	if len(groupsOf(slide)) == 0 {
		return area
	}
	return slidelayout.Rect{area[0], area[1] + legendHeight, area[2], area[3] - legendHeight}
}

func checkGroups(slot int, slide briefSlide) []rowError {
	errors := []rowError{}
	groups := groupsOf(slide)
	if len(groups) > maxGroups {
		errors = append(errors, rowError{Src: slot, Field: "group", Error: "MAX_3_GROUPS", Actual: len(groups),
			Allowed: "2-3 groups, e.g. \"AI\" and \"Business system\""})
	}
	for _, group := range groups {
		if len([]rune(group)) > 24 {
			errors = append(errors, rowError{Src: slot, Field: "group", Error: "GROUP_NAME_TOO_LONG", Max: 24, Actual: len([]rune(group))})
		}
	}
	return errors
}

// legendRects: o mau va nhan cua dong chu thich (cung hinh hoc voi legendSteps) - vat can cho duong noi.
func legendRects(groups []string, content slidelayout.Rect) []slidelayout.Rect {
	rects := []slidelayout.Rect{}
	const square, labelWidth = 14.0, 200.0
	for g := range groups {
		if g >= maxGroups {
			break
		}
		left := content[0] + float64(g)*(square+8+labelWidth+16)
		rects = append(rects, slidelayout.Rect{left, content[1] + 5, square, square},
			slidelayout.Rect{left + square + 8, content[1], labelWidth, 24})
	}
	return rects
}

// legendSteps: o mau + nhan cua tung nhom, mot hang can trai o dau vung noi dung.
func legendSteps(groups []string, palette []groupSwatch, content slidelayout.Rect, swatch func(slidelayout.Rect, groupSwatch) string,
	label func(string, slidelayout.Rect) string) []func() string {
	steps := []func() string{}
	const square, labelWidth = 14.0, 200.0
	y := content[1]
	for g, group := range groups {
		if g >= len(palette) {
			break
		}
		left := content[0] + float64(g)*(square+8+labelWidth+16)
		box := slidelayout.Rect{slidelayout.Round1(left), slidelayout.Round1(y + 5), square, square}
		text := slidelayout.Rect{slidelayout.Round1(left + square + 8), slidelayout.Round1(y), labelWidth, 24}
		group, colors := group, palette[g]
		steps = append(steps, func() string { return swatch(box, colors) }, func() string { return label(group, text) })
	}
	return steps
}

func floatOf(value any, fallback float64) float64 {
	if number, ok := value.(float64); ok && number > 0 {
		return number
	}
	return fallback
}

// checkReplace: replace chi cho slide do CHINH luot nay dung (so doi tuong cua luot, owned.go) - khong thay slide cua
// nguoi dung; moi so slide chi thay mot lan trong mot lan goi.
func checkReplace(slides []briefSlide, run *RunContext, target string) []rowError {
	errors := []rowError{}
	seen := map[int]bool{}
	for i, slide := range slides {
		if slide.Replace == 0 {
			continue
		}
		params := map[string]any{"slide": float64(slide.Replace)}
		if target != "" {
			params["presentation"] = target
		}
		switch {
		case slide.Replace < 0 || seen[slide.Replace]:
			errors = append(errors, rowError{Src: i + 1, Field: "replace", Error: "BAD_REPLACE", Actual: slide.Replace,
				Allowed: "the number of one slide built in this request, each at most once"})
		case !run.ownedObjects().covers("wpp.deleteSlide", params):
			errors = append(errors, rowError{Src: i + 1, Field: "replace", Error: "NOT_BUILT_IN_THIS_REQUEST", Actual: slide.Replace,
				Allowed: "replace only slides this tool built in this request; slides that were already in the deck are not replaced"})
		}
		seen[slide.Replace] = true
	}
	return errors
}

// logIssues: loi bo cuc cua slide vua dung (loai + chi tiet, chi co ten hinh va kich thuoc - khong co chu cua tai lieu)
// vao core.log - core.db chi luu kich thuoc ket qua nen truoc day khong soat lai duoc model da thay loi gi.
func logIssues(runID, source string, slide int, issues []any) {
	for _, item := range issues {
		issue, _ := item.(map[string]any)
		corelog.Info("run %s: %s slide %d: %s - %s", runID, source, slide, textOf(issue["type"]),
			model.Truncate(textOf(issue["detail"]), 200))
	}
}

// briefSpecs: mo ta cua tung slide (JSON, bo replace) - de nhan ra model gui lai dung mot mo ta da dung.
func briefSpecs(value any) []string {
	specs := []string{}
	list, _ := value.([]any)
	for _, item := range list {
		entry, _ := item.(map[string]any)
		copied := map[string]any{}
		for key, field := range entry {
			if key != "replace" {
				copied[key] = field
			}
		}
		raw, _ := json.Marshal(copied)
		specs = append(specs, string(raw))
	}
	return specs
}

// checkSameSpec: cung mot mo ta thi dung ra dung slide cu voi dung loi cu - luot that 25/10/2026: model gui lai slide
// so do gan y het ~17 lan qua replace, lan nao cung cung loi duong noi, toi khi bi chan (42 vong).
func checkSameSpec(specs []string, run *RunContext) []rowError {
	errors := []rowError{}
	for i, spec := range specs {
		if issues, seen := run.briefSpecs[spec]; seen && issues > 0 {
			errors = append(errors, rowError{Src: i + 1, Field: "slide", Error: "SAME_SPEC_SAME_ISSUES", Actual: issues,
				Allowed: "this exact slide was already built in this request and had layout issues - the same spec builds the " +
					"same slide again. Change it (shorter text, fewer nodes or edges, another layer for a node, another layout) " +
					"or keep the slide as it is and tell the user"})
		}
	}
	return errors
}
