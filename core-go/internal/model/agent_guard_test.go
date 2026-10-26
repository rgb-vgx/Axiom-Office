package model

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Tool bao da xong CA viec (Final): luot ket thuc ngay voi cau tra loi cua tool, khong goi model them. Luot that
// 13/10/2026: slides_from_template xong ca bai o vong 5, model con chay 25 vong roi het ngan sach.
func TestRunAgentEndsWhenToolReportsFinal(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall, fmt.Sprintf(replyText, "khong duoc toi day")})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Mutating: true, ResultJSON: `{"ok":true}`,
				Final: true, FinalText: "Đã dựng xong 95 slide."}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if !result.OK || result.Text != "Đã dựng xong 95 slide." || result.Rounds != 1 || len(bodies()) != 1 {
		t.Fatalf("phai ket thuc sau vong 1: %+v, %d request", result, len(bodies()))
	}
}

// Lan goi y het thu 5 bi chan (khong chay tool), model nhan loi va doi cach. Lenh doc (khong sua tai lieu) lap lai khi
// tai lieu chua doi thi tu lan 2 da khong chay ma tra "khong doi" (docStateGuard) - nhung van duoc dem.
func TestRunAgentBlocksIdenticalCalls(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	executed := 0
	options := DefaultAgentOptions()
	options.MaxRounds = 7
	options.Verify = false
	var blocked string
	client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{ToolFinished: func(result ToolResult) {
			if strings.Contains(result.ResultJSON, "blocked") {
				blocked = result.ResultJSON
			}
		}})
	if executed != 1 || blocked == "" {
		t.Fatalf("chay %d lan (muon 1 - cac lan sau la 'khong doi'), loi chan: %q", executed, blocked)
	}
}

// Lenh doc y het xen giua cac lan ghi (tai lieu da doi) khong bi chan du vuot han muc: luot that 26/10/2026 -
// wpp.checkLayout ca bai sau moi lan dung lai bi chan o lan thu 5, model phai kiem tung slide mot.
func TestRunAgentIdenticalReadAfterWriteNotBlocked(t *testing.T) {
	replies := []string{}
	for i := 1; i <= 6; i++ {
		replies = append(replies, fmt.Sprintf(replyToolCallArgs, fmt.Sprintf("r%d", i), `{"doc":"check"}`))
		if i < 6 {
			replies = append(replies, fmt.Sprintf(replyToolCallArgs, fmt.Sprintf("w%d", i), fmt.Sprintf(`{"write":%d}`, i)))
		}
	}
	server, _ := verifyServer(t, append(replies, fmt.Sprintf(replyText, "xong")))
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	reads, blocked := 0, 0
	options := DefaultAgentOptions()
	options.Verify = false
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			write := strings.Contains(call.Arguments, "write")
			if !write {
				reads++
			}
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Mutating: write, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{ToolFinished: func(result ToolResult) {
			if strings.Contains(result.ResultJSON, "blocked") {
				blocked++
			}
		}})
	if !result.OK || reads != 6 || blocked != 0 {
		t.Fatalf("6 lan doc xen 5 lan ghi phai chay het: doc %d, chan %d, %+v", reads, blocked, result)
	}
}

// Lan goi hong nhung DA ghi mot phan (slides_from_brief dung 3 slide roi loi: ok:false + Mutating) van lam tai lieu
// doi: lenh doc y het sau do phai chay that, khong duoc tra "unchanged" (26/10/2026).
func TestRunAgentFailedPartialWriteCountsAsChange(t *testing.T) {
	server, _ := verifyServer(t, []string{
		fmt.Sprintf(replyToolCallArgs, "r1", `{"doc":"list"}`),
		fmt.Sprintf(replyToolCallArgs, "w1", `{"write":1}`),
		fmt.Sprintf(replyToolCallArgs, "r2", `{"doc":"list"}`),
		fmt.Sprintf(replyText, "xong"),
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	reads := 0
	options := DefaultAgentOptions()
	options.Verify = false
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			if strings.Contains(call.Arguments, "write") {
				return ToolResult{CallID: call.ID, Name: call.Name, OK: false, Mutating: true,
					ResultJSON: `{"ok":false,"error":"slide 4 failed","built":3}`}
			}
			reads++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{})
	if !result.OK || reads != 2 {
		t.Fatalf("lenh doc sau lan ghi hong mot phan phai chay lai: doc %d, %+v", reads, result)
	}
}

// Lenh ghi y het XEN lenh ghi khac (ngat trang sau moi chuong) khong bi chan; lap LIEN thi van bi chan (bo do Word
// 26/10/2026: writer.insertPageBreak bi chan tu lan 5 khi viet bao cao nhieu chuong).
func TestRunAgentIdenticalWriteBetweenOtherWritesNotBlocked(t *testing.T) {
	run := func(replies []string) (int, int) {
		server, _ := verifyServer(t, append(replies, fmt.Sprintf(replyText, "xong")))
		client := NewClient(server.Client(), "openai", server.URL, "", "m")
		breaks, blocked := 0, 0
		options := DefaultAgentOptions()
		options.Verify = false
		client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
			func(ctx context.Context, call ToolCall) ToolResult {
				if strings.Contains(call.Arguments, "break") {
					breaks++
				}
				return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Mutating: true, ResultJSON: `{"ok":true}`}
			}, options, AgentCallbacks{ToolFinished: func(result ToolResult) {
				if strings.Contains(result.ResultJSON, "blocked") {
					blocked++
				}
			}})
		return breaks, blocked
	}
	interleaved := []string{}
	for i := 1; i <= 6; i++ {
		interleaved = append(interleaved, fmt.Sprintf(replyToolCallArgs, fmt.Sprintf("t%d", i), fmt.Sprintf(`{"text":"chuong %d"}`, i)),
			fmt.Sprintf(replyToolCallArgs, fmt.Sprintf("b%d", i), `{"break":true}`))
	}
	if breaks, blocked := run(interleaved); breaks != 6 || blocked != 0 {
		t.Fatalf("6 lan ngat trang xen 6 doan chu phai chay het: chay %d, chan %d", breaks, blocked)
	}
	repeated := []string{}
	for i := 1; i <= 6; i++ {
		repeated = append(repeated, fmt.Sprintf(replyToolCallArgs, fmt.Sprintf("b%d", i), `{"break":true}`))
	}
	if breaks, blocked := run(repeated); breaks != 4 || blocked == 0 {
		t.Fatalf("ngat trang lap lien phai bi chan sau 4 lan: chay %d, chan %d", breaks, blocked)
	}
}

// replyToolCallArgs: tool call voi id va doi so tuy bien (doc roi doc lai, hay roi ghi roi doc).
const replyToolCallArgs = `{"choices":[{"message":{"role":"assistant","content":null,` +
	`"tool_calls":[{"id":"%s","function":{"name":"t","arguments":%q}}]}}],` +
	`"usage":{"prompt_tokens":10,"completion_tokens":1}}`

// Lenh doc Y HET lap lai khi tai lieu khong doi (khong co lenh ghi o giua): lan sau khong chay lai tool, thay
// bang ket qua "unchanged"; ket qua dau van nam lai trong ngu canh cho model (docStateGuard khong tao ra lich
// su dang ra lai khac), nhung van duoc dem de chong trung lap hang loat.
func TestRunAgentDocStateGuardSkipsIdenticalRead(t *testing.T) {
	server, bodies := verifyServer(t, []string{
		fmt.Sprintf(replyToolCallArgs, "r1", `{"doc":"list"}`),
		fmt.Sprintf(replyToolCallArgs, "r2", `{"doc":"list"}`),
		fmt.Sprintf(replyText, "khong co gi moi"),
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	executed := 0
	results := []string{}
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true,
				ResultJSON: `{"ok":true,"result":"3 slide, 0 loi bo cuc"}`}
		}, DefaultAgentOptions(), AgentCallbacks{ToolFinished: func(call ToolResult) {
			results = append(results, call.ResultJSON)
		}})
	if !result.OK || result.Text != "khong co gi moi" || result.Rounds != 3 || len(bodies()) != 3 {
		t.Fatalf("luot phai di 3 vong (doc, doc lai, tra loi): %+v (%d request)", result, len(bodies()))
	}
	if executed != 1 {
		t.Fatalf("lan doc lap lai phai KHONG chay lai tool: thuc chay %d lan", executed)
	}
	if len(results) != 2 || !strings.Contains(results[1], "unchanged") {
		t.Fatalf("lan hai phai nhan ket qua thay the 'khong doi': %v", results)
	}
	// Ket qua thay the di vao hoi thoai lan goi thu 3 (tin `tool` cho r2), thay cho ban ket qua tang dai kha.
	// Lan goi thu 2 VAN giu ket qua dau (no la bang chung cho doi chieu).
	sent := bodies()
	if !strings.Contains(sent[1], `3 slide, 0 loi bo cuc`) || strings.Contains(sent[1], "unchanged") {
		t.Fatalf("request vong 2 phai giu ket qua doc dau: %.500s", sent[1])
	}
	if !strings.Contains(sent[2], "unchanged") {
		t.Fatalf("request vong 3 phai chua ket qua 'unchanged' thay cho doc lap lai: %.500s", sent[2])
	}
}

// Doc lai sau khi SUA ma ket qua doc la suon chung kiem tra (Verifies): verifyGuard khong bat them mot vong
// doc lai nua - luot tra loi o vong thu 3 thay vi thu 4.
func TestRunAgentEvidenceReadAfterWriteSkipsVerifyNudge(t *testing.T) {
	server, bodies := verifyServer(t, []string{
		fmt.Sprintf(replyToolCallArgs, "w1", `{"step":"write"}`),
		fmt.Sprintf(replyToolCallArgs, "r1", `{"step":"read"}`),
		fmt.Sprintf(replyText, "da kiem tra, dung"),
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			if strings.Contains(call.Arguments, `"step":"write"`) {
				return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Mutating: true, ResultJSON: `{"ok":true}`}
			}
			// Lenh doc kiem tra: bang chung kiem tra tai lieu o phien ban hien tai.
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Verifies: true,
				ResultJSON: `{"ok":true,"checked":"doc moi giong gi da ghi"}`}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if !result.OK || result.Text != "da kiem tra, dung" || result.Rounds != 3 {
		t.Fatalf("phai ket thuc o vong 3 (doc la ban chung, khong can nhac): %+v", result)
	}
	if !result.Verified {
		t.Fatal("da doc lai sau khi sua phai tinh la luot da tu kiem chung (Verified)")
	}
	sent := bodies()
	if len(sent) != 3 || strings.Contains(sent[2], "check the work you just did") {
		t.Fatalf("vong dung phai khong mang loi nhac kiem chung: %d request, %.200s", len(sent), sent[2])
	}
}

// Doc lai TRUOC khi sua thi khong tinh la bang chung o phien ban moi: sau lan sua cuoi khong co doc lai thi
// verifyGuard van bat doc lai mot lan truoc khi tra loi (ket thuc o vong 4 thay vi vong 3).
func TestRunAgentReadBeforeWriteIsNotEvidence(t *testing.T) {
	server, bodies := verifyServer(t, []string{
		fmt.Sprintf(replyToolCallArgs, "r1", `{"step":"read"}`),
		fmt.Sprintf(replyToolCallArgs, "w1", `{"step":"write"}`),
		fmt.Sprintf(replyText, "ban nhap"),
		fmt.Sprintf(replyText, "ket qua cuoi"),
	})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			if strings.Contains(call.Arguments, `"step":"write"`) {
				return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Mutating: true, ResultJSON: `{"ok":true}`}
			}
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Verifies: true,
				ResultJSON: `{"ok":true,"checked":"tai lieu truoc khi sua"}`}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if !result.OK || result.Text != "ket qua cuoi" || result.Rounds != 4 || !result.Verified {
		t.Fatalf("doc truoc sua khong tinh bang chung, phai co vong nhac: %+v", result)
	}
	sent := bodies()
	if !strings.Contains(sent[3], "check the work you just did") {
		t.Fatalf("request cuoi phai mang loi nhac kiem chung: %.200s", sent[3])
	}
}

// Ngan sach 90%: tat tool va nhac tra loi; model tra loi duoc thay vi bi cat cung o 100%.
func TestRunAgentTurnsToolsOffNearBudget(t *testing.T) {
	heavy := `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","function":{"name":"t","arguments":"{\"n\":%d}"}}]}}],"usage":{"prompt_tokens":95000,"completion_tokens":1}}`
	server, bodies := verifyServer(t, []string{fmt.Sprintf(heavy, 1), fmt.Sprintf(heavy, 2), fmt.Sprintf(replyText, "Đã xong phần lớn, còn thiếu slide 90-95.")})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	options := DefaultAgentOptions()
	options.Verify = false
	options.MaxTokens = 200_000 // mac dinh khong tran: ben goi dat tran thi ngan sach ba muc moi chay
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{})
	sent := bodies()
	if !result.OK || !strings.Contains(result.Text, "còn thiếu") || len(sent) != 3 {
		t.Fatalf("phai tra loi o vong 3: %+v (%d request)", result, len(sent))
	}
	if strings.Contains(sent[2], `"tools"`) || !strings.Contains(sent[2], "tools are now off") {
		t.Fatalf("vong 3 phai tat tool va co loi nhac: %s", sent[2][:min(400, len(sent[2]))])
	}
	if strings.Contains(sent[1], "tools are now off") || !strings.Contains(sent[1], `"tools"`) {
		t.Fatal("vong 2 (47% ngan sach) chua duoc tat tool")
	}
}

func TestClearOldToolResultsKeepsRecentAndNeedsEnoughGain(t *testing.T) {
	big := strings.Repeat("x", 30_000)
	turns := []Message{
		map[string]any{"role": "user", "content": "u"},
		map[string]any{"role": "tool", "tool_call_id": "1", "content": big},
		map[string]any{"role": "tool", "tool_call_id": "2", "content": big},
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "3", "content": big}}},
		map[string]any{"role": "tool", "tool_call_id": "4", "content": "gan nhat 1"},
		map[string]any{"role": "tool", "tool_call_id": "5", "content": "gan nhat 2"},
		map[string]any{"role": "tool", "tool_call_id": "6", "content": "gan nhat 3"},
	}
	if cleared := clearOldToolResults(turns, 3, 40_000); cleared < 80_000 {
		t.Fatalf("thu hoi %d", cleared)
	}
	if turns[1].(map[string]any)["content"] != clearedToolResult || turns[6].(map[string]any)["content"] != "gan nhat 3" {
		t.Fatal("phai don ket qua cu, giu 3 ket qua gan nhat")
	}
	block := turns[3].(map[string]any)["content"].([]any)[0].(map[string]any)
	if block["content"] != clearedToolResult {
		t.Fatal("khoi tool_result (Anthropic) cung phai duoc don")
	}
	small := []Message{map[string]any{"role": "tool", "content": "a"}, map[string]any{"role": "tool", "content": "b"},
		map[string]any{"role": "tool", "content": "c"}, map[string]any{"role": "tool", "content": "d"}}
	if clearOldToolResults(small, 3, 40_000) != 0 || small[0].(map[string]any)["content"] != "a" {
		t.Fatal("thu hoi qua it thi khong don (giu cache)")
	}
}

// Ung dung Office da dong / sap: tool bao Abort -> dung luot ngay voi loi do, khong goi model them.
func TestRunAgentStopsWhenOfficeAppIsGone(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: `{"ok":false}`, Abort: "Word đã đóng"}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if result.OK || result.ErrorKind != "office" || result.Error != "Word đã đóng" || len(bodies()) != 1 {
		t.Fatalf("phai dung sau vong 1: %+v (%d request)", result, len(bodies()))
	}
}
