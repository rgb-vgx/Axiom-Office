package agent

// Lenh nhanh "Sua bang AI" (docs/architecture/ribbon-ui.md muc 2, "Trong khung chat"): nguoi dung boi den mot doan roi
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
	Edits bool
	// Group: muc nam trong menu con cua menu chuot phai - "tone" = "Doi giong ▸" (ribbon.xml: axiomCtxTone_<t|l>). Rong =
	// nam thang trong menu "Sua bang AI". Thu tu trong menu van la thu tu cua bang nay.
	Group  string
	Prompt string
}

// Ket cau chung cua cac muc sua: chi trong vung boi den (khoa vung cua bridge chan phan con lai), va bao lai
// ngan gon da doi gi de nguoi dung biet co can bam "Hoan tac luot nay" khong.
const quickEditTail = " Chỉ sửa trong đoạn đang bôi đen, sửa thẳng vào tài liệu, rồi nói ngắn gọn đã đổi những gì."

// Ket cau cua muc THEM chu (Viet tiep, Dich song ngu): khoa vung cua bridge giu phan chu truoc/sau vung boi den, nen chen
// vao CUOI vung la hop le. Do tren WPS 12.1.2 that (31/10/2026): writer.replaceAll voi "\n" trong chuoi thay tao doan moi
// ngay trong vung khoa, giu dinh dang doan goc, hoan tac mot buoc; writer.insertStyledText thi thay ca vung va khong hieu
// cu phap *nghieng* - nen cau lenh chi dinh replaceAll.
const quickInsertTail = " Giữ nguyên chữ đã bôi đen, chỉ chèn thêm trong vùng bôi đen bằng writer.replaceAll (tìm câu " +
	"cuối của đoạn, đủ dài để chỉ khớp một chỗ, rồi thay bằng chính câu đó + ký tự xuống dòng \\n + phần thêm), không " +
	"sửa phần khác của tài liệu, rồi nói ngắn gọn đã thêm những gì."

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
	// Nhom "Doi giong ▸" (docs/research/doi-chieu-wps-ai.md P1-4, doi chieu kwpsaigc cua WPS AI).
	{ID: "formal", Label: "Trang trọng hơn", Edits: true, Group: "tone", Prompt: "Viết lại đoạn văn đang bôi đen với " +
		"giọng trang trọng, lịch sự, hợp với văn bản công việc; giữ nguyên ý và ngôn ngữ của đoạn." + quickEditTail},
	{ID: "concise", Label: "Ngắn gọn hơn", Edits: true, Group: "tone", Prompt: "Viết lại đoạn văn đang bôi đen cho " +
		"ngắn gọn, súc tích hơn: bỏ từ thừa và câu rườm rà, gộp ý trùng; giữ đủ ý, số liệu, tên riêng và ngôn ngữ của " +
		"đoạn." + quickEditTail},
	{ID: "plain", Label: "Dễ hiểu hơn", Edits: true, Group: "tone", Prompt: "Viết lại đoạn văn đang bôi đen cho dễ " +
		"hiểu với người đọc phổ thông: câu ngắn, từ thông dụng, thuật ngữ khó thì nói lại bằng lời đơn giản; không đổi ý, " +
		"không bỏ số liệu; giữ ngôn ngữ của đoạn." + quickEditTail},
	{ID: "positive", Label: "Tích cực hơn", Edits: true, Group: "tone", Prompt: "Viết lại đoạn văn đang bôi đen với " +
		"giọng tích cực, xây dựng hơn: nhấn vào kết quả đạt được và hướng khắc phục thay vì lời chê trách; không phóng " +
		"đại, không bịa thêm thông tin; giữ ngôn ngữ của đoạn." + quickEditTail},
	{ID: "continue", Label: "Viết tiếp", Edits: true, Prompt: "Viết tiếp sau đoạn văn đang bôi đen một đến hai đoạn " +
		"ngắn: cùng giọng văn và ngôn ngữ của đoạn, nối mạch ý, không lặp lại ý đã có, không bịa số liệu hay sự kiện." +
		quickInsertTail},
	{ID: "bilingual", Label: "Dịch song ngữ", Edits: true, Prompt: "Dịch song ngữ đoạn văn đang bôi đen: ngay dưới " +
		"MỖI đoạn gốc chèn bản dịch của chính đoạn đó (tiếng Việt thì dịch sang tiếng Anh, tiếng nước ngoài thì dịch sang " +
		"tiếng Việt); làm lần lượt từng đoạn; không sửa chữ của bản gốc; nếu có kỹ năng dịch thuật hoặc bảng thuật ngữ " +
		"thì dùng đúng thuật ngữ trong đó." + quickInsertTail},
	{ID: "summarize", Label: "Tóm tắt", Prompt: "Tóm tắt đoạn văn đang bôi đen thành vài ý chính, viết bằng " +
		"ngôn ngữ của đoạn." + quickAnswerTail},
	{ID: "explain", Label: "Giải thích", Prompt: "Giải thích đoạn văn đang bôi đen bằng lời dễ hiểu: ý chính là " +
		"gì, các thuật ngữ khó nghĩa là gì." + quickAnswerTail},
	{ID: "translate", Label: "Dịch", Prompt: "Dịch đoạn văn đang bôi đen sang tiếng Việt; nếu đoạn đã là tiếng " +
		"Việt thì dịch sang tiếng Anh. Chỉ đưa bản dịch." + quickAnswerTail},
}

// QuickEditRule / QuickAnswerRule: noi them vao system prompt khi luot chay den tu menu "Sua bang AI" (nhu PlanModeRule
// cua che do lap ke hoach). Can vi cau lenh trong tin nhan KHONG du: luot that 20/10/2026 - "Sua loi chinh ta" tim
// ra hai loi roi HOI "ban muon toi ap dung khong?" thay vi sua, nguoi dung bam menu ma khong thay gi doi. Nguoi dung
// bam mot lenh sua la da quyet; hoi lai bien mot cu bam thanh hai luot.
const QuickEditRule = "ONE-CLICK EDIT for this turn: the user selected text and picked the menu command quoted in " +
	"their message. That IS the instruction to change the document, not a question - apply it to the selected text " +
	"now with office_action. Do not ask for confirmation, do not offer options or wait for approval: the user undoes " +
	"the whole turn with one click if they dislike the result. For small fixes (spelling, punctuation, a few words) " +
	"use writer.replaceAll/replaceMany inside the selection so formatting is kept; use writer.typeText only to replace " +
	"the whole selection (rewrite, shorten, expand, change tone). If nothing needs to change, change nothing and say " +
	"so in one sentence. Finish with one or two short lines on what changed."

const QuickAnswerRule = "ONE-CLICK ANSWER for this turn: the user selected text and picked the menu command quoted " +
	"in their message. Answer it directly about the selected text; do not ask follow-up questions."

// FindQuickAction: tra muc theo id (phan biet hoa thuong - id do Core cap, pane gui lai nguyen van).
func FindQuickAction(id string) (QuickAction, bool) {
	for _, action := range QuickActions {
		if action.ID == id {
			return action, true
		}
	}
	return QuickAction{}, false
}
