# ĐỀ BÀI: THIẾT KẾ HỆ THỐNG AI AGENT CHO NGÂN HÀNG

## 1. Bối cảnh

Bạn là một Solution Architect đang trình bày đề xuất xây dựng hệ thống AI Agent hỗ trợ chăm sóc khách hàng cho một ngân hàng số.

Đối tượng nghe gồm CTO, kiến trúc sư phần mềm, đội ngũ vận hành và lãnh đạo nghiệp vụ. Họ cần hiểu hệ thống hoạt động như thế nào, dữ liệu đi qua đâu, các rủi ro được kiểm soát ra sao và nên triển khai theo lộ trình nào.

Hãy tạo một bộ slide PowerPoint chuyên nghiệp, có chất lượng thiết kế tương đương một buổi trình bày kiến trúc tại doanh nghiệp công nghệ.

## 2. Mục tiêu

Bộ slide phải trả lời được năm câu hỏi:

1. Hệ thống giải quyết vấn đề gì?
2. Kiến trúc tổng thể gồm những thành phần nào?
3. Một yêu cầu của khách hàng đi qua hệ thống như thế nào?
4. Làm sao hạn chế hallucination, truy cập trái phép và hành động sai nghiệp vụ?
5. Hệ thống sẽ được triển khai theo lộ trình nào?

## 3. Cấu trúc bắt buộc: 10 slide

### Slide 1 — Cover
Tiêu đề: AI Agent Platform for Digital Banking

Subtitle: Secure, Governed and Observable Customer Service Automation

Thiết kế hiện đại, có điểm nhấn thị giác rõ ràng, không dùng một trang bìa chỉ gồm tiêu đề và vài dòng chữ đơn điệu.

### Slide 2 — The Problem
Trình bày ba vấn đề:
- Khối lượng yêu cầu hỗ trợ lớn và lặp lại.
- Quy trình xử lý phân tán giữa nhiều hệ thống.
- Yêu cầu cao về bảo mật, tính chính xác và khả năng truy vết.

Dùng bố cục trực quan, có phân cấp thông tin. Không tự bịa số liệu thống kê.

### Slide 3 — Proposed Solution
Giới thiệu hệ thống AI Agent gồm:
- Customer Channels.
- API Gateway.
- Agent Orchestrator.
- Knowledge Retrieval.
- Banking Business APIs.
- Guardrails and Policy Enforcement.
- Observability.

Thể hiện rõ vai trò của từng thành phần và giá trị của kiến trúc.

### Slide 4 — System Architecture
Tạo sơ đồ kiến trúc có các thành phần và luồng kết nối rõ ràng.

Luồng tổng quát:

Customer → API Gateway → Agent Orchestrator → Policy Check → Retrieval / Banking APIs → Response Validation → Customer.

Observability thu thập trace, log và metric xuyên suốt hệ thống.

Không chỉ liệt kê các thành phần thành những chiếc hộp độc lập. Phải thể hiện quan hệ, hướng luồng dữ liệu và ranh giới tin cậy.

### Slide 5 — Request Lifecycle
Mô tả tuần tự một yêu cầu: khách hàng hỏi về trạng thái giao dịch.

1. Tiếp nhận câu hỏi và xác thực danh tính.
2. Phân loại intent.
3. Truy xuất dữ liệu hoặc kiến thức phù hợp.
4. Kiểm tra quyền truy cập.
5. Gọi API nghiệp vụ được phép.
6. Kiểm tra kết quả và tạo câu trả lời.
7. Ghi nhận trace và audit event.

Dùng sequence diagram hoặc process diagram. Phân biệt rõ bước xử lý của AI với bước xác minh của hệ thống nghiệp vụ.

### Slide 6 — Security and Guardrails
Minh họa các lớp kiểm soát:
- Authentication và authorization.
- Input validation.
- Tool allowlist và quyền tối thiểu.
- Kiểm tra chính sách nghiệp vụ.
- Output validation.
- Audit trail.

Làm rõ rằng LLM không được tự quyết định quyền giao dịch hoặc tự tạo ra trạng thái giao dịch có tính xác thực.

### Slide 7 — Knowledge and Business Data
So sánh hai nhóm dữ liệu:
- Knowledge Base / RAG: chính sách, FAQ, hướng dẫn.
- Banking APIs: số dư, giao dịch và dữ liệu khách hàng cần quyền truy cập.

Minh họa cách hệ thống chọn nguồn dữ liệu phù hợp. Không trình bày RAG như một cơ chế thay thế cho API nghiệp vụ có dữ liệu cập nhật.

### Slide 8 — Failure Handling
Trình bày cách hệ thống xử lý:
- LLM timeout.
- Retrieval không tìm được bằng chứng.
- Banking API lỗi.
- Người dùng không đủ quyền.
- Kết quả không nhất quán hoặc không thể xác minh.

Thể hiện fallback, retry có giới hạn, thông báo phù hợp và chuyển nhân viên hỗ trợ khi cần.

### Slide 9 — Observability and Evaluation
Đề xuất các chỉ số cần theo dõi:
- Task completion rate.
- Grounded answer rate.
- Tool-call success rate.
- Policy violation rate.
- Latency p95.
- Cost per resolved request.

Đây là các chỉ số đề xuất, không phải số liệu vận hành đã đo. Có thể dùng KPI cards và một biểu đồ minh họa, nhưng phải ghi rõ dữ liệu giả lập nếu sử dụng.

### Slide 10 — Implementation Roadmap
Chia thành ba giai đoạn:
- Phase 1: Read-only FAQ và truy xuất kiến thức.
- Phase 2: Tích hợp API nghiệp vụ có kiểm soát.
- Phase 3: Mở rộng workflow, đánh giá và tối ưu vận hành.

Nêu mục tiêu, điều kiện hoàn thành và rủi ro chính của mỗi giai đoạn. Kết thúc bằng một thông điệp kiến trúc ngắn gọn.

## 4. Yêu cầu thiết kế

- Tỷ lệ slide 16:9.
- Phong cách enterprise technology, hiện đại, tinh gọn và nhất quán.
- Có hệ thống typography, màu sắc, spacing và lề thống nhất.
- Mỗi slide chỉ có một thông điệp chính.
- Tiêu đề nên truyền đạt kết luận của slide, không chỉ đặt tên chủ đề.
- Ưu tiên sơ đồ, biểu đồ, timeline và visual hierarchy hơn các đoạn văn dài.
- Không sử dụng một layout duy nhất cho cả 10 slide.
- Không nhồi tất cả nội dung vào các card giống nhau.
- Không sử dụng hình ảnh trang trí không đóng góp cho nội dung.
- Các thành phần kiến trúc, text, đường kết nối, bảng và biểu đồ nên là đối tượng PowerPoint có thể chỉnh sửa khi khả thi.
- Không biến toàn bộ slide thành ảnh chụp.

## 5. Yêu cầu về kỹ thuật và kiểm tra chất lượng

- Tận dụng các khả năng hiện có của Axiom.
- Không tự đặt tọa độ tùy ý nếu hệ thống có thể giải bố cục theo constraints.
- Kiểm tra overflow, overlap, off-slide, chữ quá nhỏ và sự nhất quán giữa các slide.
- Render và xem lại từng slide nếu môi trường hỗ trợ.
- Sửa các lỗi nghiêm trọng trước khi hoàn tất.
- Đảm bảo nội dung không bị mất hoặc thay đổi sai ý trong quá trình sửa thiết kế.

## 6. Tiêu chí nghiệm thu

Bộ slide được đánh giá theo:
1. Storyline rõ ràng và có tính thuyết phục.
2. Bố cục đa dạng nhưng nhất quán.
3. Typography và khoảng trắng hợp lý.
4. Sơ đồ kiến trúc dễ hiểu, đúng hướng luồng.
5. Không có lỗi hình học nghiêm trọng.
6. Không bịa số liệu hoặc khẳng định tính năng chưa được xác minh.
7. Có thể chỉnh sửa nội dung trực tiếp trong PowerPoint.
8. Chất lượng trình bày phù hợp với đối tượng CTO và Solution Architect.

Hãy hoàn thành toàn bộ bộ slide, không chỉ đưa ra outline hoặc mô tả cách thực hiện.
