package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return "Add NEW slides at the end of the open presentation from a short spec, without a template file: you choose " +
		"each slide's layout and write the words; the tool places everything on the grid (positions, font sizes, " +
		"colours) and checks the layout. Use it instead of wpp.addText / wpp.addTable for new slides. Layouts: " +
		"cover {title, subtitle}; section {title}; content {title, points, callout}; cards {title, cards: [{title, " +
		"points}] 2-4}; table {title, table: [[header...], [row...]...] up to 7 rows x 5 columns}; figures {title, " +
		"figures: [{value, label}] 2-3 big numbers}; process {title, steps: [{title, text}] 3-5 steps joined by arrows}; " +
		"timeline {title, milestones: [{date, text}] 3-6}; diagram {title, nodes: [{id, text, layer}] up to 10 (layer 1 = " +
		"top row, up to 4 nodes per layer), edges: [{from, to}] arrows between node ids}. Any slide may add notes (speaker notes) and source (one line). " +
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
var briefLayouts = []string{"cover", "section", "content", "cards", "table", "figures", "process", "timeline", "diagram"}

type briefFigure struct{ Value, Label string }

// briefItem: mot buoc (title, text) hoac mot moc (date, text).
type briefItem struct{ Title, Text string }

type briefNode struct {
	ID, Text string
	Layer    int
}

type briefEdge struct{ From, To string }

type briefSlide struct {
	composeRow
	Figures    []briefFigure
	Steps      []briefItem
	Milestones []briefItem
	Nodes      []briefNode
	Edges      []briefEdge
	Source     string
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
			items = append(items, briefItem{Title: strings.TrimSpace(textOf(entry[titleKey])), Text: strings.TrimSpace(textOf(entry[textKey]))})
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

func parseBrief(value any) ([]briefSlide, []rowError) {
	list, _ := value.([]any)
	if len(list) == 0 {
		return nil, []rowError{{Field: "slides", Error: "EMPTY", Allowed: "one or more slides"}}
	}
	rows, errors := parseCompose(value)
	slides := []briefSlide{}
	for i, row := range rows {
		slide := briefSlide{composeRow: row}
		if entry, ok := list[i].(map[string]any); ok {
			slide.Source = strings.TrimSpace(textOf(entry["source"]))
			slide.Steps = briefItems(entry["steps"], "title", "text")
			slide.Milestones = briefItems(entry["milestones"], "date", "text")
			if nodes, ok := entry["nodes"].([]any); ok {
				for _, item := range nodes {
					if node, ok := item.(map[string]any); ok {
						slide.Nodes = append(slide.Nodes, briefNode{ID: strings.TrimSpace(textOf(node["id"])),
							Text: strings.TrimSpace(textOf(node["text"])), Layer: asInt(node["layer"])})
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
		// Dong nguon: hop 24 pt chi cho dung MOT dong (cong thuc do can 24,1 pt cho mot dong 14 pt) - dem ky tu.
		if source := grid.Source(); slide.Source != "" && slidelayout.EstimateLines(slide.Source, source[2], tokens.Slide.Type.Caption) > 1 {
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
				errors = append(errors, rowError{Src: slot, Field: "steps", Error: "NEED_3_TO_5_STEPS", Actual: len(slide.Steps)})
				break
			}
			for s, box := range slidelayout.Process(area, len(slide.Steps)) {
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
				errors = append(errors, rowError{Src: slot, Field: "milestones", Error: "NEED_3_TO_6_MILESTONES", Actual: len(slide.Milestones)})
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
			rows := slidelayout.Diagram(area, layerCounts(layers))
			for l, layer := range layers {
				for n, node := range layer {
					fits(slot, "nodes["+node.ID+"]", node.Text, shapeText(rows[l][n]), body)
				}
			}
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
		return nil, []rowError{{Field: "nodes", Error: "NEED_2_TO_10_NODES", Actual: len(slide.Nodes)}}
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
	if errors := checkBrief(slides, grid, tokens); len(errors) > 0 {
		return Result{JSON: OKJSON(map[string]any{"built": 0, "errors": errors,
			"how": "nothing was built: fix every error (shorten the text named in field) and call again with ALL slides"})}
	}

	built := []map[string]any{}
	issueCount := 0
	for _, slide := range slides {
		index, issues, problem := buildBriefSlide(bridge, grid, tokens, slide)
		if problem != "" {
			return Result{JSON: OKJSON(map[string]any{"built": len(built), "slides": built, "error": problem}), Mutating: len(built) > 0}
		}
		issueCount += len(issues)
		built = append(built, map[string]any{"slide": index, "layout": slide.Layout, "title": slide.Title, "issues": issues})
	}
	return Result{JSON: OKJSON(map[string]any{"built": len(built), "slides": built, "issueCount": issueCount}), OK: true, Mutating: true}
}

// buildBriefSlide: dung mot slide - slide trong (layout 12), tieu de, cac hop cua kieu, dong nguon, ghi chu, roi soat.
func buildBriefSlide(bridge func(string, map[string]any) (map[string]any, string), grid slidelayout.Grid, tokens briefTokens,
	slide briefSlide) (int, []any, string) {
	added, problem := bridge("wpp.addSlide", map[string]any{"layout": 12})
	if problem != "" {
		return 0, nil, problem
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
	switch slide.Layout {
	case "cover":
		title, subtitle := grid.Cover()
		steps = append(steps, func() string {
			return text([]sourcePara{{T: slide.Title, B: true, A: "center"}}, title, tokens.Slide.Type.Hero, colors.Primary,
				map[string]any{"anchor": "middle"})
		})
		if slide.Subtitle != "" {
			steps = append(steps, func() string {
				return text(parasFromText(slide.Subtitle), subtitle, tokens.Slide.Type.Subtitle, colors.Muted, nil)
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
		steps = append(steps, briefBody(bridge, index, grid, tokens, slide, text)...)
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
			return index, nil, problem
		}
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

// briefBody: cac hop noi dung cua content / cards / table / figures (hinh hoc tu slidelayout).
func briefBody(bridge func(string, map[string]any) (map[string]any, string), index int, grid slidelayout.Grid, tokens briefTokens,
	slide briefSlide, text func([]sourcePara, slidelayout.Rect, float64, string, map[string]any) string) []func() string {
	colors := tokens.Colors
	steps := []func() string{}
	switch slide.Layout {
	case "process", "timeline", "diagram":
		return briefShapes(bridge, index, grid, tokens, slide, text)
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
		return steps
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
	return steps
}

// briefShapes: quy trinh, dong thoi gian, so do - ve bang hinh khoi va duong noi goc (wpp.addShape, wpp.addConnector),
// nguoi dung keo sua duoc; duong noi bam theo hinh. Ten hinh chi biet khi da ve, nen buoc noi doc ten qua con tro.
func briefShapes(bridge func(string, map[string]any) (map[string]any, string), index int, grid slidelayout.Grid, tokens briefTokens,
	slide briefSlide, text func([]sourcePara, slidelayout.Rect, float64, string, map[string]any) string) []func() string {
	colors := tokens.Colors
	steps := []func() string{}
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
	connect := func(from, to *string, elbow bool) func() string {
		return func() string {
			_, problem := bridge("wpp.addConnector", map[string]any{"slide": index, "from": *from, "to": *to, "elbow": elbow,
				"color": colors.Muted})
			return problem
		}
	}
	style := func(filled bool) map[string]any {
		values := map[string]any{"line": colors.Primary, "boldFirst": true}
		if filled {
			values["fill"] = slidelayout.Tint(colors.Primary, 0.9)
		}
		return values
	}
	area := grid.Content()
	switch slide.Layout {
	case "process":
		names := make([]string, len(slide.Steps))
		for s, rect := range slidelayout.Process(area, len(slide.Steps)) {
			s, rect, step := s, rect, slide.Steps[s]
			steps = append(steps, func() string {
				var problem string
				names[s], problem = shape(rect, "rounded", stepText(step), style(true))
				return problem
			})
		}
		for s := 1; s < len(slide.Steps); s++ {
			steps = append(steps, connect(&names[s-1], &names[s], false))
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
		layerOf := map[string]int{}
		for l, layer := range layers {
			for n, node := range layer {
				rect, node, name, first := rows[l][n], node, new(string), l == 0
				names[node.ID], layerOf[node.ID] = name, l
				steps = append(steps, func() string {
					var problem string
					*name, problem = shape(rect, "rounded", node.Text, style(first))
					return problem
				})
			}
		}
		for _, edge := range slide.Edges {
			steps = append(steps, connect(names[edge.From], names[edge.To], layerOf[edge.From] != layerOf[edge.To]))
		}
	}
	return steps
}

func floatOf(value any, fallback float64) float64 {
	if number, ok := value.(float64); ok && number > 0 {
		return number
	}
	return fallback
}
