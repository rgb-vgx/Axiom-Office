package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/store"
)

func TestNormalizeVietnamese(t *testing.T) {
	cases := map[string]string{
		"Người dùng là Trưởng phòng Kế toán": "nguoi dung la truong phong ke toan",
		"  Đặng   Văn   A!! ":                "dang van a",
		"Times New Roman 13":                 "times new roman 13",
		"":                                   "",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
	// Cung noi dung, khac dau/cau -> cung hash (chong trung).
	if Hash("Người ký công văn: Nguyễn Văn A") != Hash("người ký công văn: nguyễn văn a.") {
		t.Fatal("hash phai bo qua dau va dau cau")
	}
}

func TestQueryTermsAndFtsQuery(t *testing.T) {
	terms := QueryTerms("Cơ quan: Sở Giáo dục và Đào tạo Hà Nội")
	joined := strings.Join(terms, ",")
	if !strings.Contains(joined, "giao") || !strings.Contains(joined, "ha") || strings.Contains(joined, "va") {
		t.Fatalf("terms = %v", terms)
	}
	if got := FtsQuery(terms); !strings.HasPrefix(got, `"`) || !strings.Contains(got, " OR ") {
		t.Fatalf("FtsQuery = %q", got)
	}
	if FtsQuery(nil) != "" {
		t.Fatal("khong co tu -> chuoi rong")
	}
}

func TestSensitiveAndValidDate(t *testing.T) {
	for _, text := range []string{
		"Số CCCD của tôi là 012345678901", "Thẻ 4111 1111 1111 1111",
		"mật khẩu của tôi là abc", "key sk-abcdefghijklmnopqrstuv",
	} {
		if !IsSensitive(text) {
			t.Errorf("%q phai bi coi la nhay cam", text)
		}
	}
	if IsSensitive("Người dùng là Trưởng phòng Kế toán") {
		t.Fatal("cau binh thuong khong phai nhay cam")
	}
	if ValidDate("2026-10-15") != "2026-10-15" || ValidDate("15/10/2026") != "" || ValidDate("") != "" {
		t.Fatal("ValidDate sai")
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(db)
}

func TestAddBatchDedupAndLink(t *testing.T) {
	repo := newTestStore(t)
	first := repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "Người dùng là Trưởng phòng Kế toán"}},
		ActorExtract, "r_1", nil, "")
	if len(first) != 1 || first[0].Event != "ADD" || first[0].Item == nil {
		t.Fatalf("first = %+v", first)
	}
	old := first[0].Item.ID

	// Cung noi dung khac dau cau -> DUPLICATE, khong tao ban moi.
	duplicate := repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "người dùng là trưởng phòng kế toán."}},
		ActorExtract, "r_2", nil, "")
	if duplicate[0].Event != "DUPLICATE" || duplicate[0].Item == nil || duplicate[0].Item.ID != old {
		t.Fatalf("duplicate = %+v", duplicate)
	}
	if items := repo.List("", "", false, "", 100); len(items) != 1 {
		t.Fatalf("phai chi co 1 memory: %d", len(items))
	}

	// Fact chuyen doi: link toi ban cu; id bia bi bo.
	change := repo.AddBatch([]NewMemory{{Scope: ScopeUser,
		Text:      "Chức vụ người dùng đổi từ Trưởng phòng Kế toán sang Phó giám đốc từ 10/2026",
		LinkedIDs: []string{old, "m_khong_ton_tai"}}}, ActorExtract, "r_3", nil, "")
	if change[0].Event != "ADD" || len(change[0].Item.Links) != 1 || change[0].Item.Links[0] != old {
		t.Fatalf("change = %+v", change[0])
	}

	// Loc: qua dai, nhay cam, scope sai, rong.
	skipped := repo.AddBatch([]NewMemory{
		{Scope: ScopeUser, Text: "  "},
		{Scope: "khong-hop-le", Text: "abc"},
		{Scope: ScopeUser, Text: "mật khẩu là abc"},
		{Scope: ScopeUser, Text: strings.Repeat("a", MaxFactLength+1)},
	}, ActorAgent, "r_4", nil, "")
	for index, result := range skipped {
		if result.Event != "SKIPPED" {
			t.Errorf("fact %d phai bi bo qua: %+v", index, result)
		}
	}
}

func TestUpdateDeleteRestoreHistory(t *testing.T) {
	repo := newTestStore(t)
	item := repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "Người ký công văn: Nguyễn Văn A"}},
		ActorUser, "", nil, "")[0].Item
	if item.Source != ActorUser || item.Confidence == nil || *item.Confidence != 1 {
		t.Fatalf("fact cua nguoi dung phai confidence=1: %+v", item)
	}

	card, _ := repo.Update(item.ID, "Người ký công văn: Nguyễn Văn B", "", "", false, nil, "doi nguoi ky")
	if card == nil || card.Text != "Người ký công văn: Nguyễn Văn B" {
		t.Fatalf("update = %+v", card)
	}
	pinned := true
	if updated, err := repo.Update(item.ID, "", "", "", false, &pinned, ""); err != nil || !updated.Pinned {
		t.Fatalf("ghim loi: %+v %v", updated, err)
	}
	if _, err := repo.Update(item.ID, strings.Repeat("a", 400), "", "", false, nil, ""); err == nil {
		t.Fatal("text qua dai phai bao loi")
	}
	if _, err := repo.Update(item.ID, "", "", "15/10/2026", false, nil, ""); err == nil {
		t.Fatal("expiresAt sai dinh dang phai bao loi")
	}

	if !repo.Delete(item.ID, "nguoi dung xoa") || repo.Delete(item.ID, "") {
		t.Fatal("Delete sai")
	}
	if items := repo.List("", "", false, "", 100); len(items) != 0 {
		t.Fatalf("memory da xoa khong duoc hien: %d", len(items))
	}
	if items := repo.List("", "", true, "", 100); len(items) != 1 {
		t.Fatalf("includeDeleted phai thay: %d", len(items))
	}
	if !repo.Restore(item.ID) || repo.Restore(item.ID) {
		t.Fatal("Restore sai")
	}
	events := []string{}
	for _, entry := range repo.History(item.ID) {
		events = append(events, entry.Event)
	}
	if strings.Join(events, ",") != "ADD,UPDATE,PIN,DELETE,RESTORE" {
		t.Fatalf("history = %v", events)
	}
	if removed := repo.Purge(ScopeUser); removed != 1 {
		t.Fatalf("Purge = %d", removed)
	}
}

func TestRetrieverRanksChangeBeforeOld(t *testing.T) {
	repo := newTestStore(t)
	repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "Người dùng là Trưởng phòng Kế toán"}}, ActorExtract, "", nil, "")
	time.Sleep(5 * time.Millisecond)
	repo.AddBatch([]NewMemory{{Scope: ScopeUser,
		Text: "Chức vụ người dùng đổi từ Trưởng phòng Kế toán sang Phó giám đốc từ 10/2026"}}, ActorExtract, "", nil, "")

	retriever := NewRetriever(repo)
	contextResult := retriever.BuildContext("Chức vụ trưởng phòng của tôi hiện nay còn đúng không?", "", Today(), nil)
	joined := strings.Join(contextResult.Lines, "\n")
	change := strings.Index(joined, "đổi từ Trưởng phòng")
	old := strings.Index(joined, "Người dùng là Trưởng phòng Kế toán")
	if change < 0 || (old >= 0 && change > old) {
		t.Fatalf("ban chuyen doi phai dung truoc ban cu: %q", joined)
	}
	if len(contextResult.IDs) != len(contextResult.Lines) {
		t.Fatal("ids va lines phai khop")
	}

	// Cau hoi khong lien quan: khong keo memory user vao ngu canh (duoi nguong).
	unrelated := retriever.BuildContext("Viết một câu chào ngắn", "", Today(), nil)
	if len(unrelated.Lines) != 0 {
		t.Fatalf("khong nen co memory nao: %v", unrelated.Lines)
	}
}

func TestShouldSkipAndParseFacts(t *testing.T) {
	for _, prompt := range []string{"In đậm dòng đầu tiên", "ngắn quá"} {
		if skip, _ := ShouldSkip(prompt); !skip {
			t.Errorf("%q phai bi bo qua", prompt)
		}
	}
	for _, prompt := range []string{
		"Tôi là trưởng phòng Kế toán, công văn ký tên Nguyễn Văn A",
		"Từ nay luôn dùng font Times New Roman 13 cho văn bản của tôi",
	} {
		if skip, why := ShouldSkip(prompt); skip {
			t.Errorf("%q khong duoc bo qua (%s)", prompt, why)
		}
	}

	tempIDs := map[string]string{"0": "m_that"}
	facts, problem := ParseFacts(`{"facts":[
		{"text":"Người dùng là Trưởng phòng Kế toán","scope":"user","category":"identity","confidence":0.9,"entities":["Kế toán"],"linkedIds":["0","9"]},
		{"text":"Quá mơ hồ","confidence":0.3},
		{"text":"Số CCCD 012345678901","confidence":0.9},
		{"text":"","confidence":1}]}`, tempIDs, "")
	if problem != "" {
		t.Fatal(problem)
	}
	if len(facts) != 1 || len(facts[0].LinkedIDs) != 1 || facts[0].LinkedIDs[0] != "m_that" {
		t.Fatalf("facts = %+v", facts)
	}

	if _, problem := ParseFacts("khong phai json", nil, ""); problem == "" {
		t.Fatal("JSON hong phai bao loi")
	}
	if _, problem := ParseFacts(`{"khac":[]}`, nil, ""); !strings.Contains(problem, "missing 'facts'") {
		t.Fatalf("thieu facts: %q", problem)
	}
	// facts rong la hop le (khong co gi dang nho).
	if facts, problem := ParseFacts(`{"facts":[]}`, nil, ""); problem != "" || len(facts) != 0 {
		t.Fatalf("facts rong = %+v %q", facts, problem)
	}
	// Code fence + van ban thua van doc duoc.
	fenced := "```json\n{\"facts\":[{\"text\":\"Người ký: Nguyễn Văn A\",\"confidence\":0.9}]}\n```"
	if facts, problem := ParseFacts(fenced, nil, ""); problem != "" || len(facts) != 1 {
		t.Fatalf("fenced = %+v %q", facts, problem)
	}
}

func TestBuildUserMessageUsesTempIDs(t *testing.T) {
	input := ExtractionInput{
		Prompt: "Tôi đã lên phó giám đốc", Reply: "Chúc mừng anh.",
		Recent:     []RecentMessage{{Role: "user", Content: "xin chao"}, {Role: "assistant", Content: "chao ban"}},
		Related:    []Item{{ID: "m_abcdef123456", Text: "Người dùng là Trưởng phòng Kế toán"}},
		ObservedOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	message, tempIDs := BuildUserMessage(input)
	if tempIDs["0"] != "m_abcdef123456" {
		t.Fatalf("tempIDs = %v", tempIDs)
	}
	for _, want := range []string{"Ngày quan sát: 2026-10-01", "(chưa lưu)", "0: Người dùng là Trưởng phòng", "Người dùng: xin chao",
		"Trợ lý: chao ban", "Trả về JSON {\"facts\": [...]}"} {
		if !strings.Contains(message, want) {
			t.Errorf("thieu %q trong:\n%s", want, message)
		}
	}
	if strings.Contains(message, "m_abcdef123456") {
		t.Fatal("model khong duoc thay id that")
	}
}

func TestServiceExtractsInBackground(t *testing.T) {
	repo := newTestStore(t)
	conversations := repo.db.Conversations()
	// LLM gia: tra ve mot fact.
	server := fakeChatServer(t, `{"facts":[{"text":"Người dùng là Trưởng phòng Kế toán","confidence":0.9}]}`)
	client := model.NewClient(server.Client(), "openai", server.URL, "k", "m")

	// load tra ve bien cfg: doi cfg sau do co hieu luc ngay (giong doc lai HKCU moi lan).
	cfg := config.Config{MemoryEnabled: true, MemoryAutoExtract: true}
	load := func() config.Config { return cfg }
	service := NewService(repo.db, conversations, repo.db.Runs(), load,
		func(config.Config) *model.Client { return client }, server.Client(), true)
	t.Cleanup(service.Close)

	job := ExtractionJob{RunID: "r_1", ConversationID: "c_1",
		Prompt: "Tôi là trưởng phòng Kế toán, công văn ký tên Nguyễn Văn A", Reply: "Đã ghi nhận."}
	if status := service.QueueExtraction(job); status != "queued" {
		t.Fatalf("status = %q", status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(service.Store().List("", "", false, "", 10)) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	items := service.Store().List("", "", false, "", 10)
	if len(items) != 1 || items[0].Source != ActorExtract {
		t.Fatalf("items = %+v", items)
	}

	// Lenh thao tac thuan -> skipped, khong goi LLM.
	if status := service.QueueExtraction(ExtractionJob{RunID: "r_2", ConversationID: "c_1", Prompt: "In đậm dòng đầu tiên"}); status != "skipped" {
		t.Fatalf("status = %q", status)
	}
	// Tat memory -> khong doc, khong ghi.
	cfg.MemoryEnabled = false
	if service.Enabled() {
		t.Fatal("MemoryEnabled=false -> Enabled phai false")
	}
	if lines := service.Context(context.Background(), "x", "").Lines; len(lines) != 0 {
		t.Fatalf("khong duoc dua memory vao ngu canh: %v", lines)
	}
}

// fakeChatServer: may chu chat gia tra ve dung noi dung cho truoc (khong can mang).
func fakeChatServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server
}
