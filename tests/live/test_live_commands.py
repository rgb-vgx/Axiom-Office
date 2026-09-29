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
import winreg
import zlib

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
    with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\AxiomOffice") as key:
        return winreg.QueryValueEx(key, "Token")[0]


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

    def ask(self, prompt, document_key):
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


def tiny_png(path):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    raw = b"".join(b"\x00" + b"\xcc\x44\x22" * 16 for _ in range(16))
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 16, 16, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b"")
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
    b.cmd("writer.insertStyledText", {"text": " Chữ đỏ đậm", "bold": True, "color": "#C00000", "size": 14})
    b.cmd("writer.heading", {"level": 1, "text": "Tiêu đề một"})
    b.cmd("writer.formatSelection", {"italic": True, "alignment": "center"})
    b.cmd("writer.setParagraphAlignment", {"alignment": "justify"})
    table = b.cmd("writer.insertTable", {"values": [["Tên", "Điểm"], ["An", 9.5], ["Bình", 8]], "style": "Table Grid"}, key="writer.insertTable values")
    check(table and table.get("rows") == 3 and table.get("cols") == 2, "Word insertTable suy ra 3x2 từ values", table)
    b.cmd("writer.insertTable", {"rows": 2, "cols": 2, "values": {"item": [{"item": ["x", "y"]}]}}, key="writer.insertTable wrapped")
    b.cmd("writer.insertTable", {"rows": 2, "cols": 2, "values": {"a": 1}}, expect_ok=False, key="writer.insertTable invalid")
    b.cmd("writer.insertTable", {}, expect_ok=False, key="writer.insertTable missing")
    b.cmd("writer.insertPageBreak")
    b.cmd("writer.insertImage", {"path": png, "width": 40, "height": 40})
    b.cmd("writer.insertHyperlink", {"url": "https://example.com", "text": "ví dụ"})
    replaced = b.cmd("writer.replaceAll", {"find": "Axiom", "replace": "AXIOM"})
    b.cmd("writer.undo", {"count": 1})
    after = b.cmd("writer.getText", {"maxChars": 40}, key="writer.getText after undo")
    check(after and "Axiom" in after["text"], "Word undo hoàn tác replaceAll", {"replace": replaced, "text": after})
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
    values = b.cmd("et.readRange", {"range": "A1:C3"})
    check(values and values["values"][1][2] == 19, "Excel công thức =B2*2 tính ra 19", values)
    b.cmd("et.readRange", {"range": "E1:K3"}, key="et.readRange others")
    b.cmd("et.formatRange", {"range": "A1:C1", "bold": True, "fillColor": "#FFFF00", "fontColor": "#0000FF", "numFmt": "0.00", "horizontal": "center", "wrap": True})
    b.cmd("et.activateSheet", {"sheet": "Sheet1"})
    b.cmd("et.writeRange", {"range": "A5", "values": [["hoàn tác tôi"]]}, key="et.writeRange before undo")
    b.cmd("et.undo", {"count": 1})
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


# ---------------------------------------------------------------- PowerPoint / WPP

def test_presentation(b, out, png, run_ai):
    b.cmd("app.info", key="app.info (trước khi có tài liệu)")
    b.cmd("wpp.newPresentation")
    b.cmd("wpp.addSlide", {"layout": 1})
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
    pdf = os.path.join(out, "wpp.pdf")
    b.cmd("wpp.exportPdf", {"path": pdf})
    check(os.path.getsize(pdf) > 1000, "PowerPoint exportPdf tạo file", pdf)
    pptx = os.path.join(out, "wpp.pptx")
    b.cmd("wpp.saveAs", {"path": pptx})
    b.cmd("wpp.save")
    b.cmd("wpp.open", {"path": pptx})
    b.cmd("wpp.listSlides", key="wpp.listSlides reopened")
    if run_ai:
        b.ask("Thêm một slide mới ở cuối với tiêu đề 'Kết luận'", "activePresentation")
        slides = b.cmd("wpp.listSlides", record=False) or {}
        last = (slides.get("slides") or [{}])[-1].get("shapeTexts", [])
        check(slides.get("slideCount") == 3 and any("Kết luận" in t for t in last), "PowerPoint ai.ask thêm slide 'Kết luận'", slides)
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


AGENT_TOOL_PS = r"""
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$tool = [Reflection.Assembly]::LoadFrom('%s').GetType('AxiomOffice.Ai.OfficeActionTool', $true)
$flags = [Reflection.BindingFlags]'Static,Public,NonPublic'
$out = @{}
foreach ($action in 'writer.closeAll', 'ai.ask', 'nosuch.action') {
    $out[$action] = $tool.GetMethod('Execute', $flags).Invoke($null, @($null, 'office_action', ('{"action":"' + $action + '"}')))
}
foreach ($kind in 'wps', 'et', 'wpp') {
    $def = $tool.GetMethod('Definition', $flags).Invoke($null, @($kind))
    $out['definition:' + $kind] = $def.GetType().GetField('Description').GetValue($def)
}
$out | ConvertTo-Json -Compress
"""


def test_agent_tool():
    """Tool office_action của agent (không cần Office): chỉ nhận lệnh có trong mô tả tool."""
    output = subprocess.run(["powershell", "-NoProfile", "-Command", AGENT_TOOL_PS % ADDIN_DLL],
                            capture_output=True, text=True, encoding="utf-8", timeout=60).stdout
    data = json.loads(output)
    for action in ("writer.closeAll", "ai.ask", "nosuch.action"):
        reply = json.loads(data[action])
        check(reply["ok"] is False and "not an available action" in reply["error"], "office_action từ chối " + action, reply)
    writer = data["definition:wps"]
    check("writer.typeText {text}" in writer and "writer.appendText {text}" in writer, "office_action (Writer) có typeText/appendText", writer)
    listed = " ".join(data["definition:" + kind] for kind in ("wps", "et", "wpp"))
    check("writer.closeAll" not in listed and "ai.ask {" not in listed, "office_action không liệt kê writer.closeAll / ai.ask", listed)


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

    registry = registered_actions()
    if registry is not None and args.apps == "word,excel,ppt":
        missing = sorted(registry - TESTED - ({"ai.ask"} if not args.ai else set()))
        check(not missing, "mọi lệnh trong registry đều có test", missing)

    if args.record:
        with open(args.record, "w", encoding="utf-8") as handle:
            json.dump(RECORD, handle, ensure_ascii=False, indent=1, sort_keys=True)
        print("recorded %d results -> %s" % (len(RECORD), args.record))
    if args.compare:
        with open(args.compare, encoding="utf-8") as handle:
            golden = json.load(handle)
        for key in sorted(set(golden) | set(RECORD)):
            check(golden.get(key) == RECORD.get(key), "golden " + key,
                  "trước: %s\n     sau:  %s" % (json.dumps(golden.get(key), ensure_ascii=False)[:280], json.dumps(RECORD.get(key), ensure_ascii=False)[:280]))

    failed = [r for r in RESULTS if not r[0]]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
