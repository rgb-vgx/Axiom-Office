package memory

import (
	"context"
	_ "embed"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/model"
)

// Prompt cua bo trich xuat memory: nguon duy nhat trong repo, nhung vao binary bang go:embed.
//
//go:embed prompts/extract.txt
var extractPrompt string

const (
	MinConfidence      = 0.6
	MaxFacts           = 5
	MinPromptLength    = 15
	MaxRecentMessages  = 10
	MaxRelatedMemories = 10
)

// SystemPrompt: prompt he thong cua lan trich xuat (nhan dien duoc trong log/kiem thu).
func SystemPrompt() string { return extractPrompt }

// ExtractionInput: dau vao cua mot lan trich xuat sau run (muc 8.5.5). KHONG chua noi dung tai lieu hay ket qua
// getText/readRange - chi tin nhan cua nguoi dung, cau tra loi cuoi va tin nhan gan nhat.
type ExtractionInput struct {
	Prompt      string
	Reply       string
	Recent      []RecentMessage
	Related     []Item
	ObservedOn  time.Time
	DocumentKey string
}

type RecentMessage struct {
	Role    string
	Content string
}

var (
	// Tu chi thao tac tren tai lieu (da chuan hoa khong dau).
	operationVerb = regexp.MustCompile(`^(in dam|in nghieng|gach chan|can (giua|trai|phai|deu)|xoa|chen|them|doi mau|to mau|sua|dinh dang|luu|mo|dong|hoan tac|undo|copy|sao chep|di chuyen|thay|tao|ve|ke|lam|viet|dien|ghi|doc|tom tat|dich|sap xep|loc|tinh|chuyen|doi)\b`)
	// Dau hieu co thong tin dang nho: ve ban than, to chuc, thoi quen, tien do.
	infoMarker = regexp.MustCompile(`\b(toi la|minh la|em la|chung toi|co quan|cong ty|to chuc|don vi|phong ban|chuc vu|truong phong|pho giam doc|giam doc|nguoi ky|ky ten|luon|thuong|thich|uu tien|mac dinh|lan sau|tu nay|nho rang|ghi nho|da xong|con thieu|tien do|han nop|deadline|email|so dien thoai)\b`)
)

// ShouldSkip: bo qua (khong ton lenh goi LLM) khi prompt qua ngan hoac chi la lenh thao tac thuan khong mang
// thong tin ve nguoi dung / to chuc / tien do (vd "in dam dong nay").
func ShouldSkip(prompt string) (bool, string) {
	text := strings.TrimSpace(prompt)
	if len([]rune(text)) < MinPromptLength {
		return true, "prompt too short"
	}
	normalized := Normalize(text)
	if infoMarker.MatchString(normalized) {
		return false, ""
	}
	if operationVerb.MatchString(normalized) && len([]rune(text)) < 160 {
		return true, "pure document operation"
	}
	return false, ""
}

// BuildUserMessage: noi dung gui LLM + bang id tam -> id that (model khong thay id that: chong bia id).
func BuildUserMessage(input ExtractionInput) (string, map[string]string) {
	tempIDs := map[string]string{}
	var builder strings.Builder
	builder.WriteString("Ngày quan sát: " + input.ObservedOn.Format("2006-01-02") + "\n")
	document := "(chưa lưu)"
	if input.DocumentKey != "" {
		document = baseName(input.DocumentKey)
	}
	builder.WriteString("Tài liệu đang mở: " + document + "\n\n")

	builder.WriteString("Ghi nhớ liên quan (id tạm: nội dung):\n")
	index := 0
	for _, item := range input.Related {
		if index >= MaxRelatedMemories {
			break
		}
		temp := strconv.Itoa(index)
		tempIDs[temp] = item.ID
		builder.WriteString(temp + ": " + item.Text + "\n")
		index++
	}
	if index == 0 {
		builder.WriteString("(không có)\n")
	}

	builder.WriteString("\nTin nhắn gần nhất trong hội thoại:\n")
	recent := input.Recent
	if len(recent) > MaxRecentMessages {
		recent = recent[len(recent)-MaxRecentMessages:]
	}
	for _, message := range recent {
		who := "Trợ lý: "
		if message.Role == "user" {
			who = "Người dùng: "
		}
		builder.WriteString(who + model.Truncate(message.Content, 500) + "\n")
	}

	builder.WriteString("\nLượt vừa xong:\nNgười dùng: " + model.Truncate(input.Prompt, 2000) + "\n")
	builder.WriteString("Trợ lý: " + model.Truncate(input.Reply, 1000) + "\n")
	builder.WriteString("\nTrả về JSON {\"facts\": [...]}.")
	return builder.String(), tempIDs
}

// ParseFacts kiem tra dau ra cua LLM (muc 8.5.5): bo fact confidence < 0.6, rong, > 300 ky tu, nhay cam;
// bo moi linkedIds khong nam trong bang id tam (van giu fact); expiresAt phai la YYYY-MM-DD.
func ParseFacts(output string, tempIDs map[string]string, documentKey string) ([]NewMemory, string) {
	// Con tro: phan biet "thieu khoa facts" (loi) voi "facts: []" (khong co gi dang nho).
	var root struct {
		Facts *[]map[string]any `json:"facts"`
	}
	if err := json.Unmarshal([]byte(stripFence(output)), &root); err != nil {
		return nil, "invalid JSON: " + err.Error()
	}
	if root.Facts == nil {
		return nil, "missing 'facts' array"
	}

	facts := []NewMemory{}
	for _, fact := range *root.Facts {
		if len(facts) >= MaxFacts {
			break
		}
		text := strings.TrimSpace(textValue(fact["text"]))
		confidence := numberValue(fact["confidence"])
		if confidence == nil {
			fallback := 0.7
			confidence = &fallback
		}
		if text == "" || len([]rune(text)) > MaxFactLength || *confidence < MinConfidence || IsSensitive(text) {
			continue
		}

		category := textValue(fact["category"])
		scope := ScopeUser
		if textValue(fact["scope"]) == ScopeDocument || (textValue(fact["scope"]) == "" && category == "progress") {
			scope = ScopeDocument
		}
		if scope == ScopeDocument && documentKey == "" {
			scope = ScopeUser
		}

		linked := []string{}
		if raw, ok := fact["linkedIds"].([]any); ok {
			for _, entry := range raw {
				temp := textValue(entry)
				if temp == "" {
					continue
				}
				if real, ok := tempIDs[temp]; ok {
					linked = append(linked, real)
				}
			}
		}

		entities := []string{}
		if raw, ok := fact["entities"].([]any); ok {
			for _, entry := range raw {
				if entity := strings.TrimSpace(textValue(entry)); entity != "" {
					entities = append(entities, entity)
				}
			}
		}

		scopeKey := ""
		if scope == ScopeDocument {
			scopeKey = documentKey
		}
		facts = append(facts, NewMemory{
			Scope: scope, ScopeKey: scopeKey, Text: text, Category: category, Entities: entities,
			LinkedIDs: linked, ExpiresAt: ValidDate(textValue(fact["expiresAt"])), Confidence: confidence,
		})
	}
	return facts, ""
}

// Extract: goi LLM (timeout toi thieu 30s, cho bang het gio request cua model); JSON loi thi thu lai 1 lan.
func Extract(ctx context.Context, client *model.Client, input ExtractionInput) ([]NewMemory, string) {
	user, tempIDs := BuildUserMessage(input)
	lastError := "no reply"
	for attempt := 0; attempt < 2; attempt++ {
		timeout := client.RequestTimeout
		if timeout < 30*time.Second {
			timeout = 30 * time.Second
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		message := user
		if attempt > 0 {
			message += "\n\nCHỈ trả về một đối tượng JSON hợp lệ, không có chữ nào khác."
		}
		text, errText, ok := client.Chat(callCtx, SystemPrompt(), message, 4096)
		cancel()
		if !ok {
			return nil, firstNonEmpty(errText, "no reply")
		}
		facts, problem := ParseFacts(text, tempIDs, input.DocumentKey)
		if problem == "" {
			return facts, ""
		}
		lastError = problem
	}
	return nil, lastError
}

func stripFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		firstLine := strings.Index(trimmed, "\n")
		lastFence := strings.LastIndex(trimmed, "```")
		if firstLine > 0 && lastFence > firstLine {
			trimmed = trimmed[firstLine+1 : lastFence]
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func textValue(value any) string {
	text, _ := value.(string)
	return text
}

func numberValue(value any) *float64 {
	switch number := value.(type) {
	case float64:
		return &number
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64); err == nil {
			return &parsed
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func baseName(path string) string {
	normalized := strings.ReplaceAll(path, "\\", "/")
	if index := strings.LastIndex(normalized, "/"); index >= 0 {
		return normalized[index+1:]
	}
	return normalized
}
