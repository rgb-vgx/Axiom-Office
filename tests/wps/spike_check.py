"""Spike WPS Office for Linux: JS add-in co nap duoc va tra loi duoc 3 cau hoi kien truc khong.

Day la SPEC cua spike (nhanh spec/wps-jsaddon-spike) - khong sua file nay de qua bai.

Hop dong voi phan can lam:
- `scripts/wps/install_jsaddon.py --home <HOME> --report-url <URL>` cai add-in trong `src/AxiomOffice.WPS/`
  vao WPS cua HOME do (`<HOME>/.local/share/Kingsoft/wps/jsaddons/...`). Duoc ghi them cau hinh WPS can thiet
  trong CHINH HOME do (HOME ma test dua la thu muc tam), khong bao gio dung toi HOME that.
- Khi WPS Writer mo (duoi Xvfb, HOME tam), add-in TU NAP va POST mot JSON toi <URL> voi it nhat:
    loaded      true
    wpsVersion  chuoi phien ban WPS (Application.Version hoac tuong duong)
    undoRecord  true/false: Application.UndoRecord (gom nhieu thao tac thanh 1 buoc Undo) co dung duoc khong
  Them truong nao huu ich cho viec thiet ke bridge thi cu them (vd cac API da thu, loi gap phai).
- Server ben duoi tu ghi header `Origin` cua request (de quyet dinh guard cua Agent Core) - add-in khong can bao.

Gia tri cua undoRecord la gi cung DAT: muc dich spike la BIET, khong phai bat buoc phai co.

    python3 tests/wps/spike_check.py [--timeout 120] [--keep]
"""
from __future__ import annotations

import argparse
import http.server
import json
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
INSTALLER = os.path.join(ROOT, "scripts", "wps", "install_jsaddon.py")
REPORT_FILE = os.path.join(ROOT, "tests", "wps", "out", "spike-report.json")
REQUIRED = ("loaded", "wpsVersion", "undoRecord")

reports: list = []


class Handler(http.server.BaseHTTPRequestHandler):
    def _cors(self) -> None:
        # Add-in chay trong trinh duyet nhung cua WPS: cho phep moi origin de do duoc ca truong hop co preflight.
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "*")

    def do_OPTIONS(self) -> None:  # noqa: N802
        reports.append({"preflight": True, "origin": self.headers.get("Origin")})
        self.send_response(204)
        self._cors()
        self.end_headers()

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length).decode("utf-8", errors="replace")
        try:
            body = json.loads(raw)
        except ValueError:
            body = {"_raw": raw}
        reports.append({"path": self.path, "origin": self.headers.get("Origin"),
                        "contentType": self.headers.get("Content-Type"), "body": body})
        self.send_response(200)
        self._cors()
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"ok":true}')

    def log_message(self, *args) -> None:
        pass


def wps_running() -> list:
    out = subprocess.run(["pgrep", "-u", str(os.getuid()), "-f", "office6/(wps|et|wpp)( |$)"],
                         capture_output=True, text=True).stdout
    return out.split()


def fail(message: str) -> int:
    print("FAIL " + message)
    return 1


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--timeout", type=int, default=120, help="giay cho bao cao tu add-in")
    parser.add_argument("--keep", action="store_true", help="giu HOME tam de xem lai")
    args = parser.parse_args()

    if not os.path.exists(INSTALLER):
        return fail("chua co %s (cai add-in JS cho WPS Linux)" % os.path.relpath(INSTALLER, ROOT))
    wps = shutil.which("wps")
    xvfb = shutil.which("xvfb-run")
    if not wps or not xvfb:
        return fail("can wps (WPS Office for Linux) va xvfb-run tren may (wps=%s, xvfb-run=%s)" % (wps, xvfb))
    if wps_running():
        return fail("WPS dang chay cho nguoi dung nay (pid %s) - dong WPS roi chay lai, tranh WPS moi chuyen "
                    "lenh sang ban dang mo" % " ".join(wps_running()))

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    url = "http://127.0.0.1:%d/report" % server.server_address[1]

    home = tempfile.mkdtemp(prefix="axiom-wps-spike-")
    env = {key: value for key, value in os.environ.items() if not key.startswith("XDG_")}
    env["HOME"] = home
    process = None
    try:
        installed = subprocess.run([sys.executable, INSTALLER, "--home", home, "--report-url", url],
                                   capture_output=True, text=True, timeout=120, env=env)
        print(installed.stdout.strip())
        if installed.returncode != 0:
            return fail("install_jsaddon.py loi (exit %d): %s" % (installed.returncode, installed.stderr.strip()[-800:]))
        jsaddons = os.path.join(home, ".local", "share", "Kingsoft", "wps", "jsaddons")
        if not os.path.isdir(jsaddons) or not os.listdir(jsaddons):
            return fail("install_jsaddon.py khong ghi gi vao %s" % jsaddons)

        process = subprocess.Popen([xvfb, "-a", "-s", "-screen 0 1280x900x24", wps], env=env, cwd=home,
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        deadline = time.time() + args.timeout
        while time.time() < deadline and not any("body" in item for item in reports):
            time.sleep(1.0)
        time.sleep(3.0)  # add-in co the gui them bao cao ngay sau bao cao dau
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
        server.shutdown()
        if not args.keep:
            shutil.rmtree(home, ignore_errors=True)
        else:
            print("HOME tam: " + home)

    os.makedirs(os.path.dirname(REPORT_FILE), exist_ok=True)
    with open(REPORT_FILE, "w", encoding="utf-8") as handle:
        json.dump(reports, handle, ensure_ascii=False, indent=2)
    print("== bao cao nhan duoc (%s):" % os.path.relpath(REPORT_FILE, ROOT))
    print(json.dumps(reports, ensure_ascii=False, indent=2)[:6000])

    posts = [item for item in reports if "body" in item]
    if not posts:
        return fail("khong nhan duoc bao cao nao tu add-in trong %ds (add-in khong nap, hoac khong goi duoc %s)"
                    % (args.timeout, url))
    body = posts[0]["body"] if isinstance(posts[0]["body"], dict) else {}
    missing = [key for key in REQUIRED if key not in body]
    if missing:
        return fail("bao cao thieu truong %s: %s" % (", ".join(missing), json.dumps(body, ensure_ascii=False)[:500]))
    if body.get("loaded") is not True:
        return fail("loaded phai la true: %r" % body.get("loaded"))
    if not isinstance(body.get("undoRecord"), bool):
        return fail("undoRecord phai la true/false: %r" % body.get("undoRecord"))
    if not str(body.get("wpsVersion") or "").strip():
        return fail("wpsVersion rong")

    print("PASS add-in nap trong WPS %s; Origin=%r; undoRecord=%s"
          % (body["wpsVersion"], posts[0]["origin"], body["undoRecord"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
