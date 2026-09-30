// Package tools: cac tool ma agent goi duoc trong mot luot chay - ban Go cua Tools/*.cs.
package tools

import (
	"context"
	"encoding/json"
	"strings"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
)

// Result: ket qua mot lan chay tool. JSON = nguyen van de gui cho model; Action = ten lenh office (de ghi audit).
// ImageDataURL: anh tool tra kem (QA thi giac) - codec gui cho model duoi dang anh that.
type Result struct {
	JSON         string
	OK           bool
	Action       string
	ImageDataURL string
}

// RunContext: trang thai cua mot luot chay ma tool can (muc 8.3).
type RunContext struct {
	RunID          string
	ConversationID string
	Office         *office.Session
	Bridge         *office.BridgeClient
	Prompt         string
	// Event phat su kien ra SSE cua luot chay (vd skill.loaded).
	Event func(eventType string, data map[string]any)
	// Confirm hoi nguoi dung truoc lenh rui ro (confirm.required -> POST /v1/runs/{id}/confirm).
	// nil = khong co ai de hoi (ai.ask cua agent ben ngoai) -> coi nhu tu choi.
	Confirm func(action, reason, paramsPreview string) (bool, error)
}

func (c *RunContext) ConfirmOrDecline(action, reason, paramsPreview string) (bool, error) {
	if c.Confirm == nil {
		return false, nil
	}
	return c.Confirm(action, reason, paramsPreview)
}

// Tool: mot tool trong luot chay.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result
}

// Registry: tap tool cua mot luot chay.
type Registry struct{ tools []Tool }

func NewRegistry(list []Tool) *Registry { return &Registry{tools: list} }

func (r *Registry) All() []Tool { return r.tools }

func (r *Registry) Find(name string) Tool {
	for _, tool := range r.tools {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}

// ModelTools chuyen sang dinh dang ma model client can.
func (r *Registry) ModelTools() []model.Tool {
	list := make([]model.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		list = append(list, model.Tool{Name: tool.Name(), Description: tool.Description(), Parameters: tool.Parameters()})
	}
	return list
}

// ErrorJSON: {"ok":false,"error":"..."} khong escape ky tu (thong bao doc duoc nguyen van).
func ErrorJSON(message string) string { return model.ErrorJSON(message) }

// OKJSON: {"ok":true,"result":...}; tra nguyen van neu phan result khong marshal duoc.
func OKJSON(result any) string {
	return model.MarshalRelaxed(map[string]any{"ok": true, "result": result})
}

// utf8BOM: ky tu BOM dau file (dinh nghia bang byte de nguon khong lan BOM that).
var utf8BOM = string([]byte{0xEF, 0xBB, 0xBF})

// stringArg doc mot truong chuoi trong tham so model gui.
func stringArg(arguments map[string]any, name string) string {
	value, _ := arguments[name].(string)
	return strings.TrimSpace(value)
}
