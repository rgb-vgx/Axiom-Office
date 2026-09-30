package tools

import (
	"context"
	"encoding/json"

	"axiomoffice/core/internal/model"
)

// VisualTool: look_at_document - chup cua so app (bridge app.screenshot) va gui ANH cho model de soat bo cuc
// bang mat (QA thi giac, muc 8.4.6). Chi dang ky khi nguoi dung bat VisualQaEnabled (ton token).
type VisualTool struct{}

const (
	VisualToolName = "look_at_document"
	VisualMaxWidth = 1280
)

func NewVisualTool() *VisualTool { return &VisualTool{} }

func (t *VisualTool) Name() string { return VisualToolName }

func (t *VisualTool) Description() string {
	return "Take a screenshot of the document window and look at it, to check the visual layout (text overflowing, " +
		"overlapping shapes, alignment, contrast) after creating or redesigning slides, tables or pages. " +
		"It costs many tokens: call it at most once or twice per task, after the structural checks."
}

func (t *VisualTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"reason":{"type":"string","description":"What you want to check"}}}`)
}

func (t *VisualTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	shot := run.Bridge.Command(ctx, run.Office.Port, "app.screenshot", map[string]any{"maxWidth": VisualMaxWidth})
	base64, _ := shot.Result["base64"].(string)
	if !shot.OK || base64 == "" {
		return Result{JSON: ErrorJSON("screenshot unavailable: " + firstNonEmptyText(shot.Error, "no image")), Action: VisualToolName}
	}

	mime, _ := shot.Result["mime"].(string)
	if mime == "" {
		mime = "image/png"
	}
	// Anh khong vao audit: tool call chi ghi tham so (khong co base64), anh di kem ket qua cho codec.
	summary := map[string]any{
		"ok": true,
		"result": map[string]any{
			"width": shot.Result["width"], "height": shot.Result["height"],
			"note": "The screenshot is attached as an image.",
		},
	}
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
