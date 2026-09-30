"""Test wizard thiet lap tren LibreOffice that (axiom/setupwizard.py + setup.py), co Agent Core that.

Chay rieng (khong nam trong test_live_libreoffice.py) vi wizard la DIALOG THAT:
  - can cua so LibreOffice: `--headless` khong mo duoc (tren may khong co phien do hoa: dung xvfb-run);
  - tu no cai extension vao MOT PROFILE RIENG va khoi dong LLM gia (tests/core/fake_llm.py), nen khong dung
    vao LibreOffice/cau hinh dang dung cua nguoi dung va khong can khoa API that.

    python3 tests/live/test_setup_wizard.py [--keep]
    xvfb-run -a python3 tests/live/test_setup_wizard.py        # CI

Kiem tra: mo wizard -> doc trang thai tu Core -> nhay buoc -> nap danh sach model tu may chu ->
thu ket noi (thanh cong + that bai, cau tieng Viet) -> ket qua thu cu het hieu luc khi doi gia tri
(go tay hoac chon trong danh sach) -> mo dialog Cai dat nang cao (cua so roi) -> luu cau hinh ->
bam X tren thanh tieu de -> dong.
"""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from test_live_commands import RESULTS, check  # noqa: E402  (dung chung cach in PASS/FAIL + bang ket qua)

FAKE_LLM = os.path.join(ROOT, "tests", "core", "fake_llm.py")
PORT = 47851
IS_WINDOWS = sys.platform.startswith("win")
TITLE = "Thiết lập Axiom Office"        # tieu de cua so wizard (setupwizard.open_setup)
ADVANCED_TITLE = "Cài đặt"              # tieu de dialog mo tu "Tuy chon nang cao" (dialogs.open_settings)

def config_path() -> str:
    base = os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config")
    if IS_WINDOWS:      # Windows: cau hinh o HKCU, wizard cua extension LibreOffice van dung config.json
        base = os.path.join(os.environ.get("LOCALAPPDATA", os.path.expanduser("~")), "AxiomOffice")
    return os.path.join(base, "axiom-office", "config.json")

def read_config() -> dict:
    try:
        with open(config_path(), encoding="utf-8") as handle:
            return json.load(handle)
    except (OSError, ValueError):
        return {}

def write_config(data: dict) -> None:
    path = config_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(data, handle, indent=2, ensure_ascii=False)

def health(port: int = PORT) -> dict:
    try:
        with urllib.request.urlopen("http://127.0.0.1:%d/health" % port, timeout=2) as response:
            return json.loads(response.read().decode("utf-8")).get("result") or {}
    except Exception:  # noqa: BLE001
        return {}

def call(params: dict, timeout: int = 60) -> dict:
    """Goi ui.setup qua bridge; tra ve `result` (rong neu loi/khong co)."""
    body = json.dumps({"action": "ui.setup", "params": params}).encode("utf-8")
    request = urllib.request.Request("http://127.0.0.1:%d/cmd" % PORT, data=body, method="POST")
    request.add_header("Content-Type", "application/json")
    request.add_header("X-Auth-Token", read_config().get("Token", ""))
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        payload = json.loads(exc.read().decode("utf-8") or "{}")
    except OSError:
        return {}
    result = payload.get("result")
    return result if isinstance(result, dict) else {}

def wait(ready, seconds: float = 30.0) -> dict:
    """Thao tac nang cua wizard chay o thread nen -> goi lai voi params rong de doc ket qua."""
    deadline = time.time() + seconds
    state: dict = {}
    while time.time() < deadline:
        state = call({})
        if ready(state):
            return state
        time.sleep(0.5)
    return state

def free_port() -> int:
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        return probe.getsockname()[1]

def start_fake_llm(work: str) -> tuple:
    port = free_port()
    script = os.path.join(work, "wizard-script.json")
    with open(script, "w", encoding="utf-8") as handle:
        json.dump([{"text": "OK"}], handle)
    process = subprocess.Popen(
        [sys.executable, FAKE_LLM, "--port", str(port), "--script", script, "--models", "fake-model,fake-model-2"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    deadline = time.time() + 15
    while time.time() < deadline:
        try:
            with urllib.request.urlopen("http://127.0.0.1:%d/__state" % port, timeout=1):
                return process, port
        except Exception:  # noqa: BLE001
            time.sleep(0.2)
    process.kill()
    raise SystemExit("fake LLM khong khoi dong duoc")

def dismiss_first_run() -> None:
    """Dong hop thoai lan dau (Tip of the Day) neu co: modal se chan viec tra ket qua ve main thread."""
    if IS_WINDOWS or not shutil.which("xdotool"):
        return
    for _ in range(5):
        windows = subprocess.run(["xdotool", "search", "--name", "Tip of the Day"], capture_output=True, text=True).stdout.split()
        if not windows:
            return
        subprocess.run(["xdotool", "windowactivate", "--sync", windows[0]], capture_output=True)
        subprocess.run(["xdotool", "key", "--window", windows[0], "Escape"], capture_output=True)
        time.sleep(1.0)

def search_windows(name: str) -> list:
    """Id cua so tren X co tieu de chua `name` (rong tren Windows hoac khi khong co xdotool)."""
    if IS_WINDOWS or not shutil.which("xdotool"):
        return []
    found = subprocess.run(["xdotool", "search", "--name", name], capture_output=True, text=True)
    return found.stdout.split()

def close_window(name: str) -> bool:
    """Bam X tren thanh tieu de (gui WM_DELETE_WINDOW). True neu tim thay cua so de dong."""
    windows = search_windows(name)
    if not windows:
        return False
    subprocess.run(["xdotool", "windowclose", windows[0]], capture_output=True)
    return True

def click_in_window(name: str, x: int, y: int, model: tuple = (640, 524)) -> bool:
    """Bam chuot THAT vao (x, y) tinh theo model cua wizard (640x524), khong qua `ui.setup`.

    Can thiet cho loi "nut bam duoc nhung cu bam bi dieu khien khac nuot": chi chuot that moi lo ra.
    Cua so co the co vien/thanh tieu de (may co trinh quan ly cua so) -> tru phan lech do.
    Tra ve False neu khong tim thay cua so hoac khong co xdotool (Windows).
    """
    windows = search_windows(name)
    if not windows or not shutil.which("xdotool"):
        return False
    win = windows[0]
    geometry = subprocess.run(["xdotool", "getwindowgeometry", "--shell", win],
                              capture_output=True, text=True).stdout
    box = dict(line.split("=") for line in geometry.strip().splitlines() if "=" in line)
    border = max(0, (int(box.get("WIDTH", model[0])) - model[0]) // 2)
    offset_y = max(0, int(box.get("HEIGHT", model[1])) - model[1] - border)
    subprocess.run(["xdotool", "windowraise", win], capture_output=True)
    subprocess.run(["xdotool", "windowfocus", win], capture_output=True)
    time.sleep(0.5)
    subprocess.run(["xdotool", "mousemove", "--window", win, str(x + border), str(y + offset_y)], capture_output=True)
    time.sleep(0.3)
    subprocess.run(["xdotool", "click", "1"], capture_output=True)
    return True

def wait_until(ready, seconds: float = 15.0) -> bool:
    deadline = time.time() + seconds
    while time.time() < deadline:
        if ready():
            return True
        time.sleep(0.5)
    return ready()

def launch_fresh(work: str) -> subprocess.Popen:
    """LibreOffice voi PROFILE RIENG da cai extension: khong dung app/cau hinh dang dung cua nguoi dung."""
    oxt = subprocess.run([sys.executable, os.path.join(ROOT, "scripts", "package_oxt.py"), "--print-path"],
                         capture_output=True, text=True, check=True).stdout.strip()
    profile = os.path.join(work, "profile")
    os.makedirs(profile, exist_ok=True)
    uri = pathlib.Path(profile).as_uri()
    unopkg = shutil.which("unopkg") or ("/usr/lib/libreoffice/program/unopkg" if not IS_WINDOWS else "unopkg")
    added = subprocess.run([unopkg, "-env:UserInstallation=" + uri, "add", "--force", oxt],
                           capture_output=True, text=True)
    if added.returncode != 0:
        raise SystemExit("khong cai duoc extension vao profile tam: %s" % (added.stderr or added.stdout)[:300])
    soffice = shutil.which("soffice") or shutil.which("soffice.exe") or "/usr/lib/libreoffice/program/soffice"
    process = subprocess.Popen([soffice, "-env:UserInstallation=" + uri, "--writer", "--norestore", "--nologo",
                                "--nofirststartwizard"])
    deadline = time.time() + 90
    while time.time() < deadline:
        if health():
            dismiss_first_run()
            return process
        if process.poll() is not None:
            raise SystemExit("LibreOffice thoat som (exit %s)" % process.returncode)
        time.sleep(1.0)
    process.terminate()
    raise SystemExit("bridge LibreOffice khong len (extension trong profile tam?)")

def stop(process: subprocess.Popen) -> None:
    try:
        process.terminate()
        process.wait(timeout=20)
    except Exception:  # noqa: BLE001 - khong tat duoc thi thoi
        try:
            process.kill()
        except Exception:  # noqa: BLE001
            pass

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--keep", action="store_true", help="khong tat LibreOffice sau khi chay")
    args = parser.parse_args()

    if health():
        raise SystemExit("port %d da co bridge LibreOffice dang mo - dong LibreOffice roi chay lai" % PORT)

    work = tempfile.mkdtemp(prefix="axiom-wizard-")
    before = read_config()
    process = None
    fake = None
    try:
        process = launch_fresh(work)
        print("== LibreOffice profile tam: %s" % work)

        # Doi "kiem tra may" (chay nen luc mo) xong: thao tac bam trong luc wizard ban se bi bo qua.
        state = wait(lambda item: item.get("step") in ("welcome", "checks") and not item.get("busy"))
        # May da thiet lap (co LlmEndpoint + LlmModel) thi wizard vao thang man kiem tra (sua loi).
        check(state.get("step") in ("welcome", "checks"), "wizard mo duoc", json.dumps(state, ensure_ascii=False)[:200])
        check(state.get("coreError") == "", "wizard doc duoc trang thai tu Agent Core", state.get("coreError"))
        # Nut "Quay lai" phai bam duoc bang chuot that: nhan trang thai o hang nut tung de len no (rong hon)
        # nen nuot het cu bam, va `ui.setup` thi khong bao gio lo ra loi nay.
        check(call({"step": "checks"}).get("step") == "checks", "wizard ve lai buoc kiem tra may", "")
        # Dong trang thai nam trong vung noi dung, duoi danh sach kiem tra: kiem tra xong thi no phai co cau KET
        # LUAN (khong phai cau tien do "Đang xử lý…" cua mot thao tac nao khac, va khong duoc de trong).
        state = wait(lambda item: item.get("step") == "checks" and not item.get("busy"), 20)
        verdict = state.get("statusLine") or ""
        check(bool(verdict) and not verdict.startswith("Đang "),
              "buoc kiem tra may: dong trang thai la cau ket luan", json.dumps(verdict, ensure_ascii=False))
        if click_in_window(TITLE, 71, 485):        # giua nut "Quay lai": PAD + BACK_W/2, y 470..500 (setupwizard)
            state = wait(lambda item: item.get("step") == "welcome", 10)
            check(state.get("step") == "welcome", "bam chuot vao nut \"Quay lai\" -> lui ve buoc chao mung",
                  state.get("step"))

        check(call({"step": "connect"}).get("step") == "connect", "wizard nhay sang buoc ket noi", "")
        # Buoc ket noi co dong ket qua rieng (test_line) o dung cho do -> dong trang thai de trong, khong chen
        # them mot cau khac vao cung mot cho.
        line = call({}).get("statusLine")
        check(line == "", "buoc ket noi: khong dung them dong trang thai", json.dumps(line, ensure_ascii=False))

        fake, port = start_fake_llm(work)
        call({"endpoint": "http://127.0.0.1:%d/v1" % port, "provider": "company", "model": ""})
        started = call({"models": True})
        check(started.get("loadingModels") is True or started.get("busy") is True,
              "wizard nap danh sach model chay nen (tra ve ngay)", json.dumps(started, ensure_ascii=False)[:200])
        state = wait(lambda item: not item.get("busy") and not item.get("loadingModels"))
        check(state.get("models") == ["fake-model", "fake-model-2"], "wizard doc danh sach model tu may chu",
              json.dumps(state.get("models"), ensure_ascii=False))
        check(state.get("model") == "fake-model", "wizard tu chon model dau tien cho nguoi dung", state.get("model"))

        call({"test": True})
        state = wait(lambda item: not item.get("busy") and item.get("testKind"))
        check(state.get("testKind") == "ok" and "Kết nối tốt" in (state.get("testLine") or ""),
              "wizard thu ket noi voi may chu that -> bao ket noi tot", state.get("testLine"))
        check(state.get("nextLabel") == "Tiếp tục →",
              "thu ket noi tot -> nut chinh doi thanh \"Tiep tuc\"", state.get("nextLabel"))

        # Chon model KHAC trong danh sach: ket qua thu cu (do model cu) phai het hieu luc - neu khong, nguoi
        # dung bam "Tiep tuc" la luu luon model chua he duoc thu.
        picked = call({"modelIndex": 1})
        check(picked.get("model") == "fake-model-2" and picked.get("testKind") == ""
              and picked.get("nextLabel") == "Kiểm tra kết nối",
              "chon model khac trong danh sach -> ket qua thu cu het hieu luc",
              json.dumps({key: picked.get(key) for key in ("model", "testKind", "nextLabel", "selectedPos")},
                         ensure_ascii=False))
        call({"modelIndex": 0})        # tra ve model dau cho cac buoc sau

        # Go tay ten model khac: chu trong o thang danh sach (bo muc dang chon), nen lan sau bam "Tiep tuc"
        # khong bi danh sach ghi de nguoc lai.
        typed = call({"model": "model-tu-go"})
        check(typed.get("model") == "model-tu-go" and typed.get("selectedPos") == -1
              and typed.get("nextLabel") == "Kiểm tra kết nối",
              "go tay ten model -> bo muc dang chon trong danh sach",
              json.dumps({key: typed.get(key) for key in ("model", "selectedPos", "nextLabel")}, ensure_ascii=False))
        call({"model": "fake-model"})  # tra ve model cu cho cac buoc sau

        # Doi dia chi sau khi da thu tot: ket qua thu cu het hieu luc, phai thu lai truoc khi cho di tiep
        # (neu khong, nguoi dung bam "Tiep tuc" la luu luon dia chi chua he duoc thu).
        changed = call({"endpoint": "http://127.0.0.1:59999/v1"})
        check(changed.get("testKind") == "" and changed.get("nextLabel") == "Kiểm tra kết nối",
              "doi dia chi -> ket qua thu cu het hieu luc",
              json.dumps({key: changed.get(key) for key in ("testKind", "nextLabel")}, ensure_ascii=False))

        call({"test": True})
        state = wait(lambda item: not item.get("busy") and item.get("testKind"))
        line = state.get("testLine") or ""
        check(state.get("testKind") == "error" and "Không kết nối được" in line and "Exception" not in line,
              "wizard bao loi bang cau tieng Viet de hieu (khong lo chi tiet ky thuat)", line)

        saved_state = call({"endpoint": "http://127.0.0.1:%d/v1" % port, "model": "fake-model",
                            "features": {"MemoryEnabled": False, "VisualQaEnabled": True}, "save": True})
        after = read_config()
        check(after.get("LlmEndpoint") == "http://127.0.0.1:%d/v1" % port and after.get("LlmModel") == "fake-model",
              "wizard luu dia chi + model vao cau hinh", json.dumps(after, ensure_ascii=False)[:200])
        check(str(after.get("MemoryEnabled")) in ("0", "False") and str(after.get("VisualQaEnabled")) in ("1", "True"),
              "wizard luu co tinh nang", {key: after.get(key) for key in ("MemoryEnabled", "VisualQaEnabled")})
        check(saved_state.get("step") in ("connect", "features", "done"), "wizard o buoc hop le sau khi luu",
              saved_state.get("step"))
        check(call({"close": True}).get("closed") is True, "wizard dong duoc", "")

        # Dialog "Cai dat nang cao" mo TU wizard phai la cua so ROI (floating): cua so con nam duoi wizard nen
        # bi che, nen no phai hien thanh cua so rieng tren X moi bam duoc.
        call({})                       # wizard da dong -> open_setup dung cua so moi
        call({"advanced": True})
        check(wait_until(lambda: bool(search_windows(ADVANCED_TITLE))),
              "mo \"Tuy chon nang cao\" tu wizard -> dialog Cai dat hien thanh cua so rieng",
              json.dumps(search_windows(ADVANCED_TITLE)))
        close_window(ADVANCED_TITLE)

        # Bam X tren thanh tieu de: cua so tao bang toolkit KHONG tu dong dong, phai co TopWindowListener
        # (awt.Dialog.on_close) moi tat duoc - va phai don khoi CURRENT de lan sau mo lai ra wizard moi.
        check(bool(search_windows(TITLE)), "wizard la cua so that tren X (tim duoc theo tieu de)", "")
        closed = close_window(TITLE)
        check(closed and wait_until(lambda: not search_windows(TITLE)),
              "bam X tren thanh tieu de dong han wizard", json.dumps(search_windows(TITLE)))
        reopened = call({})
        check(reopened.get("step") in ("welcome", "checks"),
              "mo lai sau khi bam X -> wizard MOI (trang thai cu da duoc don)", reopened.get("step"))
    finally:
        if fake is not None:
            fake.terminate()
            try:
                fake.wait(timeout=5)
            except subprocess.TimeoutExpired:
                fake.kill()
        write_config(before)           # tra lai cau hinh cu (test khong duoc doi cau hinh nguoi dung)
        if process is not None and not args.keep:
            stop(process)
        shutil.rmtree(work, ignore_errors=True)

    failed = [name for ok, name, _ in RESULTS if not ok]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0

if __name__ == "__main__":
    sys.exit(main())
