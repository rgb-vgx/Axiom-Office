package model

import "testing"

// Proxy 9router (gap that ngay 01/10/2026) tra mot JSON hoan chinh roi noi them `data: [DONE]` cua
// SSE vao cuoi body. Truoc day Parse dung json.Unmarshal ca body nen bao
// `invalid character 'd' after top-level value` va moi luot chay deu hong.
func TestParseToleratesTrailingSSETerminator(t *testing.T) {
	openAIBody := []byte(`{"choices":[{"message":{"role":"assistant","content":"OK"}}],` +
		`"usage":{"prompt_tokens":3,"completion_tokens":1}}` + "data: [DONE]")
	turn, errText := (openAICodec{}).Parse(openAIBody)
	if errText != "" {
		t.Fatalf("codec openai tu choi body co duoi rac: %s", errText)
	}
	if !turn.HasText || turn.Text != "OK" || turn.InputTokens != 3 || turn.OutputTokens != 1 {
		t.Fatalf("turn = %+v", turn)
	}

	anthropicBody := []byte(`{"content":[{"type":"text","text":"OK"}],` +
		`"usage":{"input_tokens":3,"output_tokens":1}}` + "data: [DONE]")
	turn, errText = (anthropicCodec{}).Parse(anthropicBody)
	if errText != "" {
		t.Fatalf("codec anthropic tu choi body co duoi rac: %s", errText)
	}
	if !turn.HasText || turn.Text != "OK" {
		t.Fatalf("turn = %+v", turn)
	}

	names := ParseModels([]byte(`{"data":[{"id":"m1"},{"id":"m2"}]}` + "data: [DONE]"))
	if len(names) != 2 || names[0] != "m1" || names[1] != "m2" {
		t.Fatalf("ParseModels = %v", names)
	}
}

// finish_reason phai duoc giu lai: model suy luan het ngan sach token thi content rong va
// finish_reason="length" - do la manh moi de chan doan "provider returned an empty reply".
func TestParseCarriesFinishReason(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"length"}],"usage":{}}`)
	turn, errText := (openAICodec{}).Parse(body)
	if errText != "" {
		t.Fatalf("loi parse: %s", errText)
	}
	if turn.FinishReason != "length" {
		t.Fatalf("FinishReason = %q, muon \"length\"", turn.FinishReason)
	}

	// Anthropic tra "max_tokens"/"tool_use"/"end_turn"; quy ve cung bo gia tri voi OpenAI de phan
	// xu ly dung chung o tang tren (vi du nhanh bao "het ngan sach token khi suy luan").
	for _, item := range []struct{ stopReason, want string }{
		{"max_tokens", "length"}, {"tool_use", "tool_calls"}, {"end_turn", "stop"}, {"stop_sequence", "stop"},
	} {
		anthropicBody := []byte(`{"content":[{"type":"text","text":"x"}],"stop_reason":"` + item.stopReason + `","usage":{}}`)
		turn, errText = (anthropicCodec{}).Parse(anthropicBody)
		if errText != "" {
			t.Fatalf("loi parse anthropic: %s", errText)
		}
		if turn.FinishReason != item.want {
			t.Fatalf("stop_reason %q -> %q, muon %q", item.stopReason, turn.FinishReason, item.want)
		}
	}
}

// Body hong that thi van phai bao loi - khong duoc nuot loi.
func TestParseStillRejectsGarbage(t *testing.T) {
	if _, errText := (openAICodec{}).Parse([]byte("day khong phai JSON")); errText == "" {
		t.Fatal("body khong phai JSON phai bao loi")
	}
}
