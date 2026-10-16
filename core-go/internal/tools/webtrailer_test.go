package tools

import (
	"strings"
	"testing"
)

func TestParseWebTrailer(t *testing.T) {
	body := "Truy vấn: visa úc\n\nKết quả 1. [VI] Visa nông nghiệp\nhttps://a.example/x\n\nNội dung...\n" +
		`<<<AXIOM_WEB v1 {"kind":"research","query":"visa úc","sources":[` +
		`{"url":"https://a.example/x","title":"A","lang":"vi","mode":"full","score":4,"content":"nội dung A"},` +
		`{"url":"khong-phai-url"},` +
		`{"url":"https://b.example/y","ok":false,"error":"timeout"}]} AXIOM_WEB>>>`
	clean, trailer := ParseWebTrailer(body)
	if trailer == nil {
		t.Fatal("khong tach duoc trailer")
	}
	if strings.Contains(clean, "AXIOM_WEB") {
		t.Fatalf("trailer con sot trong ket qua sach: %q", clean)
	}
	if !strings.HasPrefix(clean, "Truy vấn: visa úc") || !strings.HasSuffix(clean, "Nội dung...") {
		t.Fatalf("phan ket qua bi cat sai: %q", clean)
	}
	if trailer.Kind != "research" || trailer.Query != "visa úc" {
		t.Fatalf("header = %+v", trailer)
	}
	if len(trailer.Sources) != 2 {
		t.Fatalf("sources = %d, want 2 (bo url khong hop le)", len(trailer.Sources))
	}
	first := trailer.Sources[0]
	if first.URL != "https://a.example/x" || !first.OK || first.Score != 4 || first.Content != "nội dung A" {
		t.Fatalf("source[0] = %+v", first)
	}
	if trailer.Sources[1].OK || trailer.Sources[1].Error != "timeout" {
		t.Fatalf("source[1] = %+v", trailer.Sources[1])
	}
}

func TestParseWebTrailerNoTrailer(t *testing.T) {
	body := "Ket qua binh thuong, khong co trailer."
	clean, trailer := ParseWebTrailer(body)
	if trailer != nil || clean != body {
		t.Fatalf("ket qua phai giu nguyen: %q / %+v", clean, trailer)
	}
}

func TestParseWebTrailerBrokenJSON(t *testing.T) {
	body := "abc\n<<<AXIOM_WEB v1 {hong json} AXIOM_WEB>>>"
	clean, trailer := ParseWebTrailer(body)
	if trailer != nil {
		t.Fatal("JSON hong phai coi nhu khong co trailer (giu nguyen ket qua)")
	}
	if clean != body {
		t.Fatalf("ket qua phai giu nguyen: %q", clean)
	}
}

func TestParseWebTrailerCutsHugeContent(t *testing.T) {
	huge := strings.Repeat("x", webTrailerMaxChars+5000)
	body := `<<<AXIOM_WEB v1 {"kind":"fetch","query":"q","sources":[{"url":"https://a.example/big","content":"` + huge + `"}]} AXIOM_WEB>>>`
	clean, trailer := ParseWebTrailer(body)
	if trailer == nil || len(trailer.Sources) != 1 {
		t.Fatal("khong tach duoc trailer")
	}
	if clean != "" {
		t.Fatalf("clean = %q, want rong", clean)
	}
	if got := len(trailer.Sources[0].Content); got != webTrailerMaxChars {
		t.Fatalf("content = %d ky tu, want %d", got, webTrailerMaxChars)
	}
}

func TestRunContextSaveWebSources(t *testing.T) {
	var saved int
	recorder := saverFunc(func(runID, conversationID, kind, query string, docs []WebSourceDoc) int {
		saved = len(docs)
		if runID != "r_1" || conversationID != "c_1" || kind != "search" || query != "q" {
			t.Fatalf("tham so sai: %s %s %s %s", runID, conversationID, kind, query)
		}
		return len(docs)
	})
	run := &RunContext{RunID: "r_1", ConversationID: "c_1", WebSources: recorder}
	run.SaveWebSources("search", "q", []WebSourceDoc{{URL: "https://a.example/1"}})
	if saved != 1 {
		t.Fatalf("saved = %d, want 1", saved)
	}
	// Khong bat luu (nil) -> im lang, khong panic.
	(&RunContext{RunID: "r_1"}).SaveWebSources("search", "q", []WebSourceDoc{{URL: "https://a.example/1"}})
}

type saverFunc func(runID, conversationID, kind, query string, docs []WebSourceDoc) int

func (f saverFunc) SaveWebSources(runID, conversationID, kind, query string, docs []WebSourceDoc) int {
	return f(runID, conversationID, kind, query, docs)
}
