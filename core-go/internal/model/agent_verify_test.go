package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// replyToolCall / replyText: hai hinh dang tra loi cua nha cung cap (khong stream - ParseStream tu roi
// ve Parse khi body khong phai SSE).
const (
	replyToolCall = `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","function":{"name":"t","arguments":"{}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`
	replyText     = `{"choices":[{"message":{"role":"assistant","content":"%s"}}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`
)

// verifyServer: tra loi theo kich ban, ghi lai moi body nhan duoc.
func verifyServer(t *testing.T, replies []string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	bodies := []string{}
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		index := step
		step++
		mu.Unlock()
		if index >= len(replies) {
			index = len(replies) - 1
		}
		_, _ = io.WriteString(w, replies[index])
	}))
	t.Cleanup(server.Close)
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string{}, bodies...)
	}
}

func mutatingTools() []Tool {
	return []Tool{{Name: "t", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}}
}

func runOn(server *httptest.Server, mutating bool, options AgentOptions) AgentResult {
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	return client.RunAgent(context.Background(), "sys", nil, "lam di", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: `{"ok":true}`, OK: true, Mutating: mutating}
		}, options, AgentCallbacks{})
}

// Luot co SUA tai lieu thi phai doc lai truoc khi tra loi: cau tra loi dau tien chua duoc coi la cuoi
// cung, va mot luot nua duoc chay voi loi nhac kiem chung.
func TestRunAgentVerifiesAfterMutatingTool(t *testing.T) {
	server, bodies := verifyServer(t, []string{
		replyToolCall,
		`{"choices":[{"message":{"role":"assistant","content":"da xong"}}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"da kiem tra, dung"}}]}`,
	})
	result := runOn(server, true, DefaultAgentOptions())

	if !result.OK || result.Text != "da kiem tra, dung" {
		t.Fatalf("phai tra loi cau SAU khi kiem chung: %+v", result)
	}
	if !result.Verified || result.Rounds != 3 {
		t.Fatalf("phai co dung mot vong kiem chung: verified=%t rounds=%d", result.Verified, result.Rounds)
	}
	sent := bodies()
	if len(sent) != 3 {
		t.Fatalf("phai goi model 3 lan: %d", len(sent))
	}
	if !strings.Contains(sent[2], "check the work you just did") {
		t.Fatalf("lan goi thu 3 phai mang loi nhac kiem chung")
	}
	// Cau tra loi nhap phai duoc gui lai lam tin assistant, khong thi model khong biet dang kiem gi.
	if !strings.Contains(sent[2], "da xong") {
		t.Fatalf("ban nhap phai nam trong hoi thoai gui lai: %s", sent[2])
	}
}

// Luot CHI DOC thi khong ton them vong nao.
func TestRunAgentSkipsVerifyForReadOnlyTool(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall, `{"choices":[{"message":{"role":"assistant","content":"xong"}}]}`})
	result := runOn(server, false, DefaultAgentOptions())

	if !result.OK || result.Text != "xong" || result.Verified || result.Rounds != 2 {
		t.Fatalf("luot chi doc khong duoc kiem chung: %+v", result)
	}
	if len(bodies()) != 2 {
		t.Fatalf("phai goi model dung 2 lan: %d", len(bodies()))
	}
}

// Kiem chung chi chay MOT lan: tung thay mot luot sua doi hoi doc lai mai mai thi luot chay khong bao
// gio ket thuc.
func TestRunAgentVerifiesOnlyOnce(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall, replyToolCall, replyToolCall,
		`{"choices":[{"message":{"role":"assistant","content":"ban nhap"}}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"ket qua cuoi"}}]}`})
	result := runOn(server, true, DefaultAgentOptions())

	// 3 vong tool, 1 vong tra loi nhap, 1 vong tra loi sau kiem chung = 5. Neu kiem chung lai bat lai
	// sau moi cau tra loi thi luot chay khong bao gio ket thuc.
	if !result.OK || !result.Verified || result.Rounds != 5 || result.Text != "ket qua cuoi" {
		t.Fatalf("chi duoc kiem chung mot lan: %+v", result)
	}
}

// Nguoi dung tat kiem chung thi luot chay dung ngay o cau tra loi dau tien.
func TestRunAgentVerifyCanBeDisabled(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall, `{"choices":[{"message":{"role":"assistant","content":"xong"}}]}`})
	options := DefaultAgentOptions()
	options.Verify = false
	result := runOn(server, true, options)

	if !result.OK || result.Verified || result.Rounds != 2 || len(bodies()) != 2 {
		t.Fatalf("tat kiem chung ma van kiem: %+v", result)
	}
}

// Suy luan cua model duoc day ra ngoai (pane hien cho nguoi dung), tach khoi cau tra loi.
func TestRunAgentReportsReasoning(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"nghi buoc mot\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\",\"delta\":{}}]}\n\n" +
		"data: [DONE]\n\n"
	server, _ := verifyServer(t, []string{body})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	type seen struct {
		round int
		text  string
	}
	got := []seen{}
	result := client.RunAgent(context.Background(), "s", nil, "u", nil, nil, DefaultAgentOptions(),
		AgentCallbacks{Reasoning: func(round int, text string) { got = append(got, seen{round, text}) }})

	if !result.OK || result.Text != "OK" {
		t.Fatalf("result = %+v", result)
	}
	if len(got) != 1 || got[0].round != 1 || got[0].text != "nghi buoc mot" {
		t.Fatalf("suy luan phai duoc day ra ngoai: %+v", got)
	}
}

// So token doc tu cache: ba nha cung cap ba kieu ten khac nhau, va khong co thi la 0 (khong phai loi).
func TestCachedTokensAreRead(t *testing.T) {
	deepseek := `{"choices":[{"message":{"content":"x"}}],"usage":{"prompt_tokens":100,"completion_tokens":5,"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20}}`
	if turn, errText := (openAICodec{}).Parse([]byte(deepseek)); errText != "" || turn.CachedTokens != 80 {
		t.Fatalf("deepseek cache hit: %+v (%s)", turn, errText)
	}
	openai := `{"choices":[{"message":{"content":"x"}}],"usage":{"prompt_tokens":100,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":64}}}`
	if turn, _ := (openAICodec{}).Parse([]byte(openai)); turn.CachedTokens != 64 {
		t.Fatalf("openai prompt_tokens_details: %+v", turn)
	}
	none := `{"choices":[{"message":{"content":"x"}}],"usage":{"prompt_tokens":100,"completion_tokens":5}}`
	if turn, _ := (openAICodec{}).Parse([]byte(none)); turn.CachedTokens != 0 {
		t.Fatalf("khong bao cache thi phai la 0: %+v", turn)
	}

	anthropic := `{"content":[{"type":"text","text":"x"}],"usage":{"input_tokens":100,"output_tokens":5,"cache_read_input_tokens":72}}`
	if turn, _ := (anthropicCodec{}).Parse([]byte(anthropic)); turn.CachedTokens != 72 {
		t.Fatalf("anthropic cache_read_input_tokens: %+v", turn)
	}

	// Duong stream cung phai doc duoc: usage den o khung cuoi cung.
	stream := "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":5,\"prompt_cache_hit_tokens\":80}}\n\ndata: [DONE]\n\n"
	if turn, _ := (openAICodec{}).ParseStream([]byte(stream)); turn.CachedTokens != 80 {
		t.Fatalf("stream cache: %+v", turn)
	}
	anthropicStream := "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":100,\"cache_read_input_tokens\":72}}}\n\n"
	if turn, _ := (anthropicCodec{}).ParseStream([]byte(anthropicStream)); turn.CachedTokens != 72 {
		t.Fatalf("anthropic stream cache: %+v", turn)
	}
}

// Mot phan hoi bi cat ngang vi het ngan sach token cua CHINH no (finish_reason=length, content rong)
// khong duoc huy ca luot chay: hai luot chay that ngay 02/10/2026 chet han o day sau khi da lam duoc
// nhieu. Nhac viet nho lai roi lam tiep.
func TestRunAgentRecoversFromTruncatedReply(t *testing.T) {
	truncated := `{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":""}}],"usage":{"prompt_tokens":5,"completion_tokens":9}}`
	server, bodies := verifyServer(t, []string{truncated, truncated,
		`{"choices":[{"message":{"role":"assistant","content":"xong"}}]}`})
	result := runOn(server, false, DefaultAgentOptions())

	if !result.OK || result.Text != "xong" {
		t.Fatalf("phai cuu duoc luot chay: %+v", result)
	}
	if result.Rounds != 3 {
		t.Fatalf("hai loi nhac khac nhau roi moi tra loi: rounds=%d", result.Rounds)
	}
	sent := bodies()
	// Lan nhac thu hai phai noi ve VIEC BI CAT, khac lan nhac "tra loi rong" chung chung.
	if !strings.Contains(sent[1], "empty") {
		t.Fatalf("vong 2 phai la loi nhac tra loi rong: %s", sent[1])
	}
	if !strings.Contains(sent[2], "cut off by the response token limit") {
		t.Fatalf("vong 3 phai la loi nhac viet nho lai: %s", sent[2])
	}

	// Cat ngang mai thi van phai dung lai, khong nhac vo han.
	forever, _ := verifyServer(t, []string{truncated})
	failed := runOn(forever, false, DefaultAgentOptions())
	if failed.OK || !strings.Contains(failed.Error, "finish_reason=length") {
		t.Fatalf("cat mai thi phai bao loi ro ly do: %+v", failed)
	}
	if failed.Rounds != 3 {
		t.Fatalf("chi nhac hai lan: rounds=%d", failed.Rounds)
	}
}

// Lap lai Y HET mot tool call da hong thi dung han. Do ngay 02/10/2026: mot luot chay goi cung mot lenh
// cung mot loi 90 lan va dot het ngan sach token vao do - model khong tu biet dung.
func TestRunAgentStopsOnRepeatedIdenticalFailure(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: false,
				ResultJSON: `{"ok":false,"error":"no active spreadsheet"}`}
		}, DefaultAgentOptions(), AgentCallbacks{})

	if result.OK || !strings.Contains(result.Error, "repeated the same failing call 3 times") {
		t.Fatalf("phai dung han khi lap lai cung mot loi: %+v", result)
	}
	if result.Rounds != 3 {
		t.Fatalf("dung ngay o vong lap thu 3: rounds=%d", result.Rounds)
	}
}

// Loi DOI moi vong thi khong phai la lap: model co the dang thu cach khac, khong duoc cat ngang.
func TestRunAgentKeepsGoingWhenFailureChanges(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	attempt := 0
	options := DefaultAgentOptions()
	options.MaxRounds = 6
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			attempt++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: false,
				ResultJSON: fmt.Sprintf(`{"ok":false,"error":"lan thu %d"}`, attempt)}
		}, options, AgentCallbacks{})

	if strings.Contains(result.Error, "repeated the same failing call") {
		t.Fatalf("loi doi moi vong khong duoc coi la lap: %+v", result)
	}
	if attempt != 6 {
		t.Fatalf("phai chay het 6 vong: %d", attempt)
	}
}

// Ngan sach token chi tinh phan phai tra that: token doc tu cache gan nhu mien phi. Do ngay 02/10/2026,
// mot luot chay dung 1.005.301 token trong do 806.400 doc tu cache (97% trung) ma van bi dung vi "het
// ngan sach" - tuc la ngan sach bi an mat phan lon.
func TestBillableTokensIgnoresCache(t *testing.T) {
	cases := []struct {
		result AgentResult
		want   int
	}{
		// Khong cache: y nguyen nhu cu.
		{AgentResult{InputTokens: 1000, OutputTokens: 200}, 1200},
		// Phan lon doc tu cache.
		{AgentResult{InputTokens: 1_005_301 - 170_823, OutputTokens: 170_823, CachedTokens: 806_400}, 198_901},
		// Nha cung cap bao cache nhieu hon ca input (vo ly) thi khong duoc am.
		{AgentResult{InputTokens: 100, OutputTokens: 50, CachedTokens: 500}, 150},
	}
	for _, item := range cases {
		if got := item.result.BillableTokens(); got != item.want {
			t.Errorf("BillableTokens(%+v) = %d, want %d", item.result, got, item.want)
		}
	}
}

// Cache lam ngan sach dung lau hon: cung mot luot, khong cache thi dung, co cache thi chay tiep.
func TestRunAgentBudgetCountsOnlyBillableTokens(t *testing.T) {
	reply := `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c","function":{"name":"t","arguments":"{}"}}]}}],"usage":{"prompt_tokens":20000,"completion_tokens":100,"prompt_cache_hit_tokens":19000}}`
	server, _ := verifyServer(t, []string{reply})
	options := DefaultAgentOptions()
	options.MaxTokens = 5000   // nho hon raw (20100/vong) nhung lon hon billable (1100/vong)
	options.MaxRounds = 4
	result := runOn(server, false, options)

	if result.Stopped {
		t.Fatalf("phai tinh theo token phai tra that, khong dung vi raw: %+v", result)
	}
	if result.CachedTokens != 19000*4 {
		t.Fatalf("phai cong don cache: %+v", result.CachedTokens)
	}
}

// So cache ca luot la tong cac vong.
func TestRunAgentSumsCachedTokens(t *testing.T) {
	cachedCall := `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","function":{"name":"t","arguments":"{}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1,"prompt_cache_hit_tokens":7}}`
	cachedText := `{"choices":[{"message":{"role":"assistant","content":"xong"}}],"usage":{"prompt_tokens":10,"completion_tokens":1,"prompt_cache_hit_tokens":9}}`
	server, _ := verifyServer(t, []string{cachedCall, cachedText})
	result := runOn(server, false, DefaultAgentOptions())

	if result.CachedTokens != 16 {
		t.Fatalf("phai cong don cache cua cac vong: %+v", result)
	}
}
