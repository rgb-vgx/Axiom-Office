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
	Group string
	// Document: lenh lam tren CA TAI LIEU (nut ribbon, khong nam trong menu chuot phai): khong can boi den, Core khong khoa
	// vung chon. Chi co trong DocumentActions.
	Document bool
	// Track: mac dinh bat Track Changes cho luot (ben goi gui options.trackChanges thi theo ben goi).
	Track bool
	// App: lenh ca tai lieu chi co nghia o mot ung dung ("wps" Writer, "wpp" Presentation, "et" bang tinh); rong = Writer
	// (nut ribbon doc:<id> cu). Pane/ribbon an nut o ung dung khac.
	App string
	// Input: lenh can nguoi dung GO them yeu cau (Soan van ban: "soan cai gi"). Khac rong = goi y hien trong o nhap; bam
	// nut thi pane khong chay ngay ma cho nguoi dung go, roi gui {quickAction, prompt}. Core ghep: Prompt + yeu cau.
	Input  string
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
	// Hoc thuat (WPS AI "Make Academic", 01/11/2026).
	{ID: "academic", Label: "Học thuật hơn", Edits: true, Group: "tone", Prompt: "Viết lại đoạn văn đang bôi đen với " +
		"văn phong học thuật: khách quan, chính xác, dùng thuật ngữ đúng chuyên ngành, câu chặt chẽ, tránh khẩu ngữ và " +
		"cảm xúc cá nhân; không thêm trích dẫn, số liệu hay nguồn không có trong đoạn; giữ nguyên ý và ngôn ngữ của đoạn." +
		quickEditTail},
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

// Lenh CA TAI LIEU (nut ribbon doc:<id>): hai quy tac tren noi ve "doan boi den" - sai voi lenh lam tren ca bai (truoc
// 01/11/2026 Soat chinh ta, Lam dep bang, Soan van ban deu nhan QuickEditRule). Cung tinh than: bam la da quyet, khong hoi lai.
const DocumentEditRule = "ONE-CLICK DOCUMENT COMMAND for this turn: the user clicked the ribbon command quoted in " +
	"their message (with their request after it, if any). It works on the WHOLE open document, not on a selection. That " +
	"IS the instruction to change the document - do it now with the tools named in the message. Do not ask for " +
	"confirmation or offer options: the user undoes the whole turn with one click. If nothing needs to change, change " +
	"nothing and say so in one sentence. Finish with a short summary of what changed."

const DocumentAnswerRule = "ONE-CLICK DOCUMENT ANSWER for this turn: the user clicked the ribbon command quoted in " +
	"their message. Read the whole open document and answer in the chat; do not change the document and do not ask " +
	"follow-up questions."

// DocumentActions: lenh lam tren CA TAI LIEU, chay tu nut ribbon (docs/research/doi-chieu-wps-ai.md P1-1, P1-3). Cung
// duong {quickAction: id} voi menu "Sua bang AI" nhung khong can boi den va khong nam trong menu do. Cau lenh chi dinh
// lenh bridge cu the vi model hay tu "viet lai cho hay hon" khi duoc bao soat chinh ta, va hay ve lai bang tu dau thay vi
// dinh dang bang co san.
var DocumentActions = []QuickAction{
	{ID: "proofread", Label: "Soát chính tả cả bài", Edits: true, Document: true, Track: true, Prompt: "Soát chính tả " +
		"cả tài liệu đang mở, theo ngôn ngữ của tài liệu. Đọc hết bằng writer.outline với maxChars 4000 (có số đoạn). " +
		"Chỉ sửa lỗi chắc chắn: sai chính tả, sai hoặc thiếu dấu thanh, gõ nhầm, lặp từ, thừa hoặc thiếu khoảng trắng, dấu " +
		"câu đặt sai. Sửa bằng writer.replaceMany, gom nhiều chỗ vào một lần gọi; mỗi find lấy cả cụm quanh chỗ sai để chỉ " +
		"khớp đúng chỗ đó, giữ nguyên chữ hoa/thường. Không viết lại câu, không đổi văn phong hay cách dùng từ vốn đúng; " +
		"không sửa tên riêng, tên cơ quan, số liệu, ngày tháng, trích dẫn, thuật ngữ và từ nước ngoài. Chỗ đáng ngờ mà không " +
		"chắc (câu tối nghĩa, từ có thể dùng sai, thuật ngữ không thống nhất) thì không sửa, thêm ghi chú bằng " +
		"writer.addComment (find = cụm đó) nói lý do và gợi ý. Có kỹ năng chính tả tiếng Việt thì làm theo. Các chỗ sửa " +
		"được ghi bằng Track Changes để người dùng chấp nhận hay từ chối từng chỗ. Cuối cùng báo ngắn gọn số chỗ đã sửa, " +
		"số ghi chú và vài ví dụ; không thấy lỗi thì nói rõ."},
	{ID: "tableTidy", Label: "Làm đẹp bảng", Edits: true, Document: true, Prompt: "Làm đẹp các bảng dữ liệu trong tài " +
		"liệu đang mở mà không đổi chữ trong bảng. Gọi writer.checkTables để biết có những bảng nào. Với mỗi bảng dữ liệu " +
		"gọi writer.formatTable một lần (table = số thứ tự): style \"Table Grid\", font và cỡ chữ giống phần thân bài " +
		"(không rõ thì Times New Roman 13 cho văn bản hành chính, 12 nếu bảng nhiều cột), borders true (WPS không tự kẻ khung theo " +
		"style), headerBold true để hàng tiêu đề đậm và lặp lại khi sang trang, autoFit \"window\" để bảng vừa lề trang. Bảng dàn trang (writer.checkTables ghi " +
		"layout, như phần đầu công văn hay phần chữ ký) thì để nguyên. Sau đó gọi writer.checkTables lần nữa và báo ngắn " +
		"gọn đã làm gì với từng bảng; mục nào trả về trong skipped (WPS không làm được) thì nói rõ."},
	// Soan van ban (Smart Create): cau lenh o day lo CACH viet (mot lan writer.writeContent, khong bia so lieu), con
	// VIET GI la cau nguoi dung go o pane (Input). doc_from_brief chi co khi add-in co du lenh (tools/needs.go).
	{ID: "smartCreate", Label: "Soạn văn bản", Edits: true, Document: true, App: "wps",
		Input: "Bạn muốn soạn văn bản gì? Ví dụ: thông báo nghỉ Tết cho nhân viên, có thời gian nghỉ, lịch trực và lưu ý",
		Prompt: "Soạn một văn bản mới theo yêu cầu của người dùng ở cuối tin nhắn này và viết thẳng vào CUỐI tài liệu đang " +
			"mở. Trước khi viết, đọc nhanh tài liệu bằng writer.outline (maxChars 2000): tài liệu đã có nội dung thì viết " +
			"nối tiếp, không xoá hay sửa phần có sẵn. Văn bản hành chính (tờ trình, công văn, thông báo, biên bản, báo cáo, " +
			"kế hoạch) thì dùng tool doc_from_brief nếu có; còn lại dựng cả bài trong MỘT lần gọi writer.writeContent: tiêu " +
			"đề chính là heading level 1, các mục là heading level 2, đoạn văn {text}, liệt kê {bullets} hoặc {numbers}, số " +
			"liệu so sánh thì {table} kèm caption. Viết bằng ngôn ngữ của yêu cầu, giọng văn hợp với loại văn bản, độ dài " +
			"vừa đủ (khoảng 1-2 trang nếu người dùng không nói). Không bịa tên người, số liệu, ngày tháng, số hiệu văn bản: " +
			"chỗ còn thiếu để \"…\" rồi liệt kê ở cuối câu trả lời những chỗ người dùng cần điền. Có kỹ năng phù hợp (văn " +
			"bản hành chính, thể thức, trình bày) thì làm theo. Cuối cùng báo ngắn gọn đã viết những phần nào.\n\nYêu cầu " +
			"của người dùng: "},
	// Tao slide tu tai lieu: chay o Presentation, doc tai lieu Writer dang mo qua other_document (moi tien trinh WPS mot
	// phien - luot o Presentation khong goi thang writer.* duoc), dung slide bang slides_from_brief.
	{ID: "slidesFromDoc", Label: "Tạo slide từ tài liệu", Edits: true, Document: true, App: "wpp", Prompt: "Dựng bài " +
		"trình chiếu từ tài liệu văn bản (Writer) đang mở. Bước 1: tìm tài liệu văn bản trong danh sách tài liệu khác của " +
		"tool other_document; không có thì dừng, không tạo slide, và nói người dùng mở tài liệu cần chuyển thành slide " +
		"trong WPS Writer rồi bấm lại. Có nhiều tài liệu văn bản thì chọn tài liệu hợp nhất và nói rõ đã chọn tài liệu nào. " +
		"Bước 2: đọc nội dung bằng other_document với writer.outline (maxChars 8000), cần chi tiết thì writer.getText. " +
		"Bước 3: thêm slide vào bài trình chiếu đang mở bằng slides_from_brief: một slide bìa (cover), mỗi mục lớn của " +
		"tài liệu một đến hai slide, số liệu thì table, figures hoặc chart, quy trình thì process, các mốc thời gian thì " +
		"timeline, cuối cùng là slide tóm tắt; thường 6-12 slide tuỳ độ dài tài liệu. Tiêu đề slide là một câu kết luận " +
		"ngắn, các ý ngắn gọn; giữ đúng số liệu và tên riêng của tài liệu, không thêm điều tài liệu không có. Viết bằng " +
		"ngôn ngữ của tài liệu. Bài trình chiếu đã có slide thì thêm vào sau, không xoá slide cũ. Cuối cùng báo đã dựng " +
		"bao nhiêu slide từ tài liệu nào."},
	// Sua thong minh (WPS AI "Smart Edit"): nguoi dung go cach sua ca bai; sua bang Track Changes de duyet tung cho.
	{ID: "smartEdit", Label: "Sửa thông minh", Edits: true, Document: true, Track: true, App: "wps",
		Input: "Bạn muốn sửa tài liệu thế nào? Ví dụ: viết gọn lại phần mở đầu, thống nhất cách xưng hô, đổi giọng trang trọng hơn",
		Prompt: "Sửa tài liệu đang mở theo yêu cầu của người dùng ở cuối tin nhắn này. Đọc hết bằng writer.outline với " +
			"maxChars 4000 (có số đoạn), cần chi tiết thì writer.getText. Chỉ sửa đúng phần và đúng kiểu người dùng yêu " +
			"cầu; phần khác giữ nguyên. Đổi câu chữ trong đoạn có sẵn bằng writer.replaceMany (mỗi find lấy cả câu hoặc cả " +
			"cụm để chỉ khớp đúng chỗ đó, giữ chữ hoa/thường); thêm nội dung mới thì writer.writeContent hoặc chèn sau câu " +
			"bằng writer.replaceAll (câu đó + ký tự xuống dòng \\n + phần thêm); đổi kiểu tiêu đề thì writer.setStyle. Không " +
			"bịa số liệu, tên riêng, ngày tháng. Các chỗ sửa được ghi bằng Track Changes để người dùng chấp nhận hay từ " +
			"chối từng chỗ. Cuối cùng báo ngắn gọn đã sửa những gì.\n\nYêu cầu của người dùng: "},
	// Tom tat ca bai (WPS AI "Summarize" tren ca tai lieu): chi tra loi trong khung chat.
	{ID: "summarizeDoc", Label: "Tóm tắt cả bài", Document: true, App: "wps", Prompt: "Tóm tắt toàn bộ tài liệu đang " +
		"mở. Đọc hết bằng writer.outline với maxChars 8000, phần nào cần chi tiết thì writer.getText. Trả lời trong khung " +
		"chat, bằng ngôn ngữ của tài liệu: một câu nêu tài liệu nói về gì, rồi các ý chính theo từng mục lớn (mỗi ý một " +
		"dòng ngắn, giữ đúng số liệu, tên riêng, mốc thời gian), cuối cùng là việc cần làm hoặc kết luận nếu tài liệu có. " +
		"Không thêm điều tài liệu không có. Không sửa tài liệu."},
	// Dich song ngu ca bai (WPS AI "Parallel Translate"): ban dich nam ngay duoi tung doan goc.
	{ID: "translateDoc", Label: "Dịch song ngữ cả bài", Edits: true, Document: true, App: "wps", Prompt: "Dịch song ngữ " +
		"toàn bộ tài liệu đang mở: ngay dưới mỗi đoạn văn và mỗi tiêu đề, chèn bản dịch của chính đoạn đó, giữ đoạn gốc. " +
		"Tài liệu tiếng Việt thì dịch sang tiếng Anh; ngôn ngữ khác thì dịch sang tiếng Việt. Đọc hết bằng writer.outline " +
		"với maxChars 8000 (có số đoạn). Chèn bằng writer.replaceMany, mỗi lần tối đa vài chục đoạn: find là câu cuối của " +
		"đoạn (đủ dài để chỉ khớp một chỗ), replace là chính câu đó + ký tự xuống dòng \\n + bản dịch của cả đoạn. Bỏ qua " +
		"đoạn trống, ô bảng, số liệu đứng riêng, và đoạn đã có bản dịch ngay dưới. Dịch sát nghĩa, giữ tên riêng, số " +
		"liệu, đơn vị; thuật ngữ dịch thống nhất trong cả bài. Cuối cùng báo đã dịch bao nhiêu đoạn và sang ngôn ngữ nào."},
}

// FindQuickAction: tra muc theo id trong menu "Sua bang AI" roi trong lenh ca tai lieu (phan biet hoa thuong - id do Core
// cap, pane gui lai nguyen van).
func FindQuickAction(id string) (QuickAction, bool) {
	if action, ok := findIn(QuickActions, id); ok {
		return action, true
	}
	return findIn(DocumentActions, id)
}

func findIn(list []QuickAction, id string) (QuickAction, bool) {
	for _, action := range list {
		if action.ID == id {
			return action, true
		}
	}
	return QuickAction{}, false
}
