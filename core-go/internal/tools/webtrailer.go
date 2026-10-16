package tools

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Trailer AXIOM_WEB: tool chrome-search (mcp/web_search) gan cuoi ket qua mot dong JSON de Agent Core
// tach rieng phan nguon (luu kho core.db) TRUOC khi ket qua vao ngu canh model. Model khong bao gio
// thay trailer; JSON hong/khong co thi ket qua giu nguyen (huong sai an toan).
const (
	webTrailerOpen      = "<<<AXIOM_WEB v1 "
	webTrailerClose     = " AXIOM_WEB>>>"
	webTrailerMaxItems  = 50
	webTrailerMaxChars  = 64000
	webTrailerMaxTitle  = 500
	webTrailerMaxErrMsg = 300
)

// WebSourceDoc: mot nguon web trong trailer (va la don vi de luu kho).
type WebSourceDoc struct {
	URL     string
	Title   string
	Lang    string // "vi" | "en" | ""
	Mode    string // search | lead | full | fetch | fail
	Content string // markdown da doc (co the rong khi chi co snippet)
	Snippet string // doan trich khi chua doc noi dung
	Score   int
	OK      bool
	Error   string
}

// WebTrailer: thong tin mot ket qua tool web kem theo danh sach nguon.
type WebTrailer struct {
	Kind    string // search | fetch | research
	Query   string
	Sources []WebSourceDoc
}

// ParseWebTrailer: tach dong trailer o CUOI ket qua tool. Khong co trailer -> tra nguyen van + nil.
// Co -> tra phan chu da cat trailer + thong tin nguon (de ghi kho va noteWebSource).
func ParseWebTrailer(text string) (string, *WebTrailer) {
	start := strings.LastIndex(text, webTrailerOpen)
	if start < 0 {
		return text, nil
	}
	body := text[start+len(webTrailerOpen):]
	end := strings.Index(body, webTrailerClose)
	if end < 0 {
		return text, nil
	}
	var payload struct {
		Kind    string `json:"kind"`
		Query   string `json:"query"`
		Sources []struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Lang    string `json:"lang"`
			Mode    string `json:"mode"`
			Score   int    `json:"score"`
			Content string `json:"content"`
			Snippet string `json:"snippet"`
			OK      *bool  `json:"ok"`
			Error   string `json:"error"`
		} `json:"sources"`
	}
	if err := json.Unmarshal([]byte(body[:end]), &payload); err != nil {
		return text, nil
	}
	clean := text[:start] + body[end+len(webTrailerClose):]
	clean = strings.TrimRight(clean, " \r\n\t")

	trailer := &WebTrailer{Kind: strings.TrimSpace(payload.Kind), Query: cutString(payload.Query, 500)}
	for _, raw := range payload.Sources {
		if len(trailer.Sources) >= webTrailerMaxItems {
			break
		}
		url := strings.TrimSpace(raw.URL)
		if !strings.HasPrefix(strings.ToLower(url), "http") {
			continue
		}
		doc := WebSourceDoc{
			URL:     url,
			Title:   cutString(strings.TrimSpace(raw.Title), webTrailerMaxTitle),
			Lang:    cutString(strings.TrimSpace(raw.Lang), 16),
			Mode:    cutString(strings.TrimSpace(raw.Mode), 16),
			Score:   raw.Score,
			Content: cutChars(raw.Content, webTrailerMaxChars),
			Snippet: cutChars(raw.Snippet, webSourceSnippetChars),
			OK:      raw.OK == nil || *raw.OK,
			Error:   cutString(strings.TrimSpace(raw.Error), webTrailerMaxErrMsg),
		}
		trailer.Sources = append(trailer.Sources, doc)
	}
	return clean, trailer
}

// cutString: cat chuoi theo so ky tu (rune), giu UTF-8 hop le.
func cutString(value string, max int) string {
	if value == "" || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

// cutChars: nhu cutString nhung tran theo so BYTE truoc de khoi copy chuoi lon vo ich.
func cutChars(value string, max int) string {
	if len(value) <= max {
		return value
	}
	cut := value[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
