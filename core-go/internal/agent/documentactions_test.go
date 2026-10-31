package agent

import (
	"context"
	"strings"
	"testing"
)

// Lenh "ca tai lieu" (docs/research/doi-chieu-wps-ai.md P1-1, P1-3): nut ribbon chay cung duong {quickAction: id} nhung
// KHONG can boi den, KHONG khoa vung chon va khong nam trong menu "Sua bang AI" (QuickActions).
func TestDocumentActionsTable(t *testing.T) {
	ids := []string{}
	for _, action := range DocumentActions {
		ids = append(ids, action.ID)
		if action.Label == "" || action.Prompt == "" || !action.Edits || !action.Document {
			t.Fatalf("muc hong: %+v", action)
		}
		if strings.Contains(action.Prompt, "đang bôi đen") {
			t.Fatalf("%s: lenh ca tai lieu khong duoc noi ve doan boi den: %q", action.ID, action.Prompt)
		}
		if _, clash := findIn(QuickActions, action.ID); clash {
			t.Fatalf("%s trung id voi menu Sua bang AI", action.ID)
		}
	}
	if got := strings.Join(ids, " "); got != "proofread tableTidy" {
		t.Fatalf("lenh ca tai lieu: %s", got)
	}
	for _, action := range QuickActions {
		if action.Document || action.Track {
			t.Fatalf("%s: muc menu boi den khong phai lenh ca tai lieu", action.ID)
		}
	}

	proofread, _ := FindQuickAction("proofread")
	for _, want := range []string{"writer.outline", "writer.replaceMany", "writer.addComment", "tên riêng"} {
		if !strings.Contains(proofread.Prompt, want) {
			t.Fatalf("proofread: cau lenh thieu %q: %q", want, proofread.Prompt)
		}
	}
	if !proofread.Track {
		t.Fatal("proofread: sua bang Track Changes de nguoi dung duyet tung cho")
	}
	tidy, _ := FindQuickAction("tableTidy")
	// borders: tren WPS that style "Table Grid" khong tu ke khung (do 01/11/2026) - phai bat borders.
	for _, want := range []string{"writer.checkTables", "writer.formatTable", "skipped", "borders true"} {
		if !strings.Contains(tidy.Prompt, want) {
			t.Fatalf("tableTidy: cau lenh thieu %q: %q", want, tidy.Prompt)
		}
	}
	if tidy.Track {
		t.Fatal("tableTidy: dinh dang bang khong ghi thanh revision chu")
	}
}

func TestParseRequestDocumentAction(t *testing.T) {
	office := map[string]any{"port": float64(1)}
	request, problem := ParseRequest(map[string]any{"quickAction": "proofread", "office": office})
	if problem != "" {
		t.Fatalf("proofread: %q", problem)
	}
	proofread, _ := FindQuickAction("proofread")
	if request.Prompt != proofread.Prompt || request.QuickAction != "proofread" || request.ReadOnly {
		t.Fatalf("proofread: cau lenh tu bang Core, la luot SUA: %+v", request)
	}
	if request.RequireSelection || !request.WholeDocument {
		t.Fatalf("proofread: khong can boi den, lam tren ca tai lieu: %+v", request)
	}
	if request.TrackChanges == nil || !*request.TrackChanges {
		t.Fatal("proofread: mac dinh bat Track Changes")
	}
	// Ben goi tat ro rang thi ton trong.
	off := false
	request, _ = ParseRequest(map[string]any{"quickAction": "proofread", "office": office,
		"options": map[string]any{"trackChanges": off}})
	if request.TrackChanges == nil || *request.TrackChanges {
		t.Fatal("options.trackChanges=false phai thang mac dinh cua proofread")
	}
	request, _ = ParseRequest(map[string]any{"quickAction": "tableTidy", "office": office})
	if request.RequireSelection || !request.WholeDocument || request.TrackChanges != nil {
		t.Fatalf("tableTidy: ca tai lieu, theo cau hinh Track Changes: %+v", request)
	}
	request, _ = ParseRequest(map[string]any{"quickAction": "polish", "office": office})
	if request.WholeDocument {
		t.Fatal("lenh menu boi den khong phai ca tai lieu")
	}
}

// Lenh ca tai lieu KHONG khoa vung chon: nguoi dung dang boi den mot chu thi soat chinh ta van phai sua duoc ca bai.
func TestLockSelectionSkippedForWholeDocument(t *testing.T) {
	o, port, calls := trackingBridge(t, TrackingAuthor)
	run := newRun("r1", port)
	if problem := o.lockSelection(context.Background(), run, &Request{Port: port, WholeDocument: true}, "wps"); problem != "" {
		t.Fatalf("khong khoa thi khong co loi: %q", problem)
	}
	if len(calls()) != 0 || run.SelectionLock {
		t.Fatalf("lenh ca tai lieu khong duoc goi lenh khoa: %v", calls())
	}
	o.lockSelection(context.Background(), run, &Request{Port: port}, "wps")
	actions := []string{}
	for _, call := range calls() {
		actions = append(actions, call.Action)
	}
	if strings.Join(actions, " ") != "writer.selection writer.lockSelection" {
		t.Fatalf("luot thuong van khoa nhu cu: %v", actions)
	}
}
