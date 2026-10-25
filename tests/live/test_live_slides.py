"""Live test tren PowerPoint THAT cho cac lenh slide moi (docs/plans/18-s0, 19-h0). Can PowerPoint dang mo voi add-in
Axiom (bridge cong 47833); token doc tu HKCU. Tao bai moi, khong dong bai nao cua nguoi dung.

    python tests/live/test_live_slides.py [--port 47833]

Kiem:
  - H3: wpp.listSlides doc chu trong bang ("[table RxC]") va trong hinh khoi.
  - H1: lenh khong tu chon bai (wpp.addSlide) + tham so presentation -> ghi dung bai du bai khac dang active;
        bai da dong -> DOCUMENT_CLOSED.
  - H4: wpp.addTable tra ten shape.
  - P2: wpp.addShape + wpp.addConnector (duong noi bam hai hinh), checkLayout khong bao chong; so do 3 tang noi
        theo huong "down" khong co duong cat hop.
  - Duong noi: checkLayout tinh duong di that (connector-crossing, connector-overlap); addConnector fromSide/toSide +
        offset ve duong vong duoi hang khong cat hop.
  - Bieu do goc: wpp.addChart (cot co don vi, tron), listSlides doc lai gia tri, checkLayout sach.
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

    # P2: so do 3 tang kieu s17 (docs/plans/18-s0 9.3) - noi theo huong (day cha -> dinh con), khong duong nao cat hop.
    b.ok("wpp.addSlide", {"layout": 12, "presentation": a})
    boxes = {"g": (360, 136, 240), "p1": (222, 270, 240), "p2": (498, 270, 240)}
    for n, x in enumerate((40, 269, 498, 727)):
        boxes["c%d" % n] = (x, 404, 193)
    names = {}
    for key, (left, top, width) in boxes.items():
        names[key] = b.ok("wpp.addShape", {"presentation": a, "slide": 3, "shape": "rounded", "left": left, "top": top,
                                           "width": width, "height": 90, "text": key, "line": "#1F4E79"})["shape"]
    for parent, child in (("g", "p1"), ("g", "p2"), ("p1", "c0"), ("p1", "c1"), ("p2", "c2"), ("p2", "c3")):
        b.ok("wpp.addConnector", {"presentation": a, "slide": 3, "from": names[parent], "to": names[child], "elbow": True,
                                  "direction": "down"})
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": 3})
    check(layout.get("issueCount") == 0, "P2 so do 3 tang: duong noi theo huong khong cat hop", layout.get("slides"))
    bad = b.call("wpp.addConnector", {"presentation": a, "slide": 3, "from": names["g"], "to": names["p1"], "direction": "cheo"})
    count = len(b.ok("wpp.checkLayout", {"presentation": a, "slide": 3})["slides"][0]["shapes"])
    check(not bad.get("ok") and count == len(layout["slides"][0]["shapes"]), "P2 direction sai -> loi, khong de lai duong noi", bad)
    # Bo qua duong noi khong duoc lam mat loi that: hai hop chong nhau van bao.
    b.ok("wpp.addShape", {"presentation": a, "slide": 3, "shape": "rect", "left": 380, "top": 150, "width": 200, "height": 60,
                          "text": "chong", "fill": "#FFFFFF"})
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": 3})
    check(any(i.get("type") == "overlap" for i in layout["slides"][0]["issues"]), "P2 hai hop chong that van bao overlap",
          layout["slides"][0]["issues"])

    # Day cat hop (luot that 25/10/2026, so do kien truc): checkLayout tinh duong di THAT cua duong noi.
    b.ok("wpp.addSlide", {"layout": 12, "presentation": a})
    slide = 4
    row = {}
    for n, key in enumerate(("p", "r", "k", "v")):
        row[key] = b.ok("wpp.addShape", {"presentation": a, "slide": slide, "shape": "rounded", "left": 40 + n * 229, "top": 300,
                                         "width": 193, "height": 60, "text": key, "line": "#1F4E79"})["shape"]
    head = b.ok("wpp.addShape", {"presentation": a, "slide": slide, "shape": "rounded", "left": 360, "top": 150, "width": 240,
                                 "height": 60, "text": "c", "line": "#1F4E79"})["shape"]
    straight = b.ok("wpp.addConnector", {"presentation": a, "slide": slide, "from": row["p"], "to": row["k"]})["shape"]
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": slide})["slides"][0]
    crossing = [i for i in layout["issues"] if i.get("type") == "connector-crossing"]
    check(any(set(i.get("shapes", [])) == {straight, row["r"]} for i in crossing), "Day thang xuyen qua hop -> connector-crossing",
          layout["issues"])
    entry = next((s for s in layout["shapes"] if s["name"] == straight), {})
    check(len(entry.get("path") or []) == 2 and entry.get("from") == row["p"] and entry.get("to") == row["k"],
          "checkLayout tra duong di + hai dau cua duong noi", entry)
    b.ok("wpp.deleteShapes", {"presentation": a, "slide": slide, "shapes": [straight]})
    # Duong vong duoi hang (fromSide = toSide = bottom): khong cat hop; offset dat do sau, hai vong khong trung nhau.
    loops = [b.ok("wpp.addConnector", {"presentation": a, "slide": slide, "from": row[x], "to": row[y], "elbow": True,
                                       "fromSide": "bottom", "toSide": "bottom", "offset": offset})["shape"]
             for x, y, offset in (("p", "k", 16), ("r", "v", 26))]
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": slide})["slides"][0]
    wires = [i for i in layout["issues"] if i.get("type", "").startswith("connector-")]
    check(not wires, "Duong vong duoi hang khong cat hop, khong chong nhau", wires)
    depths = [max(point[1] for point in next(s for s in layout["shapes"] if s["name"] == name)["path"]) for name in loops]
    check(abs(depths[0] - 376) < 3 and abs(depths[1] - 386) < 3, "Duong vong sau dung offset (day hop 360 + 16 / + 26)", depths)
    # Hai duong nguoc chieu tren cung mot doan -> connector-overlap.
    b.ok("wpp.addConnector", {"presentation": a, "slide": slide, "from": row["r"], "to": head, "direction": "up",
                              "elbow": True})
    b.ok("wpp.addConnector", {"presentation": a, "slide": slide, "from": head, "to": row["r"], "direction": "down",
                              "elbow": True})
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": slide})["slides"][0]
    check(any(i.get("type") == "connector-overlap" for i in layout["issues"]), "Hai duong chong nhau -> connector-overlap",
          layout["issues"])
    bad = b.call("wpp.addConnector", {"presentation": a, "slide": slide, "from": row["p"], "to": row["v"], "fromSide": "cheo"})
    check(not bad.get("ok"), "fromSide sai -> loi", bad)

    # Bieu do goc: so lieu nam trong bang tinh nhung, listSlides doc lai duoc gia tri kem don vi.
    b.ok("wpp.addSlide", {"layout": 12, "presentation": a})
    column = b.ok("wpp.addChart", {"presentation": a, "slide": 5, "type": "column", "categories": ["Q1", "Q2", "Q3", "Q4"],
                                   "series": [{"name": "Doanh thu", "values": [28, 31, 33, 36]}], "unit": " tỷ",
                                   "colors": ["#1F4E79"], "font": "Arial", "left": 40, "top": 130, "width": 420, "height": 330})
    pie = b.ok("wpp.addChart", {"presentation": a, "slide": 5, "type": "pie", "categories": ["Nhân sự", "Văn phòng", "Khác"],
                                "series": [{"name": "Chi phí", "values": [55, 25, 20]}], "unit": "%",
                                "colors": ["#1F4E79", "#2E75B6", "#849EB5"], "left": 500, "top": 130, "width": 420, "height": 330})
    texts = "\n".join(b.ok("wpp.listSlides", {"presentation": a})["slides"][4]["shapeTexts"])
    check(bool(column.get("shape")) and bool(pie.get("shape")), "Chart addChart tra ten", (column, pie))
    check("Q4 36 tỷ" in texts and "Nhân sự 55%" in texts, "Chart listSlides doc gia tri kem don vi", texts)
    layout = b.ok("wpp.checkLayout", {"presentation": a, "slide": 5})
    check(layout.get("issueCount") == 0, "Chart checkLayout sach", layout.get("slides"))
    bad = b.call("wpp.addChart", {"presentation": a, "slide": 5, "type": "pie", "categories": ["a", "b"],
                                  "series": [{"name": "x", "values": [1, 2]}, {"name": "y", "values": [1, 2]}]})
    check(not bad.get("ok"), "Chart pie hai chuoi -> loi", bad)

    # P3: render slide theo so.
    shot = b.ok("wpp.renderSlide", {"presentation": a, "slide": 2, "maxWidth": 640})
    png = base64.b64decode(shot.get("base64") or "")
    check(png[:8] == b"\x89PNG\r\n\x1a\n" and shot.get("width") == 640 and shot.get("slide") == 2, "P3 renderSlide tra PNG",
          {k: v for k, v in shot.items() if k != "base64"})
    bad = b.call("wpp.renderSlide", {"presentation": a, "slide": 19})
    check(not bad.get("ok"), "P3 renderSlide slide khong co -> loi", bad)

    print("\n%d dat, %d loi (bai thu de lai mo: %s, %s - dong khong luu)" % (len(passed), len(failed), a, other))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
