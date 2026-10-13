package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// modelServer: tra loi theo ten model trong body (reply[model]); ghi lai thu tu model va body da goi.
func modelServer(t *testing.T, reply func(model string, call int, body string) (int, string)) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	models := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		models = append(models, body.Model)
		call := len(models)
		mu.Unlock()
		status, text := reply(body.Model, call, string(raw))
		w.WriteHeader(status)
		_, _ = io.WriteString(w, text)
	}))
	t.Cleanup(server.Close)
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string{}, models...)
	}
}

// Model chinh het han muc free (403) -> chuyen sang model du phong cho phan con lai cua luot, bao cho ben goi.
func TestRunAgentSwitchesToFallbackModel(t *testing.T) {
	server, models := modelServer(t, func(model string, call int, _ string) (int, string) {
		if model == "free" {
			return 403, `{"error":{"message":"FreeTierError: quota exceeded"}}`
		}
		if call == 2 {
			return 200, replyToolCall
		}
		return 200, strings.Replace(replyText, "%s", "Xong.", 1)
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "free")
	client.RetryDelays = nil
	options := DefaultAgentOptions()
	options.Verify = false
	options.FallbackModels = []string{"free", "backup", "third"}
	var switched []string
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{ModelSwitched: func(from, to, reason string) {
			switched = append(switched, from+">"+to+" "+reason)
		}})
	if !result.OK || result.Text != "Xong." || result.Model != "backup" {
		t.Fatalf("phai xong tren model du phong: %+v", result)
	}
	if got := strings.Join(models(), ","); got != "free,backup,backup" {
		t.Fatalf("thu tu model: %s (model chinh chi thu mot lan, sau do giu model du phong)", got)
	}
	if len(switched) != 1 || !strings.Contains(switched[0], "free>backup HTTP 403") {
		t.Fatalf("callback: %v", switched)
	}
}

// Sai khoa (401) la loi cau hinh - model nao cung hong nhu vay, khong chuyen.
func TestRunAgentDoesNotFallBackOnAuthError(t *testing.T) {
	server, models := modelServer(t, func(string, int, string) (int, string) {
		return 401, `{"error":"invalid api key"}`
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "main")
	options := DefaultAgentOptions()
	options.FallbackModels = []string{"backup"}
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult { return ToolResult{} }, options, AgentCallbacks{})
	if result.OK || !strings.HasPrefix(result.Error, "HTTP 401") || len(models()) != 1 {
		t.Fatalf("401 phai dung ngay: %+v, %v", result, models())
	}
}

// Nha cung cap bao vuot cua so ngu canh -> don ket qua tool cu roi goi lai vong do, khong bo ca luot.
func TestRunAgentRecoversFromContextOverflow(t *testing.T) {
	big := strings.Repeat("x", 5000)
	toolCall := func(id string) string {
		return `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"` + id +
			`","function":{"name":"t","arguments":"{\"n\":\"` + id + `\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`
	}
	var retried string
	server, _ := modelServer(t, func(_ string, call int, body string) (int, string) {
		switch call {
		case 1:
			return 200, toolCall("a")
		case 2:
			return 200, toolCall("b")
		case 3:
			return 400, `{"error":{"message":"This model's maximum context length is 32768 tokens","code":"context_length_exceeded"}}`
		}
		retried = body
		return 200, strings.Replace(replyText, "%s", "Đã xong.", 1)
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	options := DefaultAgentOptions()
	options.Verify = false
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: big}
		}, options, AgentCallbacks{})
	if !result.OK || result.Text != "Đã xong." {
		t.Fatalf("phai cuu duoc luot: %+v", result)
	}
	if strings.Count(retried, big) != 1 || !strings.Contains(retried, `\"cleared\":true`) {
		t.Fatalf("lan goi lai phai chi giu ket qua tool gan nhat: %d ban day du", strings.Count(retried, big))
	}
}

func TestIsContextOverflow(t *testing.T) {
	cases := map[string]bool{
		"This model's maximum context length is 128000 tokens":            true,
		`{"type":"invalid_request_error","message":"prompt is too long"}`: true,
		"context_length_exceeded":                                         true,
		"Input exceeds the model's maximum context":                       true,
		"invalid api key":         false,
		"tools are not supported": false,
	}
	for text, want := range cases {
		if got := IsContextOverflow(400, text); got != want {
			t.Errorf("%q: %v", text, got)
		}
	}
	if !IsContextOverflow(413, "") || IsContextOverflow(401, "maximum context length") {
		t.Fatal("413 luon la tran; 401 khong bao gio")
	}
}

func TestFitWindowOnlyLowersThresholds(t *testing.T) {
	base := DefaultAgentOptions()
	small := base
	small.ContextWindow = 32_768
	small = small.FitWindow()
	if small.ContextClearAt != 16_384 || small.MaxResponseTokens != 8192 {
		t.Fatalf("cua so 32k: don o %d, max_tokens %d", small.ContextClearAt, small.MaxResponseTokens)
	}
	huge := base
	huge.ContextWindow = 1_000_000
	huge = huge.FitWindow()
	if huge.ContextClearAt != DefaultContextClearAt || huge.MaxResponseTokens != DefaultMaxResponseTokens {
		t.Fatalf("cua so 1M khong duoc noi nguong: %+v", huge)
	}
	if unknown := base.FitWindow(); unknown.ContextClearAt != DefaultContextClearAt {
		t.Fatal("khong biet cua so thi giu nguyen")
	}
}

// Dang /models cua gateway nguoi dung (13/10/2026) va OpenRouter.
func TestParseModelInfo(t *testing.T) {
	body := `{"data":[
		{"id":"ocg/deepseek-v4.1-flash","capabilities":{"tools":true,"contextWindow":1000000},"context_length":1000000},
		{"id":"chat-only","capabilities":{"tools":false,"contextWindow":32000}},
		{"id":"or/model","top_provider":{"context_length":65536}},
		{"id":"bare"}]}`
	infos := ParseModelInfo([]byte(body))
	if infos["ocg/deepseek-v4.1-flash"].ContextWindow != 1_000_000 || infos["ocg/deepseek-v4.1-flash"].NoTools {
		t.Fatalf("gateway: %+v", infos["ocg/deepseek-v4.1-flash"])
	}
	if infos["chat-only"] != (ModelInfo{ContextWindow: 32000, NoTools: true}) || infos["or/model"].ContextWindow != 65536 ||
		infos["bare"] != (ModelInfo{}) {
		t.Fatalf("%+v", infos)
	}
}

// Cua so nguoi dung khai (LlmContextWindow) thang so /models; /models chi doc mot lan.
func TestClientModelInfoOverrideAndCache(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, `{"data":[{"id":"m","context_length":200000},{"id":"x","capabilities":{"tools":false}}]}`)
	}))
	defer server.Close()
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	if client.ModelInfo(context.Background(), "m").ContextWindow != 200_000 || !client.ModelInfo(context.Background(), "x").NoTools {
		t.Fatal("phai doc tu /models")
	}
	client.ContextWindowOverride = 8192
	if client.ModelInfo(context.Background(), "m").ContextWindow != 8192 || hits != 1 {
		t.Fatalf("override 8192, /models goi %d lan", hits)
	}
}

// Model du phong ma nha cung cap khai khong goi tool thi bo qua.
func TestFallbackSkipsModelsWithoutTools(t *testing.T) {
	server, models := modelServer(t, func(model string, _ int, _ string) (int, string) {
		if model == "main" {
			return 429, `{"error":"rate limited"}`
		}
		return 200, strings.Replace(replyText, "%s", "ok", 1)
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"chat","capabilities":{"tools":false}},{"id":"tooly","capabilities":{"tools":true}}]}`)
	})
	mux.Handle("/", server.Config.Handler)
	wrapped := httptest.NewServer(mux)
	defer wrapped.Close()
	client := NewClient(wrapped.Client(), "openai", wrapped.URL, "", "main")
	client.RetryDelays = nil
	options := DefaultAgentOptions()
	options.FallbackModels = []string{"chat", "tooly"}
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult { return ToolResult{} }, options, AgentCallbacks{})
	if !result.OK || result.Model != "tooly" || strings.Join(models(), ",") != "main,tooly" {
		t.Fatalf("%+v %v", result, models())
	}
}
