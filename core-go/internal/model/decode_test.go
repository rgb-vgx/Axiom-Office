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

	anthropicBody := []byte(`{"content":[{"type":"text","text":"x"}],"stop_reason":"max_tokens","usage":{}}`)
	turn, errText = (anthropicCodec{}).Parse(anthropicBody)
	if errText != "" {
		t.Fatalf("loi parse anthropic: %s", errText)
	}
	if turn.FinishReason != "max_tokens" {
		t.Fatalf("FinishReason = %q, muon \"max_tokens\"", turn.FinishReason)
	}
}

// Body hong that thi van phai bao loi - khong duoc nuot loi.
func TestParseStillRejectsGarbage(t *testing.T) {
	if _, errText := (openAICodec{}).Parse([]byte("day khong phai JSON")); errText == "" {
		t.Fatal("body khong phai JSON phai bao loi")
	}
}
