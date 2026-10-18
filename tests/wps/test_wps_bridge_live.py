"""Spec T2/T3 chay that (docs/wps-bridge.md): Agent Core + JS add-in + ca ba app WPS duoi Xvfb.

Build Core, cai add-in vao HOME tam (`install_jsaddon.py --core-url ... --test-profile`), roi LAN LUOT mo
Writer (`wps`), Spreadsheets (`et`) va Presentation (`wpp`) voi mot tai lieu mau, cho add-in ket noi bridge
WPS va goi tung lenh cua app do qua POST /cmd, doc lai de kiem.

Mot Core cho ca ba luot: bridge WPS chi phuc vu app poll gan nhat (docs/wps-bridge.md muc 6), nen giua hai
app phai doi bridge bao mat ket noi (va file session bi xoa) truoc khi mo app ke tiep.

Can: WPS Office for Linux, xvfb-run, xdotool (boi den that trong bai do khoa vung), JsApiPlugin=true trong
oem.ini (tests/wps/REFERENCE.md), khong co WPS dang chay cho nguoi dung nay.

    python3 tests/wps/test_wps_bridge_live.py [--keep] [--apps wps,et,wpp] [--timeout 120]
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import signal
import struct
import subprocess
import sys
import tempfile
import time
import zlib

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_wps_bridge_e2e as base  # noqa: E402

ROOT = base.ROOT
OEM_INI = "/opt/kingsoft/wps-office/office6/cfgs/oem.ini"
TEMPLATES = os.path.join(ROOT, "core-go", "internal", "templates")
BLANK_DOCX = os.path.join(TEMPLATES, "default.docx")
BLANK_PPTX = os.path.join(TEMPLATES, "default.pptx")
# ET khong co template nhung san (core-go/internal/templates chi co docx/pptx): dung so mau cua goi xlsx
# (2 sheet, A1:C3 co du lieu) - bai test chi dua vao du lieu no tu ghi, tru mot cho: A1 = "Tên".
BLANK_XLSX = os.path.join(ROOT, "core-go", "internal", "xlsx", "testdata", "sample.xlsx")
check = base.check

# Nguong "mat ket noi" phai la mac dinh cua san pham (wpsbridge.DefaultDisconnectSeconds), khong dung 4s cua
# tang e2e. Ly do (do 12/10/2026): add-in don luong nen trong luc chay lenh no KHONG poll duoc, va quan sat
# duoc la `et.exportPdf` mat 11s; sau lenh dai do, lastPoll da cu hon nguong 4s nen lenh ke bi tu choi
# "WPS add-in is not connected" du app van song (probe ET ALIVE lien tuc). Day khong phai loi cua app that:
# do la cau hinh qua chat cho mot tang chay app that.
DISCONNECT_SECONDS = 45


def wps_running() -> list:
    out = subprocess.run(["pgrep", "-u", str(os.getuid()), "-f", "office6/(wps|et|wpp)( |$)"],
                         capture_output=True, text=True).stdout
    return out.split()


def wps_display_env() -> dict:
    """DISPLAY/XAUTHORITY cua WPS dang chay: `xvfb-run -a` cap so hien thi dong nen khong doan duoc -
    phai doc tu /proc/<pid>/environ cua chinh tien trinh WPS."""
    out = subprocess.run(["pgrep", "-u", str(os.getuid()), "-f", "office6/wps( |$)"],
                         capture_output=True, text=True).stdout
    for pid in out.split():
        try:
            with open("/proc/%s/environ" % pid, "rb") as handle:
                raw = handle.read().decode("utf-8", "replace")
        except OSError:
            continue
        env = dict(item.split("=", 1) for item in raw.split("\0") if "=" in item)
        if env.get("DISPLAY"):
            return {"DISPLAY": env["DISPLAY"], "XAUTHORITY": env.get("XAUTHORITY", "")}
    return {}


class WpsKeys:
    """Ban phim THAT vao cua so WPS (xdotool/XTEST) - cach duy nhat tao vung boi den ma khong phai them
    mot lenh gia nao vao add-in. Duoi Xvfb khong co window manager nen `windowactivate` bao loi
    _NET_ACTIVE_WINDOW; `windowfocus` (XSetInputFocus) la du, da do 18/10/2026."""

    def __init__(self, live):
        self.env = dict(os.environ)
        self.env.update(wps_display_env())
        self.window = ""
        for pattern in ["%s - Writer" % live.document, "Writer"]:
            found = self._xdotool("search", "--onlyvisible", "--name", pattern)
            if found:
                self.window = found.split()[0]
                break

    def ready(self) -> bool:
        return bool(self.window) and self.env.get("DISPLAY") is not None

    def _run(self, *args: str) -> tuple:
        """(chay duoc khong, stdout). Phai tach khoi stdout: `xdotool key` thanh cong ma KHONG in gi."""
        try:
            done = subprocess.run(["xdotool", *args], env=self.env, capture_output=True, text=True, timeout=20)
        except (OSError, subprocess.TimeoutExpired):
            return False, ""
        return done.returncode == 0, done.stdout.strip()

    def _xdotool(self, *args: str) -> str:
        return self._run(*args)[1]

    def send(self, *keys: str) -> bool:
        """Mot to hop phim (vd "ctrl+a"). Tra True neu xdotool chay duoc - khong bao dam app da nhan."""
        if not self.window:
            return False
        self._xdotool("windowfocus", self.window)
        return self._run("key", "--clearmodifiers", *keys)[0]


def is_pdf(path: str) -> bool:
    try:
        with open(path, "rb") as handle:
            return handle.read(5) == b"%PDF-"
    except OSError:
        return False


def write_png(path: str, width: int = 4, height: int = 3) -> None:
    """PNG RGB nho nhung dung that (`wpp.addImage` can file anh that; khong dung thu vien ngoai)."""
    rows = b""
    for y in range(height):
        rows += b"\x00" + b"".join(bytes([(x * 60) % 256, (y * 90) % 256, 128]) for x in range(width))

    def chunk(tag: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)

    with open(path, "wb") as handle:
        handle.write(b"\x89PNG\r\n\x1a\n"
                     + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
                     + chunk(b"IDAT", zlib.compress(rows))
                     + chunk(b"IEND", b""))


class Session:
    """Mot luot chay app that: giu tien trinh WPS, goi lenh qua bridge, doi ket noi/mat ket noi."""

    def __init__(self, bridge: str, auth: dict, core, home: str, session_dir: str, timeout: int):
        self.bridge = bridge
        self.auth = auth
        self.core = core
        self.home = home
        self.session_dir = session_dir
        self.timeout = timeout
        self.kind = ""
        self.document = ""
        self.process = None

    # --- bridge WPS ---
    def health(self) -> dict:
        status, _, text = base.request(self.bridge + "/health", timeout=3)
        return (base.as_json(text).get("result") or {}) if status == 200 else {}

    def connected(self) -> bool:
        return self.health().get("connected") is True

    def app_name(self) -> str:
        return str(self.health().get("app") or "")

    def cmd(self, action: str, params: dict | None = None, timeout: int = 90) -> dict:
        status, _, text = base.request(self.bridge + "/cmd", "POST",
                                       {"action": action, "params": params or {}}, self.auth, timeout=timeout)
        reply = base.as_json(text)
        return reply if status == 200 else {"ok": False, "error": "HTTP %s: %s" % (status, text[:300])}

    def commands(self) -> list:
        status, _, text = base.request(self.bridge + "/commands", headers=self.auth)
        return [c.get("name") for c in (base.as_json(text).get("result") or {}).get("commands") or []]

    def session_file(self) -> str:
        return os.path.join(self.session_dir, "%d-wps.json" % self.core.process.pid)

    def session_data(self) -> dict:
        try:
            with open(self.session_file(), encoding="utf-8") as handle:
                return json.load(handle)
        except (OSError, ValueError):
            return {}

    # --- vong doi app ---
    def start(self, kind: str, exe: str, source: str, document: str) -> None:
        self.kind = kind
        self.document = document
        shutil.copyfile(source, os.path.join(self.home, document))
        env = {k: v for k, v in os.environ.items() if not k.startswith("XDG_")}
        env["HOME"] = self.home
        self.process = subprocess.Popen(
            ["xvfb-run", "-a", "-s", "-screen 0 1280x900x24", exe, os.path.join(self.home, document)],
            env=env, cwd=self.home, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)

    def wait_connected(self) -> bool:
        return base.wait_for(lambda: self.connected() and self.app_name() == self.kind, self.timeout)

    def stop(self) -> None:
        if self.process is not None:
            try:
                os.killpg(self.process.pid, signal.SIGTERM)
                self.process.wait(timeout=20)
            except Exception:  # noqa: BLE001
                try:
                    os.killpg(self.process.pid, signal.SIGKILL)
                except Exception:  # noqa: BLE001
                    pass
            self.process = None
        # Cho theo dung nguong cua bridge: no chi bao mat ket noi sau DISCONNECT_SECONDS khong thay poll.
        check(base.wait_for(lambda: not self.connected(), DISCONNECT_SECONDS + 20),
              "bridge bao mat ket noi sau khi dong %s" % self.kind, self.health())
        check(base.wait_for(lambda: not os.path.exists(self.session_file()), 20),
              "file session bi xoa khi %s mat ket noi" % self.kind, os.listdir(os.path.dirname(self.session_file())))


def run_writer(live: Session) -> None:
    names = live.commands()
    check("writer.appendText" in names and "writer.undo" in names and "et.readRange" not in names,
          "bridge /commands = catalog cua add-in that (chi lenh writer.*)", names)

    def text_now() -> str:
        reply = live.cmd("writer.getText")
        return str((reply.get("result") or {}).get("text") or "")

    info = live.cmd("app.info")
    check(info.get("ok") is True, "app.info", info)
    reply = live.cmd("writer.getText", {"maxChars": 1})
    result = reply.get("result") or {}
    check(reply.get("ok") is True and isinstance(result.get("totalChars"), int) and "text" in result,
          "writer.getText tra text + totalChars (policy cua Core can totalChars)", reply)

    reply = live.cmd("writer.appendText", {"text": "Dòng AI một"})
    check(reply.get("ok") is True and "Dòng AI một" in text_now(), "writer.appendText", reply)
    reply = live.cmd("writer.undo", {"count": 1})
    check(reply.get("ok") is True and "Dòng AI một" not in text_now(),
          "writer.undo 1 buoc go dung mot lenh AI (UndoRecord)", (reply, text_now()[-200:]))

    reply = live.cmd("writer.appendText", {"text": "Dòng AI hai"})
    check(reply.get("ok") is True, "writer.appendText lan 2", reply)
    reply = live.cmd("writer.insertStyledText", {"text": " chữ đậm", "bold": True, "color": "#C00000"})
    check(reply.get("ok") is True and "chữ đậm" in text_now(), "writer.insertStyledText", reply)
    reply = live.cmd("writer.heading", {"text": "Tiêu đề A", "level": 1})
    check(reply.get("ok") is True and "Tiêu đề A" in text_now(), "writer.heading", reply)
    reply = live.cmd("writer.typeText", {"text": "Gõ ở con trỏ"})
    check(reply.get("ok") is True and "Gõ ở con trỏ" in text_now(), "writer.typeText", reply)
    reply = live.cmd("writer.replaceAll", {"find": "Dòng AI hai", "replace": "Dòng đã thay"})
    now = text_now()
    check(reply.get("ok") is True and "Dòng đã thay" in now and "Dòng AI hai" not in now,
          "writer.replaceAll", (reply, now[-200:]))
    # Chuoi thay chua chinh chuoi tim (review T2b 11/10/2026): vong Find.Execute(Wrap=continue, ReplaceOne)
    # tim lai tu dau tai lieu nen thay mai toi tran vong lap -> treo + nhan ban van ban.
    started = time.time()
    reply = live.cmd("writer.replaceAll", {"find": "Dòng đã thay", "replace": "Dòng đã thay (lần 2)"}, timeout=60)
    now = text_now()
    check(reply.get("ok") is True and (reply.get("result") or {}).get("count") == 1
          and now.count("Dòng đã thay (lần 2)") == 1 and "(lần 2) (lần 2)" not in now and time.time() - started < 20,
          "writer.replaceAll khi chuoi thay chua chuoi tim: thay dung 1 lan, khong lap", (reply, now[-200:]))
    reply = live.cmd("writer.insertTable", {"values": [["A1", "B1"], ["A2", "B2"]]})
    check(reply.get("ok") is True and "B2" in text_now(), "writer.insertTable (values 2x2)", reply)
    reply = live.cmd("writer.selection")
    check(reply.get("ok") is True and "text" in (reply.get("result") or {}), "writer.selection", reply)
    reply = live.cmd("writer.formatSelection", {"bold": True, "size": 13})
    check(reply.get("ok") is True, "writer.formatSelection", reply)
    reply = live.cmd("writer.setParagraphAlignment", {"alignment": "center"})
    check(reply.get("ok") is True, "writer.setParagraphAlignment", reply)
    reply = live.cmd("writer.insertPageBreak")
    check(reply.get("ok") is True, "writer.insertPageBreak", reply)

    # ---- Khoa vung boi den (ban WPS cua CommandDispatcher.WriterScope.cs). Nguoi dung boi den bang chuot
    # roi hoi thi AI chi duoc sua trong vung do; o day boi den bang BAN PHIM THAT qua xdotool/XTEST - dung
    # duong nguoi dung di, khong phai them mot lenh gia nao vao add-in chi de test duoc.
    def sel_now() -> dict:
        return live.cmd("writer.selection").get("result") or {}

    keys = WpsKeys(live)
    check(keys.ready(), "dieu khien duoc cua so WPS bang xdotool (XTEST)",
          (keys.env.get("DISPLAY"), keys.window))
    check(keys.send("ctrl+a"), "gui duoc Ctrl+A vao cua so WPS", keys.window)
    check(base.wait_for(lambda: (sel_now().get("end") or 0) > (sel_now().get("start") or 0), 15),
          "Ctrl+A that: writer.selection thay vung chon (Selection.Start/End doc duoc tren WPS that)", sel_now())

    locked = live.cmd("writer.lockSelection")
    region = locked.get("result") or {}
    check(locked.get("ok") is True and int(region.get("chars") or 0) > 0,
          "writer.lockSelection khoa duoc vung boi den THAT, tra so do vung", locked)
    # tablesKnown=false nghia la doc.Tables.Count khong doc duoc -> luoi an toan mat phan bang. Phai la true.
    check(region.get("tablesKnown") is True and int(region.get("tables") or 0) >= 1,
          "khoa: doc.Tables doc duoc va dem dung bang nam tron trong vung (bang da chen o tren)", locked)
    check(int(region.get("docEnd") or 0) >= int(region.get("end") or 0) > 0,
          "khoa: Content.End doc duoc (docEnd bao ngoai vung chon)", locked)
    print("     vung khoa do duoc (Ctrl+A): %s" % region)

    # Lenh ghi ra ngoai vung bi tu choi TRUOC khi cham vao tai lieu (LOCK_REFUSED).
    before = text_now()
    refused = live.cmd("writer.appendText", {"text": "KHONG-DUOC-XUAT-HIEN"})
    check(refused.get("ok") is False and "Refused" in str(refused.get("error") or ""),
          "dang khoa: writer.appendText (ghi ra ngoai vung) bi tu choi", refused)
    check("KHONG-DUOC-XUAT-HIEN" not in text_now() and text_now() == before,
          "dang khoa: lenh bi tu choi khong de lai dau vet trong tai lieu", text_now()[-200:])
    check(live.cmd("writer.getText", {"maxChars": 1}).get("ok") is True,
          "dang khoa: lenh doc van chay binh thuong", "")

    # Boi den MOT PHAN (Ctrl+Home roi Shift+Right 5 lan) de do duong quan trong nhat: con tro bi keo ve
    # vung khoa truoc khi lenh chay, chu khong go ra cho con tro dang dung.
    check(keys.send("ctrl+Home"), "gui duoc Ctrl+Home (con tro ve dau tai lieu)", keys.window)
    for _ in range(5):
        keys.send("shift+Right")
    partial = live.cmd("writer.lockSelection")
    part = partial.get("result") or {}
    check(int(part.get("chars") or 0) > 0 and int(part.get("end") or 0) < int(part.get("docEnd") or 0),
          "khoa lai vung boi den MOT PHAN: vung khoa moi thay vung cu, khong phai ca tai lieu", partial)
    start, end = int(part.get("start") or 0), int(part.get("end") or 0)
    print("     vung khoa do duoc (Ctrl+Home + Shift+Right 5): %s" % part)

    check(keys.send("ctrl+End"), "gui duoc Ctrl+End (con tro ra CUOI tai lieu, ngoai vung khoa)", keys.window)
    before = text_now()
    typed = live.cmd("writer.typeText", {"text": "Z"})
    after = text_now()
    pulled_back = after == before[:start] + "Z" + before[end:]
    undone = after == before
    check(pulled_back or undone,
          "dang khoa: con tro o ngoai vung -> hoac bi KEO VE vung khoa, hoac lenh bi hoan tac; "
          "chu ngoai vung khong bao gio doi", (typed, before[-80:], after[-80:]))
    print("     con tro o cuoi tai lieu: %s" % ("keo ve vung khoa" if pulled_back else "lenh bi hoan tac"))

    unlocked = live.cmd("writer.unlockSelection")
    check(unlocked.get("ok") is True and (unlocked.get("result") or {}).get("locked") is False,
          "writer.unlockSelection", unlocked)
    after_unlock = live.cmd("writer.appendText", {"text": " SAU-KHI-MO-KHOA"})
    check(after_unlock.get("ok") is True and "SAU-KHI-MO-KHOA" in text_now(),
          "mo khoa roi thi lenh ghi ca tai lieu chay lai binh thuong", after_unlock)

    saved = os.path.join(live.home, "ket-qua.docx")
    reply = live.cmd("writer.saveAs", {"path": saved})
    check(reply.get("ok") is True and os.path.exists(saved) and os.path.getsize(saved) > 0, "writer.saveAs .docx",
          reply)
    reply = live.cmd("writer.save")
    check(reply.get("ok") is True, "writer.save (sau saveAs)", reply)
    pdf = os.path.join(live.home, "ket-qua.pdf")
    reply = live.cmd("writer.exportPdf", {"path": pdf}, timeout=180)
    check(reply.get("ok") is True and os.path.exists(pdf) and os.path.getsize(pdf) > 1000, "writer.exportPdf", reply)

    # Xuat PDF la lenh dai, va add-in don luong nen trong luc chay no khong poll duoc. Phai kiem bridge VAN
    # SONG ngay sau do: neu khong, phep kiem "lenh la -> ok:false" ben duoi se xanh vi ly do sai (bi tu choi
    # truoc khi gui, chu khong phai vi ten lenh sai).
    reply = live.cmd("writer.getText", {"maxChars": 1})
    check(reply.get("ok") is True and "totalChars" in (reply.get("result") or {}),
          "sau writer.exportPdf bridge van song (add-in poll lai duoc)", reply)

    reply = live.cmd("writer.khongCo")
    check(reply.get("ok") is False and "not connected" not in str(reply.get("error") or ""),
          "lenh la -> ok:false vi ten lenh sai (khong phai vi mat ket noi)", reply)


def run_et(live: Session) -> None:
    names = live.commands()
    check("et.readRange" in names and "et.undo" in names and "writer.appendText" not in names
          and "wpp.addSlide" not in names,
          "catalog cua Spreadsheets chi co lenh et.* (catalog loc theo app)", names)

    sheets = live.cmd("et.listSheets")
    shown = (sheets.get("result") or {}).get("sheets") or []
    check(sheets.get("ok") is True and len(shown) >= 1 and (sheets.get("result") or {}).get("activeSheet") in shown,
          "et.listSheets", sheets)

    info = live.cmd("et.sheetInfo")
    rows = (info.get("result") or {}).get("sheets") or []
    check(info.get("ok") is True and len(rows) == len(shown)
          and all(isinstance(s.get(c), int) for s in rows for c in ("rows", "cols"))
          and all("empty" in s for s in rows), "et.sheetInfo (kich thuoc that tung sheet)", info)

    # sample.xlsx co du lieu that o A1:C3 (tests/wps/REFERENCE.md: doc bang Cells.Item(r,c).Value2).
    read = live.cmd("et.readRange", {"range": "A1:C3"})
    values = (read.get("result") or {}).get("values") or []
    check(read.get("ok") is True and len(values) == 3 and len(values[0]) == 3 and values[0][0] == "Tên",
          "et.readRange doc du lieu that cua so (A1 = Ten, chu Viet)", read)

    # Ghi roi doc lai, roi et.undo khoi phuc (ET khong co UndoRecord - add-in tu chup gia tri o).
    write = live.cmd("et.writeRange", {"range": "H10", "values": [["Xin chao"], [42]]})
    back = (live.cmd("et.readRange", {"range": "H10:H11"}).get("result") or {}).get("values")
    check(write.get("ok") is True and (write.get("result") or {}).get("written") == 2 and back == [["Xin chao"], [42]],
          "et.writeRange roi et.readRange doc lai", (write, back))

    undone = live.cmd("et.undo", {})
    restored = (live.cmd("et.readRange", {"range": "H10:H11"}).get("result") or {}).get("values") or [[None]]
    check(undone.get("ok") is True and (undone.get("result") or {}).get("undone") == 1 and restored[0][0] is None,
          "et.undo khoi phuc o da ghi (ET khong co undo cua WPS)", (undone, restored))

    # fillRange: mot cong thuc roi dien, tham chieu tuong doi tinh theo dich.
    live.cmd("et.writeRange", {"range": "I1:I2", "values": [[2], [3]]})
    filled = live.cmd("et.fillRange", {"range": "J1:J2", "formula": "=I1*2"})
    filled_back = (live.cmd("et.readRange", {"range": "J1:J2"}).get("result") or {}).get("values")
    check(filled.get("ok") is True and (filled.get("result") or {}).get("filled") == 2 and filled_back == [[4], [6]],
          "et.fillRange dien cong thuc + doc lai gia tri da tinh", (filled, filled_back))

    fmt = live.cmd("et.formatRange", {"range": "I1:I2", "bold": True, "fontColor": "#FF0000",
                                      "fillColor": "#00FF00", "horizontal": "center", "numFmt": "#,##0.00"})
    check(fmt.get("ok") is True and (fmt.get("result") or {}).get("range") == "I1:I2",
          "et.formatRange (co chu, mau BGR, numFmt, can le)", fmt)

    added = live.cmd("et.addSheet", {"name": "Axiom"})
    renamed = live.cmd("et.renameSheet", {"sheet": "Axiom", "name": "Axiom2"})
    activated = live.cmd("et.activateSheet", {"sheet": "Axiom2"})
    active = (live.cmd("et.listSheets").get("result") or {}).get("activeSheet")
    check(added.get("ok") is True and (added.get("result") or {}).get("sheet") == "Axiom"
          and (renamed.get("result") or {}).get("sheet") == "Axiom2"
          and (activated.get("result") or {}).get("active") == "Axiom2" and active == "Axiom2",
          "et.addSheet / et.renameSheet / et.activateSheet", (added, renamed, activated, active))

    # Bang do chinh bai test ghi. LUU Y (do 11/10/2026, tests/wps/REFERENCE.md muc "ET ep kieu khi ghi"):
    # ET ep kieu khi gan vao Value2 ("42" -> so, "01/01" -> ngay), nen duong GHI khong tao duoc "so dang
    # text". Vi vay o day do dung he qua cua su ep kieu (doc lai o do ra so 42), roi soat bang theo huong
    # nguoc lai: cot "So" phai SACH (khong numbers-as-text), cot "Ngay" lan kieu (ngay + chu "x") phai bao
    # mixed-types. `numbers-as-text` chi khang dinh duoc tren o gieo san (duong doc) - spec offline lo viec
    # do: tests/lo/test_range_report.py.
    live.cmd("et.writeRange", {"range": "A1:C3", "sheet": "Axiom2",
                               "values": [["Ten", "So", "Ngay"], ["An", "42", "01/01"], ["Binh", 7, "x"]]})
    back = (live.cmd("et.readRange", {"range": "A1:C3", "sheet": "Axiom2"}).get("result") or {}).get("values") or []
    report = live.cmd("et.checkRange", {"range": "A1:C3", "sheet": "Axiom2"})
    data = report.get("result") or {}
    kinds = [i.get("type") for i in data.get("issues") or []]
    check(report.get("ok") is True and data.get("rows") == 3 and data.get("cols") == 3
          and data.get("issueCount") == len(data.get("issues") or [])
          and len(back) > 1 and len(back[1]) > 1 and back[1][1] == 42
          and "numbers-as-text" not in kinds and "mixed-types" in kinds,
          "et.checkRange soat bang du lieu that (ET ep kieu khi ghi: '42' -> so)", (report, back))

    formatted = live.cmd("et.setConditionalFormat", {"range": "B2:B3", "sheet": "Axiom2",
                                                     "rules": [{"operator": "less", "formula1": "50",
                                                                "fillColor": "#FF0000"}]})
    listed = live.cmd("et.listConditionalFormats", {"range": "B2:B3", "sheet": "Axiom2"})
    formats = (listed.get("result") or {}).get("formats") or []
    check(formatted.get("ok") is True and len((formatted.get("result") or {}).get("rules") or []) == 1
          and any(r.get("operator") == "less" for f in formats for r in f.get("rules") or []),
          "et.setConditionalFormat / et.listConditionalFormats", (formatted, listed))

    chart = live.cmd("et.addChart", {"range": "A1:C3", "sheet": "Axiom2", "type": "column", "title": "Thu"})
    charts = live.cmd("et.listCharts", {"sheet": "Axiom2"})
    check(chart.get("ok") is True and (chart.get("result") or {}).get("diagram") == "column"
          and len((charts.get("result") or {}).get("charts") or []) >= 1,
          "et.addChart / et.listCharts", (chart, charts))

    csv_path = os.path.join(live.home, "ban-hang.csv")
    with open(csv_path, "w", encoding="utf-8") as handle:
        handle.write("Ten,So\nAn,42\nBinh,7\n")
    imported = live.cmd("et.importCsv", {"path": csv_path})
    imported_sheet = (imported.get("result") or {}).get("sheet") or "khong-co"
    imported_back = (live.cmd("et.readRange", {"range": "A1:B3", "sheet": imported_sheet}).get("result") or {})
    check(imported.get("ok") is True and imported_sheet == "ban-hang" and (imported.get("result") or {}).get("rows") == 3
          and imported_back.get("values") == [["Ten", "So"], ["An", 42], ["Binh", 7]],
          "et.importCsv (khong co range -> tao sheet theo ten file)", (imported, imported_back))

    ranges = live.cmd("et.writeRanges", {"sheet": "Axiom2", "writes": [
        {"range": "E1", "values": [["a", "b"]]}, {"range": "E2", "values": [["c", "d"]]}]})
    ranges_back = (live.cmd("et.readRange", {"range": "E1:F2", "sheet": "Axiom2"}).get("result") or {}).get("values")
    check(ranges.get("ok") is True and (ranges.get("result") or {}).get("ranges") == 2
          and ranges_back == [["a", "b"], ["c", "d"]], "et.writeRanges gop nhieu vung", (ranges, ranges_back))

    pdf = os.path.join(live.home, "bang.pdf")
    exported = live.cmd("et.exportPdf", {"path": pdf}, timeout=180)
    check(exported.get("ok") is True and is_pdf(pdf) and os.path.getsize(pdf) > 300,
          "et.exportPdf (API goc khong ghi file -> tu dung PDF that)", exported)

    saved = os.path.join(live.home, "ket-qua.xlsx")
    saved_as = live.cmd("et.saveAs", {"path": saved})
    check(saved_as.get("ok") is True and os.path.exists(saved) and os.path.getsize(saved) > 0, "et.saveAs .xlsx",
          saved_as)
    saved_again = live.cmd("et.save")
    check(saved_again.get("ok") is True, "et.save (sau saveAs)", saved_again)

    made = live.cmd("et.newWorkbook")
    opened = live.cmd("et.open", {"path": saved})
    check(made.get("ok") is True and (made.get("result") or {}).get("opened") is True
          and (opened.get("result") or {}).get("name") == "ket-qua.xlsx", "et.newWorkbook / et.open", (made, opened))

    closed = live.cmd("et.closeAll", timeout=180)
    check(closed.get("ok") is True and (closed.get("result") or {}).get("closed", 0) >= 1,
          "et.closeAll dong het so (cuu ban chua luu truoc khi dong)", closed)


def run_wpp(live: Session) -> None:
    names = live.commands()
    check("wpp.addSlide" in names and "wpp.checkLayout" in names and "et.readRange" not in names
          and "writer.appendText" not in names,
          "catalog cua Presentation chi co lenh wpp.* (catalog loc theo app)", names)

    listed = live.cmd("wpp.listSlides")
    slides = (listed.get("result") or {}).get("slides") or []
    check(listed.get("ok") is True and (listed.get("result") or {}).get("slideCount") == len(slides),
          "wpp.listSlides (bai mau chua co slide nao)", listed)
    before = len(slides)

    added = live.cmd("wpp.addSlide", {})
    check(added.get("ok") is True and (added.get("result") or {}).get("slide") == before + 1
          and (added.get("result") or {}).get("layout") == 12, "wpp.addSlide (mac dinh layout 12 = trong)", added)
    layout = live.cmd("wpp.addSlide", {"layout": 1})
    check(layout.get("ok") is True and (layout.get("result") or {}).get("layout") == 1, "wpp.addSlide {layout:1}",
          layout)

    text = live.cmd("wpp.addText", {"text": "Xin chao tu Axiom", "fontSize": 32, "bold": True,
                                    "color": "#FF0000", "align": "center"})
    where = (text.get("result") or {}).get("slide")
    after = (live.cmd("wpp.listSlides").get("result") or {}).get("slides") or []
    target = next((s for s in after if s.get("index") == where), {})
    check(text.get("ok") is True and isinstance((text.get("result") or {}).get("shape"), str)
          and "Xin chao tu Axiom" in (target.get("shapeTexts") or []),
          "wpp.addText (co chu, mau, can le) + doc lai bang wpp.listSlides", (text, target))

    boxed = live.cmd("wpp.addTextBox", {"text": "Chi chu", "left": 30, "top": 40})
    check(boxed.get("ok") is True and isinstance((boxed.get("result") or {}).get("shape"), str), "wpp.addTextBox",
          boxed)

    table = live.cmd("wpp.addTable", {"values": [["Ten", "Diem"], ["An", 9.5]]})
    check(table.get("ok") is True and (table.get("result") or {}).get("rows") == 2
          and (table.get("result") or {}).get("cols") == 2 and (table.get("result") or {}).get("filled") == 4,
          "wpp.addTable (rows/cols suy ra tu values)", table)

    png = os.path.join(live.home, "anh.png")
    write_png(png)
    image = live.cmd("wpp.addImage", {"path": png, "width": 100})
    check(image.get("ok") is True and isinstance((image.get("result") or {}).get("shape"), str),
          "wpp.addImage (anh that do bai test tao)", image)

    notes = live.cmd("wpp.setNotes", {"text": "Noi dung thuyet trinh"})
    check(notes.get("ok") is True and (notes.get("result") or {}).get("notes") == len("Noi dung thuyet trinh"),
          "wpp.setNotes ghi vao placeholder ghi chu", notes)

    layout_report = live.cmd("wpp.checkLayout")
    data = layout_report.get("result") or {}
    check(layout_report.get("ok") is True and data.get("slideWidth", 0) > 0 and data.get("slideHeight", 0) > 0
          and data.get("issueCount") == sum(len(s.get("issues") or []) for s in data.get("slides") or []),
          "wpp.checkLayout (kho slide that + issueCount khop)", layout_report)

    deleted = live.cmd("wpp.deleteSlide", {})
    count_now = (live.cmd("wpp.listSlides").get("result") or {}).get("slideCount")
    check(deleted.get("ok") is True and (deleted.get("result") or {}).get("slideCount") == count_now,
          "wpp.deleteSlide xoa slide cuoi", (deleted, count_now))

    pdf = os.path.join(live.home, "bai.pdf")
    exported = live.cmd("wpp.exportPdf", {"path": pdf}, timeout=180)
    check(exported.get("ok") is True and is_pdf(pdf) and os.path.getsize(pdf) > 300,
          "wpp.exportPdf (API goc khong ghi file -> tu dung PDF that)", exported)

    saved = os.path.join(live.home, "bai.pptx")
    saved_as = live.cmd("wpp.saveAs", {"path": saved})
    check(saved_as.get("ok") is True and os.path.exists(saved) and os.path.getsize(saved) > 0, "wpp.saveAs .pptx",
          saved_as)
    saved_again = live.cmd("wpp.save")
    check(saved_again.get("ok") is True, "wpp.save (sau saveAs)", saved_again)

    made = live.cmd("wpp.newPresentation")
    opened = live.cmd("wpp.open", {"path": saved})
    check(made.get("ok") is True and (made.get("result") or {}).get("opened") is True
          and (opened.get("result") or {}).get("name") == "bai.pptx", "wpp.newPresentation / wpp.open",
          (made, opened))

    # Dat ro tien de cua phep kiem duoi: co NHIEU hon mot bai dang mo, de `closed` chung minh vong lap trong
    # cmdWppCloseAll dong duoc nhieu bai (khong phai chi "dong duoc mot cai roi thoi"). Day cung chinh la cau
    # hinh da dung de do ra loi `Presentation.Close(false)` bi WPP tu choi — xem tests/wps/REFERENCE.md.
    second = os.path.join(live.home, "bai-2.pptx")
    saved2 = live.cmd("wpp.saveAs", {"path": second})
    opened2 = live.cmd("wpp.open", {"path": second})
    check(saved2.get("ok") is True and (opened2.get("result") or {}).get("opened") is True,
          "wpp.saveAs + wpp.open bai thu hai (tien de cua phep kiem closeAll)", (saved2, opened2))

    # So ky vong lay tu do that (12/10/2026): hai bai dang mo -> `closed` = 2 va `Presentations.Count` ve 0.
    closed = live.cmd("wpp.closeAll", timeout=180)
    check(closed.get("ok") is True and (closed.get("result") or {}).get("closed", 0) >= 2,
          "wpp.closeAll dong het 2 bai dang mo (cuu ban chua luu truoc khi dong)", closed)


APPS = {
    "wps": {"brief": "Writer", "exe": "wps", "source": BLANK_DOCX, "document": "blank.docx", "run": run_writer},
    "et": {"brief": "Spreadsheets", "exe": "et", "source": BLANK_XLSX, "document": "blank.xlsx", "run": run_et},
    "wpp": {"brief": "Presentation", "exe": "wpp", "source": BLANK_PPTX, "document": "blank.pptx", "run": run_wpp},
}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--keep", action="store_true", help="giu thu muc lam viec (dat trong tests/wps/out/)")
    parser.add_argument("--timeout", type=int, default=120, help="giay cho add-in ket noi moi app")
    parser.add_argument("--apps", default="wps,et,wpp", help="app chay (mac dinh ca ba)")
    args = parser.parse_args()

    apps = [a.strip() for a in args.apps.split(",") if a.strip()]
    unknown = [a for a in apps if a not in APPS]
    if unknown:
        print("FAIL app la: %s (chon trong %s)" % (", ".join(unknown), ", ".join(APPS)))
        return 1

    oem = ""
    try:
        with open(OEM_INI, encoding="utf-8", errors="replace") as handle:
            oem = handle.read()
    except OSError:
        pass
    if "JsApiPlugin=true" not in oem.replace(" ", ""):
        print("FAIL %s chua bat JsApiPlugin=true (can sudo, xem tests/wps/REFERENCE.md)" % OEM_INI)
        return 1
    missing = [name for name in ("wps", "et", "wpp", "xvfb-run", "xdotool") if not shutil.which(name)]
    if missing:
        print("FAIL can wps, et, wpp, xvfb-run va xdotool (thieu: %s)" % ", ".join(missing))
        return 1
    if wps_running():
        print("FAIL WPS dang chay cho nguoi dung nay (pid %s) - dong WPS roi chay lai" % " ".join(wps_running()))
        return 1

    out_dir = os.path.join(ROOT, "tests", "wps", "out")
    if args.keep:
        os.makedirs(out_dir, exist_ok=True)
    work = tempfile.mkdtemp(prefix="live-" if args.keep else "axiom-wps-live-", dir=out_dir if args.keep else None)
    home = os.path.join(work, "home")
    session_dir = os.path.join(work, "sessions")
    os.makedirs(home)
    os.makedirs(session_dir)
    bridge_port = base.free_port()
    bridge = "http://127.0.0.1:%d" % bridge_port
    auth = {"X-Auth-Token": base.TOKEN}
    addin_dir = os.path.join(ROOT, "src", "AxiomOffice.WPS")

    core = base.Core(base.core_exe(work), work, base.TOKEN, bridge_port, addin_dir, session_dir,
                     extra_env={"AXIOM_WPS_DISCONNECT_SECONDS": str(DISCONNECT_SECONDS)})
    live = Session(bridge, auth, core, home, session_dir, args.timeout)
    try:
        env = {k: v for k, v in os.environ.items() if not k.startswith("XDG_")}
        env["HOME"] = home
        installed = subprocess.run([sys.executable, os.path.join(ROOT, "scripts", "wps", "install_jsaddon.py"),
                                    "--home", home, "--core-url", core.base, "--test-profile"],
                                   capture_output=True, text=True, timeout=60, env=env)
        check(installed.returncode == 0, "install_jsaddon.py --core-url --test-profile", installed.stderr)

        for kind in apps:
            spec = APPS[kind]
            live.start(kind, spec["exe"], spec["source"], spec["document"])
            connected = live.wait_connected()
            check(connected, "add-in trong WPS %s ket noi bridge; /health app = %s" % (spec["brief"], kind),
                  (live.health(), live.process.pid))
            if not connected:
                raise SystemExit(1)
            session_path = live.session_file()
            check(base.wait_for(lambda: os.path.exists(session_path), 10),
                  "file session WPS xuat hien (%s)" % kind, os.listdir(session_dir))
            data = live.session_data()
            check(data.get("app") == kind and data.get("document") == spec["document"]
                  and data.get("family") == "wps-linux",
                  "file session cua %s: app=%s, document=%s" % (kind, kind, spec["document"]), data)
            spec["run"](live)
            live.stop()
    finally:
        if live.process is not None:
            try:
                os.killpg(live.process.pid, signal.SIGTERM)
                live.process.wait(timeout=20)
            except Exception:  # noqa: BLE001
                try:
                    os.killpg(live.process.pid, signal.SIGKILL)
                except Exception:  # noqa: BLE001
                    pass
        core.stop()
        if not args.keep:
            shutil.rmtree(work, ignore_errors=True)
        else:
            print("thu muc lam viec: " + work)

    failed = [name for ok, name in base.RESULTS if not ok]
    print("\n%d passed, %d failed" % (len(base.RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
