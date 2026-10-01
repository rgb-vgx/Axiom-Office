"""Doi chieu MCP server Go voi MCP server C# tren cung mot kich ban, tung buoc mot.

Hai ban MCP server cua project nay doc lap nhau (Windows: AxiomOffice.Host.exe net48; Linux: subcommand
`mcp` cua Agent Core Go) nhung phai tra ket qua giong nhau. Script nay chay cung mot kich ban tren ca
hai roi so JSON tung buoc, de phat hien lech som.

Can build truoc:
    powershell -File scripts/build.ps1        (ra AxiomOffice.Host.exe va AxiomOffice.Core.exe)

    python tests/mcp-host/oracle_diff.py

Dat lai duong dan bang AXIOM_CS_EXE / AXIOM_GO_EXE neu build ra cho khac.
"""
from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import tempfile

try:
    sys.stdout.reconfigure(encoding="utf-8")
except Exception:
    pass

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
BUILD_DIR = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release")

FAILURES: list[str] = []
CHECKS = 0


class Server:
    def __init__(self, command: list[str]):
        self.command = command
        self.process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=subprocess.PIPE, text=True, encoding="utf-8", bufsize=1)
        self._id = 0
        self.send("initialize", {"protocolVersion": "2025-06-18", "clientInfo": {"name": "diff", "version": "1"}})

    def send(self, method: str, params=None):
        self._id += 1
        message = {"jsonrpc": "2.0", "id": self._id, "method": method}
        if params is not None:
            message["params"] = params
        self.process.stdin.write(json.dumps(message) + "\n")
        self.process.stdin.flush()
        while True:
            line = self.process.stdout.readline()
            if not line:
                raise SystemExit("server %s thoat; stderr:\n%s" % (self.command, self.process.stderr.read()[:2000]))
            line = line.strip()
            if not line:
                continue
            reply = json.loads(line)
            if reply.get("id") == self._id:
                return reply

    def call(self, name: str, **arguments):
        reply = self.send("tools/call", {"name": name, "arguments": arguments})
        if "error" in reply:
            return {"__rpc_error__": reply["error"]}
        result = reply["result"]
        text = result["content"][0]["text"] if result.get("content") else ""
        data = json.loads(text) if text else None
        if result.get("isError"):
            return {"__tool_error__": data}
        return data

    def tools(self) -> list[str]:
        return sorted(tool["name"] for tool in self.send("tools/list")["result"]["tools"])

    def close(self):
        try:
            self.process.stdin.close()
            self.process.wait(timeout=10)
        except Exception:
            self.process.kill()


TIMESTAMP = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}")


def normalize(value, out: str):
    """Bo duong dan thu muc tam va moc thoi gian (moi server ghi gio cua chinh no)."""
    if isinstance(value, str):
        return TIMESTAMP.sub("<TS>", value.replace(out, "<OUT>").replace(out.replace("\\", "/"), "<OUT>"))
    if isinstance(value, list):
        return [normalize(item, out) for item in value]
    if isinstance(value, dict):
        return {key: normalize(item, out) for key, item in value.items()}
    return value


def compare(label: str, got, want) -> None:
    global CHECKS
    CHECKS += 1
    if got == want:
        print("  PASS " + label)
        return
    FAILURES.append(label)
    print("  FAIL " + label)
    print("     go: " + json.dumps(got, ensure_ascii=False, default=str)[:600])
    print("     cs: " + json.dumps(want, ensure_ascii=False, default=str)[:600])


def pptx_scenario(server: Server, out: str) -> dict:
    path = os.path.join(out, "bai-giang.pptx")
    results = {}
    results["create"] = server.call("ppt_create", path=path, slides=[
        "Trang bìa",
        {"title": "Mục tiêu", "bullets": ["Hiểu OOXML", "Viết được tool"], "layout": 1},
        {"title": "Kết", "bullets": ["Hết"]},
        {"title": "Chỉ tiêu đề"},
    ])
    results["create_again"] = server.call("ppt_create", path=path)
    results["create_overwrite"] = server.call("ppt_create", path=path, overwrite=True,
                                              slides=[{"title": "Làm lại", "bullets": ["a"]}])
    results["add_slide"] = server.call("ppt_add_slide_file", path=path, title="Thêm sau",
                                       bullets=["x", "y"])
    results["add_slide_no_title"] = server.call("ppt_add_slide_file", path=path, layout=0)
    results["profile"] = server.call("ppt_profile", path=path)
    results["get_text"] = server.call("ppt_get_text", path=path)
    results["get_text_trunc"] = server.call("ppt_get_text", path=path, max_chars=20)
    results["missing"] = server.call("ppt_profile", path=os.path.join(out, "khong-co.pptx"))
    results["not_pptx"] = server.call("ppt_profile", path=os.path.join(out, "a.xlsx"))
    return results


def docx_scenario(server: Server, out: str) -> dict:
    path = os.path.join(out, "van-ban.docx")
    results = {}
    results["create"] = server.call("doc_create", path=path, title="Báo cáo", author="Tester", paragraphs=[
        "Xin chào thế giới",
        {"text": "Tiêu đề chính", "style": "Heading 1"},
        {"text": "Dòng 1\nDòng 2\tcó tab", "style": "List Bullet"},
        {"table": [["Họ tên", "Điểm"], ["An", 9.5], ["Bình", None]]},
        "Kết thúc",
    ])
    results["create_again"] = server.call("doc_create", path=path)
    results["create_overwrite"] = server.call("doc_create", path=path, overwrite=True, paragraphs=["mới"])
    results["profile"] = server.call("doc_profile", path=path)
    results["get_text"] = server.call("doc_get_text", path=path)
    results["get_text_no_tables"] = server.call("doc_get_text", path=path, include_tables=False)
    results["get_text_trunc"] = server.call("doc_get_text", path=path, max_chars=12)
    results["find"] = server.call("doc_find_text", path=path, query="dòng")
    results["find_all"] = server.call("doc_find_text", path=path, query="a", max_results=2)
    results["find_none"] = server.call("doc_find_text", path=path, query="không có gì")
    results["extract"] = server.call("doc_extract_table", path=path, index=0)
    results["extract_bad"] = server.call("doc_extract_table", path=path, index=5)
    results["bad_style"] = server.call("doc_create", path=os.path.join(out, "loi.docx"),
                                       overwrite=True, paragraphs=[{"text": "x", "style": "Không có style này"}])
    results["missing"] = server.call("doc_profile", path=os.path.join(out, "khong-co.docx"))
    results["not_docx"] = server.call("doc_profile", path=os.path.join(out, "a.xlsx"))
    return results


def excel_scenario(server: Server, out: str) -> dict:
    path = os.path.join(out, "book.xlsx")
    results = {}
    results["create"] = server.call("excel_create", path=path, sheets=[
        {"name": "Data", "values": [["Tên", "Điểm"], ["An", 9.5], ["Bình", 8]]},
    ])
    results["write"] = server.call("excel_write", path=path, sheet="Data", start_cell="D1",
                                   values=[["Ghi chú"], ["ok"]])
    results["read_range"] = server.call("excel_read", path=path, cell_range="A1:D4")
    results["read_all"] = server.call("excel_read", path=path)
    results["read_page"] = server.call("excel_read", path=path, offset=1, limit=2)
    results["read_formula"] = server.call("excel_read", path=path, show_formula=True)
    results["read_sheet"] = server.call("excel_read", path=path, sheet="Data", cell_range="A1:B3")
    results["profile"] = server.call("excel_profile", path=path)
    results["format_simple"] = server.call("excel_format_range", path=path, sheet="Data",
                                           cell_range="A2:B3", styles={"bold": True})
    results["format_rich"] = server.call("excel_format_range", path=path, sheet="Data", cell_range="A1:B1",
                                          styles={"font": {"bold": True, "color": "#FF0000", "size": 14},
                                                  "fill": {"color": "#FFFF00"},
                                                  "alignment": {"horizontal": "center", "wrap": True},
                                                  "numFmt": "#,##0.00"})
    results["format_bad_matrix"] = server.call("excel_format_range", path=path, sheet="Data",
                                               cell_range="A1:B2", styles=[[{"bold": True}]])
    results["table"] = server.call("excel_create_table", path=path, sheet="Data",
                                   cell_range="A1:D3", table_name="BangDiem")
    results["table_bad_name"] = server.call("excel_create_table", path=path, sheet="Data",
                                            cell_range="A1:D3", table_name="A1")
    results["table_dup"] = server.call("excel_create_table", path=path, sheet="Data",
                                       cell_range="A1:D3", table_name="BangDiem")
    results["sheet_create"] = server.call("excel_create_sheet", path=path, sheet="Trống")
    results["sheet_create_dup"] = server.call("excel_create_sheet", path=path, sheet="Trống")
    results["sheet_create_over"] = server.call("excel_create_sheet", path=path, sheet="Trống", overwrite=True)
    results["sheet_copy"] = server.call("excel_copy_sheet", path=path, src_sheet="Data", dst_sheet="Bản sao")
    results["sheet_copy_dup"] = server.call("excel_copy_sheet", path=path, src_sheet="Data", dst_sheet="Bản sao")
    results["sheet_rename"] = server.call("excel_rename_sheet", path=path, sheet="Bản sao", new_name="Đổi tên")
    results["sheet_rename_missing"] = server.call("excel_rename_sheet", path=path, sheet="Không có", new_name="X")
    results["sheet_delete"] = server.call("excel_delete_sheet", path=path, sheet="Đổi tên")
    results["sheet_delete_missing"] = server.call("excel_delete_sheet", path=path, sheet="Không có")
    results["profile2"] = server.call("excel_profile", path=path)
    results["convert"] = server.call("excel_convert", path=path, sheet="Data")
    results["convert_parquet"] = server.call("excel_convert", path=path, to="parquet")
    results["convert_bad_to"] = server.call("excel_convert", path=path, to="xml")
    results["missing_file"] = server.call("excel_profile", path=os.path.join(out, "khong-co.xlsx"))
    results["bad_format"] = server.call("excel_read", path=os.path.join(out, "a.parquet"))
    results["write_xls"] = server.call("excel_write", path=os.path.join(out, "cu.xls"), sheet="S",
                                       start_cell="A1", values=[["x"]])

    # csv di duong rieng
    csv_path = os.path.join(out, "bang.csv")
    results["csv_create"] = server.call("excel_create", path=csv_path,
                                        sheets=[{"values": [["Tên", "Điểm"], ["An", 9.5], ["Bình, Hà", 8]]}])
    results["csv_write"] = server.call("excel_write", path=csv_path, sheet="bang", start_cell="C1",
                                       values=[["Ghi chú"], ['có "ngoặc"']])
    results["csv_read"] = server.call("excel_read", path=csv_path)
    results["csv_read_range"] = server.call("excel_read", path=csv_path, cell_range="A1:C3")
    results["csv_profile"] = server.call("excel_profile", path=csv_path)
    results["csv_edit_sheet"] = server.call("excel_create_sheet", path=csv_path, sheet="X")
    return results


def main() -> int:
    go_exe = os.environ.get("AXIOM_GO_EXE") or os.path.join(BUILD_DIR, "AxiomOffice.Core.exe")
    cs_exe = os.environ.get("AXIOM_CS_EXE") or os.path.join(BUILD_DIR, "AxiomOffice.Host.exe")
    for label, required in (("Go", go_exe), ("C#", cs_exe)):
        if not os.path.exists(required):
            raise SystemExit("thieu %s: %s (chay scripts/build.ps1 truoc)" % (label, required))

    go = Server([go_exe, "mcp", "all"])
    cs = Server([cs_exe, "mcp", "all"])
    try:
        go_tools, cs_tools = set(go.tools()), set(cs.tools())
        # Tool chua port (lan live, pptx) duoc bao rieng chu khong tinh la lech.
        pending = sorted(cs_tools - go_tools)
        print("chua port: " + (", ".join(pending) if pending else "khong con"))
        extra = sorted(go_tools - cs_tools)
        compare("tools/list (khong tinh phan chua port)", extra, [])

        for name, scenario in (("docx", docx_scenario), ("pptx", pptx_scenario), ("excel", excel_scenario)):
            go_out = tempfile.mkdtemp(prefix="go-oracle-")
            cs_out = tempfile.mkdtemp(prefix="cs-oracle-")
            go_results = normalize(scenario(go, go_out), go_out)
            cs_results = normalize(scenario(cs, cs_out), cs_out)
            for key in go_results:
                compare(name + "/" + key, go_results[key], cs_results[key])
    finally:
        go.close()
        cs.close()

    print("\n%d/%d buoc khop" % (CHECKS - len(FAILURES), CHECKS))
    for label in FAILURES:
        print("  FAIL " + label)
    return 1 if FAILURES else 0


if __name__ == "__main__":
    sys.exit(main())
