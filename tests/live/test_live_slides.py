"""Live test tren PowerPoint THAT cho cac lenh slide moi (docs/plans/18-s0, 19-h0). Can PowerPoint dang mo voi add-in
Axiom (bridge cong 47833); token doc tu HKCU. Tao bai moi, khong dong bai nao cua nguoi dung.

    python tests/live/test_live_slides.py [--port 47833]

Kiem:
  - H3: wpp.listSlides doc chu trong bang ("[table RxC]") va trong hinh khoi.
  - H1: lenh khong tu chon bai (wpp.addSlide) + tham so presentation -> ghi dung bai du bai khac dang active;
        bai da dong -> DOCUMENT_CLOSED.
  - H4: wpp.addTable tra ten shape.
  - P2: wpp.addShape + wpp.addConnector (duong noi bam hai hinh), checkLayout khong bao chong.
  - P3: wpp.renderSlide tra PNG cua slide theo so.
"""
from __future__ import annotations

import argparse
import base64
import json
import sys
import urllib.request

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def token() -> str:
    try:
        import winreg
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\AxiomOffice") as key:
            return str(winreg.QueryValueEx(key, "Token")[0])
    except OSError:
        return ""


class Bridge:
    def __init__(self, port: int):
        self.url = "http://127.0.0.1:%d/cmd" % port
        self.token = token()

    def call(self, action: str, params: dict | None = None) -> dict:
        body = json.dumps({"action": action, "params": params or {}}, ensure_ascii=False).encode("utf-8")
        request = urllib.request.Request(self.url, data=body, method="POST")
        request.add_header("Content-Type", "application/json; charset=utf-8")
        request.add_header("X-Auth-Token", self.token)
        with urllib.request.urlopen(request, timeout=150) as response:
            return json.loads(response.read().decode("utf-8"))

    def ok(self, action: str, params: dict | None = None) -> dict:
        reply = self.call(action, params)
        if not reply.get("ok"):
            raise AssertionError("%s that bai: %s" % (action, reply.get("error")))
        return reply.get("result") or {}


passed, failed = [], []


def check(condition: bool, name: str, detail="") -> None:
    (passed if condition else failed).append(name)
    print(("  ok   " if condition else "  LOI  ") + name + ("" if condition else "  " + str(detail)[:300]))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=47833)
    args = parser.parse_args()
    b = Bridge(args.port)

    a = b.ok("wpp.newPresentation")["name"]
    other = b.ok("wpp.newPresentation")["name"]  # bai thu hai thanh active
    print("bai A=%s, bai B (active)=%s" % (a, other))

    # H1: addSlide nhan presentation -> ghi vao A du B dang active.
    b.ok("wpp.addSlide", {"layout": 12, "presentation": a})
    count_a = b.ok("wpp.listSlides", {"presentation": a})["slideCount"]
    count_b = b.ok("wpp.listSlides", {"presentation": other})["slideCount"]
    check(count_a == 1 and count_b == 0, "H1 addSlide {presentation} ghi dung bai", (count_a, count_b))
    closed = b.call("wpp.addSlide", {"presentation": "Khong-ton-tai.pptx"})
    check(not closed.get("ok") and "DOCUMENT_CLOSED" in str(closed.get("error")), "H1 bai khong mo -> DOCUMENT_CLOSED", closed)

    # H3 + H4: bang doc lai duoc, addTable tra ten shape.
    table = b.ok("wpp.addTable", {"presentation": a, "slide": 1, "values": [["Năm", "Nhập học"], ["2024", "470"]],
                                  "left": 40, "top": 130, "width": 400, "height": 80})
    check(bool(table.get("shape")), "H4 addTable tra ten shape", table)
    texts = "\n".join(b.ok("wpp.listSlides", {"presentation": a})["slides"][0]["shapeTexts"])
    check("[table 2x2]" in texts and "Nhập học" in texts and "470" in texts, "H3 listSlides doc chu trong bang", texts)

    # P2: hai hinh + duong noi.
    b.ok("wpp.addSlide", {"layout": 12, "presentation": a})
    first = b.ok("wpp.addShape", {"presentation": a, "slide": 2, "shape": "rounded", "left": 60, "top": 200, "width": 220,
                                  "height": 90, "text": "Tiếp nhận\nkiểm tra hồ sơ", "boldFirst": True,
                                  "fill": "#DEEAF6", "line": "#1F4E79", "fontSize": 20, "font": "Arial"})
    second = b.ok("wpp.addShape", {"presentation": a, "slide": 2, "shape": "rounded", "left": 400, "top": 200, "width": 220,
                                   "height": 90, "text": "Phê duyệt", "fill": "#DEEAF6", "line": "#1F4E79", "fontSize": 20})
    connector = b.ok("wpp.addConnector", {"presentation": a, "slide": 2, "from": first["shape"], "to": second["shape"]})
    check(bool(first.get("shape")) and bool(connector.get("shape")), "P2 addShape + addConnector tra ten", (first, connector))
    texts = b.ok("wpp.listSlides", {"presentation": a})["slides"][1]["shapeTexts"]
    check("Phê duyệt" in texts and any(t.startswith("Tiếp nhận") for t in texts), "P2 chu trong hinh doc lai duoc", texts)
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": 2})
    check(layout.get("issueCount") == 0, "P2 checkLayout sach (duong noi khong tinh la chong)", layout)

    # P3: render slide theo so.
    shot = b.ok("wpp.renderSlide", {"presentation": a, "slide": 2, "maxWidth": 640})
    png = base64.b64decode(shot.get("base64") or "")
    check(png[:8] == b"\x89PNG\r\n\x1a\n" and shot.get("width") == 640 and shot.get("slide") == 2, "P3 renderSlide tra PNG",
          {k: v for k, v in shot.items() if k != "base64"})
    bad = b.call("wpp.renderSlide", {"presentation": a, "slide": 9})
    check(not bad.get("ok"), "P3 renderSlide slide khong co -> loi", bad)

    print("\n%d dat, %d loi (bai thu de lai mo: %s, %s - dong khong luu)" % (len(passed), len(failed), a, other))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
