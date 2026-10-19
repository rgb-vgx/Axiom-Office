# Hướng dẫn sử dụng Axiom Office

Cập nhật: 20/10/2026

Axiom Office là trợ lý AI làm việc **ngay trong tài liệu bạn đang mở**: bạn gõ yêu cầu ("soạn đơn xin nghỉ phép",
"tạo bảng điểm 5 học sinh", "làm 5 slide giới thiệu công ty"), AI tự viết, sửa, định dạng trên chính tài liệu đó,
trước mắt bạn. Bạn luôn xem được AI đã làm gì và hoàn tác được.

Axiom Office chạy trên:

| Nền tảng | Ứng dụng |
|---|---|
| **Windows** | Microsoft Word, Excel, PowerPoint (bản 64-bit) · WPS Office cho Windows (64-bit) |
| **Linux** | WPS Office for Linux · LibreOffice Writer, Calc, Impress |

Phần lớn tính năng giống nhau trên mọi nơi; chỗ nào khác nhau, hướng dẫn ghi rõ. Bảng tổng hợp ở
[mục 11](#11-tính-năng-theo-từng-nền-tảng).

## Mục lục

1. [Trước khi bắt đầu](#1-trước-khi-bắt-đầu)
2. [Cài đặt](#2-cài-đặt)
3. [Thiết lập lần đầu: kết nối máy chủ AI](#3-thiết-lập-lần-đầu-kết-nối-máy-chủ-ai)
4. [Trò chuyện với AI (khung Ask AI)](#4-trò-chuyện-với-ai-khung-ask-ai)
5. [Sửa nhanh bằng chuột phải (WPS trên Linux)](#5-sửa-nhanh-bằng-chuột-phải-wps-trên-linux)
6. [Các nút trên ribbon và menu](#6-các-nút-trên-ribbon-và-menu)
7. [Soát tài liệu — không dùng AI](#7-soát-tài-liệu--không-dùng-ai)
8. [Bộ nhớ AI](#8-bộ-nhớ-ai)
9. [Dữ liệu của bạn](#9-dữ-liệu-của-bạn)
10. [Khi có sự cố](#10-khi-có-sự-cố)
11. [Tính năng theo từng nền tảng](#11-tính-năng-theo-từng-nền-tảng)
12. [Gỡ cài đặt](#12-gỡ-cài-đặt)

---

## 1. Trước khi bắt đầu

Bạn cần:

- **Một máy chủ AI để kết nối.** Có thể là máy chủ của công ty bạn, hoặc tài khoản OpenAI, Anthropic (Claude),
  Google Gemini. Bạn cần **địa chỉ máy chủ**, **tên model** và (thường) một **khoá truy cập**. Không rõ thì hỏi
  bộ phận IT.
- **Windows:** Windows 10/11 64-bit, Office hoặc WPS **bản 64-bit**. Không cần quyền quản trị.
- **Linux:** WPS Office for Linux và/hoặc LibreOffice cài từ kho của hệ điều hành (gói `.deb`/`.rpm`). Bản
  LibreOffice dạng **Snap hoặc Flatpak không dùng được**.

---

## 2. Cài đặt

### 2.1 Trên Windows

1. Giải nén gói cài vào một thư mục **cố định**, ví dụ `C:\Tools\AxiomOffice`. Đừng di chuyển hay xoá thư mục
   này sau khi cài (nếu đã lỡ di chuyển, chạy lại bước 3 ở chỗ mới).
2. Đóng Word, Excel, PowerPoint và WPS.
3. Nhấp đúp **`install.cmd`**. Nếu Windows cảnh báo tệp tải từ Internet, chọn *Run* / *Chạy*.
4. Khi thấy dòng *"Xong. Mở Word/Excel/PowerPoint hoặc WPS: trên ribbon có tab "Axiom Office"."* là xong.

Muốn chắc Office là bản 64-bit trước khi cài: mở PowerShell, chạy
`(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Office\ClickToRun\Configuration').Platform` — kết quả phải là `x64`.

### 2.2 Trên Linux

1. Cài các gói cần thiết (Ubuntu/Debian):

   ```bash
   sudo apt install -y libreoffice python3-uno
   sudo apt install -y libsecret-tools   # tuỳ chọn: cất khoá AI vào kho khoá của hệ thống
   ```

2. **Đóng LibreOffice và WPS**, giải nén gói `axiom-office-linux-x64-<phiên bản>.tar.gz`, rồi trong thư mục đó:

   ```bash
   ./install.sh --systemd
   ```

   `--systemd` cho Axiom Office chạy nền ngay từ khi bạn đăng nhập — **nên dùng nếu bạn dùng WPS**, vì WPS phải
   mở *sau* khi phần nền của Axiom Office đã chạy. Có thể điền sẵn máy chủ AI ngay lúc cài:
   `./install.sh --systemd --endpoint <địa chỉ> --model <tên model> --api-key -` (dấu `-` nghĩa là gõ khoá ở dòng
   tiếp theo, khoá không bị lưu trong lịch sử lệnh).

3. **Riêng WPS, một lần duy nhất:** nếu trình cài in ra lệnh có `sudo … JsApiPlugin=true …`, hãy chạy đúng lệnh
   đó (cần mật khẩu quản trị). Lệnh này cho phép WPS nạp add-in. Trình cài không tự chạy lệnh `sudo`.

**Cài lại hoặc nâng cấp:** chạy `./install.sh` của gói mới như trên. Lịch sử hội thoại và Bộ nhớ AI **được giữ
nguyên**. (Gói phát hành trước ngày 20/10/2026 có lỗi làm mất dữ liệu này khi cài lại — nếu buộc phải dùng gói cũ,
sao lưu tệp `~/.local/share/axiom-office/core/core.db` trước.)

### 2.3 Kiểm tra đã cài xong

- **Word / Excel / PowerPoint / WPS:** trên ribbon có tab **Axiom Office**.
- **LibreOffice:** trên thanh menu có mục **Axiom Office**, và ở thanh bên (sidebar) có thẻ *Axiom Office*.

Không thấy? Xem [mục 10](#10-khi-có-sự-cố).

---

## 3. Thiết lập lần đầu: kết nối máy chủ AI

### 3.1 Windows và LibreOffice: trình hướng dẫn 5 bước

Bấm **Thiết lập…** (Windows: trên ribbon; LibreOffice: menu *Axiom Office → Thiết lập…*). Lần đầu mở khung
Ask AI khi chưa thiết lập, trình hướng dẫn cũng tự hiện.

| Bước | Bạn làm gì |
|---|---|
| 1. Chào mừng | Đọc giới thiệu, bấm **Tiếp tục →** (hoặc *Để sau*). |
| 2. Kiểm tra máy | Xem các mục kiểm tra. Mục nào báo lỗi có sẵn nút sửa ngay bên cạnh. |
| 3. Kết nối máy chủ AI | Chọn nơi cung cấp: *Máy chủ của công ty*, *OpenAI*, *Anthropic (Claude)* hoặc *Google Gemini*. Điền **địa chỉ máy chủ** (OpenAI/Anthropic/Gemini tự điền sẵn), **khoá truy cập**, bấm **Tải danh sách model** rồi chọn model. Bấm **Kiểm tra kết nối** — thấy *"Kết nối tốt …"* là được. Chưa có khoá: bấm *Lấy khoá ở đâu?*. |
| 4. Tính năng | Bật/tắt: *Nhớ những điều tôi đã dặn*, *Tự rút ra điều đáng nhớ sau mỗi lượt*, *Tự kiểm chứng sau khi sửa*, *Hiện phần AI đang suy nghĩ*, *Cho AI xem ảnh trang tài liệu*. Trên Windows có thêm ô khoá **Tìm kiếm web (Tavily)** (tuỳ chọn). |
| 5. Hoàn tất | Xem tóm tắt. Nút **Thử ngay trên tài liệu đang mở** chạy AI thật và **viết một câu chào vào đầu tài liệu đang mở** — chỉ bấm khi đang mở một tài liệu nháp. |

Trên Windows, khoá truy cập được mã hoá theo tài khoản Windows của bạn, nên mỗi máy phải nhập khoá riêng.

### 3.2 WPS trên Linux: màn hình Thiết lập trong khung chat

Bấm **Thiết lập…** trên ribbon (hoặc nút **⋯** trong khung chat → *Thiết lập*). Điền **Máy chủ AI**, **Model**,
**Khoá API**, bật/tắt các tính năng, bấm **Lưu** — có hiệu lực ngay. Để trống ô khoá nghĩa là giữ khoá đã lưu.
Bấm **Kiểm tra Core** để xem phần nền của Axiom Office có đang chạy không, và *Khác ▸ Kiểm tra mô hình* trên ribbon
để thử kết nối với model.

---

## 4. Trò chuyện với AI (khung Ask AI)

### 4.1 Mở khung chat

- **Word / Excel / PowerPoint / WPS:** tab **Axiom Office** → **Ask AI**. Khung chat hiện bên cạnh tài liệu.
- **LibreOffice:** menu **Axiom Office → Hỏi AI**, hoặc mở thẻ *Axiom Office* ở thanh bên.

### 4.2 Gửi yêu cầu

Gõ yêu cầu vào ô *"Nhập yêu cầu cho tài liệu này…"* rồi nhấn **Enter** (hoặc nút **Gửi**). **Shift+Enter** để
xuống dòng. Khung trống có sẵn vài **gợi ý** — bấm vào để điền.

Trong lúc AI làm, mỗi thao tác hiện thành một dòng: **✓** là xong, **✕** là lỗi (kèm lý do). Bấm **Dừng** để dừng
ngay. Xong việc, AI trả lời ngắn gọn đã làm gì.

**Mẹo viết yêu cầu:** nói kết quả bạn muốn, càng cụ thể càng tốt — *"Tạo bảng 5 cột: STT, Họ tên, Toán, Văn,
Trung bình; tô đậm dòng tiêu đề"* tốt hơn *"làm cái bảng"*. Muốn làm tiếp việc lần trước, cứ nói *"làm tiếp"*:
AI nhớ cuộc trò chuyện của từng tài liệu.

### 4.3 Chỉ sửa đúng đoạn bạn bôi đen

**Bôi đen một đoạn rồi mới gửi yêu cầu** thì AI **chỉ được sửa trong đoạn đó**; phần còn lại của tài liệu được
khoá. Có trên **Word (Windows)** và **WPS trên Linux** (văn bản). LibreOffice **chưa có** — trên LibreOffice, AI
có thể sửa cả tài liệu dù bạn đang bôi đen. Muốn AI sửa cả tài liệu thì bỏ bôi đen rồi hỏi lại.

### 4.4 Hoàn tác, chèn câu trả lời

- **Hoàn tác lượt này** (dưới khung chat): bỏ toàn bộ thay đổi AI vừa làm trong lượt đó. Có trên Word; WPS trên
  Linux (văn bản và bảng tính); LibreOffice (cả ba ứng dụng). Nếu bạn đã tự sửa tài liệu sau lượt đó, có thể chỉ
  hoàn tác được một phần — phần còn lại dùng **Ctrl+Z**.
- Trên Word, mỗi thao tác của AI cũng là một bước **Ctrl+Z**.
- **Chèn trả lời**: chèn câu trả lời của AI vào tài liệu (Word và WPS văn bản).

### 4.5 Lập kế hoạch trước

Bật **○ Lập kế hoạch trước** (Windows và WPS trên Linux) khi việc lớn và bạn muốn duyệt trước: AI chỉ đọc tài
liệu và đưa ra kế hoạch từng bước, **chưa sửa gì**. Đồng ý thì bấm **Thực hiện kế hoạch**.

### 4.6 AI xin phép bạn

Trước những việc rủi ro — lưu hay xuất tệp khi bạn không yêu cầu, ghi đè tệp đã có, xoá slide, thay thế hàng loạt
trong tài liệu rất dài, dùng công cụ bên ngoài — khung chat hiện thẻ hỏi **Đồng ý / Từ chối**. Không trả lời trong
2 phút thì coi như từ chối. AI cũng **không tự lưu tài liệu**: bạn tự quyết khi nào lưu và lưu ở đâu.

Việc chạy quá lâu (rất nhiều bước) thì khung chat hỏi bạn có chạy tiếp hay dừng.

### 4.7 Các mục khác trong khung chat

| Mục | Làm gì | Ở đâu |
|---|---|---|
| **Trò chuyện mới** | Bắt đầu lại từ đầu (cuộc cũ vẫn được lưu) | WPS: nút **⋯**. Windows, LibreOffice: liên kết ở đầu khung chat |
| **Hội thoại trước…** | Xem lại, mở, xoá các cuộc trò chuyện cũ | Nút **⋯** (Windows, WPS trên Linux) |
| **Ghi nhớ** | Xem những điều AI đã ghi nhớ — [mục 8](#8-bộ-nhớ-ai) | WPS: nút **⋯**. LibreOffice: liên kết đầu khung. Windows: ribbon *Bộ nhớ AI…* |
| **Nhật ký** | Danh sách mọi thao tác AI đã chạy (giờ, lệnh, kết quả) | Nút **⋯** (Windows: *Nhật ký hoạt động…*; LibreOffice: menu *Nhật ký hoạt động…*) |
| **Đính kèm** | Gửi kèm tối đa 3 tệp Word/Excel/PowerPoint để AI đọc | Chỉ Windows: nút **+ Đính kèm** |

Khi AI ghi nhớ điều gì, khung chat hiện thẻ **Đã ghi nhớ** kèm nút **Xoá** nếu bạn không muốn nó nhớ.

---

## 5. Sửa nhanh bằng chuột phải (WPS trên Linux)

Trong văn bản WPS: **bôi đen một đoạn → chuột phải → Sửa bằng AI ▸** rồi chọn:

| Nhóm | Lệnh | Kết quả |
|---|---|---|
| Sửa vào tài liệu | **Trau chuốt** · **Viết lại** · **Rút gọn** · **Mở rộng** · **Sửa lỗi chính tả** · **Trang trọng hơn** | AI sửa thẳng đoạn bôi đen, giữ nguyên ngôn ngữ của đoạn |
| Chỉ trả lời | **Tóm tắt** · **Giải thích** · **Dịch** | AI trả lời trong khung chat, **không** đụng vào tài liệu. *Dịch*: sang tiếng Việt; đoạn đã là tiếng Việt thì sang tiếng Anh |

Chọn một lệnh là **khung chat mini** hiện ra và làm ngay. Trong khung mini bạn có thể:

- gõ thêm yêu cầu cho chính đoạn đó (ví dụ *"ngắn hơn nữa"*) rồi Enter;
- bấm **Hoàn tác** để bỏ thay đổi vừa rồi, **Dừng** để dừng giữa chừng, **Đóng** (hoặc phím **Esc**) để đóng —
  đang chạy mà đóng thì việc đang làm được dừng trước.

Mục **Hỏi AI về đoạn này** (cũng trong menu chuột phải) mở khung chat mini trống với các nút lệnh nhanh và ô để bạn
tự gõ yêu cầu.

Mọi việc trong khung mini **chỉ làm trên đoạn đang bôi đen**. Nếu thấy *"Hãy bôi đen đoạn văn cần làm trước, rồi
thử lại."* thì bôi đen lại đoạn cần sửa (sau khi AI viết lại cả đoạn, vùng bôi đen có thể mất).

Trên Windows, menu chuột phải của Word chỉ có **Hỏi AI về đoạn này** (mở khung chat lớn). LibreOffice chưa có
menu chuột phải.

---

## 6. Các nút trên ribbon và menu

| Nút | Làm gì | Windows | WPS Linux | LibreOffice |
|---|---|:-:|:-:|:-:|
| **Ask AI** / **Hỏi AI** | Mở khung chat | ✓ | ✓ | ✓ |
| **Tìm web** | Chọn nguồn tìm kiếm web cho AI: *Tắt*, *Tavily*, hoặc công cụ ngoài | ✓ | — | — |
| **Nhìn tài liệu** | Cho AI xem ảnh trang tài liệu để tự kiểm tra trình bày (tốn thêm token) | ✓ | — | trong *Cài đặt nâng cao…* |
| **Soát bố cục / Soát bảng / Soát vùng dữ liệu** | Soát lỗi trình bày, không dùng AI — [mục 7](#7-soát-tài-liệu--không-dùng-ai) | ✓ | ✓ | *Soát tài liệu…* |
| **Bộ nhớ AI…** | Quản lý những điều AI ghi nhớ — [mục 8](#8-bộ-nhớ-ai) | ✓ | ✓ | liên kết *Ghi nhớ* |
| **Kỹ năng…** | Xem các kỹ năng AI đang có cho ứng dụng này; *Nạp lại* sau khi thêm/sửa kỹ năng. AI tự chọn kỹ năng hợp với yêu cầu | ✓ | ✓ | ✓ |
| **Công cụ ngoài…** | Thêm, sửa, tắt công cụ ngoài (MCP) cho AI | ✓ | — | — |
| **Thiết lập…** | Kết nối máy chủ AI — [mục 3](#3-thiết-lập-lần-đầu-kết-nối-máy-chủ-ai) | ✓ | ✓ | ✓ |
| **Ngôn ngữ · Language** | Tiếng Việt / English / tự động | ✓ | — | theo ngôn ngữ LibreOffice |
| **Trạng thái** | Kiểm tra thật: add-in có trả lời không, phần nền có chạy không, đang dùng model nào | ✓ | ✓ | — |
| **Mở nhật ký** | Xem nhật ký kỹ thuật (để gửi cho IT khi có lỗi) | ✓ | ✓ | — |
| Khác ▸ **Kiểm tra mô hình** | Gửi một câu hỏi ngắn tới model để xem có trả lời được không | ✓ | ✓ | — |
| Khác ▸ **Sao chép API URL** | Chép địa chỉ kiểm tra kỹ thuật (dành cho IT) | ✓ | ✓ | — |
| Khác ▸ **Cài đặt nâng cao…** | Tuỳ chọn chi tiết (model dự phòng, ngữ cảnh, …) | ✓ | — | ✓ |
| Khác ▸ **Tắt Agent Core…** | Tắt phần nền (lần hỏi AI sau tự bật lại) | ✓ | — | trong *Cài đặt nâng cao…* |

Mỗi nút *Soát* dành cho một loại tài liệu: bố cục cho trình chiếu, bảng cho văn bản, vùng dữ liệu cho bảng tính.
Trên Windows, ribbon chỉ hiện nút hợp với ứng dụng đang mở. Trên WPS có thể thấy cả ba; bấm nhầm nút của ứng dụng
khác thì có thông báo nút đó dành cho ứng dụng nào.

---

## 7. Soát tài liệu — không dùng AI

Các nút *Soát* chạy ngay, **không gọi AI, không tốn phí, không sửa tài liệu**:

| Nút | Ứng dụng | Tìm gì |
|---|---|---|
| **Soát bố cục** | PowerPoint, WPS / LibreOffice trình chiếu | Chữ tràn khung, khung ra ngoài slide, khung chồng nhau, chữ quá nhỏ, quá nhiều chữ |
| **Soát bảng** | Word, WPS / LibreOffice văn bản | Ô trống, ô tiêu đề quá dài (lẫn đoạn văn) |
| **Soát vùng dữ liệu** | Excel, WPS / LibreOffice bảng tính | Cột thiếu tiêu đề, số lưu dạng chữ, ô lỗi, dữ liệu lạc ngoài bảng, cột lẫn số và chữ |

Muốn AI sửa giúp những lỗi tìm được: mở Ask AI và nhờ *"soát lại và sửa các lỗi trình bày"*.

---

## 8. Bộ nhớ AI

AI có thể **nhớ qua các lần làm việc**: những điều đúng với mọi tài liệu (bạn là ai, cơ quan, người ký, thói quen
trình bày) và những điều riêng của một tài liệu đã lưu. Bạn dặn *"nhớ là …"*, hoặc AI tự rút ra sau mỗi lượt (nếu
bật). AI **không lưu** mật khẩu, số thẻ, số CCCD, khoá API.

Mở **Bộ nhớ AI…** để xem và dọn:

- **Windows:** đủ việc — thêm, sửa, ghim, đặt hạn dùng, xoá tạm, khôi phục (trong 30 ngày), xem lịch sử, xoá vĩnh
  viễn.
- **WPS trên Linux:** lọc theo phạm vi, xoá tạm, khôi phục, xoá vĩnh viễn.
- **LibreOffice:** tìm, xoá.

Chỉ **bạn** sửa hoặc xoá được ghi nhớ; AI chỉ thêm mới. Không muốn dùng: tắt *Nhớ những điều tôi đã dặn* trong
Thiết lập.

---

## 9. Dữ liệu của bạn

- **Nội dung gửi đi đâu:** khi làm việc, phần tài liệu AI cần đọc và yêu cầu của bạn được gửi tới **máy chủ AI mà bạn
  (hoặc công ty) đã chọn** ở bước Thiết lập — không đi đâu khác. Tìm kiếm web (Tavily) chỉ chạy khi bạn có khoá
  Tavily và bật nó.
- **Lưu trên máy bạn:** lịch sử hội thoại, Bộ nhớ AI, nhật ký thao tác nằm trong tệp `core.db`:
  - Windows: `%LOCALAPPDATA%\AxiomOffice\core\core.db`
  - Linux: `~/.local/share/axiom-office/core/core.db`
- **Khoá truy cập AI:** Windows mã hoá theo tài khoản Windows; Linux cất trong kho khoá của hệ thống (nếu có
  `libsecret-tools`), không thì trong tệp cấu hình chỉ bạn đọc được.

---

## 10. Khi có sự cố

| Bạn thấy | Làm gì |
|---|---|
| Không có tab **Axiom Office** (Windows) | Đóng hẳn ứng dụng, chạy lại `install.cmd`, mở lại. Vẫn không thấy: kiểm tra Office/WPS là bản **64-bit** (mục 2.1). Office báo add-in bị vô hiệu hoá sau khi ứng dụng lỗi: chạy lại `install.cmd`. Máy công ty chặn add-in chưa ký số: liên hệ IT. |
| Không có tab **Axiom Office** trong WPS (Linux) | Phần nền phải chạy **trước** khi mở WPS: cài với `--systemd` (mục 2.2), hoặc chạy `systemctl --user start axiom-office-core`, rồi đóng hẳn và mở lại WPS. Đã chạy lệnh `sudo … JsApiPlugin=true` chưa (mục 2.2, bước 3)? |
| *"Chưa kết nối với WPS. Mở một tài liệu trong WPS rồi thử lại."* | Mở (hoặc tạo) một tài liệu trong WPS rồi gửi lại. |
| *"Không gọi được Agent Core. Kiểm tra Core đang chạy trên máy này rồi thử lại."* | Phần nền không chạy. Linux: `systemctl --user start axiom-office-core`. Windows: mở lại ứng dụng (phần nền tự bật khi cần). |
| *"Chưa thiết lập AI — bấm Thiết lập ở trên"* | Làm [mục 3](#3-thiết-lập-lần-đầu-kết-nối-máy-chủ-ai). |
| **Kiểm tra kết nối** báo lỗi | Đọc gợi ý đi kèm. Lỗi **mạng**: máy chủ AI của công ty thì máy phải vào được mạng công ty (VPN / cùng mạng) — đây không phải lỗi khoá. Lỗi **khoá**: nhập lại khoá. Lỗi **model**: chọn lại model trong danh sách. |
| *"Hãy bôi đen đoạn văn cần làm trước, rồi thử lại."* | Bôi đen đoạn cần sửa rồi chọn lại lệnh. |
| *"Đang có một lượt chạy — chờ xong rồi chọn lại …"* | Đợi việc đang làm xong (hoặc bấm **Dừng**) rồi chọn lại. |
| AI làm sai ý | Bấm **Hoàn tác lượt này**, nói rõ hơn rồi gửi lại. |
| Thẻ lỗi *"Có lỗi xảy ra"* | Bấm **Thử lại**; vẫn lỗi thì **Mở Cài đặt** kiểm tra kết nối. |
| LibreOffice không nhận phần mở rộng (Linux) | Kiểm tra LibreOffice không phải bản Snap: `readlink -f "$(command -v soffice)"` ra `/snap/…` thì gỡ bản Snap, cài bản từ kho (`sudo apt install libreoffice`). |

Cần gửi cho IT: bấm **Trạng thái** và **Mở nhật ký**, chụp lại nội dung. Tệp nhật ký nằm ở
`%LOCALAPPDATA%\AxiomOffice\` (Windows) hoặc `~/.local/share/axiom-office/` (Linux).

---

## 11. Tính năng theo từng nền tảng

| Tính năng | Windows (Word/Excel/PowerPoint, WPS) | WPS trên Linux | LibreOffice |
|---|:-:|:-:|:-:|
| Khung chat Ask AI | ✓ | ✓ | ✓ |
| Trình hướng dẫn thiết lập 5 bước | ✓ | màn hình Thiết lập | ✓ |
| AI chỉ sửa đoạn bôi đen | Word | văn bản | — |
| Hoàn tác lượt này | Word | văn bản, bảng tính | ✓ |
| Chèn trả lời | Word | văn bản | — |
| Lập kế hoạch trước | ✓ | ✓ | — |
| Hội thoại trước… | ✓ | ✓ | — |
| Đính kèm tệp | ✓ | — | — |
| Chuột phải → Sửa bằng AI + khung chat mini | — | ✓ | — |
| Chuột phải → Hỏi AI về đoạn này | Word | ✓ | — |
| Soát tài liệu (không dùng AI) | ✓ | ✓ | ✓ |
| Bộ nhớ AI | đầy đủ | xem, xoá, khôi phục | xem, tìm, xoá |
| Tìm web, Công cụ ngoài, Nhìn tài liệu | ✓ | — | Nhìn tài liệu |
| Đổi ngôn ngữ giao diện | ✓ | — | theo LibreOffice |

---

## 12. Gỡ cài đặt

**Windows:** nhấp đúp **`uninstall.cmd`** — giữ lại cấu hình AI. Muốn xoá sạch cả cấu hình, lịch sử hội thoại, Bộ
nhớ AI và nhật ký: mở Command Prompt tại thư mục cài, chạy `uninstall.cmd -Purge`. Sau đó có thể xoá thư mục cài.

**Linux:** đóng LibreOffice và WPS, trong thư mục gói cài chạy:

```bash
./install.sh --uninstall           # gỡ, giữ cấu hình AI, lịch sử hội thoại và Bộ nhớ AI
./install.sh --uninstall --purge   # gỡ và xoá cả cấu hình, dữ liệu
```
