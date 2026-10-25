package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
)

// VisualTool: look_at_document - lay anh trang/slide hien tai (bridge app.screenshot) va gui ANH cho model de
// soat bo cuc bang mat (QA thi giac, muc 8.4.6). Chi dang ky khi nguoi dung bat VisualQaEnabled (ton token).
// Trinh chieu: bridge xuat thang slide (Windows: Slide.Export, LibreOffice: filter PNG); ban Windows bao them
// "slide"/"slideCount" de model biet minh dang nhin slide nao - chi co slide dang mo, khong chon duoc slide khac.
type VisualTool struct{}

const (
	VisualToolName = "look_at_document"
	VisualMaxWidth = 1280
)

func NewVisualTool() *VisualTool { return &VisualTool{} }

func (t *VisualTool) Name() string { return VisualToolName }

func (t *VisualTool) Description() string {
	return "Look at an image of the page or slide currently shown in the document window, to check the visual " +
		"layout (text overflowing, overlapping shapes, alignment, contrast) after creating or redesigning slides, " +
		"tables or pages. It shows the current page/slide (the result says which slide); in a presentation pass " +
		"`slide` to look at that slide instead. " +
		"It costs many tokens: call it at most once or twice per task, after the structural checks."
}

func (t *VisualTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"reason":{"type":"string","description":"What you want to check"},` +
		`"slide":{"type":"integer","description":"presentation only: slide number to look at (default: the slide shown)"}}}`)
}

func (t *VisualTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	// Slide theo so (docs/plans/18-s0 P3): wpp.renderSlide xuat dung slide do ma khong doi cua so nguoi dung dang xem;
	// bridge chua co lenh nay (WPS, ban cu) thi quay ve chup slide dang hien.
	var shot office.BridgeResult
	if slide := asInt(arguments["slide"]); slide > 0 && run.Office.App == "wpp" {
		params := map[string]any{"slide": slide, "maxWidth": VisualMaxWidth}
		if strings.TrimSpace(run.TargetDocument) != "" {
			params["presentation"] = run.TargetDocument
		}
		shot = run.Bridge.Command(ctx, run.Office.Port, "wpp.renderSlide", params)
	}
	if !shot.OK {
		shot = run.Bridge.Command(ctx, run.Office.Port, "app.screenshot", map[string]any{"maxWidth": VisualMaxWidth})
	}
	base64, _ := shot.Result["base64"].(string)
	if !shot.OK || base64 == "" {
		return Result{JSON: ErrorJSON("screenshot unavailable: " + firstNonEmptyText(shot.Error, "no image")), Action: VisualToolName}
	}

	mime, _ := shot.Result["mime"].(string)
	if mime == "" {
		mime = "image/png"
	}
	// Anh khong vao audit: tool call chi ghi tham so (khong co base64), anh di kem ket qua cho codec.
	result := map[string]any{
		"width": shot.Result["width"], "height": shot.Result["height"],
		"note": "The image is attached.",
	}
	if slide, ok := shot.Result["slide"]; ok {
		result["slide"] = slide
		result["slideCount"] = shot.Result["slideCount"]
		result["note"] = fmt.Sprintf("The image is attached: slide %v of %v (the slide shown in the window), "+
			"not the whole presentation.", slide, shot.Result["slideCount"])
	}
	summary := map[string]any{"ok": true, "result": result}
	return Result{
		JSON:         model.MarshalRelaxed(summary),
		OK:           true,
		Action:       VisualToolName,
		ImageDataURL: "data:" + mime + ";base64," + base64,
	}
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
