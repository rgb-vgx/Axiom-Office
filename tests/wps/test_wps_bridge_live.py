"""Spec T2 chay that (docs/wps-bridge.md): Agent Core + JS add-in + WPS Writer that duoi Xvfb.

Build Core, cai add-in vao HOME tam (`install_jsaddon.py --core-url ... --test-profile`), mo blank.docx trong WPS,
cho add-in ket noi bridge WPS roi goi tung lenh writer.* qua POST /cmd va doc lai tai lieu de kiem.

Can: WPS Office for Linux, xvfb-run, JsApiPlugin=true trong oem.ini (tests/wps/REFERENCE.md), khong co WPS
dang chay cho nguoi dung nay.

    python3 tests/wps/test_wps_bridge_live.py [--keep]
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_wps_bridge_e2e as base  # noqa: E402

ROOT = base.ROOT
OEM_INI = "/opt/kingsoft/wps-office/office6/cfgs/oem.ini"
BLANK_DOCX = os.path.join(ROOT, "core-go", "internal", "templates", "default.docx")
check = base.check


def wps_running() -> list:
    out = subprocess.run(["pgrep", "-u", str(os.getuid()), "-f", "office6/(wps|et|wpp)( |$)"],
                         capture_output=True, text=True).stdout
    return out.split()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--keep", action="store_true", help="giu thu muc lam viec (dat trong tests/wps/out/)")
    parser.add_argument("--timeout", type=int, default=120, help="giay cho add-in ket noi")
    args = parser.parse_args()

    oem = ""
    try:
        with open(OEM_INI, encoding="utf-8", errors="replace") as handle:
            oem = handle.read()
    except OSError:
        pass
    if "JsApiPlugin=true" not in oem.replace(" ", ""):
        print("FAIL %s chua bat JsApiPlugin=true (can sudo, xem tests/wps/REFERENCE.md)" % OEM_INI)
        return 1
    if not shutil.which("wps") or not shutil.which("xvfb-run"):
        print("FAIL can wps va xvfb-run")
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

    def cmd(action, params=None, timeout=90):
        status, _, text = base.request(bridge + "/cmd", "POST", {"action": action, "params": params or {}}, auth,
                                       timeout=timeout)
        reply = base.as_json(text)
        return reply if status == 200 else {"ok": False, "error": "HTTP %s: %s" % (status, text[:300])}

    def text_now() -> str:
        reply = cmd("writer.getText")
        return str((reply.get("result") or {}).get("text") or "")

    core = base.Core(base.core_exe(work), work, base.TOKEN, bridge_port, addin_dir, session_dir)
    process = None
    try:
        env = {k: v for k, v in os.environ.items() if not k.startswith("XDG_")}
        env["HOME"] = home
        installed = subprocess.run([sys.executable, os.path.join(ROOT, "scripts", "wps", "install_jsaddon.py"),
                                    "--home", home, "--core-url", core.base, "--test-profile"],
                                   capture_output=True, text=True, timeout=60, env=env)
        check(installed.returncode == 0, "install_jsaddon.py --core-url --test-profile", installed.stderr)
        document = os.path.join(home, "blank.docx")
        shutil.copyfile(BLANK_DOCX, document)
        process = subprocess.Popen(["xvfb-run", "-a", "-s", "-screen 0 1280x900x24", "wps", document], env=env,
                                   cwd=home, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                   start_new_session=True)

        def connected() -> bool:
            status, _, text = base.request(bridge + "/health", timeout=3)
            return status == 200 and (base.as_json(text).get("result") or {}).get("connected") is True

        ok = base.wait_for(connected, args.timeout)
        check(ok, "add-in trong WPS ket noi bridge WPS (/health connected=true)", "")
        if not ok:
            raise SystemExit(1)

        status, _, text = base.request(bridge + "/commands", headers=auth)
        names = [c.get("name") for c in (base.as_json(text).get("result") or {}).get("commands") or []]
        check("writer.appendText" in names and "writer.undo" in names, "bridge /commands = catalog cua add-in that", names)
        session_path = os.path.join(session_dir, "%d-wps.json" % core.process.pid)
        check(base.wait_for(lambda: os.path.exists(session_path), 10), "file session WPS xuat hien",
              os.listdir(session_dir))

        info = cmd("app.info")
        check(info.get("ok") is True, "app.info", info)
        reply = cmd("writer.getText", {"maxChars": 1})
        result = reply.get("result") or {}
        check(reply.get("ok") is True and isinstance(result.get("totalChars"), int) and "text" in result,
              "writer.getText tra text + totalChars (policy cua Core can totalChars)", reply)

        reply = cmd("writer.appendText", {"text": "Dòng AI một"})
        check(reply.get("ok") is True and "Dòng AI một" in text_now(), "writer.appendText", reply)
        reply = cmd("writer.undo", {"count": 1})
        check(reply.get("ok") is True and "Dòng AI một" not in text_now(),
              "writer.undo 1 buoc go dung mot lenh AI (UndoRecord)", (reply, text_now()[-200:]))

        reply = cmd("writer.appendText", {"text": "Dòng AI hai"})
        check(reply.get("ok") is True, "writer.appendText lan 2", reply)
        reply = cmd("writer.insertStyledText", {"text": " chữ đậm", "bold": True, "color": "#C00000"})
        check(reply.get("ok") is True and "chữ đậm" in text_now(), "writer.insertStyledText", reply)
        reply = cmd("writer.heading", {"text": "Tiêu đề A", "level": 1})
        check(reply.get("ok") is True and "Tiêu đề A" in text_now(), "writer.heading", reply)
        reply = cmd("writer.typeText", {"text": "Gõ ở con trỏ"})
        check(reply.get("ok") is True and "Gõ ở con trỏ" in text_now(), "writer.typeText", reply)
        reply = cmd("writer.replaceAll", {"find": "Dòng AI hai", "replace": "Dòng đã thay"})
        now = text_now()
        check(reply.get("ok") is True and "Dòng đã thay" in now and "Dòng AI hai" not in now,
              "writer.replaceAll", (reply, now[-200:]))
        reply = cmd("writer.insertTable", {"values": [["A1", "B1"], ["A2", "B2"]]})
        check(reply.get("ok") is True and "B2" in text_now(), "writer.insertTable (values 2x2)", reply)
        reply = cmd("writer.selection")
        check(reply.get("ok") is True and "text" in (reply.get("result") or {}), "writer.selection", reply)
        reply = cmd("writer.formatSelection", {"bold": True, "size": 13})
        check(reply.get("ok") is True, "writer.formatSelection", reply)
        reply = cmd("writer.setParagraphAlignment", {"alignment": "center"})
        check(reply.get("ok") is True, "writer.setParagraphAlignment", reply)
        reply = cmd("writer.insertPageBreak")
        check(reply.get("ok") is True, "writer.insertPageBreak", reply)

        saved = os.path.join(home, "ket-qua.docx")
        reply = cmd("writer.saveAs", {"path": saved})
        check(reply.get("ok") is True and os.path.exists(saved) and os.path.getsize(saved) > 0, "writer.saveAs .docx",
              reply)
        reply = cmd("writer.save")
        check(reply.get("ok") is True, "writer.save (sau saveAs)", reply)
        pdf = os.path.join(home, "ket-qua.pdf")
        reply = cmd("writer.exportPdf", {"path": pdf}, timeout=180)
        check(reply.get("ok") is True and os.path.exists(pdf) and os.path.getsize(pdf) > 1000, "writer.exportPdf", reply)

        reply = cmd("writer.khongCo")
        check(reply.get("ok") is False, "lenh la -> ok:false", reply)
        with open(session_path, encoding="utf-8") as handle:
            data = json.load(handle)
        check(data.get("app") == "wps" and data.get("document") in ("blank.docx", "ket-qua.docx"),
              "file session co tai lieu dang mo", data)
    finally:
        if process is not None:
            try:
                os.killpg(process.pid, signal.SIGTERM)
                process.wait(timeout=20)
            except Exception:  # noqa: BLE001
                try:
                    os.killpg(process.pid, signal.SIGKILL)
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
