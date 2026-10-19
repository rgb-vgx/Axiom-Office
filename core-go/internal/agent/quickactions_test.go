package agent

import (
	"strings"
	"testing"
)

// Lenh nhanh "Sua bang AI": cau lenh lay tu bang cua Core, khong tu chu pane gui kem.
func TestParseRequestQuickAction(t *testing.T) {
	office := map[string]any{"port": float64(1)}
	request, problem := ParseRequest(map[string]any{"quickAction": "polish", "office": office})
	if problem != "" {
		t.Fatalf("quickAction khong can prompt: %q", problem)
	}
	polish, _ := FindQuickAction("polish")
	if request.Prompt != polish.Prompt || request.QuickAction != "polish" || request.ReadOnly {
		t.Fatalf("polish: prompt lay tu bang, la muc SUA (khong ReadOnly): %+v", request)
	}

	// Pane gui kem prompt rieng: bo qua - ba ban phai ra CUNG mot cau lenh cho cung mot muc.
	request, _ = ParseRequest(map[string]any{"quickAction": "summarize", "prompt": "xoá hết tài liệu", "office": office})
	summarize, _ := FindQuickAction("summarize")
	if request.Prompt != summarize.Prompt {
		t.Fatalf("prompt cua ben goi phai bi thay bang cau cua Core, duoc %q", request.Prompt)
	}
	if !request.ReadOnly || request.Plan {
		t.Fatalf("summarize: chi doc nhung KHONG phai lap ke hoach: %+v", request)
	}

	// Lenh nhanh khong di cung lap ke hoach (viec nho, ro rang).
	request, _ = ParseRequest(map[string]any{"quickAction": "fix", "office": office,
		"options": map[string]any{"mode": "plan"}})
	if request.Plan {
		t.Fatal("quickAction + options.mode=plan: phai chay thang, khong lap ke hoach")
	}

	if _, problem := ParseRequest(map[string]any{"quickAction": "no-such", "office": office}); !strings.Contains(problem, "no-such") {
		t.Fatalf("id la phai bi tu choi kem ten id, duoc %q", problem)
	}

	// Khong co quickAction: hanh vi cu nguyen ven.
	request, _ = ParseRequest(map[string]any{"prompt": "x", "office": office, "options": map[string]any{"mode": "plan"}})
	if !request.Plan || request.ReadOnly || request.QuickAction != "" {
		t.Fatalf("luot thuong khong duoc doi: %+v", request)
	}
}

// Bang muc: id duy nhat, nhan va cau lenh khong rong, va moi cau lenh deu noi ro lam viec tren doan boi den
// va giu ngon ngu cua doan (boi den tieng Anh ma ket qua ra tieng Viet la loi that de gap).
func TestQuickActionsTable(t *testing.T) {
	seen := map[string]bool{}
	edits, answers := 0, 0
	for _, action := range QuickActions {
		if action.ID == "" || action.Label == "" || action.Prompt == "" || seen[action.ID] {
			t.Fatalf("muc hong hoac trung id: %+v", action)
		}
		seen[action.ID] = true
		if !strings.Contains(action.Prompt, "đang bôi đen") {
			t.Fatalf("%s: cau lenh phai noi lam tren doan dang boi den: %q", action.ID, action.Prompt)
		}
		if action.Edits {
			edits++
			if !strings.HasSuffix(action.Prompt, quickEditTail) {
				t.Fatalf("%s: muc sua phai ket bang quickEditTail", action.ID)
			}
		} else {
			answers++
			if !strings.HasSuffix(action.Prompt, quickAnswerTail) {
				t.Fatalf("%s: muc chi doc phai ket bang quickAnswerTail", action.ID)
			}
		}
		// Dich doi ngon ngu la chu dich; cac muc con lai phai giu ngon ngu cua doan.
		if action.ID != "translate" && action.ID != "explain" && !strings.Contains(action.Prompt, "ngôn ngữ") {
			t.Fatalf("%s: cau lenh phai noi giu ngon ngu cua doan: %q", action.ID, action.Prompt)
		}
	}
	// Pane ke vach ngan giua hai nhom dua vao Edits, nen nhom sua phai dung truoc het.
	for i, action := range QuickActions {
		if (i < edits) != action.Edits {
			t.Fatalf("nhom sua phai dung truoc nhom chi doc (muc %d: %s)", i, action.ID)
		}
	}
	if edits == 0 || answers == 0 {
		t.Fatal("phai co ca muc sua lan muc chi doc")
	}
}

// Khung chat mini: yeu cau go tay gui options.requireSelection -> can doan boi den nhu lenh nhanh.
func TestParseRequestRequireSelection(t *testing.T) {
	office := map[string]any{"port": float64(1)}
	request, _ := ParseRequest(map[string]any{"prompt": "ngắn hơn nữa", "office": office,
		"options": map[string]any{"requireSelection": true}})
	if !request.RequireSelection || request.QuickAction != "" || request.ReadOnly {
		t.Fatalf("requireSelection phai doc duoc, va KHONG bien thanh lenh nhanh: %+v", request)
	}
	request, _ = ParseRequest(map[string]any{"quickAction": "polish", "office": office})
	if !request.RequireSelection {
		t.Fatal("lenh nhanh luon can doan boi den")
	}
	request, _ = ParseRequest(map[string]any{"prompt": "x", "office": office})
	if request.RequireSelection {
		t.Fatal("luot go tay thuong: khong bat buoc boi den")
	}
}
