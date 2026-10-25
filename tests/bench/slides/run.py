"""Chay bo brief slide qua Axiom Core tren PowerPoint THAT va ghi du lieu de score.py cham.

    python tests/bench/slides/run.py --core-port 47840 --bridge-port 47833 \
        --label baseline-mimo [--only smoke|s01-bia,s05-...] [--repeat 3] [--skip-done] [--restart-every 6]

Can san: PowerPoint dang mo voi add-in Axiom (bridge len, lay port/pid tu GET /health cua bridge), Core chay va
da cau hinh model. Moi lan chay: tao bai moi (wpp.newPresentation) -> chay lenh bridge cua brief.seed neu co
(slide loi cai san, de do luot sua) -> POST /v1/runs (interactive, runner tu bam dong y moi xac nhan) -> doc
SSE den khi ket thuc -> wpp.checkLayout + wpp.listSlides + luu .pptx, .pdf, slide-NN.png de xem lai bang mat. Bai da
luu van mo (dong hai bai cung luc qua COM lam PowerPoint 2019 crash 0xc0000409); thay vao do cu --restart-every lan
chay thi thoat PowerPoint (bo thay doi chua luu) va mo lai - 25/10/2026 ~50 bai mo lam may 4 GB het RAM.
Trong luc chay giu may khong ngu (SetThreadExecutionState): may ngu giua luot lam mat request model va
session cua bridge qua han - ca bo chay hong ma khong phai loi cua san pham.

Ghi vao <out>/<label>/<brief>/<lan>/: brief.json, run.json, check.json, slides.json, events.log, deck.pptx, deck.pdf,
slide-NN.png. --skip-done bo qua lan chay da co run.json (chay tiep bo do bi ngat).
"""
from __future__ import annotations

import argparse
import base64
import ctypes
import json
import os
import struct
import subprocess
import sys
import time
import urllib.request
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))

from run_prompt import TERMINAL, post  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def registry_token() -> str:
    """Token chung cua bridge va Core (install.ps1 ghi vao HKCU) - khong phai truyen tren dong lenh."""
    try:
        import winreg
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\AxiomOffice") as key:
            return str(winreg.QueryValueEx(key, "Token")[0])
    except (ImportError, OSError):
        return ""


def bridge(port: int, token: str, action: str, params: dict | None = None) -> dict:
    payload = post("http://127.0.0.1:%d/cmd" % port, {"action": action, "params": params or {}}, token)
    if not payload.get("ok"):
        raise RuntimeError("%s: %s" % (action, payload.get("error")))
    return payload.get("result") or {}


def bridge_pid(port: int) -> int:
    with urllib.request.urlopen("http://127.0.0.1:%d/health" % port, timeout=10) as response:
        return int(json.loads(response.read().decode("utf-8"))["result"]["pid"])


# Thoat PowerPoint (moi bai da luu; bai do dang thi bo) roi mo lai. Chi dung tren may do rieng.
# COM khong lay duoc (GetActiveObject: MK_E_UNAVAILABLE, gap 25/10/2026) thi dong process: moi bai benchmark da luu.
RESTART_PS = r"""
$id = (Get-Process POWERPNT -ErrorAction SilentlyContinue | Select-Object -First 1).Id
try {
  $pp = [Runtime.InteropServices.Marshal]::GetActiveObject('PowerPoint.Application')
  foreach ($p in @($pp.Presentations)) { $p.Saved = -1 }
  $pp.Quit(); [Runtime.InteropServices.Marshal]::ReleaseComObject($pp) | Out-Null
} catch {}
if ($id) { Wait-Process -Id $id -Timeout 60 -ErrorAction SilentlyContinue; Stop-Process -Id $id -Force -ErrorAction SilentlyContinue }
Start-Sleep 2
Start-Process 'C:\Program Files\Microsoft Office\Root\Office16\POWERPNT.EXE'
"""


def restart_powerpoint(port: int) -> int:
    subprocess.run(["powershell", "-NoProfile", "-Command", RESTART_PS], check=False, timeout=180)
    for _ in range(60):
        time.sleep(2)
        try:
            return bridge_pid(port)
        except OSError:
            continue
    raise RuntimeError("bridge khong len lai sau khi mo lai PowerPoint")


def placeholder_png(path: str, width: int = 640, height: int = 400) -> str:
    """Anh xam trung tinh cho brief can anh (khong kem anh nhi phan vao repo)."""
    row = b"\x00" + bytes([200, 205, 212]) * width
    raw = zlib.compress(row * height)

    def chunk(kind: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)

    with open(path, "wb") as handle:
        handle.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
                     + chunk(b"IDAT", raw) + chunk(b"IEND", b""))
    return path


def stream(core: str, run_id: str, token: str, log_path: str) -> dict:
    """Doc SSE cua luot; tra ve so do gon cho run.json."""
    request = urllib.request.Request(core + "/v1/runs/" + run_id + "/events")
    if token:
        request.add_header("X-Auth-Token", token)
    started = time.time()
    metrics = {"outcome": "unknown", "toolCalls": 0, "failures": 0, "byTool": {}, "confirmations": []}
    event = None
    with open(log_path, "w", encoding="utf-8") as log, urllib.request.urlopen(request, timeout=60 * 60) as response:
        for raw in response:
            line = raw.decode("utf-8").rstrip("\r\n")
            log.write(line + "\n")
            if line.startswith("event:"):
                event = line[6:].strip()
                continue
            if not line.startswith("data:"):
                continue
            try:
                data = json.loads(line[5:].strip()).get("data") or {}
            except json.JSONDecodeError:
                continue
            if event == "confirm.required":
                # Nhu nguoi dung bam "Co" / chon phuong an dau: interactive=false thi Core TU CHOI moi lenh can xac
                # nhan (xoa slide, xoa hinh), agent khong sua duoc loi cua chinh no - do sai chat luong (luot 25/10/2026).
                answer = {"confirmationId": data.get("confirmationId"), "approved": True}
                if data.get("choices"):
                    answer = {"confirmationId": data.get("confirmationId"), "choice": data["choices"][0].get("id")}
                metrics["confirmations"].append(str(data.get("action") or "?"))
                post(core + "/v1/runs/" + run_id + "/confirm", answer, token)
            elif event == "tool.finished":
                action = str(data.get("action") or data.get("tool") or "?")
                metrics["toolCalls"] += 1
                metrics["byTool"][action] = metrics["byTool"].get(action, 0) + 1
                metrics["failures"] += 0 if data.get("ok") else 1
            elif event == "run.started":
                metrics["model"] = data.get("model")
            elif event in TERMINAL:
                cached = int(data.get("cachedTokens") or 0)
                metrics.update({
                    "outcome": event, "rounds": data.get("rounds"), "verified": data.get("verified"),
                    "inputTokens": data.get("inputTokens"), "outputTokens": data.get("outputTokens"),
                    "billable": max(int(data.get("inputTokens") or 0) - cached, 0) + int(data.get("outputTokens") or 0),
                    "reply": data.get("reply") or data.get("error"),
                })
                break
    metrics["seconds"] = round(time.time() - started, 1)
    return metrics


def keep_awake(on: bool) -> None:
    """Chan Windows ngu (ca connected standby) trong luc chay; khong doi cau hinh nguon cua may."""
    if os.name != "nt":
        return
    ES_CONTINUOUS, ES_SYSTEM_REQUIRED = 0x80000000, 0x00000001
    ctypes.windll.kernel32.SetThreadExecutionState(ES_CONTINUOUS | (ES_SYSTEM_REQUIRED if on else 0))


def run_once(args, brief: dict, folder: str) -> dict:
    os.makedirs(folder, exist_ok=True)
    prompt = brief["prompt"]
    for key, name in (brief.get("fixtures") or {}).items():
        prompt = prompt.replace("{%s}" % key, placeholder_png(os.path.join(folder, os.path.basename(name))))
    created = bridge(args.bridge_port, args.token, "wpp.newPresentation")
    name = created.get("presentation") or created.get("name") or ""
    # Bai co loi cai san (brief.seed): chay cac lenh bridge tren bai vua tao TRUOC khi chay luot. De do luot sua
    # loi CO SAN trong tai lieu - model phai doc de thay so sai (khong co trong prompt) roi sua dung cho.
    for item in brief.get("seed") or []:
        params = dict(item.get("params") or {})
        params.setdefault("presentation", name)
        bridge(args.bridge_port, args.token, item["action"], params)
    core = "http://127.0.0.1:%d" % args.core_port
    started = post(core + "/v1/runs", {
        "prompt": prompt,
        "office": {"port": args.bridge_port, "pid": bridge_pid(args.bridge_port), "app": "wpp", "family": "office"},
        "document": {"name": name},
        "options": {"interactive": True, "maxRounds": args.max_rounds, "maxTokens": args.max_tokens,
                    "maxSeconds": args.max_seconds},
    }, args.token)
    if not started.get("ok"):
        raise RuntimeError("khong tao duoc luot: %s" % started)
    metrics = stream(core, started["result"]["runId"], args.token, os.path.join(folder, "events.log"))
    metrics["label"], metrics["prompt"] = args.label, prompt
    target = {"presentation": name} if name else {}
    outputs = {
        "check.json": bridge(args.bridge_port, args.token, "wpp.checkLayout", target),
        "slides.json": bridge(args.bridge_port, args.token, "wpp.listSlides", dict(target, detail=True)),
        "brief.json": brief,
        "run.json": metrics,
    }
    for file, data in outputs.items():
        with open(os.path.join(folder, file), "w", encoding="utf-8") as handle:
            json.dump(data, handle, ensure_ascii=False, indent=2)
    # Bai 0 slide: PowerPoint hien hop thoai loi khi luu/xuat - hop thoai modal chan moi lenh COM sau do
    # (luot 25/10/2026: ca bo chay treo cho den khi bam OK).
    count = int(outputs["slides.json"].get("slideCount") or 0)
    if count:
        # Anh tung slide de nguoi xem lai (khong cham bang LLM - docs/plans/17 muc 5). Bridge cu khong co
        # wpp.renderSlide thi bo qua, PDF van du.
        for index in range(1, count + 1):
            try:
                shot = bridge(args.bridge_port, args.token, "wpp.renderSlide", dict(target, slide=index))
            except RuntimeError:
                break
            with open(os.path.join(folder, "slide-%02d.png" % index), "wb") as handle:
                handle.write(base64.b64decode(shot.get("base64") or ""))
        bridge(args.bridge_port, args.token, "wpp.exportPdf", dict(target, path=os.path.join(folder, "deck.pdf")))
        bridge(args.bridge_port, args.token, "wpp.saveAs", dict(target, path=os.path.join(folder, "deck.pptx")))
    return metrics


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--core-port", type=int, required=True)
    parser.add_argument("--bridge-port", type=int, required=True)
    parser.add_argument("--skip-done", action="store_true", help="bo qua lan chay da co run.json")
    parser.add_argument("--restart-every", type=int, default=6, help="thoat va mo lai PowerPoint sau moi N lan chay (0 = khong)")
    parser.add_argument("--token", default="", help="mac dinh: token trong HKCU Software/AxiomOffice")
    parser.add_argument("--label", required=True, help="ten lan do, vd baseline-mimo-2026-10-25")
    parser.add_argument("--only", default="", help="'smoke' hoac danh sach id cach nhau dau phay")
    parser.add_argument("--repeat", type=int, default=3)
    parser.add_argument("--max-rounds", type=int, default=60)
    parser.add_argument("--max-tokens", type=int, default=500000)
    parser.add_argument("--max-seconds", type=int, default=900,
                        help="tran thoi gian moi luot (cat ca request model dang treo)")
    parser.add_argument("--out", default=os.path.join(HERE, "results"))
    parser.add_argument("--allow-memory", action="store_true",
                        help="cho chay khi Core bat bo nho agent (cac lan lap khong con doc lap)")
    args = parser.parse_args()
    args.token = args.token or registry_token()

    # Bo nho agent bat thi lan 2, 3 nho lan 1 ("I have memory of this exact task"): tat truoc khi do (docs/plans/18-s0 9.3).
    with urllib.request.urlopen("http://127.0.0.1:%d/health" % args.core_port, timeout=10) as response:
        memory = json.loads(response.read().decode("utf-8")).get("result", {}).get("memory")
    if memory == "on" and not args.allow_memory:
        print("Core dang bat bo nho agent: khoi dong Core voi AXIOM_MEMORY_ENABLED=false (hoac --allow-memory)", file=sys.stderr)
        return 2

    with open(os.path.join(HERE, "briefs.json"), encoding="utf-8") as handle:
        briefs = json.load(handle)["briefs"]
    if args.only == "smoke":
        briefs = [b for b in briefs if b.get("smoke")]
    elif args.only:
        wanted = set(args.only.split(","))
        briefs = [b for b in briefs if b["id"] in wanted]
    keep_awake(True)
    done = 0
    for brief in briefs:
        for n in range(1, args.repeat + 1):
            folder = os.path.join(args.out, args.label, brief["id"], str(n))
            if args.skip_done and os.path.exists(os.path.join(folder, "run.json")):
                continue
            if args.restart_every and done and done % args.restart_every == 0:
                print("-- mo lai PowerPoint (pid moi %d)" % restart_powerpoint(args.bridge_port), flush=True)
            done += 1
            print("== %s lan %d" % (brief["id"], n), flush=True)
            try:
                metrics = run_once(args, brief, folder)
                print("   %s rounds=%s tools=%s billable=%s" % (metrics["outcome"], metrics.get("rounds"),
                                                                metrics["toolCalls"], metrics.get("billable")), flush=True)
            except Exception as exc:  # noqa: BLE001 - mot brief loi khong dung ca bo
                print("   LOI: %s: %s" % (type(exc).__name__, exc), flush=True)
    keep_awake(False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
