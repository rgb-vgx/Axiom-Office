package agent

// Lenh nhanh "Sua bang AI" (docs/ribbon-ui.md muc 2, "Trong khung chat"): nguoi dung boi den mot doan roi
// chon mot muc trong menu - Trau chuot, Viet lai, Rut gon, ... - thay vi tu go cau lenh.
//
// Cau lenh cua moi muc nam O DAY, trong Core, chu khong nam trong pane: ba ban (Windows, WPS, LibreOffice)
// chi gui `quickAction: "<id>"`, nen sua cau chu mot lan la ca ba doi theo. Pane hoi danh sach qua
// GET /v1/quick-actions (ban WPS: /wps/v1/quick-actions) de khong phai chep tay nhan va id.
//
// Cau lenh viet bang tieng Viet vi no duoc luu NGUYEN VAN lam tin nhan cua nguoi dung (man hinh "Hoi thoai
// truoc..." hien lai dung cau nay), va moi cau deu noi "giu nguyen ngon ngu cua doan" - boi den mot doan
// tieng Anh roi bam Trau chuot thi ket qua van phai la tieng Anh.

// QuickAction: mot muc cua menu "Sua bang AI".
type QuickAction struct {
	ID    string
	Label string
	// Edits: muc nay SUA tai lieu. false = chi tra loi trong khung chat (Tom tat, Giai thich, Dich): luot chay
	// chi duoc goi lenh doc (cung bo tool cua che do lap ke hoach), nen "khong sua tai lieu" la dieu Core CHAN
	// that, khong chi la loi dan trong cau lenh.
	Edits  bool
	Prompt string
}

// Ket cau chung cua cac muc sua: chi trong vung boi den (khoa vung cua bridge chan phan con lai), va bao lai
// ngan gon da doi gi de nguoi dung biet co can bam "Hoan tac luot nay" khong.
const quickEditTail = " Chỉ sửa trong đoạn đang bôi đen, sửa thẳng vào tài liệu, rồi nói ngắn gọn đã đổi những gì."

// Ket cau chung cua cac muc chi doc.
const quickAnswerTail = " Trả lời ngay trong khung chat, không sửa tài liệu."

// QuickActions: thu tu o day la thu tu trong menu. Nhom sua truoc, nhom chi doc sau - pane ke mot vach ngan
// giua hai nhom dua vao Edits.
var QuickActions = []QuickAction{
	{ID: "polish", Label: "Trau chuốt", Edits: true, Prompt: "Trau chuốt đoạn văn đang bôi đen: sửa câu chữ cho " +
		"trôi chảy, mạch lạc hơn; giữ nguyên ý, giọng văn và ngôn ngữ của đoạn." + quickEditTail},
	{ID: "rewrite", Label: "Viết lại", Edits: true, Prompt: "Viết lại đoạn văn đang bôi đen bằng cách diễn đạt " +
		"khác; giữ nguyên ý và ngôn ngữ của đoạn." + quickEditTail},
	{ID: "shorten", Label: "Rút gọn", Edits: true, Prompt: "Rút gọn đoạn văn đang bôi đen còn khoảng một nửa độ " +
		"dài; giữ các ý chính, số liệu và ngôn ngữ của đoạn." + quickEditTail},
	{ID: "expand", Label: "Mở rộng", Edits: true, Prompt: "Mở rộng đoạn văn đang bôi đen: thêm giải thích, chi " +
		"tiết hoặc ví dụ phù hợp với nội dung; không bịa số liệu hay sự kiện; giữ ngôn ngữ của đoạn." + quickEditTail},
	{ID: "fix", Label: "Sửa lỗi chính tả", Edits: true, Prompt: "Sửa lỗi chính tả, ngữ pháp và dấu câu trong đoạn " +
		"văn đang bôi đen; câu nào không sai thì giữ nguyên cách diễn đạt; giữ ngôn ngữ của đoạn." + quickEditTail},
	{ID: "formal", Label: "Trang trọng hơn", Edits: true, Prompt: "Viết lại đoạn văn đang bôi đen với giọng trang " +
		"trọng, lịch sự, hợp với văn bản công việc; giữ nguyên ý và ngôn ngữ của đoạn." + quickEditTail},
	{ID: "summarize", Label: "Tóm tắt", Prompt: "Tóm tắt đoạn văn đang bôi đen thành vài ý chính, viết bằng " +
		"ngôn ngữ của đoạn." + quickAnswerTail},
	{ID: "explain", Label: "Giải thích", Prompt: "Giải thích đoạn văn đang bôi đen bằng lời dễ hiểu: ý chính là " +
		"gì, các thuật ngữ khó nghĩa là gì." + quickAnswerTail},
	{ID: "translate", Label: "Dịch", Prompt: "Dịch đoạn văn đang bôi đen sang tiếng Việt; nếu đoạn đã là tiếng " +
		"Việt thì dịch sang tiếng Anh. Chỉ đưa bản dịch." + quickAnswerTail},
}

// FindQuickAction: tra muc theo id (phan biet hoa thuong - id do Core cap, pane gui lai nguyen van).
func FindQuickAction(id string) (QuickAction, bool) {
	for _, action := range QuickActions {
		if action.ID == id {
			return action, true
		}
	}
	return QuickAction{}, false
}
