"""Test tích hợp mọi lệnh bridge (POST /cmd) trên Word/Excel/PowerPoint (hoặc WPS) thật.

Chỉ chạy trên app do chính script mở: nếu port đã có bridge của app người dùng thì dừng
(bộ test gọi writer.closeAll - đóng mọi tài liệu KHÔNG lưu). Chạy xong tự tắt các app đã mở.

    python tests/live/test_live_commands.py [--wps] [--apps word,excel,ppt] [--ai]
                                            [--record golden.json | --compare golden.json]

--record/--compare: ghi / so kết quả từng lệnh (đã bỏ phần thay đổi giữa các lần chạy như
đường dẫn thư mục tạm) để chứng minh refactor không đổi hành vi.
--ai: chạy ai.ask (gọi LLM thật) trên từng app: xong việc, không tự lưu/xuất file.
Luôn kiểm tra (không cần Office): tool office_action của agent từ chối lệnh ngoài danh sách.
Chỉ dùng thư viện chuẩn.
"""
from __future__ import annotations

import argparse
import base64
import json
import os
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import zlib

try:
    import winreg  # Windows: token o HKCU; Linux (test LibreOffice) doc ~/.config/axiom-office/config.json
except ImportError:  # pragma: no cover - chi Linux/macOS
    winreg = None

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
HOST_EXE = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Host.exe")
ADDIN_DLL = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.dll")
OFFICE = {"word": (47831, "winword.exe", ["/q", "/w"]), "excel": (47832, "excel.exe", ["/x"]), "ppt": (47833, "powerpnt.exe", [])}
WPS_EXE = os.path.join(os.environ.get("LOCALAPPDATA", ""), "Kingsoft", "WPS Office")
WPS = {"word": (47821, "wps", ["/wps"]), "excel": (47822, "wps", ["/et"]), "ppt": (47823, "wps", ["/wpp"])}

RESULTS: list[tuple[bool, str, str]] = []
RECORD: dict[str, object] = {}
TESTED: set[str] = set()


def check(ok, name, detail=""):
    RESULTS.append((bool(ok), name, detail))
    print(("PASS " if ok else "FAIL ") + name + ("" if ok or not detail else "\n     " + str(detail)[:600]))


def token():
    if winreg is not None:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\AxiomOffice") as key:
            return winreg.QueryValueEx(key, "Token")[0]
    base = os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config")
    with open(os.path.join(base, "axiom-office", "config.json"), encoding="utf-8") as handle:
        return json.load(handle).get("Token", "")


def http(port, method, path, body=None, headers=None, timeout=60):
    data = None if body is None else (body if isinstance(body, bytes) else json.dumps(body, ensure_ascii=False).encode("utf-8"))
    request = urllib.request.Request("http://127.0.0.1:%d%s" % (port, path), data=data, method=method, headers=headers or {})
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as ex:
        return ex.code, json.loads(ex.read().decode("utf-8") or "null")


class Bridge:
    def __init__(self, port, out, label):
        self.port = port
        self.out = out
        self.label = label
        self.headers = {"X-Auth-Token": token(), "Content-Type": "application/json; charset=utf-8"}

    def cmd(self, action, params=None, expect_ok=True, key=None, timeout=90, record=True):
        TESTED.add(action)
        status, reply = http(self.port, "POST", "/cmd", {"action": action, "params": params or {}}, self.headers, timeout)
        ok = status == 200 and isinstance(reply, dict) and reply.get("ok") is expect_ok
        name = "%s %s%s" % (self.label, action, "" if expect_ok else " (lỗi mong đợi)")
        check(ok, name, json.dumps(reply, ensure_ascii=False))
        if record:
            RECORD["%s|%s" % (self.label, key or action)] = normalize(reply, self.out)
        return (reply or {}).get("result") if expect_ok else (reply or {}).get("error")

    def ask(self, prompt, document_key, target=None):
        """ai.ask thật (không ghi golden): xong việc, không tự lưu/xuất file, tài liệu còn ở trạng thái chưa lưu."""
        result = self.cmd("ai.ask", {"prompt": prompt}, timeout=330, record=False) or {}
        transcript = result.get("transcript") or []
        actions = [a for line in transcript for a in re.findall(r'"action"\s*:\s*"([\w.]+)"', line)]
        print("     ai.ask (%s rounds): %s" % (result.get("rounds"), ", ".join(actions)))
        check(result.get("ok") is True, self.label + " ai.ask trả lời xong", result.get("error"))
        saved = [a for a in actions if a.endswith((".save", ".saveAs", ".exportPdf"))]
        check(not saved, self.label + " ai.ask không tự lưu/xuất file khi không được yêu cầu", saved)
        refused = [line for line in transcript if "is not an available action" in line]
        if refused:
            print("     office_action từ chối:", refused)
        info = self.cmd("app.info", record=False) or {}
        if target:
            # Core ghim tài liệu của lượt (H1) nên AI sửa đúng bài kể cả khi cửa sổ hoạt động đổi giữa chừng; đã gặp
            # (25/10/2026, không tái hiện được) cửa sổ hoạt động nhảy sang bài khác sau ai.ask - ghi lại, rồi kiểm bài đích.
            active = (info.get(document_key) or {}).get("fullName", "")
            if active.lower() != target.lower():
                print("     LƯU Ý: sau ai.ask cửa sổ hoạt động là %s, không phải bài đích" % active)
            self.cmd("wpp.open", {"path": target}, record=False)
            info = self.cmd("app.info", record=False) or {}
        check((info.get(document_key) or {}).get("saved") is False, self.label + " tài liệu vẫn chưa lưu sau ai.ask", info.get(document_key))
        return actions


def normalize(value, out):
    """Bỏ phần khác nhau giữa các lần chạy (thư mục tạm, pid) để so golden."""
    if isinstance(value, dict):
        return {k: normalize(v, out) for k, v in value.items() if k not in ("pid", "seconds", "time")}
    if isinstance(value, list):
        return [normalize(v, out) for v in value]
    if isinstance(value, str):
        return value.replace(out, "<OUT>").replace(out.replace("\\", "/"), "<OUT>")
    return value


def tiny_png(path, width=16, height=16):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    raw = b"".join(b"\x00" + b"\xcc\x44\x22" * width for _ in range(height))
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b"")
    with open(path, "wb") as handle:
        handle.write(png)
    return path


def health(port):
    try:
        status, reply = http(port, "GET", "/health", timeout=2)
        return reply.get("result") if status == 200 else None
    except Exception:
        return None


def app_path(exe):
    """Đường dẫn đầy đủ của winword/excel/powerpnt từ App Paths (CreateProcess không tự tra như Start-Process)."""
    for hive in (winreg.HKEY_CURRENT_USER, winreg.HKEY_LOCAL_MACHINE):
        try:
            with winreg.OpenKey(hive, "Software\\Microsoft\\Windows\\CurrentVersion\\App Paths\\" + exe) as key:
                return winreg.QueryValueEx(key, "")[0]
        except OSError:
            pass
    raise SystemExit("không tìm thấy %s trong App Paths" % exe)


def launch(app, use_wps):
    port, exe, args = (WPS if use_wps else OFFICE)[app]
    if health(port):
        raise SystemExit("port %d đã có bridge của một app đang mở - đóng app đó rồi chạy lại (test gọi writer.closeAll)" % port)
    if use_wps:
        exe = next(os.path.join(root, "wps.exe") for root, _, files in os.walk(WPS_EXE) if "wps.exe" in files and root.endswith("office6"))
    else:
        exe = app_path(exe)
    subprocess.Popen([exe] + args)
    deadline = time.time() + 60
    while time.time() < deadline:
        info = health(port)
        if info:
            return port, info["pid"]
        time.sleep(0.8)
    raise SystemExit("bridge không lên ở port %d" % port)


def close_app(pid):
    """Đóng app do test mở: WM_CLOSE trước (tài liệu đều đã lưu nên không hỏi), quá 20s mới kill."""
    subprocess.run(["taskkill", "/PID", str(pid)], capture_output=True)
    deadline = time.time() + 20
    while time.time() < deadline:
        alive = subprocess.run(["tasklist", "/FI", "PID eq %d" % pid, "/NH"], capture_output=True, text=True).stdout
        if str(pid) not in alive:
            return
        time.sleep(0.5)
    subprocess.run(["taskkill", "/PID", str(pid), "/F"], capture_output=True)


# ---------------------------------------------------------------- Word / Writer

def test_writer(b, out, png, run_ai):
    b.cmd("app.info", key="app.info (trước khi có tài liệu)")
    b.cmd("writer.newDocument")
    b.cmd("writer.typeText", {"text": "Xin chào Axiom. "})
    b.cmd("writer.appendText", {"text": "Dòng nối cuối."})
    text = b.cmd("writer.getText")
    check(text and "Xin chào Axiom" in text["text"] and "Dòng nối cuối" in text["text"], "Word getText thấy nội dung vừa ghi", text)
    b.cmd("writer.selection")
    short = b.cmd("writer.getText", {"maxChars": "12.0"}, key="writer.getText string number")
    check(short and len(short["text"]) == 12 and short["truncated"], "Word maxChars nhận số dạng chuỗi '12.0'", short)
    b.cmd("writer.insertStyledText", {"text": " Chữ đỏ đậm", "bold": True, "color": "#C00000", "size": 14})
    error = b.cmd("writer.insertStyledText", {"text": "x", "bold": "có"}, expect_ok=False, key="writer.insertStyledText bool text")
    check("'bold' must be true or false, got 'có'" in (error or ""), "Word lỗi bool nêu tên tham số", error)
    b.cmd("writer.heading", {"level": 1, "text": "Tiêu đề một"})
    b.cmd("writer.formatSelection", {"italic": True, "alignment": "center"})
    b.cmd("writer.setParagraphAlignment", {"alignment": "justify"})
    table = b.cmd("writer.insertTable", {"values": [["Tên", "Điểm"], ["An", 9.5], ["Bình", 8]], "style": "Table Grid"}, key="writer.insertTable values")
    check(table and table.get("rows") == 3 and table.get("cols") == 2, "Word insertTable suy ra 3x2 từ values", table)
    b.cmd("writer.insertTable", {"rows": 2, "cols": 2, "values": {"item": [{"item": ["x", "y"]}]}}, key="writer.insertTable wrapped")
    # Dạng model big-pickle gửi thật (log 30/09): mỗi dòng bọc thêm một lớp, số bọc {"item": n}, dữ liệu đặt nhầm vào rows.
    double = b.cmd("writer.insertTable", {"cols": "2", "values": {"item": [{"item": {"item": ["STT", "Tên"]}}, {"item": {"item": ["1", "An"]}}]}},
                   key="writer.insertTable double-wrapped rows")
    check(double and double.get("rows") == 2 and double.get("cols") == 2 and double.get("filled") == 4, "Word insertTable gỡ dòng bọc 2 lớp", double)
    wrapped_int = b.cmd("writer.insertTable", {"rows": {"item": 2}, "cols": [3]}, key="writer.insertTable wrapped ints")
    check(wrapped_int and wrapped_int.get("rows") == 2 and wrapped_int.get("cols") == 3, "Word insertTable nhận rows {item:2}, cols [3]", wrapped_int)
    rows_data = b.cmd("writer.insertTable", {"cols": "2", "rows": {"item": [{"item": ["a", "b"]}, {"item": ["c", "d"]}]}}, key="writer.insertTable data in rows")
    check(rows_data and rows_data.get("filled") == 4, "Word insertTable dùng dữ liệu đặt nhầm vào rows", rows_data)
    # Log 30/09 01:40: bảng 4 cột chèn ngay sau bảng 3 cột bị Word gộp vào bảng cũ -> Cell(1,4) lỗi COM.
    b.cmd("writer.insertTable", {"values": [["a", "b", "c"], ["1", "2", "3"]]}, key="writer.insertTable narrow before wide")
    wide = b.cmd("writer.insertTable", {"values": [["w", "x", "y", "z"], ["1", "2", "3", "4"]]}, key="writer.insertTable wide after narrow")
    check(wide and wide.get("filled") == 8, "Word insertTable bảng rộng hơn ngay sau bảng hẹp (không bị gộp)", wide)
    b.cmd("writer.appendText", {"text": "\r"}, record=False)
    wider = b.cmd("writer.insertTable", {"values": [["1", "2", "3", "4", "5"]]}, key="writer.insertTable after appendText")
    check(wider and wider.get("filled") == 5, "Word insertTable sau appendText vẫn không gộp bảng", wider)
    # Log 30/09 01:48: sau insertTable con trỏ nằm ở ô (1,1) nên chữ gõ tiếp lọt vào ô tiêu đề.
    placed = b.cmd("writer.insertTable", {"values": [["ONE_C1", "ONE_C2"]], "style": "Grid Table 4 - Accent 1"}, key="writer.insertTable then type")
    check(placed and "styleError" not in placed, "Word insertTable đặt được style 'Grid Table 4 - Accent 1'", placed)
    b.cmd("writer.typeText", {"text": "SAU_BANG"}, record=False)
    whole = (b.cmd("writer.getText", record=False) or {}).get("text", "")
    check("SAU_BANG" in whole and whole.index("SAU_BANG") > whole.index("ONE_C2"), "Word chữ gõ sau insertTable nằm SAU bảng", whole[-120:])
    # QA cấu trúc: đoạn ghi chú lọt vào ô tiêu đề (log 30/09) + ô trống.
    b.cmd("writer.insertTable", {"values": [["Lịch họp áp dụng cho toàn bộ phòng ban - thời gian có thể điều chỉnh.Thứ", "Giờ", "Nội dung"],
                                            ["Hai", "8h", ""]]}, key="writer.insertTable qa")
    tables_qa = b.cmd("writer.checkTables", key="writer.checkTables") or {}
    table_kinds = {i.get("type") for i in tables_qa.get("issues", [])}
    check({"long-header-cell", "empty-cells"} <= table_kinds, "Word checkTables bắt ô tiêu đề lẫn đoạn văn + ô trống", tables_qa)
    borderless = b.cmd("writer.formatTable", {"borders": False}, key="writer.formatTable borders false")
    check(borderless and "borders" in borderless.get("applied", []), "Word formatTable bỏ viền bảng", borderless)
    # Bo khung = bang DAN TRANG: phai danh dau duoc, de checkTables thoi bao empty-cells (log 11/10/2026 -
    # the-thuc-van-ban bao dien "—" vao o trong cua bang dan trang NĐ30, tuc la pha dung the thuc).
    check(borderless and "layout-table" in borderless.get("applied", []),
          "Word formatTable borders:false đánh dấu bảng dàn trang (tên bảng đặt được)", borderless)
    layout_qa = b.cmd("writer.checkTables", key="writer.checkTables after borders false") or {}
    qa_row = next((t for t in layout_qa.get("tables", []) if (t.get("header") or [""])[0].startswith("Lịch họp")), None)
    qa_issues = {i.get("type") for i in layout_qa.get("issues", []) if qa_row and i.get("table") == qa_row.get("index")}
    check(qa_row is not None and qa_row.get("layout") is True, "Word checkTables thấy bảng dàn trang qua cờ layout", layout_qa)
    check(qa_row is not None and "empty-cells" not in qa_issues, "Word checkTables KHÔNG báo empty-cells cho bảng dàn trang", layout_qa)
    check(qa_row is not None and "long-header-cell" in qa_issues,
          "Word checkTables vẫn báo long-header-cell cho bảng dàn trang (chữ lọt ô tiêu đề)", layout_qa)
    styled = b.cmd("writer.formatTable", {"font": "Calibri", "size": "12", "headerFill": "1F4E79", "headerColor": "#FFFFFF",
                                          "bandFill": "#DEEAF6", "borderColor": "#8EAADB", "alignment": "center", "autoFit": "window"},
                   key="writer.formatTable last table")
    check(styled and not styled.get("skipped") and "headerFill" in styled.get("applied", []) and "bandFill" in styled.get("applied", []),
          "Word formatTable định dạng bảng có sẵn (không bỏ qua phần nào)", styled)
    # Ke khung lai = bang DU LIEU: tem bang dan trang phai duoc bo, empty-cells phai bao lai.
    rebordered = b.cmd("writer.checkTables", record=False) or {}
    back = next((t for t in rebordered.get("tables", []) if (t.get("header") or [""])[0].startswith("Lịch họp")), None)
    check(back is not None and back.get("layout") is False and "empty-cells" in
          {i.get("type") for i in rebordered.get("issues", []) if i.get("table") == back.get("index")},
          "Word kẻ khung lại thì bảng thôi là bảng dàn trang", rebordered)
    first = b.cmd("writer.formatTable", {"table": 1, "style": "Grid Table 4 - Accent 1"}, key="writer.formatTable by index")
    check(first and first.get("table") == 1 and first.get("applied") == ["style"], "Word formatTable theo chỉ số bảng", first)
    bad = b.cmd("writer.formatTable", {"table": 99}, expect_ok=False, key="writer.formatTable bad index")
    check("'table' must be between 1 and" in (bad or ""), "Word formatTable báo chỉ số bảng sai", bad)
    b.cmd("writer.insertTable", {"rows": 2, "cols": 2, "values": {"a": 1}}, expect_ok=False, key="writer.insertTable invalid")
    b.cmd("writer.insertTable", {}, expect_ok=False, key="writer.insertTable missing")
    b.cmd("writer.appendText", {"text": "\rTrân trọng,\rKính mong phản hồi."})
    multi = b.cmd("writer.replaceAll", {"find": "Trân trọng,\nKính mong", "replace": "Kính mong"}, key="writer.replaceAll newline")
    joined = b.cmd("writer.getText", record=False) or {}
    check(multi and multi.get("replaced") and "Trân trọng" not in joined.get("text", ""), "Word replaceAll khớp \\n với ngắt đoạn", multi)
    b.cmd("writer.insertPageBreak")
    b.cmd("writer.insertImage", {"path": png, "width": 40, "height": 40})
    b.cmd("writer.insertHyperlink", {"url": "https://example.com", "text": "ví dụ"})
    # "Hoàn tác lượt này" (docs/architecture/core.md 9.2): N thao tác AI = N bước Undo -> writer.undo {count: N} trả lại như cũ.
    baseline = (b.cmd("writer.getText", record=False) or {}).get("text")
    b.cmd("writer.appendText", {"text": "\rLượt AI dòng 1"}, record=False)
    b.cmd("writer.appendText", {"text": "\rLượt AI dòng 2"}, record=False)
    b.cmd("writer.replaceAll", {"find": "Lượt AI", "replace": "LƯỢT AI"}, record=False)
    b.cmd("writer.undo", {"count": 3}, key="writer.undo whole turn")
    restored = (b.cmd("writer.getText", record=False) or {}).get("text")
    check(restored == baseline, "Word hoàn tác cả lượt (3 thao tác) trả lại nội dung cũ", (restored or "")[-80:])
    replaced = b.cmd("writer.replaceAll", {"find": "Axiom", "replace": "AXIOM"})
    b.cmd("writer.undo", {"count": 1})
    after = b.cmd("writer.getText", {"maxChars": 40}, key="writer.getText after undo")
    check(after and "Axiom" in after["text"], "Word undo hoàn tác replaceAll", {"replace": replaced, "text": after})
    # Lệnh soát/dựng tài liệu: trước đây chỉ có test riêng (test_writer_format.py) hoặc chưa có test trên Word.
    many = b.cmd("writer.replaceMany", {"edits": [{"find": "Tiêu đề một", "replace": "Tiêu đề 1"},
                                                  {"find": "KHÔNG_CÓ_CHỖ_NÀY", "replace": "x"}]}) or {}
    check(many.get("counts") == [1, 0] and many.get("replaced") == 1, "Word replaceMany đếm số chỗ thay của từng cặp", many)
    b.cmd("writer.replaceMany", {"edits": []}, expect_ok=False, key="writer.replaceMany edits rỗng")
    outline = b.cmd("writer.outline", {"maxChars": 40}) or {}
    items = outline.get("items") or []
    check(outline.get("paragraphs", 0) > 0 and any(i.get("kind") == "table" for i in items)
          and any("Tiêu đề 1" in (i.get("t") or "") for i in items), "Word outline liệt kê đoạn và bảng", outline)
    blocks = b.cmd("writer.insertBlocks", {"blocks": [{"before": 1, "kind": "box", "cells": [
        {"tone": "info", "lines": [{"k": "title", "t": "Ghi chú khối"}, {"k": "text", "t": "Nội dung trong khối"}]}]}]}) or {}
    check(blocks.get("inserted") == 1, "Word insertBlocks chèn 1 khối", blocks)
    check("Ghi chú khối" in ((b.cmd("writer.getText", record=False) or {}).get("text") or ""),
          "Word insertBlocks: chữ của khối có trong tài liệu", "")
    restyled = b.cmd("writer.restyle", {}) or {}
    check(restyled.get("mode") in ("design", "administrative"), "Word restyle trả chế độ trình bày", restyled)
    # Khoá vùng bôi đen: có vùng chọn (>= 2 ký tự chữ) thì khoá, khi khoá lệnh chạm ra ngoài vùng bị từ chối;
    # không bôi đen thì chars = 0 và không khoá gì. Lệnh trước để lại vùng chọn nào thì kiểm theo vùng đó.
    selection = b.cmd("writer.selection", record=False) or {}
    has_selection = len((selection.get("text") or "").strip()) >= 2
    locked = b.cmd("writer.lockSelection", record=False) or {}
    check((locked.get("chars", 0) > 0) == has_selection, "Word lockSelection khoá đúng khi (và chỉ khi) có vùng bôi đen",
          {"lock": locked, "selection": (selection.get("text") or "")[:40]})
    if has_selection:
        refused = b.cmd("writer.appendText", {"text": "ngoài vùng"}, expect_ok=False, key="writer.appendText khi khoá vùng")
        check("Refused: the user selected part" in (refused or ""), "Word khoá vùng từ chối lệnh ghi ra ngoài vùng chọn", refused)
    b.cmd("writer.unlockSelection")
    pdf = os.path.join(out, "writer.pdf")
    b.cmd("writer.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 1000, "Word exportPdf tạo file", pdf)
    docx = os.path.join(out, "writer.docx")
    b.cmd("writer.saveAs", {"path": docx})
    b.cmd("writer.save")
    b.cmd("app.info", key="app.info (có tài liệu)")
    b.cmd("ui.askpane")
    b.cmd("writer.closeAll")
    b.cmd("writer.open", {"path": docx})
    reopened = b.cmd("writer.getText", {"maxChars": 30}, key="writer.getText reopened")
    check(reopened and reopened["name"] == "writer.docx", "Word mở lại file đã lưu", reopened)
    b.cmd("nosuch.action", expect_ok=False, key="unknown action")
    if run_ai:
        b.ask("Thêm một dòng 'Kiểm tra ai.ask' vào cuối tài liệu", "activeDocument")
        text = b.cmd("writer.getText", record=False) or {}
        check("Kiểm tra ai.ask" in text.get("text", ""), "Word ai.ask thêm được dòng mới", text.get("text", "")[-200:])
        b.cmd("writer.save", record=False)


# ---------------------------------------------------------------- Excel / ET

def test_spreadsheet(b, out, run_ai):
    b.cmd("app.info", key="app.info (trước khi có tài liệu)")
    b.cmd("et.newWorkbook")
    b.cmd("et.listSheets")
    b.cmd("et.writeRange", {"range": "A1", "values": [["Tên", "Điểm", "Nhân đôi"], ["An", 9.5, "=B2*2"], ["Bình", 8, "=B3*2"]]})
    b.cmd("et.writeRange", {"range": "E1", "values": {"item": [{"item": ["bọc", 1]}]}}, key="et.writeRange wrapped")
    b.cmd("et.writeRange", {"range": "G1", "values": "[[1,2],[3,4]]"}, key="et.writeRange json string")
    b.cmd("et.writeRange", {"range": "J1", "values": [1, 2, 3]}, key="et.writeRange 1d column")
    b.cmd("et.writeRange", {"range": "A10", "values": {"foo": 1}}, expect_ok=False, key="et.writeRange invalid")
    b.cmd("et.writeRange", {"range": "A10"}, expect_ok=False, key="et.writeRange missing values")
    b.cmd("et.writeRange", {"values": [[1]]}, expect_ok=False, key="et.writeRange missing range")
    # Mỗi lớp {"item": x} là một cấp mảng (x không phải mảng = phần tử duy nhất).
    b.cmd("et.writeRange", {"range": "A20", "values": {"item": {"item": ["Tổng", "=SUM(B2:B3)"]}}}, key="et.writeRange item row")
    b.cmd("et.writeRange", {"range": "A22", "values": {"item": {"item": "đơn"}}}, key="et.writeRange item single")
    b.cmd("et.writeRange", {"range": "A24", "values": {"item": {"item": [{"item": ["p", "q"]}, {"item": ["r", "s"]}]}}}, key="et.writeRange item overwrapped")
    b.cmd("et.writeRange", {"range": "A27", "values": [[{"item": "c1"}, "c2"]]}, key="et.writeRange item cell")
    b.cmd("et.writeRange", {"range": "A29", "values": '{"item": [["j", 1]]}'}, key="et.writeRange json object string")
    decoded = b.cmd("et.readRange", {"range": "A20:B29"}, key="et.readRange item shapes")
    expected = [["Tổng", 17.5], [None, None], ["đơn", None], [None, None], ["p", "q"], ["r", "s"], [None, None], ["c1", "c2"], [None, None], ["j", 1]]
    check(decoded and decoded["values"] == expected, "Excel giải mã values bọc {item} đúng dòng/cột", decoded)
    values = b.cmd("et.readRange", {"range": "A1:C3"})
    check(values and values["values"][1][2] == 19, "Excel công thức =B2*2 tính ra 19", values)
    # Dấu phân cách tham số: một số bản LibreOffice đòi ';' và không còn mục Tools > Options > Calc >
    # Formula > Separators để đổi, nên công thức en-US (dấu phẩy) của agent bị Err:508. Excel qua COM
    # thì luôn nhận ','. Ghi bằng ',' phải chạy được ở cả hai, và không được im lặng khi còn lỗi.
    multi = b.cmd("et.writeRange", {"range": "O1", "values": [
        ["Số", "Làm tròn", "Xếp loại"],
        [8.5, "=ROUND(AVERAGE(O2:O2),1)", '=IF(O2>=8,"Giỏi","Khá")'],
        [7.0, "=ROUND(AVERAGE(O3:O3),1)", '=IF(O3>=8,"Giỏi","Khá")']]}, key="et.writeRange công thức nhiều tham số")
    check(not (multi or {}).get("formulaErrors"), "Excel công thức nhiều tham số (dấu phẩy) không lỗi", multi)
    computed = b.cmd("et.readRange", {"range": "O2:Q3"}, key="et.readRange công thức nhiều tham số") or {}
    check(computed.get("values") == [[8.5, 8.5, "Giỏi"], [7.0, 7.0, "Khá"]],
          "Excel công thức nhiều tham số tính đúng ở mọi dấu phân cách", computed)
    # Đọc CÔNG THỨC: values không phân biệt số gõ tay với kết quả tính (bench 02/10/2026: sheet Checks
    # toàn chữ PASS gõ tay mà đọc values ra vẫn "đúng").
    fx = b.cmd("et.readRange", {"range": "O2:Q3", "formulas": True}, key="et.readRange formulas") or {}
    fx_rows = fx.get("formulas") or []
    check(len(fx_rows) == 2 and fx_rows[0][0] is None
          and all(isinstance(row[c], str) and row[c].startswith("=") for row in fx_rows for c in (1, 2)),
          "Excel readRange formulas: ô số gõ tay là null, ô công thức bắt đầu '='", fx)
    # Kích thước dùng thật từng sheet: sheet có dữ liệu phải báo đúng rows, không được nói chung chung.
    info = b.cmd("et.sheetInfo", {}, key="et.sheetInfo") or {}
    check(info.get("activeSheet") and (info.get("sheets") or [])
          and any(s.get("rows", 0) >= 3 and s.get("empty") is False for s in info["sheets"]),
          "Excel sheetInfo: sheet có dữ liệu báo đúng kích thước, không empty", info)
    multi_qa = b.cmd("et.checkRange", {"range": "O1:Q3"}, key="et.checkRange công thức nhiều tham số") or {}
    check("error-values" not in {i.get("type") for i in multi_qa.get("issues", [])},
          "Excel checkRange không báo lỗi công thức sau khi tự đổi dấu phân cách", multi_qa)
    b.cmd("et.readRange", {"range": "E1:K3"}, key="et.readRange others")
    b.cmd("et.formatRange", {"range": "A1:C1", "bold": True, "fillColor": "#FFFF00", "fontColor": "#0000FF", "numFmt": "0.00", "horizontal": "center", "wrap": True})
    # QA cấu trúc: B2 = 9.5 để General -> no-number-format; cả sheet có dữ liệu ngoài bảng A1:C3 -> outside-table.
    table_qa = b.cmd("et.checkRange", {"range": "A1:C3"}, key="et.checkRange table") or {}
    table_kinds = {i.get("type") for i in table_qa.get("issues", [])}
    check(table_qa.get("rows") == 3 and table_qa.get("cols") == 3 and "no-number-format" in table_kinds and "outside-table" not in table_kinds,
          "Excel checkRange: 3x3, số lẻ chưa có number format", table_qa)
    sheet_qa = b.cmd("et.checkRange", {}, key="et.checkRange sheet") or {}
    check("outside-table" in {i.get("type") for i in sheet_qa.get("notes", [])}, "Excel checkRange bắt dữ liệu lạc ngoài bảng (ghi chú)", sheet_qa)
    b.cmd("et.writeRange", {"range": "M1", "values": [["Mã", "Giá"], ["x", "'12"], ["y", "'15"]]}, key="et.writeRange numbers as text")
    text_qa = b.cmd("et.checkRange", {"range": "M1:N3"}, key="et.checkRange numbers as text") or {}
    check("numbers-as-text" in {i.get("type") for i in text_qa.get("issues", [])}, "Excel checkRange bắt số lưu dạng chữ", text_qa)
    # QA thị giác (docs/architecture/core.md 8.4.6): ảnh chụp cửa sổ app, thu nhỏ theo maxWidth.
    shot = b.cmd("app.screenshot", {"maxWidth": 640}, key="app.screenshot", record=False) or {}
    png = base64.b64decode(shot.get("base64", "") or "")
    check(png[:8] == b"\x89PNG\r\n\x1a\n" and 0 < shot.get("width", 0) <= 640 and shot.get("height", 0) > 0,
          "app.screenshot trả PNG thật, rộng <= maxWidth", {k: shot.get(k) for k in ("width", "height", "mime")})
    b.cmd("et.activateSheet", {"sheet": "Sheet1"})
    b.cmd("et.writeRange", {"range": "A5", "values": [["hoàn tác tôi"]]}, key="et.writeRange before undo")
    undo = b.cmd("et.undo", {"count": 1}) or {}
    # Excel không hoàn tác được thay đổi qua COM (docs/architecture/README.md, giới hạn đã biết): et.undo phải nói
    # thật (undone = 0) thay vì báo đã hoàn tác; WPS thì hoàn tác được. Kiểm theo ô A5 thật.
    a5 = ((b.cmd("et.readRange", {"range": "A5"}, record=False) or {}).get("values") or [[None]])[0][0]
    check((a5 is None) == (undo.get("undone", 0) > 0), "et.undo báo đúng số bước đã hoàn tác", {"undo": undo, "A5": a5})
    # Dọn A5 để bước ai.ask sau ghi vào ô TRỐNG (ghi đè ô có chữ thì Core hỏi xác nhận - ai.ask không ai bấm nên từ chối).
    b.cmd("et.writeRange", {"range": "A5", "values": [[None]]}, key="et.writeRange clear A5")
    test_spreadsheet_more(b, out)
    pdf = os.path.join(out, "et.pdf")
    b.cmd("et.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 500, "Excel exportPdf tạo file", pdf)
    xlsx = os.path.join(out, "et.xlsx")
    b.cmd("et.saveAs", {"path": xlsx})
    b.cmd("et.save")
    b.cmd("et.open", {"path": xlsx})
    b.cmd("et.readRange", {"range": "A1:B2"}, key="et.readRange reopened")
    if run_ai:
        b.ask("Ghi chữ 'Tổng' vào ô A5 và công thức tính tổng cột Điểm (B2:B3) vào ô B5", "activeWorkbook")
        total = b.cmd("et.readRange", {"range": "A5:B5"}, record=False) or {}
        row = (total.get("values") or [[None, None]])[0]
        check("tổng" in str(row[0]).lower() and row[1] == 17.5, "Excel ai.ask ghi Tổng = 17.5 vào A5:B5", row)
        b.cmd("et.save", record=False)


def test_spreadsheet_more(b, out):
    """Sheet, biểu đồ, điền công thức, ghi nhiều vùng, CSV, định dạng điều kiện (cùng kịch bản test LibreOffice)."""
    added = b.cmd("et.addSheet", {"name": "Raw_Data"}) or {}
    check(added.get("sheet") == "Raw_Data" and "Raw_Data" in (added.get("sheets") or []), "Excel addSheet thêm sheet có tên", added)
    auto = b.cmd("et.addSheet", {}) or {}
    renamed = b.cmd("et.renameSheet", {"sheet": auto.get("sheet"), "name": "Data_Cleaning"}) or {}
    check("Data_Cleaning" in (renamed.get("sheets") or []), "Excel renameSheet đổi tên được", renamed)
    b.cmd("et.addSheet", {"name": "Raw_Data"}, expect_ok=False, key="et.addSheet trùng tên")
    b.cmd("et.renameSheet", {"sheet": "Không có", "name": "X"}, expect_ok=False, key="et.renameSheet sheet lạ")
    b.cmd("et.activateSheet", {"sheet": "Sheet1"}, record=False)
    chart = b.cmd("et.addChart", {"sheet": "Sheet1", "range": "A1:B3", "type": "line", "title": "Điểm", "anchor": "E30"}) or {}
    check(chart.get("chart") and chart.get("type") == "line", "Excel addChart tạo biểu đồ", chart)
    charts = b.cmd("et.listCharts", {"sheet": "Sheet1"}) or {}
    check(chart.get("chart") in [c.get("name") for c in (charts.get("charts") or [])], "Excel listCharts thấy biểu đồ vừa tạo", charts)
    b.cmd("et.addChart", {"sheet": "Sheet1", "range": "A1:B3", "type": "donut"}, expect_ok=False, key="et.addChart kiểu lạ")
    filled = b.cmd("et.fillRange", {"sheet": "Raw_Data", "range": "A1:A20", "formula": "=ROW()*2"}) or {}
    column = (b.cmd("et.readRange", {"range": "A1:A20", "sheet": "Raw_Data"}, record=False) or {}).get("values") or []
    check(filled.get("filled") == 20 and len(column) == 20 and column[0][0] == 2 and column[19][0] == 40,
          "Excel fillRange điền đủ ô, tham chiếu tương đối tự dịch", {"fill": filled, "first": column[:1], "last": column[-1:]})
    batch = b.cmd("et.writeRanges", {"sheet": "Raw_Data", "writes": [
        {"range": "C1", "values": [["gộp", 1], ["một", 2]]},
        {"range": "A1", "sheet": "Data_Cleaning", "values": [["khác sheet"]]}]}) or {}
    check(batch.get("ranges") == 2 and batch.get("written") == 5, "Excel writeRanges ghi 2 vùng bằng MỘT lời gọi", batch)
    b.cmd("et.writeRanges", {"sheet": "Raw_Data", "writes": [{"range": "F1", "values": [["không được ghi"]]}, {"range": "G1"}]},
          expect_ok=False, key="et.writeRanges vùng sau thiếu values")
    check(((b.cmd("et.readRange", {"range": "F1", "sheet": "Raw_Data"}, record=False) or {}).get("values") or [[None]])[0][0] is None,
          "Excel writeRanges kiểm hết trước khi ghi (vùng đầu không bị ghi khi vùng sau sai)", "")
    raw = os.path.join(out, "raw-data.csv")
    with open(raw, "w", encoding="utf-8-sig", newline="") as handle:
        handle.write("Vùng,Mã,Số lượng,Giá,Ghi chú\nBắc,SP-001,12,1500.5,có dấu\nNam,SP-002,0,-2,\n")
    loaded = b.cmd("et.importCsv", {"path": raw}) or {}
    grid = (b.cmd("et.readRange", {"range": "A1:D3", "sheet": loaded.get("sheet") or "raw-data"}, record=False) or {}).get("values") or []
    check(loaded.get("rows") == 3 and loaded.get("columns") == 5 and grid and grid[0][0] == "Vùng" and grid[1][1] == "SP-001"
          and grid[1][3] == 1500.5, "Excel importCsv nạp file vào sheet mới, giữ chữ có dấu và số", {"load": loaded, "grid": grid})
    b.cmd("et.importCsv", {"path": os.path.join(out, "khong-co.csv")}, expect_ok=False, key="et.importCsv file thiếu")
    b.cmd("et.writeRange", {"range": "H1:H10", "sheet": "Raw_Data",
                            "values": [[10], [20], [30], [40], [50], [60], [70], [80], [90], [100]]}, record=False)
    applied = b.cmd("et.setConditionalFormat", {"sheet": "Raw_Data", "range": "H1:H10", "rules": [
        {"operator": "less", "formula1": "50", "fillColor": "#FFCCCC"},
        {"operator": "greaterEqual", "formula1": "90", "bold": True}]}) or {}
    check([r.get("operator") for r in applied.get("rules") or []] == ["less", "greaterEqual"],
          "Excel setConditionalFormat nhận 2 rule, trả đúng tên toán tử đã viết", applied)
    listed = b.cmd("et.listConditionalFormats", {"sheet": "Raw_Data"}) or {}
    mine = [f for f in (listed.get("formats") or []) if f.get("range", "").replace("$", "").endswith("H1:H10")]
    check(len(mine) == 1 and [(r.get("operator"), r.get("formula1"), r.get("formula2")) for r in mine[0].get("rules") or []]
          == [("less", "50", None), ("greaterEqual", "90", None)],
          "Excel listConditionalFormats nhóm theo vùng, đọc lại đúng toán tử và ngưỡng", listed)
    b.cmd("et.activateSheet", {"sheet": "Sheet1"}, record=False)


# ---------------------------------------------------------------- PowerPoint / WPP

def test_presentation(b, out, png, run_ai):
    b.cmd("app.info", key="app.info (trước khi có tài liệu)")
    b.cmd("wpp.newPresentation")
    b.cmd("wpp.addSlide", {"layout": 1})
    error = b.cmd("wpp.addSlide", {"layout": "Title Only"}, expect_ok=False, key="wpp.addSlide layout text")
    check("'layout' must be a whole number, got 'Title Only'" in (error or ""), "PowerPoint lỗi layout nêu tên tham số", error)
    b.cmd("wpp.addText", {"text": "Giới thiệu", "fontSize": 32, "bold": True, "color": "#1F4E79", "align": "center", "left": 40, "top": 40, "width": 600, "height": 60})
    b.cmd("wpp.addTextBox", {"text": "Hộp văn bản", "left": 60, "top": 150, "width": 400, "height": 50})
    b.cmd("wpp.addSlide", {})
    b.cmd("wpp.addImage", {"path": png, "left": 50, "top": 50, "width": 80, "height": 80})
    table = b.cmd("wpp.addTable", {"values": [["Quý", "Doanh thu"], ["Q1", 100], ["Q2", 120]]}, key="wpp.addTable values")
    check(table and table.get("rows") == 3 and table.get("cols") == 2, "PowerPoint addTable suy ra 3x2 từ values", table)
    b.cmd("wpp.addTable", {"rows": 2, "cols": 2, "values": {"a": 1}}, expect_ok=False, key="wpp.addTable invalid")
    b.cmd("wpp.setNotes", {"text": "Ghi chú thuyết trình", "slide": 1})
    listing = b.cmd("wpp.listSlides")
    check(listing and listing.get("slideCount") == 2, "PowerPoint listSlides có 2 slide", listing)
    b.cmd("wpp.addSlide", {"layout": 12}, key="wpp.addSlide third")
    b.cmd("wpp.deleteSlide", {})
    # QA cấu trúc (docs/architecture/core.md 8.4.6): ca dựng sẵn - hai hộp chồng nhau + chữ dài tràn quá đáy slide.
    clean = b.cmd("wpp.checkLayout", {"slide": 1}, key="wpp.checkLayout clean slide")
    check(clean and clean.get("issueCount") == 0, "PowerPoint checkLayout slide gọn không báo lỗi", clean)
    b.cmd("wpp.addSlide", {"layout": 12}, key="wpp.addSlide qa")
    b.cmd("wpp.addText", {"text": "Hộp A", "left": 40, "top": 100, "width": 400, "height": 100, "fontSize": 20}, key="wpp.addText qa A")
    b.cmd("wpp.addText", {"text": "Hộp B", "left": 80, "top": 105, "width": 400, "height": 100, "fontSize": 20}, key="wpp.addText qa B")
    long_text = "\r".join("Dòng nội dung dài số %d để tràn quá đáy slide" % i for i in range(1, 9))
    b.cmd("wpp.addText", {"text": long_text, "left": 40, "top": 380, "width": 600, "height": 60, "fontSize": 20}, key="wpp.addText qa overflow")
    b.cmd("wpp.addText", {"text": "chú thích nhỏ", "left": 500, "top": 20, "width": 200, "height": 30, "fontSize": 9}, key="wpp.addText qa small")
    qa = b.cmd("wpp.checkLayout", {"slide": 3}, key="wpp.checkLayout seeded issues") or {}
    kinds = {i.get("type") for s in qa.get("slides", []) for i in s.get("issues", [])}
    check({"overlap", "offslide", "small-font"} <= kinds or ({"overlap", "overflow", "small-font"} <= kinds),
          "PowerPoint checkLayout bắt được chồng + tràn + chữ nhỏ", qa)
    b.cmd("wpp.checkLayout", {"slide": 99}, expect_ok=False, key="wpp.checkLayout bad slide")
    b.cmd("wpp.deleteSlide", {}, key="wpp.deleteSlide qa")
    # Lưới của skill trinh-bay-chuyen-nghiep (đo 13/10/2026): hộp chữ giữ chiều cao đã đặt nên tiêu đề 3 dòng
    # báo overflow (trước đây hộp tự giãn, đè lên nội dung 9pt mà không lỗi nào); ảnh chỉ truyền width giữ tỉ lệ.
    b.cmd("wpp.addSlide", {"layout": 12}, key="wpp.addSlide grid")
    long_title = ("Kênh trực tuyến tăng 32%, bù được phần giảm của kênh truyền thống, nhờ chương trình khuyến mãi "
                  "tháng 9 và kênh đối tác mới mở tại ba tỉnh")
    b.cmd("wpp.addText", {"text": long_title, "left": 40, "top": 30, "width": 880, "height": 80, "fontSize": 28, "bold": True},
          key="wpp.addText 3-line title")
    wide = tiny_png(os.path.join(out, "wide.png"), 32, 18)
    b.cmd("wpp.addImage", {"path": wide, "left": 40, "top": 150, "width": 320}, key="wpp.addImage width only")
    grid = b.cmd("wpp.checkLayout", {"slide": 3}, key="wpp.checkLayout grid") or {}
    shapes = (grid.get("slides") or [{}])[0].get("shapes", [])
    kinds = [i.get("type") for s in grid.get("slides", []) for i in s.get("issues", [])]
    check(len(shapes) == 2 and shapes[0]["height"] == 80 and kinds == ["overflow"],
          "PowerPoint addText giữ chiều cao 80, tiêu đề 3 dòng báo overflow", grid)
    check(len(shapes) == 2 and abs(shapes[1]["width"] - 320) < 1 and abs(shapes[1]["height"] - 180) < 1,
          "PowerPoint addImage chỉ width giữ tỉ lệ 16:9 (320 x 180)", grid)
    b.cmd("wpp.deleteSlide", {}, key="wpp.deleteSlide grid")
    pdf = os.path.join(out, "wpp.pdf")
    b.cmd("wpp.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 1000, "PowerPoint exportPdf tạo file", pdf)
    pptx = os.path.join(out, "wpp.pptx")
    b.cmd("wpp.saveAs", {"path": pptx})
    b.cmd("wpp.save")
    test_presentation_template(b, out, pptx)
    reopened = b.cmd("wpp.open", {"path": pptx}) or {}
    back = b.cmd("wpp.listSlides", record=False) or {}
    check(back.get("presentation") == "wpp.pptx", "PowerPoint wpp.open bài đã mở đưa nó lên làm bài đang hoạt động",
          {"open": reopened, "active": back.get("presentation")})
    b.cmd("wpp.listSlides", key="wpp.listSlides reopened")
    if run_ai:
        b.ask("Thêm một slide mới ở cuối với tiêu đề 'Kết luận'", "activePresentation", target=pptx)
        slides = b.cmd("wpp.listSlides", {"presentation": pptx}, record=False) or {}
        last = (slides.get("slides") or [{}])[-1].get("shapeTexts", [])
        check(slides.get("slideCount") == 3 and any("Kết luận" in t for t in last), "PowerPoint ai.ask thêm slide 'Kết luận'", slides)
        b.cmd("wpp.save", record=False)


def test_presentation_template(b, out, pptx):
    """Làm bài theo mẫu (docs/architecture/template-design.md): bài mới từ wpp.pptx, chép slide/ảnh, sắp xếp chữ/bảng."""
    manifest = b.cmd("wpp.sourceManifest", {"from": pptx}) or {}
    check(manifest.get("slideCount") == 2 and len(manifest.get("slides") or []) == 2,
          "PowerPoint sourceManifest đọc 2 slide của bài nguồn", manifest)
    made = os.path.join(out, "from-template.pptx")
    created = b.cmd("wpp.newFromTemplate", {"template": pptx, "path": made}) or {}
    check(created.get("slideCount") == 0 and created.get("templateSlides") == 2 and os.path.exists(made),
          "PowerPoint newFromTemplate tạo bài mới rỗng, giữ khổ của mẫu", created)
    # Mẫu đang mở (wpp.pptx): trước 25/10/2026 lệnh này lấy chính bài đang mở, xoá hết slide rồi đổi tên sang `made`.
    still = b.cmd("wpp.sourceManifest", {"from": pptx}, record=False) or {}
    check(still.get("slideCount") == 2 and created.get("path", "").lower() == made.lower(),
          "PowerPoint newFromTemplate KHÔNG đụng tới bài mẫu đang mở (vẫn đủ 2 slide)", still.get("slideCount"))
    b.cmd("wpp.newFromTemplate", {"template": pptx, "path": made}, expect_ok=False, key="wpp.newFromTemplate path đã có")
    active = b.cmd("wpp.listSlides", record=False) or {}
    check(active.get("presentation") == os.path.basename(made), "PowerPoint newFromTemplate đưa bài mới lên làm bài đang hoạt động",
          active.get("presentation"))
    copied = b.cmd("wpp.addSlideFromTemplate", {"template": pptx, "kind": 1, "tag": 1}) or {}
    names = [s.get("shape") for s in (copied.get("textShapes") or [])]
    check(copied.get("slideCount") == 1 and names, "PowerPoint addSlideFromTemplate chép slide 1 của mẫu (có khung chữ)", copied)
    # Mẫu đang mở thì InsertFromFile đẩy cửa sổ mẫu lên: bài đang hoạt động phải vẫn là bài mới.
    active = b.cmd("wpp.listSlides", record=False) or {}
    check(active.get("presentation") == os.path.basename(made), "PowerPoint addSlideFromTemplate giữ bài mới là bài đang hoạt động",
          active.get("presentation"))
    if names:
        text = b.cmd("wpp.setShapeText", {"shape": names[0], "text": "Tiêu đề theo mẫu", "slide": 1}) or {}
        check(text.get("shape") == names[0] and text.get("chars") == len("Tiêu đề theo mẫu"),
              "PowerPoint setShapeText thay chữ theo tên shape", text)
    b.cmd("wpp.setShapeText", {"shape": "Không có shape này", "text": "x", "slide": 1}, expect_ok=False, key="wpp.setShapeText shape lạ")
    b.cmd("wpp.addSlide", {"layout": 12}, record=False)
    picture = b.cmd("wpp.copyPicture", {"from": pptx, "fromSlide": 2, "slide": 2, "left": 40, "top": 40, "width": 200, "height": 200}) or {}
    check(picture.get("slide") == 2 and picture.get("shape"), "PowerPoint copyPicture chép ảnh từ bài khác", picture)
    paras = b.cmd("wpp.addParagraphs", {"slide": 2, "left": 300, "top": 40, "width": 500, "height": 300, "paragraphs": [
        {"t": "Mục chính", "l": 1, "b": True}, {"t": "Ý phụ một", "l": 2, "u": True}, {"t": "Ý phụ hai", "l": 2, "u": True}]}) or {}
    check(paras.get("shape") and paras.get("overflow") is False, "PowerPoint addParagraphs chữ nhiều cấp vừa khung", paras)
    grid = b.cmd("wpp.addGrid", {"slide": 2, "left": 40, "top": 300, "width": 700, "height": 150,
                                 "values": [["Quý", "Doanh thu"], ["Q1", "100"], ["Q2", "120"]]}) or {}
    check(grid.get("shape") and grid.get("overflow") is False, "PowerPoint addGrid bảng định dạng vừa khung", grid)
    deleted = b.cmd("wpp.deleteShapes", {"slide": 2, "shapes": [picture.get("shape"), "Không có"]}) or {}
    check(deleted.get("deleted") == [picture.get("shape")] and deleted.get("notFound") == ["Không có"],
          "PowerPoint deleteShapes xoá theo tên, báo tên không thấy", deleted)
    tagged = b.cmd("wpp.deleteTagged", {"first": 1, "last": 1}) or {}
    slides = b.cmd("wpp.listSlides", record=False) or {}
    check(tagged.get("deleted") == 1 and slides.get("slideCount") == 1, "PowerPoint deleteTagged xoá slide dựng từ mẫu (tag 1)",
          {"tagged": tagged, "slides": slides.get("slideCount")})
    b.cmd("wpp.save", record=False)


def test_http(port, label):
    status, _ = http(port, "GET", "/health")
    check(status == 200, label + " /health không cần token")
    status, _ = http(port, "POST", "/cmd", b'{"action":"app.info"}', {"Content-Type": "application/json"})
    check(status == 401, label + " /cmd thiếu token -> 401", status)
    status, _ = http(port, "POST", "/cmd", b'{"action":"app.info"}', {"X-Auth-Token": token(), "Content-Type": "text/plain"})
    check(status == 415, label + " /cmd sai Content-Type -> 415", status)
    status, _ = http(port, "POST", "/cmd", b'{"action":"app.info"}', {"X-Auth-Token": token(), "Content-Type": "application/json", "Origin": "http://evil"})
    check(status == 403, label + " /cmd có Origin -> 403", status)
    status, reply = http(port, "GET", "/session", headers={"X-Auth-Token": token()})
    check(status == 200 and reply["result"]["port"] == port, label + " /session", reply)
    status, reply = http(port, "GET", "/config", headers={"X-Auth-Token": token()})
    check(status == 200 and "apiKey" in reply["result"], label + " /config (key đã che)", reply)
    test_commands(port, label)


def test_commands(port, label):
    """GET /commands: bo lenh cua dung DLL dang chay, de Agent Core dung tool office_action."""
    status, _ = http(port, "GET", "/commands")
    check(status == 401, label + " /commands thiếu token -> 401", status)

    status, reply = http(port, "GET", "/commands", headers={"X-Auth-Token": token()})
    if status != 200 or not isinstance(reply, dict) or "result" not in reply:
        check(False, label + " /commands", reply)
        return
    result = reply["result"]
    commands = result.get("commands") or []
    names = [c.get("name") for c in commands]
    check(result.get("version") == http(port, "GET", "/health")[1]["result"]["version"],
          label + " /commands version khớp /health", result.get("version"))
    check(len(commands) >= 47 and len(set(names)) == len(names), label + " /commands danh sách lệnh", len(commands))

    registry = registered_actions()
    check(registry is not None and set(names) == registry,
          label + " /commands khớp registry (Host.exe commands --json)",
          sorted(set(names) ^ (registry or set())))

    sample = next((c for c in commands if c.get("name") == "et.readRange"), None)
    check(sample is not None and sample.get("kind") == "et" and sample.get("agent") is True
          # Tham số chọn tài liệu (`workbook`, H1 - nhắm đúng tài liệu) đứng sau ba tham số của lệnh.
          and [p.get("name") for p in sample.get("params") or []][:3] == ["range", "sheet", "formulas"]
          and all("required" in p and "hint" in p for p in sample["params"]),
          label + " /commands đủ trường name/kind/agent/summary/params", sample)


AGENT_TOOL_PS = r"""
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$tool = [Reflection.Assembly]::LoadFrom('%s').GetType('AxiomOffice.Ai.OfficeActionTool', $true)
$flags = [Reflection.BindingFlags]'Static,Public,NonPublic'
$out = @{}
foreach ($action in 'writer.closeAll', 'ai.ask', 'nosuch.action', 'et.newWorkbook') {
    $out[$action] = $tool.GetMethod('Execute', $flags).Invoke($null, @($null, 'office_action', ('{"action":"' + $action + '"}')))
}
$out | ConvertTo-Json -Compress
"""


def test_agent_tool():
    """Lệnh office_action của pane (không cần Office): chỉ nhận lệnh agent (ForAgent). Mô tả tool cho model
    giờ chỉ còn ở Core (core-go/internal/tools/office_action.go)."""
    output = subprocess.run(["powershell", "-NoProfile", "-Command", AGENT_TOOL_PS % ADDIN_DLL],
                            capture_output=True, text=True, encoding="utf-8", timeout=60).stdout
    data = json.loads(output)
    for action in ("writer.closeAll", "ai.ask", "nosuch.action", "et.newWorkbook"):
        reply = json.loads(data[action])
        check(reply["ok"] is False and "not an available action" in reply["error"], "office_action từ chối " + action, reply)


def core_health():
    """Doc core.json -> /health cua Agent Core; None neu Core khong chay."""
    path = os.path.join(os.environ.get("LOCALAPPDATA", ""), "AxiomOffice", "core.json")
    if not os.path.exists(path):
        return None
    try:
        with open(path, encoding="utf-8") as handle:
            port = json.load(handle)["port"]
        status, reply = http(port, "GET", "/health", timeout=5)
        return reply["result"] if status == 200 else None
    except Exception:
        return None


def test_core_version():
    """Core va bridge phai cung version (build.ps1 truyen version DLL cho Core)."""
    core = core_health()
    if core is None:
        print("(bỏ qua: Agent Core chưa chạy nên không so version)")
        return
    status, reply = http(core["port"], "GET", "/health")
    check(status == 200 and core.get("version") == reply["result"]["version"] and core.get("protocol") == 1,
          "Agent Core /health (version khớp bridge, protocol 1)", core)


def other_live_tests():
    """Lệnh đã có test live trên Office ở file khác (slide, định dạng Word) - không lặp lại ở đây."""
    names = set()
    here = os.path.dirname(os.path.abspath(__file__))
    # test_office_*.py: lenh plan 21 (cau truc, bo cuc, van ban theo mau, dashboard) - test live rieng tung nhom.
    files = ["test_live_slides.py", "test_writer_format.py"] + sorted(
        name for name in os.listdir(here) if name.startswith("test_office_") and name.endswith(".py"))
    for name in files:
        path = os.path.join(here, name)
        if os.path.exists(path):
            with open(path, encoding="utf-8") as handle:
                names.update(re.findall(r'"((?:writer|et|wpp|app)\.[A-Za-z]+)"', handle.read()))
    return names


def registered_actions():
    """Danh sách lệnh bridge từ registry (AxiomOffice.Host.exe commands --json); None nếu bản cũ chưa có."""
    try:
        output = subprocess.run([HOST_EXE, "commands", "--json"], capture_output=True, text=True, encoding="utf-8", timeout=30).stdout
        return {item["name"] for item in json.loads(output)}
    except Exception:
        return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--wps", action="store_true", help="chạy trên WPS (47821-47823) thay vì Microsoft Office")
    parser.add_argument("--apps", default="word,excel,ppt")
    parser.add_argument("--ai", action="store_true", help="chạy thêm ai.ask (gọi LLM thật)")
    parser.add_argument("--record")
    parser.add_argument("--compare")
    parser.add_argument("--keep", action="store_true", help="không tắt app sau khi chạy")
    args = parser.parse_args()

    out = os.path.join(tempfile.gettempdir(), "axiom-live-test")
    shutil.rmtree(out, ignore_errors=True)
    os.makedirs(out)
    png = tiny_png(os.path.join(out, "logo.png"))
    test_agent_tool()
    family = "wps" if args.wps else "office"
    started = []
    try:
        for app in [a.strip() for a in args.apps.split(",") if a.strip()]:
            port, pid = launch(app, args.wps)
            started.append(pid)
            label = "%s-%s" % (family, app)
            print("== %s: port %d pid %d" % (label, port, pid))
            bridge = Bridge(port, out, label)
            test_http(port, label)
            if app == "word":
                test_writer(bridge, out, png, args.ai)
            elif app == "excel":
                test_spreadsheet(bridge, out, args.ai)
            else:
                test_presentation(bridge, out, png, args.ai)
    finally:
        if not args.keep:
            for pid in started:
                close_app(pid)

    test_core_version()

    registry = registered_actions()
    if registry is not None and args.apps == "word,excel,ppt":
        missing = sorted(registry - TESTED - other_live_tests() - ({"ai.ask"} if not args.ai else set()))
        check(not missing, "mọi lệnh trong registry đều có test", missing)

    if args.record:
        with open(args.record, "w", encoding="utf-8") as handle:
            json.dump(RECORD, handle, ensure_ascii=False, indent=1, sort_keys=True)
        print("recorded %d results -> %s" % (len(RECORD), args.record))
    if args.compare:
        with open(args.compare, encoding="utf-8") as handle:
            golden = json.load(handle)
        new_keys = sorted(set(RECORD) - set(golden))
        if new_keys:
            print("ca mới chưa có trong golden (ghi lại bằng --record):", ", ".join(new_keys))
        for key in sorted(golden):
            check(golden.get(key) == RECORD.get(key), "golden " + key,
                  "trước: %s\n     sau:  %s" % (json.dumps(golden.get(key), ensure_ascii=False)[:280], json.dumps(RECORD.get(key), ensure_ascii=False)[:280]))

    failed = [r for r in RESULTS if not r[0]]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
