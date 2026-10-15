package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"axiomoffice/core/internal/model"
)

// DocDesignTool: trinh bay lai tai lieu Word theo mau "hien dai, mau nhan nhe" (nguoi dung duyet ban giao an mau
// 14/10/2026): nen chung do bridge tu lam (writer.restyle - khong doi chu), model chi chon VAI cho dang nhan va
// viet khoi bo tron nen nhat (writer.insertBlocks). Model yeu khong ve tung khung: no chi khai bao khoi, tool kiem
// (so doan hop le, khong chong nhau, giu du moi con so cua doan bi thay) roi dung theo mau co dinh.
type DocDesignTool struct{}

func NewDocDesignTool() *DocDesignTool { return &DocDesignTool{} }

func (t *DocDesignTool) Name() string { return "doc_design" }

// maxDesignBlocks: so khoi toi da moi tai lieu - mau duoc duyet: khong phai muc nao cung co khoi.
const maxDesignBlocks = 30

func (t *DocDesignTool) Description() string {
	return "Redesign the open Word document in a clean modern style (headings with a thin accent line, two-level " +
		"bullets, light tables, page numbers) WITHOUT changing its words, then add a few soft rounded boxes for the " +
		"parts worth highlighting. Step 1: call with no arguments - the tool restyles the whole document and returns " +
		"its numbered outline. Step 2: call again with `blocks` (or blocks: [] for none): each block replaces source " +
		"paragraphs (\"replace\": \"12-15\") or is inserted before one (\"before\": 20). Block types: note " +
		"{title, lines, tone}, cards {cards: [{title, lines, tone}], 2-4 side by side}, title {title, lines} (document " +
		"title box), scores {items: [{value, label}]} (2-5 key numbers). Lines starting with \"- \" are bullets, " +
		"**bold** marks key words. Tones: blue (goal/info), amber (warning/note), green (tip/right), red " +
		"(mistake), violet, gray. Keep every fact and number of the paragraphs you replace. Use this instead of " +
		"formatting paragraph by paragraph."
}

func (t *DocDesignTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"blocks":{"type":"array","items":{"type":"object"},"description":"step 2: highlight boxes, e.g. ` +
		`{\"type\":\"note\",\"replace\":\"12-14\",\"tone\":\"blue\",\"title\":\"Mục tiêu\",\"lines\":[\"- ...\"]}"}}}`)
}

var (
	designNumber = regexp.MustCompile(`\d+(?:[.,/]\d+)*`)
	designRange  = regexp.MustCompile(`^\s*(\d+)\s*(?:-\s*(\d+))?\s*$`)
)

var designTones = map[string]string{
	"blue": "blue", "info": "blue", "goal": "blue", "note": "blue",
	"amber": "amber", "warning": "amber", "yellow": "amber", "orange": "amber",
	"green": "green", "tip": "green", "right": "green", "ok": "green",
	"red": "red", "mistake": "red", "wrong": "red", "error": "red",
	"violet": "violet", "purple": "violet", "gray": "gray", "grey": "gray", "title": "title",
}

// outlineItem: mot muc cua writer.outline.
type outlineItem struct {
	N      int
	Kind   string
	Text   string
	List   string
	Rows   int
	Cols   int
	Header []string
}

func parseOutline(result map[string]any) ([]outlineItem, int) {
	var items []outlineItem
	list, _ := result["items"].([]any)
	for _, raw := range list {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		item := outlineItem{N: asInt(entry["n"]), Kind: fmt.Sprint(entry["kind"])}
		if item.Kind == "<nil>" {
			item.Kind = "p"
		}
		item.Text, _ = entry["t"].(string)
		item.List, _ = entry["list"].(string)
		item.Rows, item.Cols = asInt(entry["rows"]), asInt(entry["cols"])
		if header, ok := entry["header"].([]any); ok {
			for _, cell := range header {
				item.Header = append(item.Header, fmt.Sprint(cell))
			}
		}
		items = append(items, item)
	}
	return items, asInt(result["paragraphs"])
}

// outlineView: dan bai gon cho model - "12| chu", bang mot dong.
func outlineView(items []outlineItem) string {
	var view strings.Builder
	for _, item := range items {
		if item.Kind == "table" {
			fmt.Fprintf(&view, "%d| [table %dx%d: %s]\n", item.N, item.Rows, item.Cols, strings.Join(item.Header, " | "))
			continue
		}
		text := item.Text
		if item.List != "" {
			text = item.List + " " + text
		}
		fmt.Fprintf(&view, "%d| %s\n", item.N, text)
	}
	return view.String()
}

func (t *DocDesignTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	bridge := func(action string, params map[string]any) (map[string]any, string) {
		result := run.Bridge.Command(ctx, run.Office.Port, action, params)
		if !result.OK {
			return nil, action + ": " + result.Error
		}
		return result.Result, ""
	}
	failed := func(problem string, mutating bool) Result {
		if AppGone(problem) {
			return Result{JSON: ErrorJSON(problem), Abort: appGoneMessage, Mutating: mutating}
		}
		return Result{JSON: ErrorJSON(problem), Mutating: mutating}
	}
	rawBlocks, hasBlocks := arguments["blocks"]
	if !run.docRestyled || !hasBlocks {
		// Buoc 1 (mot lan moi luot): nen chung roi dan bai. Goi lai buoc 1 sau khi da restyle -> chi tra dan bai.
		restyled := map[string]any{}
		if !run.docRestyled {
			stats, problem := bridge("writer.restyle", map[string]any{})
			if problem != "" {
				return failed(problem, false)
			}
			run.docRestyled = true
			restyled = stats
		}
		outline, problem := bridge("writer.outline", map[string]any{"maxChars": 140})
		if problem != "" {
			return failed(problem, true)
		}
		items, _ := parseOutline(outline)
		note := "The document is restyled (words unchanged). Now pick only the parts worth highlighting - a goal or " +
			"objectives list, a warning or note, a short summary, options side by side - usually 1-3 per page, many " +
			"sections need no box. Call doc_design again with blocks (paragraph numbers below), or blocks: [] if none."
		if hasBlocks {
			note = "Paragraph numbers changed after restyling - plan the blocks again from this outline."
		}
		return Result{JSON: OKJSON(map[string]any{"restyled": restyled, "how": note, "outline": outlineView(items)}), OK: true, Mutating: true}
	}
	list, ok := rawBlocks.([]any)
	if !ok {
		return Result{JSON: ErrorJSON("'blocks' must be an array")}
	}
	if len(list) == 0 {
		return Result{JSON: OKJSON(map[string]any{"done": "restyled, no blocks"}), OK: true, Final: true,
			FinalText: "Đã trình bày lại tài liệu (tiêu đề, gạch đầu dòng, bảng, số trang) - nội dung giữ nguyên."}
	}
	if len(list) > maxDesignBlocks {
		return Result{JSON: ErrorJSON(fmt.Sprintf("%d blocks is too many (max %d): keep only the parts worth highlighting", len(list), maxDesignBlocks))}
	}
	outline, problem := bridge("writer.outline", map[string]any{"maxChars": 100000})
	if problem != "" {
		return failed(problem, false)
	}
	items, total := parseOutline(outline)
	bridgeBlocks, errs := planBlocks(list, items, total)
	if len(errs) > 0 {
		return Result{JSON: model.MarshalRelaxed(map[string]any{"ok": false, "error": "fix these blocks and call again with ALL blocks", "problems": errs})}
	}
	result, problem := bridge("writer.insertBlocks", map[string]any{"blocks": bridgeBlocks})
	if problem != "" {
		return failed(problem, false)
	}
	inserted := asInt(result["inserted"])
	return Result{
		JSON: OKJSON(map[string]any{"inserted": inserted}), OK: true, Mutating: true, Final: true,
		FinalText: fmt.Sprintf("Đã trình bày lại tài liệu: tiêu đề, gạch đầu dòng, bảng, số trang và %d khối nhấn nền nhạt. "+
			"Nội dung giữ nguyên; nếu chưa ưng, bấm Hoàn tác (Ctrl+Z) để bỏ từng bước.", inserted),
	}
}

type blockProblem struct {
	Block int    `json:"block"`
	Error string `json:"error"`
}

// planBlocks: khoi cua model -> khoi cua writer.insertBlocks, kiem so doan, chong nhau va giu so lieu.
func planBlocks(list []any, items []outlineItem, total int) ([]map[string]any, []blockProblem) {
	byN := map[int]outlineItem{}
	for _, item := range items {
		byN[item.N] = item
	}
	var problems []blockProblem
	var out []map[string]any
	type span struct{ from, to, block int }
	var spans []span
	befores := map[int]int{}
	for index, raw := range list {
		number := index + 1
		block, _ := raw.(map[string]any)
		if block == nil {
			problems = append(problems, blockProblem{number, "each block must be an object"})
			continue
		}
		from, to, before := 0, 0, 0
		if text := blockString(block, "replace"); text != "" {
			match := designRange.FindStringSubmatch(text)
			if match == nil {
				problems = append(problems, blockProblem{number, "replace must look like \"12-15\" or \"12\""})
				continue
			}
			from, _ = strconv.Atoi(match[1])
			to = from
			if match[2] != "" {
				to, _ = strconv.Atoi(match[2])
			}
		} else if value := asInt(block["replace"]); value > 0 {
			from, to = value, value
		} else if value := asInt(block["before"]); value > 0 {
			before = value
		} else {
			problems = append(problems, blockProblem{number, "give \"replace\": \"from-to\" or \"before\": paragraph number"})
			continue
		}
		cells, problem := blockCells(block)
		if problem != "" {
			problems = append(problems, blockProblem{number, problem})
			continue
		}
		written := cellsText(cells)
		if from > 0 {
			if to < from || to > total {
				problems = append(problems, blockProblem{number, fmt.Sprintf("replace %d-%d is outside the document (1-%d)", from, to, total)})
				continue
			}
			var replaced []string
			bad := ""
			for n := from; n <= to; n++ {
				item, ok := byN[n]
				if !ok {
					continue
				}
				if item.Kind == "table" {
					bad = fmt.Sprintf("paragraph %d is a table - tables stay as they are; replace only text paragraphs", n)
					break
				}
				if item.Text == "[block or picture]" {
					bad = fmt.Sprintf("paragraph %d is a picture or an existing box - do not replace it", n)
					break
				}
				replaced = append(replaced, item.Text)
			}
			if bad == "" && tableInside(items, from, to) {
				bad = fmt.Sprintf("replace %d-%d covers a table - stop before it", from, to)
			}
			if bad != "" {
				problems = append(problems, blockProblem{number, bad})
				continue
			}
			if missing := missingNumbers(strings.Join(replaced, "\n"), written); len(missing) > 0 {
				problems = append(problems, blockProblem{number, "keep every number of the replaced paragraphs; missing: " + strings.Join(missing, ", ")})
				continue
			}
			source := utf8.RuneCountInString(strings.Join(replaced, " "))
			if source > 80 && utf8.RuneCountInString(written)*10 < source*6 {
				problems = append(problems, blockProblem{number, fmt.Sprintf(
					"the block keeps only %d of %d characters of paragraphs %d-%d - keep their content (shorten wording, not facts) or replace fewer paragraphs",
					utf8.RuneCountInString(written), source, from, to)})
				continue
			}
			spans = append(spans, span{from, to, number})
		} else {
			if before > total+1 {
				problems = append(problems, blockProblem{number, fmt.Sprintf("before %d is outside the document (1-%d)", before, total)})
				continue
			}
			befores[before] = number
		}
		entry := map[string]any{"cells": cells}
		if len(cells) > 1 {
			entry["kind"] = "row"
		} else {
			entry["kind"] = "box"
		}
		if from > 0 {
			entry["replaceFrom"], entry["replaceTo"] = from, to
		} else {
			entry["before"] = before
		}
		out = append(out, entry)
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	for i := 1; i < len(spans); i++ {
		if spans[i].from <= spans[i-1].to {
			problems = append(problems, blockProblem{spans[i].block, fmt.Sprintf("overlaps block %d (paragraphs %d-%d)", spans[i-1].block, spans[i-1].from, spans[i-1].to)})
		}
	}
	for before, number := range befores {
		for _, s := range spans {
			if before > s.from && before <= s.to {
				problems = append(problems, blockProblem{number, fmt.Sprintf("before %d falls inside block %d (paragraphs %d-%d)", before, s.block, s.from, s.to)})
			}
		}
	}
	sort.Slice(problems, func(i, j int) bool { return problems[i].Block < problems[j].Block })
	return out, problems
}

func tableInside(items []outlineItem, from, to int) bool {
	for _, item := range items {
		if item.Kind == "table" && item.N > from && item.N <= to {
			return true
		}
	}
	return false
}

func blockString(block map[string]any, name string) string {
	value, _ := block[name].(string)
	return strings.TrimSpace(value)
}

func blockLines(value any) []string {
	switch typed := value.(type) {
	case string:
		return splitLines(typed)
	case []any:
		var lines []string
		for _, line := range typed {
			if text := strings.TrimSpace(fmt.Sprint(line)); text != "" && line != nil {
				lines = append(lines, text)
			}
		}
		return lines
	}
	return nil
}

func tone(value, fallback string) string {
	if mapped, ok := designTones[strings.ToLower(strings.TrimSpace(value))]; ok {
		return mapped
	}
	return fallback
}

// textLines: dong cua model -> dong cua bridge ("- " = gach dau dong).
func textLines(lines []string) []map[string]any {
	var out []map[string]any
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, marker := range []string{"- ", "• ", "* ", "– ", "+ "} {
			if strings.HasPrefix(trimmed, marker) {
				out = append(out, map[string]any{"k": "bullet", "t": strings.TrimSpace(trimmed[len(marker):])})
				trimmed = ""
				break
			}
		}
		if trimmed != "" {
			out = append(out, map[string]any{"k": "text", "t": trimmed})
		}
	}
	return out
}

func titledCell(title string, lines []string, cellTone string) map[string]any {
	var content []map[string]any
	if title != "" {
		content = append(content, map[string]any{"k": "title", "t": title})
	}
	content = append(content, textLines(lines)...)
	return map[string]any{"tone": cellTone, "lines": content}
}

// blockCells: mot khoi cua model -> cac o (mot o = mot khung bo tron).
func blockCells(block map[string]any) ([]map[string]any, string) {
	kind := strings.ToLower(blockString(block, "type"))
	switch kind {
	case "", "note", "box", "goal", "warning", "tip", "summary":
		lines := blockLines(block["lines"])
		if len(lines) == 0 {
			lines = blockLines(block["text"])
		}
		title := blockString(block, "title")
		if len(lines) == 0 && title == "" {
			return nil, "a note needs lines"
		}
		if len(lines) > 14 {
			return nil, "a note has at most 14 lines - split it or keep the rest as normal text"
		}
		fallback := "blue"
		if kind == "warning" {
			fallback = "amber"
		} else if kind == "tip" {
			fallback = "green"
		}
		return []map[string]any{titledCell(title, lines, tone(blockString(block, "tone"), fallback))}, ""
	case "cards", "row", "pairs", "compare":
		raw, _ := block["cards"].([]any)
		if len(raw) < 2 || len(raw) > 4 {
			return nil, "cards needs 2-4 cards"
		}
		defaults := []string{"blue", "green", "amber", "violet"}
		if len(raw) == 2 && (kind == "pairs" || kind == "compare") {
			defaults = []string{"red", "green"}
		}
		var cells []map[string]any
		for i, item := range raw {
			card, _ := item.(map[string]any)
			if card == nil {
				return nil, "each card must be an object {title, lines}"
			}
			lines := blockLines(card["lines"])
			if len(lines) == 0 {
				lines = blockLines(card["text"])
			}
			if len(lines) > 10 {
				return nil, "a card has at most 10 lines"
			}
			cells = append(cells, titledCell(blockString(card, "title"), lines, tone(blockString(card, "tone"), defaults[i])))
		}
		return cells, ""
	case "title":
		title := blockString(block, "title")
		if title == "" {
			return nil, "a title block needs title"
		}
		content := []map[string]any{{"k": "center", "t": title, "bold": true, "size": 15}}
		for _, line := range blockLines(block["lines"]) {
			content = append(content, map[string]any{"k": "center", "t": line})
		}
		return []map[string]any{{"tone": "title", "lines": content}}, ""
	case "scores", "numbers", "stats":
		raw, _ := block["items"].([]any)
		if len(raw) < 2 || len(raw) > 5 {
			return nil, "scores needs 2-5 items {value, label}"
		}
		var cells []map[string]any
		for _, item := range raw {
			entry, _ := item.(map[string]any)
			if entry == nil {
				return nil, "each item must be {value, label}"
			}
			value := strings.TrimSpace(fmt.Sprint(entry["value"]))
			if entry["value"] == nil || value == "" {
				return nil, "each item needs a value"
			}
			cells = append(cells, map[string]any{"tone": tone(blockString(entry, "tone"), "blue"), "lines": []map[string]any{
				{"k": "center", "t": value, "bold": true, "size": 16},
				{"k": "center", "t": blockString(entry, "label"), "size": 11.5},
			}})
		}
		return cells, ""
	}
	return nil, "unknown block type \"" + kind + "\" (note, cards, title, scores)"
}

func cellsText(cells []map[string]any) string {
	var text strings.Builder
	for _, cell := range cells {
		lines, _ := cell["lines"].([]map[string]any)
		for _, line := range lines {
			text.WriteString(fmt.Sprint(line["t"]))
			text.WriteString("\n")
		}
	}
	return strings.ReplaceAll(text.String(), "**", "")
}

// missingNumbers: con so cua doan bi thay khong co trong khoi (so lieu, moc thoi gian, diem so khong duoc mat).
func missingNumbers(source, written string) []string {
	have := map[string]bool{}
	for _, number := range designNumber.FindAllString(written, -1) {
		have[strings.TrimRight(number, ".,/")] = true
	}
	var missing []string
	seen := map[string]bool{}
	for _, number := range designNumber.FindAllString(source, -1) {
		number = strings.TrimRight(number, ".,/")
		if have[number] || seen[number] {
			continue
		}
		seen[number] = true
		missing = append(missing, number)
	}
	return missing
}
