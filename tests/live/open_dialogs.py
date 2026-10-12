"""Mo dialog cua menu LibreOffice trong LibreOffice THAT roi chup anh man hinh de nguoi doc tu kiem.

Vi sao can script rieng: cac dialog nay la cua so CON (`awt.ChildWindow`) nam trong vung tai lieu, khong
phai cua so rieng tren X, nen khong the tim bang `xdotool search --name <tieu de>` nhu test_setup_wizard.py
lam voi wizard (wizard la cua so roi). Thu kiem duoc bang may la ANH CHUP.

Script chay LibreOffice trong PROFILE TAM va XDG TAM (khong dung cau hinh/danh sach tai lieu cua nguoi dung),
gui dung URL cua menu qua UNO - tuc la di het duong di cua menu: Addons.xcu -> ProtocolHandler.xcu ->
dispatch.py -> dialogs.open_* - chi thieu cu bam chuot vao muc menu. Dat san Agent Core da build vao
XDG_DATA_HOME tam (xem `--core`) de dialog Ky nang/Nhat ky co du lieu that.

    xvfb-run -a -s "-screen 0 1280x900x24" python3 tests/live/open_dialogs.py check
    xvfb-run -a python3 tests/live/open_dialogs.py skills --core dist/axiom-office-linux-x64-0.1.0/core
    python3 tests/live/open_dialogs.py audit --keep          # xem tay tren man hinh that

Anh chup nam o --out (mac dinh /tmp/axiom-dialogs). Moi lan chay chi mo MOT dialog: cac dialog deu nam giua
vung tai lieu nen mo nhieu cai thi cai sau che cai truoc.
"""
from __future__ import annotations

import argparse
import os
import pathlib
import shutil
import socket
import subprocess
import sys
import tempfile
import time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ACTIONS = ("check", "skills", "audit")
URL_PREFIX = "org.axiomoffice.bridge:"
BRIDGE_PORT = 47851                      # Writer (config.DEFAULT_PORT); Calc +1, Impress +2
# Bang that de `writer.checkTables` co cai ma bao: mot o de trong -> "empty-cells".
FIXTURE_TABLE = {"values": [["Họ tên", "Số điện thoại"], ["Nguyễn Văn A", ""]]}


def free_port() -> int:
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        return probe.getsockname()[1]


def run(command, **kwargs):
    return subprocess.run(command, capture_output=True, text=True, **kwargs)


def install_extension(profile: pathlib.Path) -> None:
    oxt = run([sys.executable, os.path.join(ROOT, "scripts", "package_oxt.py"), "--print-path"], check=True).stdout.strip()
    unopkg = shutil.which("unopkg") or "/usr/lib/libreoffice/program/unopkg"
    added = run([unopkg, "-env:UserInstallation=" + profile.as_uri(), "add", "--force", oxt])
    if added.returncode != 0:
        raise SystemExit("khong cai duoc extension vao profile tam: %s" % (added.stderr or added.stdout)[:400])


def stage_core(work: pathlib.Path, core_dir: str) -> str:
    """Chep ban Core da build vao XDG_DATA_HOME tam de `core.ensure()` tu khoi dong no (kem thu muc skills)."""
    target = work / "data" / "axiom-office" / "core"
    shutil.copytree(core_dir, target)
    binary = target / "AxiomOffice.Core"
    if not binary.is_file():
        raise SystemExit("khong thay AxiomOffice.Core trong %s" % core_dir)
    binary.chmod(0o755)
    return str(binary)


def launch(work: pathlib.Path, port: int) -> subprocess.Popen:
    env = dict(os.environ)
    for name, sub in (("XDG_CONFIG_HOME", "config"), ("XDG_DATA_HOME", "data"), ("XDG_CACHE_HOME", "cache")):
        env[name] = str(work / sub)
    # XDG_RUNTIME_DIR rieng: Core giu khoa single-instance o $XDG_RUNTIME_DIR/axiom-office/*.lock, nen de nguyen
    # thi Core trong profile tam thay Core dang chay cua nguoi dung va TU THOAT ("already running").
    runtime = work / "runtime"
    runtime.mkdir(mode=0o700, exist_ok=True)
    env["XDG_RUNTIME_DIR"] = str(runtime)
    soffice = shutil.which("soffice") or "/usr/lib/libreoffice/program/soffice"
    return subprocess.Popen([soffice, "-env:UserInstallation=" + (work / "profile").as_uri(),
                             "--writer", "--norestore", "--nologo", "--nofirststartwizard",
                             "--accept=socket,host=127.0.0.1,port=%d;urp;" % port],
                            env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def bridge(work: pathlib.Path, action: str, params: dict, seconds: float = 30.0) -> dict:
    """Goi lenh bridge qua HTTP (cung duong Core dung) de dung san noi dung tai lieu cho dialog "Soat".

    Nho vay dialog co bang THAT do chinh extension tao ra, khong phai mot file .fodt viet tay (ban viet tay
    de bi LibreOffice hieu khac di, va mot o trong viet sai thi "khong co van de" la ket qua SAI).
    """
    import json
    import urllib.error
    import urllib.request

    config = work / "config" / "axiom-office" / "config.json"
    deadline = time.time() + seconds
    while time.time() < deadline:
        token = ""
        try:
            token = json.loads(config.read_text(encoding="utf-8")).get("Token", "")
        except (OSError, ValueError):
            pass
        body = json.dumps({"action": action, "params": params}).encode("utf-8")
        request = urllib.request.Request("http://127.0.0.1:%d/cmd" % BRIDGE_PORT, data=body, method="POST")
        request.add_header("Content-Type", "application/json")
        request.add_header("X-Auth-Token", token)
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.loads(response.read().decode("utf-8"))
        except (urllib.error.URLError, OSError, ValueError):
            time.sleep(1.0)
    raise SystemExit("bridge khong tra loi tren cong %d (extension chua nap?)" % BRIDGE_PORT)


def connect(port: int, seconds: float = 120.0):
    """Ket noi UNO toi LibreOffice dang chay (uno cua python he thong, xem scripts/libreoffice.sh)."""
    import uno

    resolver = uno.getComponentContext().ServiceManager.createInstanceWithContext(
        "com.sun.star.bridge.UnoUrlResolver", uno.getComponentContext())
    deadline = time.time() + seconds
    last = None
    while time.time() < deadline:
        try:
            return resolver.resolve("uno:socket,host=127.0.0.1,port=%d;urp;StarOffice.ComponentContext" % port)
        except Exception as exc:  # noqa: BLE001 - chua len thi thu lai
            last = exc
            time.sleep(1.0)
    raise SystemExit("khong ket noi duoc UNO: %s" % last)


def dismiss_first_run() -> None:
    """Dong hop thoai lan dau (Tip of the Day): modal nay che dialog cua minh trong anh chup."""
    if not shutil.which("xdotool"):
        return
    for _ in range(5):
        found = run(["xdotool", "search", "--name", "Tip of the Day"]).stdout.split()
        if not found:
            return
        run(["xdotool", "windowactivate", "--sync", found[0]])
        run(["xdotool", "key", "--window", found[0], "Escape"])
        time.sleep(1.0)


def shoot(out_dir: pathlib.Path, name: str) -> str:
    """Chup ca man hinh X (xwd) roi doi sang PNG bang ffmpeg (may nay khong co ImageMagick)."""
    out_dir.mkdir(parents=True, exist_ok=True)
    raw = out_dir / (name + ".xwd")
    png = out_dir / (name + ".png")
    if run(["xwd", "-root", "-silent", "-out", str(raw)]).returncode != 0:
        return ""
    if run(["ffmpeg", "-y", "-loglevel", "error", "-i", str(raw), str(png)]).returncode != 0:
        return ""
    raw.unlink(missing_ok=True)
    return str(png)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("action", nargs="?", default="check", choices=ACTIONS)
    parser.add_argument("--out", default="/tmp/axiom-dialogs", help="thu muc anh chup")
    parser.add_argument("--core", default="", help="thu muc chua AxiomOffice.Core da build (kem skills/)")
    parser.add_argument("--keep", action="store_true", help="khong tat LibreOffice sau khi chay")
    args = parser.parse_args()

    if not os.environ.get("DISPLAY"):
        raise SystemExit("khong co DISPLAY: chay bang `xvfb-run -a python3 tests/live/open_dialogs.py %s`" % args.action)

    work = pathlib.Path(tempfile.mkdtemp(prefix="axiom-dialogs-"))
    (work / "profile").mkdir()
    install_extension(work / "profile")
    if args.core:
        print("Agent Core: %s" % stage_core(work, args.core))

    port = free_port()
    process = launch(work, port)
    try:
        ctx = connect(port)
        desktop = ctx.ServiceManager.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)
        deadline = time.time() + 60
        while time.time() < deadline and desktop.CurrentFrame is None:
            time.sleep(1.0)
        dismiss_first_run()
        if args.action == "check":
            print("writer.insertTable: %s" % bridge(work, "writer.insertTable", FIXTURE_TABLE))

        import uno

        url = uno.createUnoStruct("com.sun.star.util.URL")
        url.Complete = URL_PREFIX + args.action
        dispatcher = desktop.CurrentFrame.queryDispatch(url, "_self", 0)
        if dispatcher is None:
            raise SystemExit("menu chua duoc dang ky: queryDispatch(%s) tra None (Addons.xcu/ProtocolHandler.xcu?)" % url.Complete)
        print("dispatch %s" % url.Complete)
        dispatcher.dispatch(url, ())
        time.sleep(2.5)          # do chu tu Core (dialog Ky nang/Nhat ky goi HTTP) + ve xong

        shot = shoot(pathlib.Path(args.out), args.action)
        print("anh chup: %s" % shot if shot else "KHONG chup duoc man hinh (thieu xwd/ffmpeg?)")
        if args.keep:
            input("Enter de tat LibreOffice…")
    finally:
        if not args.keep:
            process.terminate()
            try:
                process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                process.kill()
        shutil.rmtree(work, ignore_errors=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
