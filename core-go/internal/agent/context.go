package agent

import (
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/store"
)

// AssembledContext: ngu canh dua vao prompt - tom tat (neu co) + cac luot user/assistant gan nhat trong
// ngan sach token, va danh sach hanh dong da chay (New_arch.md muc 8.5.9).
type AssembledContext struct {
	Summary           string
	PriorTurns        []model.ConversationTurn
	RecentActionLines []string
	EstimatedTokens   int
}

type ContextAssembler struct{ BudgetTokens int }

func NewContextAssembler() *ContextAssembler { return &ContextAssembler{BudgetTokens: 8000} }

// EstimateTokens: uoc luong ky tu/4 (muc 8.5.9) - du dung cho tieng Viet va tieng Anh.
func EstimateTokens(text string) int {
	return (len([]rune(text)) + 3) / 4
}

func (a *ContextAssembler) Assemble(conversation *store.ConversationRow, messages []store.MessageRow, maxActionLines int) AssembledContext {
	turns := []model.ConversationTurn{}
	actions := []string{}
	tokens := 0
	overBudget := false

	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role == "tool_summary" {
			if len(actions) < maxActionLines {
				actions = append(actions, message.Content)
			}
			continue
		}
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}

		cost := EstimateTokens(message.Content) + 4
		if !overBudget && tokens+cost > a.BudgetTokens {
			// Het ngan sach: phan con lai chi con trong ban tom tat.
			overBudget = true
		}
		if overBudget {
			continue
		}
		tokens += cost
		turns = append(turns, model.ConversationTurn{Role: message.Role, Content: message.Content})
	}

	reverse(turns)
	// Giao thuc Anthropic yeu cau message dau tien la user: bo luot assistant mo dau neu bi cat le.
	if len(turns) > 0 && turns[0].Role != "user" {
		turns = turns[1:]
	}
	reverseStrings(actions)

	summary := ""
	if conversation != nil && conversation.Summary != nil {
		summary = *conversation.Summary
	}
	return AssembledContext{Summary: summary, PriorTurns: turns, RecentActionLines: actions, EstimatedTokens: tokens}
}

func reverse(turns []model.ConversationTurn) {
	for left, right := 0, len(turns)-1; left < right; left, right = left+1, right-1 {
		turns[left], turns[right] = turns[right], turns[left]
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
