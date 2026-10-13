package office

import (
	"strings"
)

// Cat ket qua doc cua trinh chieu (luot 13/10/2026: bai 87 slide, wpp.listSlides + wpp.checkLayout ca bai nam
// nguyen trong ngu canh va duoc gui lai o ca 30 vong -> 1.271.221 token dau vao). Cung nguyen tac voi
// et.readRange/et.checkRange: cat CHI TIET, giu DU SO (slideCount, issueCount), khong cat im lang.
// Slide dang mo (FocusSlide) va hai slide ke no giu nguyen - do la cho nguoi dung muon sua.

const (
	// SlideTextBudget: tong ky tu chu cua wpp.listSlides truoc khi rut gon (~2.500 token).
	SlideTextBudget = 8000
	// slideTextFloor: moi slide giu toi thieu chung nay ky tu khi rut gon - du nhan ra slide noi ve gi.
	slideTextFloor = 60
	// LayoutDetailSlides: so slide co loi duoc giu toa do khung (ngoai slide dang mo).
	LayoutDetailSlides = 5
)

func (t Truncation) focused(index int) bool {
	return t.FocusSlide > 0 && index >= t.FocusSlide-1 && index <= t.FocusSlide+1
}

func (t Truncation) listSlides(result map[string]any) bool {
	slides, ok := result["slides"].([]any)
	if !ok {
		return false
	}
	total := 0
	for _, item := range slides {
		for _, text := range slideTexts(item) {
			total += len([]rune(text))
		}
	}
	if total <= SlideTextBudget {
		return false
	}
	perSlide := max(slideTextFloor, SlideTextBudget/max(1, len(slides)))
	for _, item := range slides {
		slide, ok := item.(map[string]any)
		if !ok || t.focused(intValue(slide["index"])) {
			continue
		}
		texts := slideTexts(item)
		if len(texts) == 0 {
			continue
		}
		// Gop cac khung chu thanh mot dong roi cat: tieu de (khung dau) luon con.
		joined := strings.Join(strings.Fields(strings.Join(texts, " | ")), " ")
		runes := []rune(joined)
		if len(runes) > perSlide {
			joined = string(runes[:perSlide]) + "…"
		}
		delete(slide, "shapeTexts")
		slide["text"] = joined
	}
	result["truncated"] = true
	result["note"] = "Slide texts are shortened to fit (the slide being viewed and its neighbours are kept in full). " +
		"Ask the user or use the slide shown in the window for full detail."
	return true
}

func (t Truncation) checkLayout(result map[string]any) bool {
	slides, ok := result["slides"].([]any)
	if !ok || len(slides) <= 3 {
		return false
	}
	kept := []any{}
	clean, detailed, issuesKept, issuesOmitted := 0, 0, 0, 0
	for _, item := range slides {
		slide, ok := item.(map[string]any)
		if !ok {
			continue
		}
		issues, _ := slide["issues"].([]any)
		index := intValue(slide["index"])
		if len(issues) == 0 && index != t.FocusSlide {
			clean++
			continue
		}
		if room := t.IssuesCap - issuesKept; len(issues) > room {
			issuesOmitted += len(issues) - max(0, room)
			issues = issues[:max(0, room)]
			slide["issues"] = issues
		}
		issuesKept += len(issues)
		if index == t.FocusSlide || (detailed < LayoutDetailSlides && len(issues) > 0) {
			if index != t.FocusSlide {
				detailed++
			}
		} else {
			delete(slide, "shapes")
		}
		kept = append(kept, slide)
	}
	result["slides"] = kept
	result["truncated"] = true
	result["slidesChecked"] = len(slides)
	result["slidesWithoutIssues"] = clean
	if issuesOmitted > 0 {
		result["issuesOmitted"] = issuesOmitted
	}
	result["note"] = "Slides without issues are omitted; shape boxes are kept only for a few slides. " +
		"Call wpp.checkLayout with {\"slide\": N} for one slide's full detail."
	return true
}

func slideTexts(item any) []string {
	slide, ok := item.(map[string]any)
	if !ok {
		return nil
	}
	raw, _ := slide["shapeTexts"].([]any)
	texts := make([]string, 0, len(raw))
	for _, value := range raw {
		if text, ok := value.(string); ok {
			texts = append(texts, text)
		}
	}
	return texts
}

func intValue(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	}
	return 0
}

// sourceManifest: wpp.sourceManifest ca bai (95 slide) tra moi chu cua bai nguon - luot that 13/10/2026 model goi
// bon lan, moi lan mot khoi lon nam lai trong lich su den het luot. Vuot SlideTextBudget thi moi doan chu cat con
// toi da 160 ky tu (giu du so slide, tieu de va danh sach anh); slide dang mo giu nguyen.
func (t Truncation) sourceManifest(result map[string]any) bool {
	slides, _ := result["slides"].([]any)
	total := 0
	for _, item := range slides {
		if slide, ok := item.(map[string]any); ok {
			if title, ok := slide["title"].(string); ok {
				total += len([]rune(title))
			}
			for _, text := range anyStrings(slide["texts"]) {
				total += len([]rune(text))
			}
		}
	}
	if total <= SlideTextBudget {
		return false
	}
	const keep = 160
	for _, item := range slides {
		slide, ok := item.(map[string]any)
		if !ok || t.focused(intValue(slide["n"])) {
			continue
		}
		texts := anyStrings(slide["texts"])
		shortened := make([]any, 0, len(texts))
		for _, text := range texts {
			if runes := []rune(text); len(runes) > keep {
				text = string(runes[:keep]) + "…"
			}
			shortened = append(shortened, text)
		}
		slide["texts"] = shortened
	}
	result["note"] = "Texts longer than 160 characters are shortened. slides_from_template reads the full source itself."
	return true
}

func anyStrings(value any) []string {
	list, _ := value.([]any)
	texts := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(string); ok {
			texts = append(texts, text)
		}
	}
	return texts
}
