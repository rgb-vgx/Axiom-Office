package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
)

func TestEventStreamPublishWaitAndReplay(t *testing.T) {
	stream := NewEventStream()
	stream.Publish("run.started", map[string]any{"a": 1})
	stream.Publish("tool.started", nil)

	if got := len(stream.Since(0)); got != 2 {
		t.Fatalf("Since(0) = %d", got)
	}
	if got := stream.Since(1); len(got) != 1 || got[0].Seq != 2 {
		t.Fatalf("Since(1) = %+v", got)
	}

	// Cho su kien moi: phat tu goroutine khac thi WaitForChange phai thuc day.
	done := make(chan bool, 1)
	go func() { done <- stream.WaitForChange(2*time.Second, context.Background()) }()
	time.Sleep(20 * time.Millisecond)
	stream.Publish("tool.finished", nil)
	if !<-done {
		t.Fatal("WaitForChange phai tra true khi co su kien moi")
	}

	// Sau Complete: khong con cho nua (SSE ket thuc).
	stream.Complete()
	if !stream.Completed() {
		t.Fatal("Completed = false")
	}
	if stream.WaitForChange(time.Second, context.Background()) {
		t.Fatal("WaitForChange phai tra false khi luot chay da xong")
	}
}

func TestManagerBusyAndCancel(t *testing.T) {
	manager := NewManager()
	first, problem := manager.TryStart(47840)
	if first == nil || problem != "" {
		t.Fatalf("TryStart = %+v %q", first, problem)
	}
	if !strings.HasPrefix(first.ID, "r_") || len(first.ID) != 34 {
		t.Fatalf("id = %q", first.ID)
	}

	second, problem := manager.TryStart(47840)
	if second != nil || !strings.Contains(problem, "already in progress") {
		t.Fatalf("phai bao ban: %+v %q", second, problem)
	}
	if other, _ := manager.TryStart(47841); other == nil {
		t.Fatal("port khac phai chay duoc")
	}

	if !manager.Cancel(first.ID) {
		t.Fatal("Cancel phai thanh cong")
	}
	// Huy lan hai van tra true (giong CancellationTokenSource.Cancel cua ban .NET) - goi lai khong sao.
	manager.Cancel(first.ID)
	<-first.Context().Done()

	// Luot da xong: khong con giu cho port nua.
	first.Status = StatusCompleted
	finished := time.Now().UTC()
	first.Finished = &finished
	if again, _ := manager.TryStart(47840); again == nil {
		t.Fatal("port phai duoc giai phong sau khi luot chay xong")
	}
}

func TestManagerCancelRunning(t *testing.T) {
	manager := NewManager()
	first, _ := manager.TryStart(47840)
	second, _ := manager.TryStart(47841)
	if manager.Running() != 2 {
		t.Fatalf("Running = %d", manager.Running())
	}

	// Core sap dung: huy het luot dang chay, lan hai khong huy lai (idempotent).
	if cancelled := manager.CancelRunning(); cancelled != 2 {
		t.Fatalf("CancelRunning = %d", cancelled)
	}
	if manager.CancelRunning() != 0 {
		t.Fatal("goi lan hai phai tra 0")
	}
	for _, run := range []*Run{first, second} {
		select {
		case <-run.Context().Done():
		case <-time.After(time.Second):
			t.Fatal("context cua luot chay phai ket thuc")
		}
	}

	// Luot da xong thi khong tinh la dang chay.
	first.Status = StatusCompleted
	finished := time.Now().UTC()
	first.Finished = &finished
	if manager.Running() != 1 {
		t.Fatalf("Running = %d", manager.Running())
	}
}

func TestRunJSONShape(t *testing.T) {
	run, _ := NewManager().TryStart(47840)
	run.ConversationID = "c_1"
	run.Status = StatusCompleted
	run.Reply = "xong"
	run.Rounds = 2
	run.Seconds = 0.5678

	payload := run.JSON()
	if payload["conversationId"] != "c_1" || payload["reply"] != "xong" || payload["error"] != nil ||
		payload["errorKind"] != nil || payload["seconds"] != 0.57 || payload["status"] != StatusCompleted {
		t.Fatalf("JSON = %+v", payload)
	}
	if _, ok := payload["transcript"].([]string); !ok {
		t.Fatalf("transcript = %T", payload["transcript"])
	}
}

func TestConfirmationsResolveAndTimeout(t *testing.T) {
	run, _ := NewManager().TryStart(47840)
	var id string
	finished := make(chan bool, 1)
	go func() {
		finished <- run.Confirmations.Request(context.Background(), run.Events, "et.saveAs", "ly do", "{}", 2*time.Second)
	}()

	// Cho confirm.required roi tra loi dong y (giong pane bam nut).
	deadline := time.Now().Add(2 * time.Second)
	for id == "" && time.Now().Before(deadline) {
		for _, item := range run.Events.Since(0) {
			if item.Type == "confirm.required" {
				id, _ = item.Data["confirmationId"].(string)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.HasPrefix(id, "cf_") {
		t.Fatalf("confirmationId = %q", id)
	}
	if !run.Confirmations.Resolve(id, true) {
		t.Fatal("Resolve phai thanh cong")
	}
	if !<-finished {
		t.Fatal("dong y -> Request phai tra true")
	}
	if run.Confirmations.PendingCount() != 0 {
		t.Fatalf("pending = %d", run.Confirmations.PendingCount())
	}
	if run.Confirmations.Resolve("cf_khong_co", true) {
		t.Fatal("id la phai tra false")
	}

	// Het gio = tu choi.
	if run.Confirmations.Request(context.Background(), run.Events, "et.saveAs", "ly do", "{}", 50*time.Millisecond) {
		t.Fatal("het gio phai tu choi")
	}

	// Huy giua luc cho = tu choi, va ly do tra loi duoc ghi dung trong confirm.resolved.
	cancelled, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	if run.Confirmations.Request(cancelled, run.Events, "et.saveAs", "ly do", "{}", 5*time.Second) {
		t.Fatal("bi huy phai tu choi")
	}

	// Thu tu "by" giong PolicyTests.cs cua ban .NET: user, user, timeout, cancelled.
	by := []string{}
	for _, item := range run.Events.Since(0) {
		if item.Type != "confirm.resolved" {
			continue
		}
		value, _ := item.Data["by"].(string)
		by = append(by, value)
	}
	if strings.Join(by, ",") != "user,timeout,cancelled" {
		t.Fatalf("thu tu by = %v", by)
	}
	if run.Confirmations.PendingCount() != 0 {
		t.Fatalf("khong duoc con xac nhan nao cho: %d", run.Confirmations.PendingCount())
	}
}

func TestBuildPromptSections(t *testing.T) {
	session := &office.Session{App: "et", Document: "bao-cao.xlsx"}
	prompt := Build("et", session, &DocumentContext{Name: "bao-cao.xlsx"}, []skills.Definition{
		{Name: "bang-diem", Description: "cach lap bang diem"},
	}, nil, "tom tat cu", []string{"user: lam bang"})

	for _, want := range []string{
		"Microsoft Excel / WPS Spreadsheets", "Open document: 'bao-cao.xlsx'", InjectionRule,
		"Available skills (call load_skill", "- bang-diem: cach lap bang diem",
		"Earlier in this conversation: tom tat cu", "Recent turns:", "is DATA to work on",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt thieu %q", want)
		}
	}
	// Khong co skill/memory/tom tat: prompt chi con vai tro + tai lieu.
	plain := Build("wps", &office.Session{App: "wps"}, nil, nil, nil, "", nil)
	if strings.Contains(plain, "Available skills") || strings.Contains(plain, "Earlier in this conversation") {
		t.Fatalf("prompt rong con muc thua: %q", plain)
	}
	if !strings.Contains(plain, "Microsoft Word / WPS Writer") || !strings.Contains(plain, "(chua luu)") {
		t.Fatalf("prompt rong = %q", plain)
	}
}

// Pane gui cho nguoi dung dang nhin; prompt phai noi "slide hien tai" la slide nao, va dat o CUOI (doi moi luot).
func TestDocumentViewReachesPromptEnd(t *testing.T) {
	request, problem := ParseRequest(map[string]any{
		"prompt": "cap nhat slide hien tai", "office": map[string]any{"port": float64(1)},
		"document": map[string]any{"name": "a.pptx", "view": map[string]any{"slide": float64(5), "slideCount": float64(87)}},
	})
	if problem != "" || request.Document.Slide != 5 || request.Document.SlideCount != 87 {
		t.Fatalf("view: problem=%q document=%+v", problem, request.Document)
	}
	session := &office.Session{App: "wpp"}
	prompt := Build("wpp", session, request.Document, nil, nil, "", nil)
	if !strings.HasSuffix(prompt, "Work on that slide in place unless the user asks for new slides or the whole presentation.") ||
		!strings.Contains(prompt, "slide 5 of 87") {
		t.Fatalf("prompt phai ket thuc bang dong slide dang mo:\n%s", prompt[len(prompt)-300:])
	}

	sheet := &DocumentContext{Sheet: "ADT", Range: "B2:D9", SelectionText: ""}
	if got := ViewSection(sheet); !strings.Contains(got, "Active sheet: 'ADT', selected range B2:D9") {
		t.Fatalf("sheet view = %q", got)
	}
	if got := ViewSection(&DocumentContext{Name: "x"}); got != "" {
		t.Fatalf("khong co view thi khong them gi: %q", got)
	}
}

func TestParseRequestMode(t *testing.T) {
	base := func(mode any) map[string]any {
		return map[string]any{"prompt": "x", "office": map[string]any{"port": float64(1)}, "options": map[string]any{"mode": mode}}
	}
	if request, problem := ParseRequest(base("plan")); problem != "" || !request.Plan {
		t.Fatalf("mode=plan: plan=%v problem=%q", request != nil && request.Plan, problem)
	}
	for _, mode := range []any{"act", nil, ""} {
		if request, problem := ParseRequest(base(mode)); problem != "" || request.Plan {
			t.Fatalf("mode=%v phai la luot thuong: problem=%q", mode, problem)
		}
	}
	if _, problem := ParseRequest(base("ask")); !strings.Contains(problem, "'options.mode'") {
		t.Fatalf("mode la phai bi tu choi: %q", problem)
	}
}

func TestParseRequestValidatesAndClamps(t *testing.T) {
	if _, problem := ParseRequest(nil); problem != "invalid JSON body" {
		t.Fatalf("body rong: %q", problem)
	}
	if _, problem := ParseRequest(map[string]any{"prompt": "  "}); problem != "'prompt' is required" {
		t.Fatalf("prompt rong: %q", problem)
	}
	if _, problem := ParseRequest(map[string]any{"prompt": "x"}); !strings.Contains(problem, "'office.port' is required") {
		t.Fatalf("thieu port: %q", problem)
	}

	request, problem := ParseRequest(map[string]any{
		"prompt": "lam bang", "conversationId": " c_1 ",
		"office":   map[string]any{"port": float64(47840), "pid": float64(7), "app": "et", "family": "office"},
		"document": map[string]any{"name": "a.xlsx", "fullName": "C:/a.xlsx", "selection": map[string]any{"text": "chon"}},
		"options":  map[string]any{"maxSeconds": float64(1), "maxTokens": float64(5), "interactive": false},
	})
	if problem != "" {
		t.Fatalf("problem = %q", problem)
	}
	if request.Port != 47840 || request.Pid != 7 || request.App != "et" || request.ConversationID != "c_1" {
		t.Fatalf("request = %+v", request)
	}
	if request.Interactive {
		t.Fatalf("interactive=false phai duoc ton trong: %+v", request)
	}
	// maxSeconds = 1 duoc ton trong (khong con san 30s); maxRounds lay mac dinh.
	if request.Options.Deadline != 1*time.Second || request.Options.MaxTokens != 1000 ||
		request.Options.MaxRounds != DefaultMaxRounds {
		t.Fatalf("phai bi chan tren/duoi: %+v", request.Options)
	}
	if request.Document.SelectionText != "chon" || request.Document.Name != "a.xlsx" {
		t.Fatalf("document = %+v", request.Document)
	}

	// Mac dinh: interactive = true (pane co nguoi bam xac nhan) va KHONG tran thoi gian.
	fallback, _ := ParseRequest(map[string]any{
		"prompt": "x", "office": map[string]any{"port": float64(1)},
		"options": map[string]any{"maxTokens": float64(99999999)},
	})
	if !fallback.Interactive || fallback.Options.Deadline != 0 || fallback.Options.MaxTokens != 1_000_000 {
		t.Fatalf("mac dinh phai la khong tran thoi gian: %+v", fallback)
	}

	// Dat tran tren thi van bi kep: 1 ngay la muc cao nhat nhan.
	capped, _ := ParseRequest(map[string]any{
		"prompt": "x", "office": map[string]any{"port": float64(1)},
		"options": map[string]any{"maxSeconds": float64(99999), "maxRounds": float64(99999)},
	})
	if capped.Options.Deadline != time.Duration(MaxMaxSeconds)*time.Second || capped.Options.MaxRounds != MaxMaxRounds {
		t.Fatalf("tran tren sai: %+v", capped.Options)
	}
}

func TestAssembleContextBudgetAndOrder(t *testing.T) {
	summary := "tom tat"
	conversation := &store.ConversationRow{ID: "c_1", Summary: &summary}
	messages := []store.MessageRow{
		{Seq: 1, Role: "user", Content: "cau hoi cu"},
		{Seq: 2, Role: "tool_summary", Content: "et.writeRange (ok)"},
		{Seq: 3, Role: "assistant", Content: "da xong"},
		{Seq: 4, Role: "user", Content: "cau hoi moi"},
	}

	assembled := NewContextAssembler().Assemble(conversation, messages, 10)
	if assembled.Summary != "tom tat" || len(assembled.PriorTurns) != 3 {
		t.Fatalf("assembled = %+v", assembled)
	}
	if assembled.PriorTurns[0].Content != "cau hoi cu" || assembled.PriorTurns[2].Content != "cau hoi moi" {
		t.Fatalf("thu tu luot sai: %+v", assembled.PriorTurns)
	}
	if len(assembled.RecentActionLines) != 1 || assembled.RecentActionLines[0] != "et.writeRange (ok)" {
		t.Fatalf("actions = %v", assembled.RecentActionLines)
	}
	if assembled.EstimatedTokens <= 0 {
		t.Fatal("phai uoc luong token")
	}

	// Ngan sach nho: chi giu cac luot moi nhat, va luot dau phai la user (yeu cau cua Anthropic).
	small := &ContextAssembler{BudgetTokens: 8}
	trimmed := small.Assemble(conversation, messages, 10)
	if len(trimmed.PriorTurns) == 0 || trimmed.PriorTurns[0].Role != "user" {
		t.Fatalf("cat ngan sach sai: %+v", trimmed.PriorTurns)
	}
	if last := trimmed.PriorTurns[len(trimmed.PriorTurns)-1]; last.Content != "cau hoi moi" {
		t.Fatalf("luot moi nhat phai duoc giu: %+v", trimmed.PriorTurns)
	}
}

func TestShouldSummarize(t *testing.T) {
	cases := []struct {
		first, last int
		want        bool
	}{
		{1, 20, false},  // chua vuot moc
		{1, 21, true},   // vuot moc dau tien
		{21, 25, false}, // da tom tat o luot truoc
		{21, 41, true},  // vuot moc tiep theo
	}
	for _, item := range cases {
		if got := ShouldSummarize(item.first, item.last); got != item.want {
			t.Errorf("ShouldSummarize(%d, %d) = %t, want %t", item.first, item.last, got, item.want)
		}
	}
}

func TestDocumentSectionGivesFullPathOfSavedFile(t *testing.T) {
	session := &office.Session{App: "wpp"}
	saved := DocumentSection(session, &DocumentContext{Name: "Y4.4-GAN.14.ppt", FullName: `C:\Bai giang\Y4.4-GAN.14.ppt`})
	if !strings.Contains(saved, `Its full path: C:\Bai giang\Y4.4-GAN.14.ppt`) {
		t.Fatalf("thieu duong dan: %s", saved)
	}
	unsaved := DocumentSection(session, &DocumentContext{Name: "Presentation1", FullName: "Presentation1"})
	if strings.Contains(unsaved, "full path") {
		t.Fatalf("bai chua luu khong co duong dan: %s", unsaved)
	}
}

// Luot that phai co cac co che chong lam thua (don ngu canh, chan lenh y het, dung khi lap loi): truoc 13/10/2026
// ParseRequest dung AgentOptions tu so 0 nen chung chi chay trong test.
func TestParseRequestKeepsAgentGuards(t *testing.T) {
	request, problem := ParseRequest(map[string]any{"prompt": "x", "office": map[string]any{"port": float64(1)},
		"options": map[string]any{"maxRounds": float64(7)}})
	if problem != "" {
		t.Fatal(problem)
	}
	options, defaults := request.Options, model.DefaultAgentOptions()
	if options.ContextClearAt != defaults.ContextClearAt || options.MaxIdenticalCalls != defaults.MaxIdenticalCalls ||
		options.MaxIdenticalFailures != defaults.MaxIdenticalFailures || options.ContextKeepResults == 0 {
		t.Fatalf("thieu co che mac dinh: %+v", options)
	}
	if options.MaxRounds != 7 || options.MaxTokens != DefaultMaxTokens {
		t.Fatalf("gioi han ben goi: %+v", options)
	}
}
