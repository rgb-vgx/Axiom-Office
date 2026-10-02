"""Test làn live LibreOffice (LibreOffice_arch.md mục 10, giai đoạn L1): mọi lệnh bridge trên Writer/Calc/Impress thật.

Khác bản Office: bridge nằm trong extension Python UNO (src\\AxiomOffice.LibreOffice), phải cài trước
(powershell -File scripts\\libreoffice.ps1 -Install) và LibreOffice phải được tắt khi cài.
Script tự mở LibreOffice bằng đúng profile người dùng (để extension được nạp), chạy xong tự đóng.
Port đã có bridge thì DỪNG (bộ test gọi *closeAll - đóng mọi tài liệu KHÔNG lưu).

    python tests/live/test_live_libreoffice.py [--apps writer,calc,impress] [--ui] [--ai] [--keep]

--ui: mở bản có cửa sổ (kiểm tra đường đi qua giao diện) thay vì --headless.
--ai: chạy ai.ask trên Writer (gọi LLM thật qua Agent Core; tốn token).
Chỉ dùng thư viện chuẩn.
"""
from __future__ import annotations

import argparse
import base64
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import zipfile

# Console Windows mac dinh la cp1252: ten lenh/du lieu trong test co tieng Viet.
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from test_live_commands import Bridge, RESULTS, check, tiny_png  # noqa: E402

IS_WINDOWS = sys.platform.startswith("win")
SOFFICE = r"C:\Program Files\LibreOffice\program\soffice.exe" if IS_WINDOWS else (shutil.which("soffice") or "/usr/bin/soffice")
PORTS = {"writer": 47851, "calc": 47852, "impress": 47853}
KINDS = {"writer": "wps", "calc": "et", "impress": "wpp"}


def health(port):
    try:
        request = urllib.request.Request("http://127.0.0.1:%d/health" % port)
        with urllib.request.urlopen(request, timeout=2) as response:
            return json.loads(response.read().decode("utf-8")).get("result")
    except Exception:
        return None


def soffice_processes():
    if not IS_WINDOWS:
        out = subprocess.run(["pgrep", "-f", "soffice.bin"], capture_output=True, text=True).stdout
        return out.split()
    out = subprocess.run(["tasklist", "/FI", "IMAGENAME eq soffice.bin", "/NH"], capture_output=True, text=True).stdout
    return [line.split()[1] for line in out.splitlines() if line.strip().startswith("soffice.bin")]


def launch(ui):
    for port in PORTS.values():
        if health(port):
            raise SystemExit("port %d đã có bridge LibreOffice đang mở - đóng LibreOffice rồi chạy lại (test gọi *closeAll)" % port)
    args = [SOFFICE, "--norestore", "--nologo", "--nofirststartwizard"]
    if not ui:
        args.append("--headless")
    subprocess.Popen(args)
    deadline = time.time() + 90
    while time.time() < deadline:
        info = health(PORTS["writer"])
        if info:
            return info["pid"]
        time.sleep(1.0)
    raise SystemExit("bridge không lên ở port %d (extension đã cài chưa? chạy scripts\\libreoffice.ps1 -Install hoặc "
                     "scripts/libreoffice.sh install)" % PORTS["writer"])


def close_app(pid):
    if not IS_WINDOWS:
        import signal

        try:
            os.kill(int(pid), signal.SIGTERM)
        except OSError:
            return
        deadline = time.time() + 20
        while time.time() < deadline:
            if str(pid) not in soffice_processes():
                return
            time.sleep(0.5)
        try:
            os.kill(int(pid), signal.SIGKILL)
        except OSError:
            pass
        return
    subprocess.run(["taskkill", "/PID", str(pid)], capture_output=True)
    deadline = time.time() + 20
    while time.time() < deadline:
        if str(pid) not in soffice_processes():
            return
        time.sleep(0.5)
    # Chi kill tien trinh do test mo (pid lay tu /health), khong dung taskkill /IM.
    subprocess.run(["taskkill", "/PID", str(pid), "/F"], capture_output=True)


def test_port_guards(b, kind):
    """Token, Origin, JSON, lệnh của app khác (giống bản Windows)."""
    try:
        request = urllib.request.Request("http://127.0.0.1:%d/commands" % b.port)
        with urllib.request.urlopen(request, timeout=5) as response:
            status = response.status
    except urllib.error.HTTPError as ex:
        status = ex.code
    check(status == 401, "LibreOffice %s thiếu token -> 401" % kind, status)
    other = "et.readRange" if KINDS[kind] != "et" else "writer.getText"
    error = b.cmd(other, {"range": "A1", "maxChars": 10}, expect_ok=False, key="wrong-kind action")
    check("is not available on the %s port" % KINDS[kind] in (error or ""), "LibreOffice %s từ chối lệnh của app khác" % kind, error)
    unknown = b.cmd("nosuch.action", expect_ok=False, key="unknown action")
    check("unknown action" in (unknown or ""), "LibreOffice %s báo lệnh lạ" % kind, unknown)


def test_writer(b, out, png, run_ai):
    b.cmd("app.info", key="app.info (trước khi có tài liệu)")
    b.cmd("writer.newDocument")
    b.cmd("writer.appendText", {"text": "Báo cáo tuần 39\nDoanh thu tăng 12%."})
    text = b.cmd("writer.getText", key="writer.getText")
    check(text and "Báo cáo tuần 39" in text["text"] and text["totalChars"] > 20, "Writer getText thấy nội dung vừa ghi", text)
    short = b.cmd("writer.getText", {"maxChars": "12.0"}, key="writer.getText string number")
    check(short and len(short["text"]) == 12 and short["truncated"], "Writer maxChars nhận số dạng chuỗi '12.0'", short)
    b.cmd("writer.selection")
    b.cmd("writer.typeText", {"text": " Gõ tại con trỏ."})
    b.cmd("writer.insertStyledText", {"text": " Chữ đỏ đậm", "bold": True, "color": "#C00000", "size": 14})
    error = b.cmd("writer.insertStyledText", {"text": "x", "bold": "có"}, expect_ok=False, key="writer.insertStyledText bool text")
    check("'bold' must be true or false, got 'có'" in (error or ""), "Writer lỗi bool nêu tên tham số", error)
    b.cmd("writer.heading", {"level": 1, "text": "Tiêu đề một"})
    levels = (b.cmd("writer.getText", record=False) or {}).get("text", "")
    check("Tiêu đề một" in levels, "Writer heading thêm được tiêu đề", levels[-60:])
    # Con tro o cuoi mot doan co chu: tieu de phai thanh doan rieng, khong noi vao doan cu.
    check("\nTiêu đề một" in levels, "Writer heading tạo đoạn riêng khi con trỏ đang ở cuối đoạn có chữ", levels[-80:])
    b.cmd("writer.formatSelection", {"italic": True, "alignment": "center"})
    b.cmd("writer.setParagraphAlignment", {"alignment": "justify"})
    table = b.cmd("writer.insertTable", {"values": [["Tên", "Điểm"], ["An", 9.5], ["Bình", 8]], "style": "Grid Table 4 - Accent 1"})
    check(table and table.get("rows") == 3 and table.get("cols") == 2 and table.get("filled") == 6, "Writer insertTable suy ra 3x2 từ values", table)
    check(table and table.get("style") == "Box List Blue", "Writer ánh xạ style Word sang style LibreOffice gần nhất", table)
    b.cmd("writer.insertTable", {"cols": "2", "values": {"item": [{"item": {"item": ["STT", "Tên"]}}, {"item": {"item": ["1", "An"]}}]}},
          key="writer.insertTable double-wrapped rows")
    wrapped = b.cmd("writer.insertTable", {"rows": {"item": 2}, "cols": [3]}, key="writer.insertTable wrapped ints")
    check(wrapped and wrapped.get("rows") == 2 and wrapped.get("cols") == 3, "Writer insertTable nhận rows {item:2}, cols [3]", wrapped)
    rows_data = b.cmd("writer.insertTable", {"cols": "2", "rows": {"item": [{"item": ["a", "b"]}, {"item": ["c", "d"]}]}},
                      key="writer.insertTable data in rows")
    check(rows_data and rows_data.get("filled") == 4, "Writer insertTable dùng dữ liệu đặt nhầm vào rows", rows_data)
    wide = b.cmd("writer.insertTable", {"values": [["w", "x", "y", "z"], ["1", "2", "3", "4"]]}, key="writer.insertTable wide after narrow")
    check(wide and wide.get("filled") == 8, "Writer insertTable bảng rộng hơn ngay sau bảng hẹp (không bị gộp)", wide)
    placed = b.cmd("writer.insertTable", {"values": [["ONE_C1", "ONE_C2"]]}, key="writer.insertTable then type")
    b.cmd("writer.typeText", {"text": "SAU_BANG"}, record=False)
    whole = (b.cmd("writer.getText", record=False) or {}).get("text", "")
    check(placed and "SAU_BANG" in whole and whole.index("SAU_BANG") > whole.index("ONE_C2"), "Writer chữ gõ sau insertTable nằm SAU bảng", whole[-120:])
    b.cmd("writer.insertTable", {"values": [["Lịch họp áp dụng cho toàn bộ phòng ban - thời gian có thể điều chỉnh.Thứ", "Giờ", "Nội dung"],
                                            ["Hai", "8h", ""]]}, key="writer.insertTable qa")
    tables_qa = b.cmd("writer.checkTables") or {}
    check({"long-header-cell", "empty-cells"} <= {i.get("type") for i in tables_qa.get("issues", [])},
          "Writer checkTables bắt ô tiêu đề lẫn đoạn văn + ô trống", tables_qa)
    styled = b.cmd("writer.formatTable", {"font": "Liberation Serif", "size": "12", "headerFill": "#1F4E79", "headerColor": "#FFFFFF",
                                           "bandFill": "#DEEAF6", "borderColor": "#8EAADB", "alignment": "center", "autoFit": "window"},
                   key="writer.formatTable last table")
    check(styled and "headerFill" in styled.get("applied", []) and "bandFill" in styled.get("applied", []),
          "Writer formatTable định dạng bảng có sẵn", styled)
    unsupported = b.cmd("writer.formatTable", {"autoFit": "content"}, key="writer.formatTable autoFit content")
    check(unsupported and any("autoFit" in item for item in unsupported.get("skipped", [])),
          "Writer formatTable báo phần host không hỗ trợ trong 'skipped' (autoFit content)", unsupported)
    borderless = b.cmd("writer.formatTable", {"borders": False}, key="writer.formatTable borders false")
    check(borderless and "borders" in borderless.get("applied", []), "Writer formatTable bỏ viền bảng", borderless)
    bad = b.cmd("writer.formatTable", {"table": 99}, expect_ok=False, key="writer.formatTable bad index")
    check("'table' must be between 1 and" in (bad or ""), "Writer formatTable báo chỉ số bảng sai", bad)
    b.cmd("writer.insertTable", {}, expect_ok=False, key="writer.insertTable missing")
    b.cmd("writer.appendText", {"text": "\nTrân trọng,\nKính mong phản hồi."})
    # LibreOffice khong tim xuyen doan: chi ngat doan o CUOI chuoi tim ('$' = cuoi doan).
    multi = b.cmd("writer.replaceAll", {"find": "Kính mong phản hồi.\n", "replace": "Hết.\n"})
    joined = b.cmd("writer.getText", record=False) or {}
    check(multi and multi.get("replaced") and "Kính mong" not in joined.get("text", ""), "Writer replaceAll khớp ngắt đoạn cuối chuỗi tìm ('$')", multi)
    middle = b.cmd("writer.replaceAll", {"find": "Trân trọng,\nKính", "replace": "x"}, expect_ok=False, key="writer.replaceAll newline in middle")
    check("line break at its end" in (middle or ""), "Writer replaceAll báo rõ khi 'find' có ngắt đoạn ở giữa", middle)
    b.cmd("writer.insertPageBreak")
    b.cmd("writer.insertImage", {"path": png, "width": 40})
    b.cmd("writer.insertHyperlink", {"url": "https://example.com", "text": "ví dụ"})
    baseline = (b.cmd("writer.getText", record=False) or {}).get("text")
    b.cmd("writer.appendText", {"text": "\nLượt AI dòng 1"}, record=False)
    b.cmd("writer.appendText", {"text": "\nLượt AI dòng 2"}, record=False)
    b.cmd("writer.replaceAll", {"find": "Lượt AI", "replace": "LƯỢT AI"}, record=False)
    undone = b.cmd("writer.undo", {"count": 3})
    restored = (b.cmd("writer.getText", record=False) or {}).get("text")
    check(undone and undone.get("undone") == 3 and restored == baseline,
          "Writer hoàn tác cả lượt (3 thao tác AI = 3 bước Undo) trả lại nội dung cũ", (restored or "")[-80:])
    shot = b.cmd("app.screenshot", {"maxWidth": 640}, record=False) or {}
    raw = base64.b64decode(shot.get("base64", "") or "")
    check(raw[:8] == b"\x89PNG\r\n\x1a\n" and 0 < shot.get("width", 0) <= 656, "app.screenshot xuất PNG thật (PixelWidth của filter chỉ là gợi ý)",
          {k: shot.get(k) for k in ("width", "height", "mime")})
    pdf = os.path.join(out, "libreoffice-writer.pdf")
    b.cmd("writer.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 1000, "Writer exportPdf tạo file", pdf)
    docx = os.path.join(out, "libreoffice-writer.docx")
    b.cmd("writer.saveAs", {"path": docx})
    b.cmd("writer.save")
    again = b.cmd("writer.open", {"path": docx})
    check(again and again.get("alreadyOpen") and not again.get("opened"),
          "Writer open mở lại chính file đang mở thì kích hoạt cửa sổ (không hỏi hộp thoại)", again)
    b.cmd("writer.closeAll")
    reopened = b.cmd("writer.open", {"path": docx}, key="writer.open after closeAll")
    check(reopened and reopened.get("opened") and reopened.get("name") == "libreoffice-writer.docx", "Writer mở file đã lưu", reopened)
    b.cmd("app.info", key="app.info (có tài liệu)")
    b.cmd("writer.closeAll")
    if run_ai:
        b.cmd("writer.newDocument", record=False)
        b.ask("Thêm một dòng 'Kiểm tra ai.ask' vào cuối tài liệu", "activeDocument")
        text = b.cmd("writer.getText", record=False) or {}
        check("Kiểm tra ai.ask" in text.get("text", ""), "Writer ai.ask (qua Agent Core) thêm được dòng mới", text.get("text", "")[-200:])


def test_calc(b, out):
    b.cmd("et.newWorkbook")
    b.cmd("et.listSheets")
    b.cmd("et.writeRange", {"range": "A1", "values": [["Tên", "Điểm", "Nhân đôi"], ["An", 9.5, "=B2*2"], ["Bình", 8, "=B3*2"]]})
    b.cmd("et.writeRange", {"range": "E1", "values": {"item": [{"item": ["bọc", 1]}]}}, key="et.writeRange wrapped")
    b.cmd("et.writeRange", {"range": "G1", "values": "[[1,2],[3,4]]"}, key="et.writeRange json string")
    b.cmd("et.writeRange", {"range": "A10", "values": {"foo": 1}}, expect_ok=False, key="et.writeRange invalid")
    b.cmd("et.writeRange", {"range": "A10"}, expect_ok=False, key="et.writeRange missing values")
    values = b.cmd("et.readRange", {"range": "A1:C3"})
    check(values and values["values"][1][2] == 19, "Calc công thức =B2*2 tính ra 19 (không bị ghi đè)", values)
    check(values and values["values"][2][0] == "Bình" and values["values"][2][1] == 8, "Calc đọc lại đúng chuỗi và số", values)
    empty = b.cmd("et.writeRange", {"range": "A5", "values": [["x", None, "y"]]})
    row = (b.cmd("et.readRange", {"range": "A5:C5"}, record=False) or {}).get("values", [[]])[0]
    check(empty and row == ["x", None, "y"], "Calc ô null để trống (không sinh lỗi #N/A)", row)
    b.cmd("et.formatRange", {"range": "A1:C1", "bold": True, "fillColor": "#FFFF00", "fontColor": "#0000FF", "numFmt": "0.00", "horizontal": "center", "wrap": True})
    table_qa = b.cmd("et.checkRange", {"range": "A1:C3"}, key="et.checkRange table") or {}
    check(table_qa.get("rows") == 3 and table_qa.get("cols") == 3 and "outside-table" not in {i.get("type") for i in table_qa.get("issues", [])},
          "Calc checkRange: 3x3, không báo dữ liệu lạc khi chỉ soát bảng", table_qa)
    sheet_qa = b.cmd("et.checkRange", {}) or {}
    check("outside-table" in {i.get("type") for i in sheet_qa.get("issues", [])}, "Calc checkRange bắt dữ liệu lạc ngoài bảng", sheet_qa)
    b.cmd("et.writeRange", {"range": "M1", "values": [["Mã", "Giá"], ["x", "12"], ["y", "15"]]}, key="et.writeRange numbers as text")
    text_qa = b.cmd("et.checkRange", {"range": "M1:N3"}, key="et.checkRange numbers as text") or {}
    check("numbers-as-text" in {i.get("type") for i in text_qa.get("issues", [])}, "Calc checkRange bắt số lưu dạng chữ", text_qa)
    # Quản lý sheet: không có et.addSheet thì agent không dựng được sổ nhiều sheet trên tài liệu đang mở
    # (log 02/10/2026: model thử ~30 tên lệnh tự nghĩ ra rồi bỏ cuộc).
    added = b.cmd("et.addSheet", {"name": "Raw_Data"}) or {}
    check(added.get("sheet") == "Raw_Data" and "Raw_Data" in (added.get("sheets") or []),
          "Calc addSheet thêm sheet có tên", added)
    auto = b.cmd("et.addSheet", {}) or {}
    check(auto.get("sheet", "").startswith("Sheet"), "Calc addSheet bỏ trống tên thì đặt tên SheetN", auto)
    sheets = b.cmd("et.listSheets") or {}
    check(sheets.get("activeSheet") == auto.get("sheet"), "Calc addSheet chuyển sang sheet mới", sheets)
    renamed = b.cmd("et.renameSheet", {"sheet": auto.get("sheet"), "name": "Data_Cleaning"}) or {}
    check("Data_Cleaning" in (renamed.get("sheets") or []), "Calc renameSheet đổi tên được", renamed)
    b.cmd("et.addSheet", {"name": "Raw_Data"}, expect_ok=False, key="et.addSheet trùng tên")
    b.cmd("et.renameSheet", {"sheet": "Không có", "name": "X"}, expect_ok=False, key="et.renameSheet sheet lạ")
    b.cmd("et.activateSheet", {"sheet": "Sheet1"})
    b.cmd("et.activateSheet", {"sheet": "Không có"}, expect_ok=False, key="et.activateSheet unknown")
    # Biểu đồ: Test 1 và Test 4 của bộ benchmark đều yêu cầu "use charts"; không có lệnh này thì
    # Dashboard chỉ là bảng số.
    chart = b.cmd("et.addChart", {"sheet": "Sheet1", "range": "A1:C3", "type": "line", "title": "Điểm",
                                  "anchor": "E10"}) or {}
    check(chart.get("chart") and chart.get("type") == "line" and chart.get("range") == "A1:C3",
          "Calc addChart tạo biểu đồ", chart)
    # Kiểu phải là kiểu ĐỌC LẠI được từ tài liệu, không phải điều vừa xin (một bản LibreOffice từng từ
    # chối đổi kiểu; khi đó typeApplied phải là false chứ không được hứa suông).
    check(chart.get("typeApplied") and chart.get("diagram") == "line",
          "Calc addChart đặt đúng kiểu và đọc lại được", chart)
    check(chart.get("titleApplied"), "Calc addChart đặt được tiêu đề (HasMainTitle)", chart)
    charts = b.cmd("et.listCharts", {"sheet": "Sheet1"}) or {}
    names = [item.get("name") for item in (charts.get("charts") or [])]
    check(chart.get("chart") in names, "Calc listCharts thấy biểu đồ vừa tạo", charts)
    check(any(item.get("diagram") == "line" for item in (charts.get("charts") or [])),
          "Calc listCharts đọc lại được kiểu biểu đồ", charts)
    b.cmd("et.addChart", {"sheet": "Sheet1", "range": "A1:C3", "type": "donut"}, expect_ok=False,
          key="et.addChart kiểu lạ")
    b.cmd("et.addChart", {"sheet": "Sheet1", "type": "line"}, expect_ok=False, key="et.addChart thiếu range")

    # Tên chart duy nhất theo TÀI LIỆU, không theo sheet. Thêm chart ở sheet khác mà để tự đặt tên thì phải
    # nhảy qua tên đã dùng - trước đây nó đâm vào "Chart1" rồi ném một lỗi pyuno không đọc được, và vì tên
    # tự sinh luôn bắt đầu lại từ Chart1 nên MỌI lần thêm chart trên sheet chưa có chart đều hỏng.
    b.cmd("et.addSheet", {"name": "ChartProbe"})
    second = b.cmd("et.addChart", {"sheet": "ChartProbe", "range": "A1:C3"}, record=False) or {}
    check(second.get("chart") and second.get("chart") != chart.get("chart"),
          "Calc addChart ở sheet khác tự đặt tên chưa dùng (tên chart duy nhất theo tài liệu)", second)
    clash = b.cmd("et.addChart", {"sheet": "ChartProbe", "range": "A1:C3", "name": chart.get("chart")},
                  expect_ok=False, record=False)
    check(isinstance(clash, str) and "already exists" in clash,
          "Calc addChart tên đã dùng ở sheet khác -> lỗi đọc được, không phải lỗi pyuno", clash)
    b.cmd("et.activateSheet", {"sheet": "Sheet1"})

    # Điền bằng công thức: chi phí token của agent tính theo TỪNG Ô nó viết ra, nên bảng nghìn dòng phải
    # đi bằng một công thức + một lệnh điền, không phải gửi từng ô.
    filled = b.cmd("et.fillRange", {"sheet": "Sheet1", "range": "P1:P20", "formula": "=ROW()*2"}) or {}
    check(filled.get("filled") == 20 and filled.get("sheet") == "Sheet1", "Calc fillRange điền đủ số ô", filled)
    column = (b.cmd("et.readRange", {"range": "P1:P20", "sheet": "Sheet1"}, record=False) or {}).get("values") or []
    check(len(column) == 20 and column[0][0] == 2 and column[19][0] == 40,
          "Calc fillRange dịch công thức theo dòng (ROW()*2 -> 2..40)", column[:2] + column[-2:])

    b.cmd("et.writeRange", {"sheet": "Sheet1", "range": "R1", "values": [[1, 10], [2, 20], [3, 30]]})
    b.cmd("et.fillRange", {"sheet": "Sheet1", "range": "T1:T3", "formula": "=R1*2"})
    shifted = (b.cmd("et.readRange", {"range": "T1:T3", "sheet": "Sheet1"}, record=False) or {}).get("values") or []
    check([row[0] for row in shifted] == [2, 4, 6],
          "Calc fillRange dịch tham chiếu TƯƠNG ĐỐI (=R1*2 -> =R2*2, =R3*2)", shifted)

    # Khối 2D: điền ngang rồi điền dọc, mỗi cột lấy hàng đầu đã điền xong làm nguồn.
    b.cmd("et.fillRange", {"sheet": "Sheet1", "range": "V1:W2", "formula": "=ROW()+COLUMN()"})
    block = (b.cmd("et.readRange", {"range": "V1:W2", "sheet": "Sheet1"}, record=False) or {}).get("values") or []
    check(block == [[23, 24], [24, 25]], "Calc fillRange điền được cả khối 2D", block)
    b.cmd("et.fillRange", {"sheet": "Sheet1", "formula": "=ROW()"}, expect_ok=False, key="et.fillRange thiếu range")

    # Công thức trỏ vào ô TRỐNG: không có gì báo lỗi, phép tính coi ô trống là 0. Đây là cách một ô điều
    # khiển "chết" mà không ai biết (xem tests/bench/results/test4-dung-sai.md).
    b.cmd("et.writeRange", {"range": "Z1:AA2", "sheet": "Sheet1",
                            "values": [["H1", "H2"], ["=Z50", "=AA1"]]})
    refs = {i.get("type") for i in (b.cmd("et.checkRange", {"range": "Z1:AA2", "sheet": "Sheet1"}) or {}).get("issues", [])}
    check("empty-reference" in refs, "Calc checkRange bắt công thức trỏ vào ô trống (=Z50)", sorted(refs))
    detail = [i for i in (b.cmd("et.checkRange", {"range": "Z1:AA2", "sheet": "Sheet1"}, record=False) or {}).get("issues", [])
              if i.get("type") == "empty-reference"]
    check(detail and any("Z50" in str(d) for d in detail[0].get("examples", [])),
          "Calc empty-reference chỉ đúng ô trống (Z50), không báo ô có nội dung (AA1)", detail)
    # AA2 = =AA1 mà AA1 có nội dung -> không được báo. Đây là chiều chống kêu oan.
    clean = {i.get("type") for i in (b.cmd("et.checkRange", {"range": "AA1:AA2", "sheet": "Sheet1"},
                                           record=False) or {}).get("issues", [])}
    check("empty-reference" not in clean, "Calc checkRange KHÔNG báo oan khi công thức trỏ vào ô có nội dung", sorted(clean))

    # Định dạng điều kiện: bốn bài benchmark đều cần (thanh màu theo ngưỡng, tô đỏ chỗ lệch kế hoạch).
    # Hai lần đo trước kết luận "không có đường nào" vì chỉ thao tác trên vật chứa; đường đúng đi qua
    # đối tượng THEO VÙNG (createByRange -> getConditionalFormats -> createEntry). Xem
    # tests/bench/results/probe-conditional-format-3.txt.
    b.cmd("et.writeRange", {"range": "AB1:AB10", "sheet": "Sheet1",
                            "values": [[10], [20], [30], [40], [50], [60], [70], [80], [90], [100]]})
    applied = b.cmd("et.setConditionalFormat", {
        "sheet": "Sheet1", "range": "AB1:AB10",
        "rules": [{"operator": "less", "formula1": "50", "styleName": "Bad"},
                  {"operator": "greaterEqual", "formula1": "90", "styleName": "Good"}]}) or {}
    check(len(applied.get("rules") or []) == 2, "Calc setConditionalFormat nhận 2 rule", applied)

    listed = b.cmd("et.listConditionalFormats", {"sheet": "Sheet1"}) or {}
    mine = [item for item in (listed.get("formats") or []) if item.get("range", "").endswith("$AB$1:$AB$10")]
    check(len(mine) == 1 and len(mine[0].get("rules") or []) == 2,
          "Calc listConditionalFormats đọc lại đúng 2 rule của vùng", listed)
    check([rule.get("operator") for rule in (mine[0].get("rules") if mine else [])] == ["less", "greaterEqual"],
          "Calc listConditionalFormats trả ĐÚNG TÊN toán tử đã ghi", mine)

    # Vòng đủ cho MỌI toán tử: ghi tên -> đọc lại phải ra đúng tên đó. Bảng số int của toán tử trong
    # calc.py (CF_OPERATOR_VALUES) là thứ duy nhất phải viết cứng; bài này khiến nó không thể lệch âm thầm.
    for name, formula2 in (("equal", None), ("notEqual", None), ("greater", None), ("greaterEqual", None),
                           ("less", None), ("lessEqual", None), ("between", "70"), ("notBetween", "70"),
                           ("formula", None)):
        rule = {"operator": name, "formula1": "=AB1>50" if name == "formula" else "30"}
        if formula2:
            rule["formula2"] = formula2
        # Công thức trần được nhận kèm dấu = nhưng LibreOffice lưu không có dấu, nên đọc lại là "AB1>50".
        expected1 = rule["formula1"].lstrip("=") if name == "formula" else rule["formula1"]
        b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10", "rules": [rule]}, record=False)
        back = ((b.cmd("et.listConditionalFormats", {"sheet": "Sheet1", "range": "AB1:AB10"},
                       record=False) or {}).get("formats") or [])
        read = (back[0].get("rules") if back else [{}])[0]
        check(read.get("operator") == name and read.get("formula1") == expected1,
              "Calc định dạng điều kiện: %s ghi rồi đọc lại đúng" % name, read)
        check(read.get("formula2") == formula2,
              "Calc định dạng điều kiện: %s chỉ trả formula2 khi toán tử dùng nó" % name, read)

    # Gọi lại trên CÙNG vùng là THAY, không phải thêm: nếu cộng dồn thì lần gọi thứ hai của agent (sau khi
    # đọc lại thấy sai) sẽ để lại hai bộ rule cùng sống.
    b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10",
                                      "rules": [{"operator": "less", "formula1": "10", "fillColor": "#FFCCCC"}]},
          record=False)
    again = b.cmd("et.listConditionalFormats", {"sheet": "Sheet1", "range": "AB1:AB10"}) or {}
    check(len(again.get("formats") or []) == 1 and len((again["formats"][0].get("rules") or [])) == 1,
          "Calc setConditionalFormat gọi lại thì THAY rule cũ của vùng", again)
    made_style = (again["formats"][0]["rules"][0].get("styleName") or "")
    check(made_style.startswith("Axiom CF "), "Calc setConditionalFormat tự tạo cell style từ fillColor", made_style)

    b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10",
                                      "rules": [{"operator": "quanhQue", "formula1": "1"}]},
          expect_ok=False, key="et.setConditionalFormat toán tử lạ")
    b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10",
                                      "rules": [{"operator": "less"}]}, expect_ok=False,
          key="et.setConditionalFormat thiếu formula1")
    b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10",
                                      "rules": [{"operator": "between", "formula1": "1"}]}, expect_ok=False,
          key="et.setConditionalFormat between thiếu formula2")
    b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10",
                                      "rules": [{"operator": "less", "formula1": "1", "styleName": "Không có"}]},
          expect_ok=False, key="et.setConditionalFormat style lạ")
    b.cmd("et.setConditionalFormat", {"sheet": "Sheet1", "range": "AB1:AB10"}, expect_ok=False,
          key="et.setConditionalFormat thiếu rules")

    # Kiểm cái THẬT SỰ vào file: lưu ra .xlsx rồi tìm <conditionalFormatting> trong gói. Đây là định dạng
    # KHÁC hẳn ODF, nên nếu cả hai đều có thì không phải trạng thái tạm trong phiên.
    cf_xlsx = os.path.join(out, "libreoffice-calc-cf.xlsx")
    b.cmd("et.saveAs", {"path": cf_xlsx})
    with zipfile.ZipFile(cf_xlsx) as archive:
        body = "".join(archive.read(name).decode("utf-8", "replace")
                       for name in archive.namelist() if name.startswith("xl/worksheets/sheet"))
    check("<conditionalFormatting" in body, "Calc setConditionalFormat vào THẬT trong file .xlsx",
          "%d ký tự, có thẻ: %s" % (len(body), "<conditionalFormatting" in body))

    b.cmd("et.writeRange", {"range": "A7", "values": [["hoàn tác tôi"]]}, key="et.writeRange before undo")
    b.cmd("et.undo", {"count": 1})
    check((b.cmd("et.readRange", {"range": "A7"}, record=False) or {}).get("values", [[None]])[0][0] is None,
          "Calc undo hoàn tác được (khác Excel qua COM)", "")
    pdf = os.path.join(out, "libreoffice-calc.pdf")
    b.cmd("et.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 500, "Calc exportPdf tạo file", pdf)
    xlsx = os.path.join(out, "libreoffice-calc.xlsx")
    b.cmd("et.saveAs", {"path": xlsx})
    b.cmd("et.save")
    b.cmd("et.closeAll")
    b.cmd("et.open", {"path": xlsx})
    check((b.cmd("et.readRange", {"range": "A1:B2"}, record=False) or {}).get("values", [[None]])[0][0] == "Tên",
          "Calc mở lại file .xlsx đã lưu", "")


def test_impress(b, out, png):
    b.cmd("wpp.newPresentation")
    layouts = []
    for layout in (1, 2, 11, 12):
        added = b.cmd("wpp.addSlide", {"layout": layout}, key="wpp.addSlide %d" % layout)
        layouts.append((added or {}).get("slide"))
    slides = b.cmd("wpp.listSlides")
    check(slides and slides.get("slideCount") == 5 and all(i is not None for i in layouts), "Impress addSlide/listSlides đếm đúng slide", slides and slides.get("slideCount"))
    check([s.get("layout") for s in (slides or {}).get("slides", [])][1:] == [1, 2, 1, 0],
          "Impress layout 1/2/11/12 -> AUTOLAYOUT_TITLE/TITLE_CONTENT/TITLE/NONE", slides and [s.get("layout") for s in slides.get("slides", [])])
    b.cmd("wpp.addSlide", {"layout": 99}, expect_ok=False, key="wpp.addSlide bad layout")
    unchanged = b.cmd("wpp.listSlides", record=False) or {}
    check(unchanged.get("slideCount") == slides.get("slideCount"), "Impress addSlide sai layout không để lại slide rác", unchanged.get("slideCount"))
    added = b.cmd("wpp.addText", {"text": "Báo cáo tuần 39", "left": 60, "top": 40, "width": 600, "height": 80, "fontSize": 28, "bold": True, "align": "center"})
    check(added and added.get("shape"), "Impress addText tạo textbox", added)
    b.cmd("wpp.addText", {"text": "Doanh thu tăng 12%\nChi phí giảm 5%\nLợi nhuận 750 triệu\nKhách hàng mới 42\nTồn kho giảm 8%\nMột dòng rất dài để chắc chắn tràn khung",
                          "left": 80, "top": 160, "width": 300, "height": 60, "fontSize": 10},
          key="wpp.addText small overflow")
    b.cmd("wpp.addTextBox", {"text": "Textbox thường", "left": 420, "top": 300}, key="wpp.addTextBox")
    table = b.cmd("wpp.addTable", {"values": [["Khu vực", "Doanh thu"], ["Miền Bắc", 520], ["Miền Nam", 430]], "left": 60, "top": 250, "width": 400, "height": 120})
    check(table and table.get("rows") == 3 and table.get("cols") == 2 and table.get("filled") == 6, "Impress addTable tạo bảng có dữ liệu", table)
    image = b.cmd("wpp.addImage", {"path": png, "left": 520, "top": 250, "width": 60, "height": 60})
    check(image and image.get("shape"), "Impress addImage chèn ảnh", image)
    notes = b.cmd("wpp.setNotes", {"text": "Nhấn mạnh mức tăng doanh thu."})
    check(notes and notes.get("notes", 0) > 10, "Impress setNotes ghi chú", notes)
    layout = b.cmd("wpp.checkLayout") or {}
    kinds = {i.get("type") for slide in layout.get("slides", []) for i in slide.get("issues", [])}
    check({"overflow", "small-font"} <= kinds, "Impress checkLayout bắt tràn chữ và chữ nhỏ", sorted(kinds))
    placeholders = [s for s in layout.get("slides", []) if any(b.get("name", "").startswith("shape") for b in s.get("shapes", []))]
    check(not placeholders, "Impress checkLayout bỏ qua placeholder trống của layout", placeholders)
    b.cmd("wpp.setNotes", {"text": "Trang một", "slide": 1}, key="wpp.setNotes slide 1")
    b.cmd("wpp.deleteSlide")
    after = b.cmd("wpp.listSlides", record=False) or {}
    check(after.get("slideCount") == slides.get("slideCount") - 1, "Impress deleteSlide xoá slide cuối", after.get("slideCount"))
    pdf = os.path.join(out, "libreoffice-impress.pdf")
    b.cmd("wpp.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 1000, "Impress exportPdf tạo file", pdf)
    pptx = os.path.join(out, "libreoffice-impress.pptx")
    b.cmd("wpp.saveAs", {"path": pptx})
    b.cmd("wpp.save")
    b.cmd("wpp.closeAll")
    b.cmd("wpp.open", {"path": pptx})
    check(((b.cmd("wpp.listSlides", record=False) or {}).get("slideCount") or 0) >= 4, "Impress mở lại file .pptx đã lưu", "")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--apps", default="writer,calc,impress")
    parser.add_argument("--ui", action="store_true", help="mở bản có cửa sổ thay vì --headless")
    parser.add_argument("--ai", action="store_true", help="chạy thêm ai.ask trên Writer (gọi LLM thật)")
    parser.add_argument("--keep", action="store_true", help="không tắt LibreOffice sau khi chạy")
    args = parser.parse_args()

    out = os.path.join(tempfile.gettempdir(), "axiom-libreoffice-test")
    shutil.rmtree(out, ignore_errors=True)
    os.makedirs(out)
    png = tiny_png(os.path.join(out, "logo.png"))

    apps = [a.strip() for a in args.apps.split(",") if a.strip()]
    pid = launch(args.ui)
    print("== LibreOffice pid %d (%s)" % (pid, "cửa sổ" if args.ui else "headless"))
    try:
        for app in apps:
            label = "libreoffice-%s" % app
            print("== %s: port %d" % (label, PORTS[app]))
            bridge = Bridge(PORTS[app], out, label)
            test_port_guards(bridge, app)
            if app == "writer":
                test_writer(bridge, out, png, args.ai)
            elif app == "calc":
                test_calc(bridge, out)
            else:
                test_impress(bridge, out, png)
    finally:
        if not args.keep:
            close_app(pid)

    failed = [r for r in RESULTS if not r[0]]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
