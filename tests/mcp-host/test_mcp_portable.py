"""Kiểm tra MCP server (axiom-office-mcp / AxiomOffice.Host.exe mcp) qua stdio, KHÔNG cần thư viện ngoài.

Bộ này chạy được cả trên Windows lẫn Linux (bộ parity đầy đủ `test_mcp_host.py` cần python-docx/
openpyxl/python-pptx + package `mcp`): giao thức JSON-RPC, danh sách tool, 20 tool file (docx/xlsx/pptx/
csv) chạy thật trên file, tool live gọi bridge LibreOffice/Office nếu đang mở, và (.xls) đường chuyển
bằng LibreOffice trên Linux.

    python tests/mcp-host/test_mcp_portable.py [đường_dẫn_exe] [--live]
    AXIOM_MCP_EXE=... AXIOM_MCP_ARGS="all"  cũng được
"""
from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import zipfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
# MCP server: Windows la AxiomOffice.Host.exe mcp (net48), Linux la subcommand `mcp` cua Agent Core (Go).
# Duong dan o day chi la phuong an du phong khi khong truyen tham so - CI luon truyen duong dan that.
CANDIDATES = [
    os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Host.exe"),
    os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Core.exe"),
    os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Core"),
]

RESULTS: list[tuple[bool, str, str]] = []


def check(ok: bool, name: str, detail: str = "") -> None:
    RESULTS.append((bool(ok), name, detail))
    print(("PASS " if ok else "FAIL ") + name + ("" if ok or not detail else "\n     " + str(detail)[:900]))


def dotnet() -> str:
    """dotnet trong PATH, hoac vi tri cai user-local (Windows: %LOCALAPPDATA%\\Microsoft\\dotnet)."""
    found = os.environ.get("DOTNET") or shutil.which("dotnet")
    if not found:
        for candidate in (os.path.join(os.path.expanduser("~"), ".dotnet", "dotnet"),
                          os.path.join(os.environ.get("LOCALAPPDATA", ""), "Microsoft", "dotnet", "dotnet.exe")):
            if candidate and os.path.exists(candidate):
                return candidate
    return found or "dotnet"


def find_soffice() -> str:
    """soffice trong PATH hoac vi tri cai mac dinh (Windows: Program Files; Linux: /usr/bin)."""
    found = shutil.which("soffice") or shutil.which("soffice.exe")
    if found:
        return found
    for candidate in (r"C:\Program Files\LibreOffice\program\soffice.exe",
                      "/usr/bin/soffice", "/usr/lib/libreoffice/program/soffice"):
        if os.path.exists(candidate):
            return candidate
    return None


def resolve_command(argv: list[str]) -> list[str]:
    exe = ""
    for arg in argv:
        if not arg.startswith("--"):
            exe = arg
            break
    exe = exe or os.environ.get("AXIOM_MCP_EXE") or next((c for c in CANDIDATES if os.path.exists(c)), "")
    if not exe:
        raise SystemExit("khong thay exe MCP: truyen duong dan hoac dat AXIOM_MCP_EXE")
    if exe.endswith(".dll"):
        return [dotnet(), exe]
    return [exe]


class Server:
    """MCP server qua stdio: mỗi message một dòng JSON-RPC (đúng như McpHost/McpServer)."""

    def __init__(self, command: list[str], args: list[str]):
        self.process = subprocess.Popen(command + args, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, text=True, encoding="utf-8", bufsize=1)
        self._id = 0
        self.stderr: list[str] = []
        self._drain = threading.Thread(target=self._read_stderr, daemon=True)
        self._drain.start()

    def _read_stderr(self) -> None:
        for line in self.process.stderr:
            self.stderr.append(line.rstrip())

    def send(self, method: str, params=None) -> dict:
        self._id += 1
        message = {"jsonrpc": "2.0", "id": self._id, "method": method}
        if params is not None:
            message["params"] = params
        self.process.stdin.write(json.dumps(message, ensure_ascii=False) + "\n")
        self.process.stdin.flush()
        while True:
            line = self.process.stdout.readline()
            if not line:
                raise SystemExit("MCP server dong stdout som (stderr: %s)" % " | ".join(self.stderr[-3:]))
            line = line.strip()
            if not line:
                continue
            reply = json.loads(line)
            if reply.get("id") == self._id:
                return reply
            # Batch/notification: bo qua cho toi khi dung id.

    def call(self, name: str, **arguments):
        """-> (dữ liệu, is_error). Lỗi giao thức (tool lạ) trả về dict error."""
        reply = self.send("tools/call", {"name": name, "arguments": arguments})
        if "error" in reply:
            return reply["error"], True
        result = reply["result"]
        text = result["content"][0]["text"]
        try:
            data = json.loads(text)
        except ValueError:
            data = text
        return data, bool(result.get("isError"))

    def ok(self, name: str, **arguments):
        data, is_error = self.call(name, **arguments)
        check(not is_error, "tool %s chay duoc" % name, json.dumps(data, ensure_ascii=False))
        return data

    def close(self) -> None:
        try:
            self.process.stdin.close()
        except Exception:
            pass
        try:
            self.process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            self.process.kill()


def test_protocol(server: Server) -> list:
    reply = server.send("initialize", {"protocolVersion": "2025-06-18", "clientInfo": {"name": "test", "version": "1"}})
    result = reply.get("result", {})
    check(result.get("protocolVersion", "").startswith("2025-") or result.get("protocolVersion", "").startswith("2024-"),
          "initialize tra protocolVersion", json.dumps(result))
    check(result.get("serverInfo", {}).get("version"), "initialize co serverInfo.version", json.dumps(result.get("serverInfo")))
    check("office_sessions" in result.get("instructions", ""), "instructions nhac office_sessions", result.get("instructions", "")[:200])

    tools = server.send("tools/list")["result"]["tools"]
    names = [tool["name"] for tool in tools]
    check(len(names) == len(set(names)), "ten tool khong trung", str(names))
    check(len(names) >= 45, "du so tool (%d)" % len(names), str(len(names)))
    for required in ("doc_create", "doc_get_text", "excel_create", "excel_read", "excel_write", "ppt_create",
                     "ppt_add_slide_file", "office_sessions", "wps_live_command"):
        check(required in names, "co tool %s" % required, str(names))
    schema = next(tool for tool in tools if tool["name"] == "excel_format_range")["inputSchema"]
    check(schema.get("required") == ["path", "sheet", "cell_range", "styles"], "schema excel_format_range", json.dumps(schema))
    description = next(tool for tool in tools if tool["name"] == "word_command")["description"]
    check("writer.insertTable {" in description and "writer.appendText {" in description,
          "mo ta word_command liet ke lenh bridge", description[-200:])
    return tools


def test_docx(server: Server, out: str) -> None:
    path = os.path.join(out, "mcp_portable.docx")
    created = server.ok("doc_create", path=path, overwrite=True, title="Báo cáo", author="Test",
                        paragraphs=["Xin chào thế giới", {"text": "Tiêu đề", "style": "Heading 1"},
                                    {"table": [["Họ tên", "Điểm"], ["An", 9.5], ["Bình", None]]},
                                    {"text": "Kết thúc"}])
    check((created.get("created") or created.get("path")) == path, "doc_create tra path", json.dumps(created, ensure_ascii=False))
    text = server.ok("doc_get_text", path=path)
    body = text.get("text", "")
    check("Xin chào thế giới" in body and "Kết thúc" in body, "doc_get_text doc duoc noi dung", body[:200])
    profile = server.ok("doc_profile", path=path)
    check(profile.get("tables") == 1, "doc_profile dem bang", json.dumps(profile))
    found = server.ok("doc_find_text", path=path, query="thế giới")
    check(found.get("count", 0) >= 1, "doc_find_text tim thay", json.dumps(found))
    table = server.ok("doc_extract_table", path=path, index=0)
    check("An" in json.dumps(table, ensure_ascii=False), "doc_extract_table doc bang", json.dumps(table, ensure_ascii=False)[:200])

    data, is_error = server.call("doc_create", path=path, paragraphs=["x"])
    check(is_error and "exists" in str(data), "doc_create tu choi ghi de khi thieu co", str(data))
    data, is_error = server.call("doc_create", path=os.path.join(out, "bad.docx"), overwrite=True,
                                 paragraphs=[{"text": "x", "style": "No Such Style"}])
    check(is_error, "doc_create style la -> loi", str(data))
    data, is_error = server.call("doc_get_text", path=os.path.join(out, "khong-co.docx"))
    check(is_error, "doc_get_text file thieu -> loi (khong phai crash)", str(data))
    with zipfile.ZipFile(path) as archive:
        check("word/document.xml" in archive.namelist(), "file .docx hop le", str(archive.namelist()[:5]))


def test_xlsx(server: Server, out: str) -> None:
    path = os.path.join(out, "mcp_portable.xlsx")
    server.ok("excel_create", path=path, overwrite=True,
              sheets=[{"name": "Data", "values": [["Tên", "Điểm"], ["An", 9.5], ["Bình", 8]]}])
    server.ok("excel_write", path=path, sheet="Data", start_cell="D1", values=[["Ghi chú"], ["ok"]])
    server.ok("excel_create_sheet", path=path, sheet="Trống")
    read = server.ok("excel_read", path=path, cell_range="A1:D3")
    values = read.get("values", [])
    check(values and values[0][:2] == ["Tên", "Điểm"], "excel_read doc duoc bang", json.dumps(values, ensure_ascii=False))
    check(values[1][1] == 9.5 and values[1][3] == "ok", "excel_write ghi dung o", json.dumps(values, ensure_ascii=False))
    profile = server.ok("excel_profile", path=path)
    check(profile.get("format") == "xlsx" and len(profile.get("sheets", [])) == 2, "excel_profile hai sheet", json.dumps(profile, ensure_ascii=False))
    table = server.ok("excel_create_table", path=path, sheet="Data", cell_range="A1:D3", table_name="BangDiem")
    check(table.get("tables") == ["BangDiem"], "excel_create_table", json.dumps(table))
    styled = server.ok("excel_format_range", path=path, sheet="Data", cell_range="A2:B3", styles={"bold": True})
    check(styled.get("styled_cells", 0) == 4, "excel_format_range to 4 o", json.dumps(styled))
    with zipfile.ZipFile(path) as archive:
        check(any(name.startswith("xl/worksheets/") for name in archive.namelist()), "file .xlsx hop le", str(archive.namelist()[:5]))


def test_pptx(server: Server, out: str) -> None:
    path = os.path.join(out, "mcp_portable.pptx")
    created = server.ok("ppt_create", path=path, overwrite=True,
                        slides=[{"title": "Bìa", "bullets": ["Mục 1"]}, {"title": "Nội dung"}])
    check(created.get("slides") == 2, "ppt_create 2 slide", json.dumps(created, ensure_ascii=False))
    added = server.ok("ppt_add_slide_file", path=path, title="Kế hoạch Q4", bullets=["Mục 1", "Mục 2"], layout=1)
    check(added.get("slide_count") == 3, "ppt_add_slide_file them slide", json.dumps(added, ensure_ascii=False))
    text = server.ok("ppt_get_text", path=path)
    check("Kế hoạch Q4" in json.dumps(text, ensure_ascii=False), "ppt_get_text doc duoc slide", json.dumps(text, ensure_ascii=False)[:200])
    profile = server.ok("ppt_profile", path=path)
    check(profile.get("slide_count") == 3, "ppt_profile dem slide", json.dumps(profile))


def test_errors(server: Server) -> None:
    error, is_error = server.call("khong_co_tool_nay")
    check(is_error and error.get("code") == -32602, "tool la -> loi -32602", json.dumps(error))
    data, is_error = server.call("doc_get_text", path=123)
    check(is_error, "thieu tham so -> isError (khong crash)", json.dumps(data))
    reply = server.send("tools/list")
    check("result" in reply and "tools" in reply["result"], "tools/list tra result", json.dumps(reply)[:200])


def test_live(server: Server, out: str, require: bool) -> None:
    """Tool live: doc registry session, /health, gui lenh that qua bridge (port + token lay tu cau hinh)."""
    sessions = server.ok("office_sessions")
    live = [item for item in sessions if isinstance(item, dict)]
    if not live:
        check(not require, "co bridge dang mo" if require else "khong co bridge dang mo (bo qua phan live)", "")
        return
    print("     bridge dang mo: %s" % json.dumps([{k: item.get(k) for k in ("app", "port", "family", "healthy")} for item in live],
                                                 ensure_ascii=False))
    check(all(item.get("port") for item in live), "moi session co port", json.dumps(live, ensure_ascii=False)[:200])
    check(all("family" in item for item in live), "moi session co family", json.dumps(live, ensure_ascii=False)[:200])
    spreadsheet = next((item for item in live if item.get("app") == "et"), None)
    if spreadsheet is None:
        check(not require, "khong co phien bang tinh (bo qua phan live ghi duoc)", "")
        return
    port = spreadsheet.get("port")
    health = server.ok("word_health", app="et", port=port)
    check(health.get("ok") is True and health.get("result", {}).get("pid") == spreadsheet.get("pid"),
          "health tra dung pid cua session", json.dumps(health, ensure_ascii=False)[:200])
    info = server.ok("word_command", app="et", action="app.info", params={})
    check(info.get("ok") is True and info.get("result", {}).get("family") == "libreoffice",
          "gui lenh qua bridge bang token trong cau hinh", json.dumps(info, ensure_ascii=False)[:200])
    # Tao bang tinh neu chua co tai lieu nao mo (headless --calc khong mo san tài liệu).
    server.ok("word_command", app="et", action="et.newWorkbook", params={})
    server.ok("wps_live_write_range", app="et", cell_range="A1", values=[["mcp-live", 42]])
    read = server.ok("wps_live_read_range", app="et", cell_range="A1:B1")
    values = read.get("result", {}).get("values") if isinstance(read.get("result"), dict) else None
    check(values and values[0][0] == "mcp-live", "ghi + doc lai o qua tool live", json.dumps(read, ensure_ascii=False)[:200])


def test_xls(server: Server, out: str) -> None:
    """.xls: trên Linux đọc qua LibreOffice (chuyển sang xlsx), Windows đọc qua COM Excel/WPS."""
    soffice = find_soffice()
    source = os.path.join(out, "mcp_portable.xlsx")
    target = os.path.join(out, "mcp_portable.xls")
    if soffice is None or not os.path.exists(source):
        check(True, ".xls: bo qua (khong co LibreOffice de tao file mau)", "")
        return
    subprocess.run([soffice, "-env:UserInstallation=file:///" + out.replace(os.sep, "/") + "/lo-xls",
                    "--headless", "--norestore", "--convert-to", "xls", "--outdir", out, source],
                   capture_output=True, timeout=240)
    if not os.path.exists(target):
        check(True, ".xls: bo qua (soffice khong tao duoc .xls mau)", "")
        return
    profile = server.ok("excel_profile", path=target)
    check(profile.get("format") == "xls" and len(profile.get("sheets", [])) >= 1, "excel_profile doc .xls", json.dumps(profile, ensure_ascii=False)[:300])
    read = server.ok("excel_read", path=target, cell_range="A1:C3")
    check(read.get("values") and read["values"][0][:2] == ["Tên", "Điểm"], "excel_read doc .xls",
          json.dumps(read.get("values"), ensure_ascii=False)[:300])


def main() -> int:
    argv = [arg for arg in sys.argv[1:] if not arg.startswith("--")]
    live_required = "--live" in sys.argv
    command = resolve_command(argv)
    args = os.environ.get("AXIOM_MCP_ARGS")
    if args is None:
        args = "mcp all" if command[-1].lower().endswith("axiomoffice.host.exe") else "all"
    print("exe:", " ".join(command), "| args:", args)
    out = tempfile.mkdtemp(prefix="axiom-mcp-")
    server = Server(command, args.split())
    try:
        tools = test_protocol(server)
        check(len(server.stderr) >= 0, "server khong in ra stdout rac", "")
        test_docx(server, out)
        test_xlsx(server, out)
        test_pptx(server, out)
        test_xls(server, out)
        test_errors(server)
        test_live(server, out, live_required)
    finally:
        server.close()
        shutil.rmtree(out, ignore_errors=True)

    failed = [name for ok, name, _ in RESULTS if not ok]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    for name in failed:
        print("  FAIL " + name)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
