# Bản đối chiếu (oracle) cho kiểm thử MCP

Ba gói trong thư mục này là **bản đối chiếu**, không phải sản phẩm:

| Gói | Bản đối chiếu cho |
|---|---|
| `word-mcp/` (`word_mcp`, 20 tool) | `doc_*` |
| `excel-mcp/` (`excel_mcp`, 17 tool) | `excel_*` |
| `ppt-mcp/` (`ppt_mcp`, 16 tool) | `ppt_*` |

`tests/mcp-host/test_mcp_host.py` chạy cùng một kịch bản trên MCP server Go và trên các gói
này (qua `python-docx`/`openpyxl`/`python-pptx`) rồi so từng kết quả. Không phát triển thêm
tính năng ở đây; cài đặt mới dùng [MCP server trong Core Go](../../core-go/README.md).

Quy ước:

- Không đưa ba gói này vào bản phát hành.
- Không import từ mã sản phẩm. Chiều dùng duy nhất là test parity import từ đây.
- `excel_mcp` là nơi **duy nhất** trong repo còn giữ `excel_query` (truy vấn SQL qua DuckDB),
  nên phải giữ lại dù MCP server Go không có tool này.

## Chạy

```bash
make test-oracle
```

Mỗi gói chạy một lượt `python3 -m unittest discover -s tests` ngay trong thư mục của gói;
một gói đỏ là cả đích đỏ. Chạy lẻ một gói:

```bash
cd tests/oracle/word-mcp && python3 -m unittest discover -s tests
cd tests/oracle/excel-mcp && python3 -m unittest discover -s tests
cd tests/oracle/ppt-mcp && python3 -m unittest discover -s tests
```
