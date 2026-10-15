package agent

import (
	"testing"

	"axiomoffice/core/internal/model"
)

func TestLiveProgressCountsDesignedSlides(t *testing.T) {
	update := model.LiveUpdate{ToolName: "slides_from_template",
		ToolArgs: `{"template":"t.pptx","first": 16, "last": 30, "slides":[{"layout":"section","title":"2. Siêu âm"},{"layout":"content","ti`}
	if got := liveProgress(update); got != "Đang thiết kế phần 16–30: 2 slide…" {
		t.Fatalf("%q", got)
	}
	if got := liveProgress(model.LiveUpdate{Reasoning: "nghĩ"}); got != "Đang suy nghĩ…" {
		t.Fatalf("%q", got)
	}
	if got := liveProgress(model.LiveUpdate{ToolName: "office_action", ToolArgs: "{}"}); got != "" {
		t.Fatalf("lenh ngan khong can dong trang thai: %q", got)
	}
	if got := tailText("abcdef", 3); got != "…def" {
		t.Fatal(got)
	}
}
