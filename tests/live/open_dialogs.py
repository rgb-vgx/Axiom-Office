"""Mo dialog cua menu LibreOffice trong LibreOffice THAT roi chup anh man hinh de nguoi doc tu kiem.

Vi sao can script rieng: LibreOffice KHONG chay `--headless` cho viec nay (phai co cua so that) va du dialog
bay gio la cua so roi (`awt.Dialog`, tim duoc bang `xdotool search --name`) thi thu THAT SU kiem duoc van la
cu bam chuot + anh chup.

Script chay LibreOffice trong PROFILE TAM va XDG TAM (khong dung cau hinh/danh sach tai lieu cua nguoi dung),
gui dung URL cua menu qua UNO - tuc la di het duong di cua menu: Addons.xcu -> ProtocolHandler.xcu ->
dispatch.py -> dialogs.open_* - chi thieu cu bam chuot vao muc menu. Dat san Agent Core da build vao
XDG_DATA_HOME tam (xem `--core`) de dialog Ky nang/Nhat ky co du lieu that.

Script cung DON cac Agent Core mo coi do chinh no sinh ra: Core tach session khoi LibreOffice nen SONG TIEP
sau khi LibreOffice bi tat, moi cai chiem mot cong TCP (47840, 47841, ...). Muoi lan chay `--keep` da chiem
kin 47841-47849 khien lan chay sau khong bind duoc cong nao va dialog chi bao *"Agent Core khong san sang sau
25s"* - loi cua rac do chinh script de lai, nhin nhu loi cua extension. Core that cua nguoi dung
(`~/.local/share/axiom-office/core/AxiomOffice.Core`) KHONG bi dung toi.

    xvfb-run -a -s "-screen 0 1280x900x24" python3 tests/live/open_dialogs.py check
    xvfb-run -a python3 tests/live/open_dialogs.py skills --core dist/axiom-office-linux-x64-0.1.0/core
    python3 tests/live/open_dialogs.py audit --keep          # xem tay tren man hinh that

Anh chup nam o --out (mac dinh /tmp/axiom-dialogs). Moi lan chay mo MOT dialog (co the bam lien tiep nhieu nut).

`--click X,Y`: bam chuot THAT (xdotool) tai toa do MAN HINH (anh chup la ca man hinh X nen toa do trong anh
chinh la toa do man hinh), roi chup them anh "<action>-sau.png".

`--click-window TIEU-DE:X,Y` (lap lai duoc): bam vao toa do TRONG long dialog cua cua so roi co tieu de do.
Duong di nay chi chay khi dialog la cua so roi (`awt.Dialog`, xem dialogs._open), vi cua so con khong co cua
so X rieng. Toa do tinh theo long dialog: nut "Dong" cua "Ky nang" la 503,414 (x 458..548, y 402..426),
"Nap lai" 72,414. Sau MOI cu bam, script hoi thang X xem cua so con do khong (`xwininfo` Map State) va in ra.

HAI CAI BAY da tra gia khi do bang script nay (13/10/2026), doc truoc khi tin vao anh chup:
1. Anh chup KHONG phan biet duoc "cua so da dong" voi "cua so da bi an nhung khong ai ve lai vung do": tren
   Xvfb khong co trinh quan ly cua so nen cua so vua dong de lai **vet cu** tren anh. Phai hoi X
   (`window_state`) chu dung nhin anh.
2. `xdotool search --name` khong co `--onlyvisible` van tra ve cua so da bi an (unmap) - loc sai cho nay thi
   ket luan "dialog van con" trong khi no da dong.
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
# Kich thuoc tung dialog (dialogs.open_*): de quy doi toa do trong cua so thanh toa do man hinh.
DIALOG_SIZES = {"check": (560, 440), "skills": (560, 440), "audit": (620, 440)}
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


def core_pids(prefixes) -> list:
    """Pid cua cac AxiomOffice.Core co duong dan chay nam trong `prefixes` (doc /proc, khong can ps)."""
    found = []
    for entry in os.listdir("/proc"):
        if not entry.isdigit():
            continue
        try:
            with open("/proc/%s/cmdline" % entry, "rb") as handle:
                argv = handle.read().split(b"\0")
        except OSError:
            continue                     # tien trinh vua chet: bo qua
        exe = argv[0].decode("utf-8", "replace") if argv and argv[0] else ""
        if exe.endswith("AxiomOffice.Core") and any(exe.startswith(prefix) for prefix in prefixes):
            found.append(int(entry))
    return found


def kill_cores(prefixes) -> list:
    """Tat Core do chinh script nay sinh ra ma con song (tra ve so pid da tat).

    Vi sao can: Core tach session khoi LibreOffice (core.launch), nen khi LibreOffice bi tat thi Core VAN SONG,
    va moi Core chiem mot cong TCP (47840, 47841, ...). Muoi lan chay `--keep` truoc do da chiem kin 47841-47849
    khien lan chay sau khong con cong nao gan do de bind, va dialog chi bao "Agent Core khong san sang sau 25s" -
    loi cua rac do chinh probe de lai, nhin nhu loi cua extension. Chi dong vao Core nam trong thu muc tam cua
    probe: Core that cua nguoi dung (~/.local/share/axiom-office/core/AxiomOffice.Core) KHONG bi dung toi.
    """
    pids = core_pids(prefixes)
    for pid in pids:
        try:
            os.kill(pid, 15)             # SIGTERM truoc, cho 5s, roi moi SIGKILL
        except OSError:
            pass
    deadline = time.time() + 5
    while pids and time.time() < deadline and core_pids(prefixes):
        time.sleep(0.2)
    for pid in core_pids(prefixes):
        try:
            os.kill(pid, 9)
        except OSError:
            pass
    return pids


def probe_prefixes(work: pathlib.Path) -> list:
    """Tien to duong dan Core cua lan chay nay VA cua cac lan chay truoc (cung kieu thu muc tam)."""
    return [str(work / "data" / "axiom-office" / "core"),
            os.path.join(os.path.dirname(str(work)), "axiom-dialogs-")]


def launch(work: pathlib.Path, port: int, quiet: bool = True) -> subprocess.Popen:
    env = dict(os.environ)
    for name, sub in (("XDG_CONFIG_HOME", "config"), ("XDG_DATA_HOME", "data"), ("XDG_CACHE_HOME", "cache")):
        env[name] = str(work / sub)
    # XDG_RUNTIME_DIR rieng: Core giu khoa single-instance o $XDG_RUNTIME_DIR/axiom-office/*.lock, nen de nguyen
    # thi Core trong profile tam thay Core dang chay cua nguoi dung va TU THOAT ("already running").
    runtime = work / "runtime"
    runtime.mkdir(mode=0o700, exist_ok=True)
    env["XDG_RUNTIME_DIR"] = str(runtime)
    soffice = shutil.which("soffice") or "/usr/lib/libreoffice/program/soffice"
    # --keep: giu stdout/stderr cua LibreOffice vao <work>/soffice.log. Loi cua Python trong extension (vd mot
    # exception trong ActionListener) chi hien o day, khong hien o dau khac.
    out = subprocess.DEVNULL if quiet else open(work / "soffice.log", "wb")
    return subprocess.Popen([soffice, "-env:UserInstallation=" + (work / "profile").as_uri(),
                             "--writer", "--norestore", "--nologo", "--nofirststartwizard",
                             "--accept=socket,host=127.0.0.1,port=%d;urp;" % port],
                            env=env, stdout=out, stderr=subprocess.STDOUT)


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


def click_in_window(title: str, x: int, y: int, size: tuple) -> bool:
    """Bam chuot THAT vao (x, y) tinh theo long dialog (rong x cao) BEN TRONG cua so co tieu de `title`.

    Chi dung duoc voi cua so roi (`awt.Dialog`) vi no co cua so X rieng; cua so con thi `xdotool search`
    khong tra ve gi - do chinh la trieu chung "cua so con khong nhan chuot". Cua so that co the co vien/thanh
    tieu de do trinh quan ly cua so ve them, nen phai tru phan lech (cung cach voi test_setup_wizard.py).
    """
    found = run(["xdotool", "search", "--name", title]).stdout.split()
    if not found:
        return False
    window = found[0]
    geometry = run(["xdotool", "getwindowgeometry", "--shell", window]).stdout
    box = dict(line.split("=") for line in geometry.strip().splitlines() if "=" in line)
    border = max(0, (int(box.get("WIDTH", size[0])) - size[0]) // 2)
    offset_y = max(0, int(box.get("HEIGHT", size[1])) - size[1] - border)
    print("cua so %s: %sx%s tai %s,%s -> lech %d,%d"
          % (window, box.get("WIDTH"), box.get("HEIGHT"), box.get("X"), box.get("Y"), border, offset_y))
    run(["xdotool", "windowraise", window])
    run(["xdotool", "windowfocus", window])
    time.sleep(0.5)
    run(["xdotool", "mousemove", "--window", window, str(x + border), str(y + offset_y)])
    time.sleep(0.3)
    run(["xdotool", "click", "1"])
    return True


def window_state(title: str) -> str:
    """Cua so co tieu de `title` dang HIEN hay khong, hoi thang X.

    Phai co ca hai ve: `xdotool search` khong co `--onlyvisible` van tra ve cua so da bi an (unmap), va tren
    Xvfb khong co trinh quan ly cua so thi cua so vua bi an de lai **vet tren anh chup** (khong ai ve lai nen
    cuoi cung vung do) - anh chup mot minh se noi "dialog van con" trong khi su that la da dong.
    """
    found = run(["xdotool", "search", "--name", title]).stdout.split()
    if not found:
        return "khong con cua so nao"
    state = [line.split(":", 1)[1].strip() for line in run(["xwininfo", "-id", found[0]]).stdout.splitlines()
             if "Map State" in line]
    return "%d cua so, Map State: %s" % (len(found), state[0] if state else "?")


def log_tail(work: pathlib.Path, count: int = 15) -> str:
    """Dong cuoi bridge.log cua extension: loi Python trong ActionListener chi hien o day (khong o man hinh)."""
    path = work / "data" / "axiom-office" / "bridge.log"
    try:
        lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        return "(chua co bridge.log)"
    return "\n".join(lines[-count:]) or "(bridge.log rong)"


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
    parser.add_argument("--click", default="", help="toa do man hinh 'X,Y' de bam chuot that sau khi mo dialog")
    parser.add_argument("--click-window", action="append", default=[], metavar="TIEU-DE:X,Y",
                        help="bam vao toa do TRONG cua so roi co tieu de do (chi cua so roi moi co cua so X); "
                             "lap lai duoc nhieu lan de bam lien tiep trong cung mot phien")
    args = parser.parse_args()

    if not os.environ.get("DISPLAY"):
        raise SystemExit("khong co DISPLAY: chay bang `xvfb-run -a python3 tests/live/open_dialogs.py %s`" % args.action)

    work = pathlib.Path(tempfile.mkdtemp(prefix="axiom-dialogs-"))
    stale = kill_cores(probe_prefixes(work))
    if stale:
        print("da tat %d Core mo coi cua lan chay truoc: %s" % (len(stale), stale))
    (work / "profile").mkdir()
    install_extension(work / "profile")
    if args.core:
        print("Agent Core: %s" % stage_core(work, args.core))

    print("thu muc lam viec: %s" % work)
    port = free_port()
    process = launch(work, port, quiet=not args.keep)
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
        if args.click:
            x, y = (int(part) for part in args.click.split(","))
            if not shutil.which("xdotool"):
                raise SystemExit("khong co xdotool: khong bam duoc")
            run(["xdotool", "mousemove", str(x), str(y)])
            run(["xdotool", "click", "1"])
            time.sleep(1.5)
            print("anh sau khi bam (%d,%d): %s" % (x, y, shoot(pathlib.Path(args.out), args.action + "-sau")))
        for index, spec in enumerate(args.click_window, start=1):
            title, _, where = spec.rpartition(":")
            x, y = (int(part) for part in where.split(","))
            clicked = click_in_window(title, x, y, DIALOG_SIZES[args.action])
            time.sleep(1.5)
            print("%s '%s' tai %d,%d -> %s - anh: %s"
                  % ("bam trong" if clicked else "KHONG thay cua so", title, x, y, window_state(title),
                     shoot(pathlib.Path(args.out), "%s-sau%d" % (args.action, index))))
        if args.keep and sys.stdin.isatty():
            input("Enter de tat LibreOffice…")
        print("--- bridge.log (cuoi) ---\n%s" % log_tail(work))
    finally:
        if args.keep:
            print("--keep: LibreOffice van dang chay, log o %s (tat bang tay)" % (work / "soffice.log"))
            print("        log cua extension: %s" % (work / "data" / "axiom-office" / "bridge.log"))
        else:
            process.terminate()
            try:
                process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                process.kill()
            shutil.rmtree(work, ignore_errors=True)
        if not args.keep:
            kill_cores([str(work / "data" / "axiom-office" / "core")])   # Core khong chet theo LibreOffice
    return 0


if __name__ == "__main__":
    sys.exit(main())
