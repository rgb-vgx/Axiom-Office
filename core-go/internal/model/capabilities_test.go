package model

import (
	"os"
	"path/filepath"
	"testing"
)

// Fixture that /models cua cac nha cung cap pho bien: OpenRouter, Anthropic, vLLM va mot model toi thieu.
func readModelFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("doc fixture %s: %v", name, err)
	}
	return body
}

func TestParseModelInfoCapabilities(t *testing.T) {
	cases := []struct {
		file  string
		model string
		want  ModelInfo
	}{
		{
			file:  "models_openrouter.json",
			model: "anthropic/claude-3.7-sonnet",
			want: ModelInfo{
				ContextWindow:        200000,
				MaxOutputTokens:      64000,
				SupportsTools:        true,
				SupportsVision:       true,
				VisionKnown:          true,
				SupportsReasoning:    true,
				InputCostPerMillion:  3,
				OutputCostPerMillion: 15,
			},
		},
		{
			file:  "models_openrouter.json",
			model: "openai/gpt-4o-mini",
			want: ModelInfo{
				ContextWindow:        128000,
				VisionKnown:          true,
				InputCostPerMillion:  0.15,
				OutputCostPerMillion: 0.6,
			},
		},
		{
			file:  "models_anthropic.json",
			model: "claude-sonnet-4-20250514",
			want: ModelInfo{
				ContextWindow:       200000,
				MaxOutputTokens:     64000,
				SupportsTools:       true,
				SupportsVision:      true,
				VisionKnown:         true,
				SupportsReasoning:   true,
				SupportsPromptCache: true,
			},
		},
		{
			file:  "models_vllm.json",
			model: "meta-llama/Llama-3.1-8B-Instruct",
			want:  ModelInfo{ContextWindow: 131072},
		},
		{
			file:  "models_minimal.json",
			model: "bare-model",
			want:  ModelInfo{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.file+"/"+tc.model, func(t *testing.T) {
			infos := ParseModelInfo(readModelFixture(t, tc.file))
			if got := infos[tc.model]; got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestModelInfoHas(t *testing.T) {
	infos := ParseModelInfo(readModelFixture(t, "models_openrouter.json"))
	vision := infos["anthropic/claude-3.7-sonnet"]
	for _, feature := range []string{FeatureTools, FeatureVision, FeatureReasoning} {
		if !vision.Has(feature) {
			t.Errorf("%+v: Has(%q) = false", vision, feature)
		}
	}
	if vision.Has(FeaturePromptCache) || vision.Has("unknown") {
		t.Errorf("%+v: nang luc khong khai phai la false", vision)
	}
	textOnly := infos["openai/gpt-4o-mini"]
	if textOnly.Has(FeatureVision) || textOnly.Has(FeatureTools) {
		t.Errorf("%+v: model chi text khong duoc co vision/tools", textOnly)
	}
}

// Chi khai RO tools=false moi la NoTools; model khong khai gi thi khong suy doan.
func TestParseModelInfoNoToolsOnlyWhenExplicit(t *testing.T) {
	infos := ParseModelInfo([]byte(`{"data":[
		{"id":"chat-only","capabilities":{"tools":false}},
		{"id":"bare"},
		{"id":"tools","supported_parameters":["tools"]}]}`))
	if info := infos["chat-only"]; !info.NoTools || info.SupportsTools || info.Has(FeatureTools) {
		t.Fatalf("tools=false: %+v", info)
	}
	if info := infos["bare"]; info.NoTools || info.SupportsTools || info.Has(FeatureTools) {
		t.Fatalf("khong khai: %+v", info)
	}
	if info := infos["tools"]; !info.SupportsTools || info.NoTools || !info.Has(FeatureTools) {
		t.Fatalf("supported_parameters: %+v", info)
	}
}
