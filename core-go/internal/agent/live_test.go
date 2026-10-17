package agent

import (
	"testing"

	"axiomoffice/core/internal/model"
)

func TestLiveProgressCountsDesignedSlides(t *testing.T) {
	update := model.LiveUpdate{ToolName: "slides_from_template",
		ToolArgs: `{"template":"t.pptx","first": 16, "last": 30, "slides":[{"layout":"section","title":"2. Siêu âm"},{"layout":"content","ti`}
	if got := liveProgress(update, "vi"); got != "Đang thiết kế phần 16–30: 2 slide…" {
		t.Fatalf("%q", got)
	}
	if got := liveProgress(model.LiveUpdate{Reasoning: "nghĩ"}, ""); got != "Đang suy nghĩ…" {
		t.Fatalf("%q", got)
	}
	if got := liveProgress(model.LiveUpdate{ToolName: "office_action", ToolArgs: "{}"}, "vi"); got != "" {
		t.Fatalf("lenh ngan khong can dong trang thai: %q", got)
	}
	if got := tailText("abcdef", 3); got != "…def" {
		t.Fatal(got)
	}
}

// P7-03: cau trang thai cua Core doi theo ngon ngu gui kem; thieu truong thi tieng Viet nhu truoc.
func TestLiveProgressUsesRequestLanguage(t *testing.T) {
	thinking := model.LiveUpdate{Reasoning: "hmm"}
	if got := liveProgress(thinking, "en"); got != "Thinking…" {
		t.Fatalf("en = %q", got)
	}
	if got := liveProgress(thinking, "en-US"); got != "Thinking…" {
		t.Fatalf("en-US = %q", got)
	}
	if got := liveProgress(thinking, "vi"); got != "Đang suy nghĩ…" {
		t.Fatalf("vi = %q", got)
	}
	writing := model.LiveUpdate{Text: "câu trả lời"}
	if got := liveProgress(writing, "en"); got != "Writing the answer…" {
		t.Fatalf("writing en = %q", got)
	}
}

// P7-03: body `language` khong lam vo request cu (thieu truong -> "vi").
func TestParseRequestLanguage(t *testing.T) {
	base := map[string]any{"prompt": "xin chào", "office": map[string]any{"port": float64(1)}}
	request, problem := ParseRequest(base)
	if problem != "" {
		t.Fatalf("request cu bi tu choi: %q", problem)
	}
	if request.Language != "vi" {
		t.Fatalf("thieu truong -> %q", request.Language)
	}
	base["language"] = "en"
	request, problem = ParseRequest(base)
	if problem != "" || request.Language != "en" {
		t.Fatalf("language=en -> %q (%q)", request.Language, problem)
	}
}
