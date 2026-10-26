"""Chay bo do Word/Excel (briefs.json) qua Axiom Core tren Office THAT, ghi du lieu de score.py cham.

    python tests/bench/office/run.py --app word|excel --label baseline-beam-free [--only w01-...,w02-...] [--repeat 1]

Can san: Core chay (khong bat bo nho agent - nhu bo slide), model da cau hinh. Tu mo Word/Excel (bridge 47831/47832)
neu chua mo. Moi lan chay: tao tai lieu moi -> chay brief.seed (tai lieu co san) -> tung prompt la mot luot
POST /v1/runs (interactive, runner TU BAM DONG Y nhu nguoi dung - ai.ask khong tuong tac thi Core tu choi moi lenh can
xac nhan, ke ca ghi de o chinh luot vua ghi: luot b-excel 26/10/2026 dung sau 57 vong) -> luu result.docx/.xlsx ->
dong tai lieu. Ghi vao <out>/<label>/<brief>/<lan>/: brief.json, run.json (tong cac luot), events-N.log, result.*.
Cham sau (score.py) - bo cham mo Word/Excel rieng de do phan trang / tinh lai, khong chay cung luc voi bo do.
"""
from __future__ import annotations

import argparse
import datetime
import importlib.util
import json
import os
import random
import sys
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(HERE)))
sys.path.insert(0, os.path.dirname(HERE))
sys.path.insert(0, os.path.join(ROOT, "tests", "live"))

# Dung lai ham cua bo slide (bridge, SSE, tu bam xac nhan). Nap duoi ten khac: hai tep cung ten run.py.
_spec = importlib.util.spec_from_file_location("slides_run", os.path.join(os.path.dirname(HERE), "slides", "run.py"))
slides_run = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(slides_run)
bridge, bridge_pid, keep_awake = slides_run.bridge, slides_run.bridge_pid, slides_run.keep_awake
post, registry_token, stream = slides_run.post, slides_run.registry_token, slides_run.stream
import test_live_commands as live  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

APPS = {
    "word": {"port": 47831, "kind": "wps", "new": "writer.newDocument", "save": "writer.saveAs", "file": "result.docx",
             "target": "document"},
    "excel": {"port": 47832, "kind": "et", "new": "et.newWorkbook", "save": "et.saveAs", "file": "result.xlsx",
              "target": "workbook"},
}


def generate(name: str) -> list:
    """Du lieu sinh cho seed ('@gen:<ten>'): co dinh theo hat giong de moi lan chay giong nhau."""
    rng = random.Random(name)
    if name == "sales40":
        rows = [["Ngày", "Khu vực", "Sản phẩm", "Số lượng", "Doanh thu"]]
        start = datetime.date(2026, 1, 5)
        for i in range(40):
            qty = rng.randint(5, 60)
            rows.append([(start + datetime.timedelta(days=i * 3)).strftime("%d/%m/%Y"), rng.choice(["Bắc", "Trung", "Nam"]),
                         rng.choice(["Máy in", "Màn hình", "Bàn phím"]), qty, qty * rng.choice([1200000, 3500000, 450000])])
        return rows
    if name == "staff200":
        rows = [["Mã NV", "Họ tên", "Phòng", "Ngày vào làm", "Lương"]]
        last = ["Nguyễn", "Trần", "Lê", "Phạm", "Hoàng", "Vũ", "Đặng", "Bùi"]
        first = ["An", "Bình", "Chi", "Dũng", "Hà", "Khánh", "Lan", "Minh", "Nam", "Phương", "Quang", "Thảo"]
        for i in range(200):
            day = datetime.date(2015, 1, 1) + datetime.timedelta(days=rng.randint(0, 4000))
            rows.append(["NV%03d" % (i + 1), "%s %s" % (rng.choice(last), rng.choice(first)),
                         rng.choice(["Hành chính", "Kế toán", "Kỹ thuật", "Kinh doanh"]), day.strftime("%d/%m/%Y"),
                         rng.randrange(8000000, 35000000, 500000)])
        return rows
    raise ValueError("khong biet bo du lieu " + name)


def resolve(value):
    """'@gen:ten' -> du lieu sinh; '@gen:ten!B2' -> mot o cua du lieu do (de so sanh mustKeepCells)."""
    if isinstance(value, str) and value.startswith("@gen:"):
        name, _, cell = value[5:].partition("!")
        rows = generate(name)
        if not cell:
            return rows
        column = ord(cell[0]) - ord("A")
        return rows[int(cell[1:]) - 1][column]
    if isinstance(value, dict):
        return {key: resolve(item) for key, item in value.items()}
    if isinstance(value, list):
        return [resolve(item) for item in value]
    return value


def ensure_app(app: str) -> int:
    spec = APPS[app]
    if live.health(spec["port"]):
        return spec["port"]
    port, _ = live.launch(app, False)
    return port


def run_once(args, brief: dict, folder: str) -> dict:
    spec = APPS[brief["app"]]
    os.makedirs(folder, exist_ok=True)
    port = ensure_app(brief["app"])
    created = bridge(port, args.token, spec["new"])
    name = created.get("name") or ""
    for item in brief.get("seed") or []:
        params = resolve(dict(item.get("params") or {}))
        bridge(port, args.token, item["action"], params)
    prompts = list(brief.get("prompts") or [])
    if brief.get("promptFile"):
        with open(os.path.join(ROOT, brief["promptFile"]), encoding="utf-8") as handle:
            prompts = [handle.read()]
    core = "http://127.0.0.1:%d" % args.core_port
    total: dict = {"turns": [], "outcome": "", "rounds": 0, "toolCalls": 0, "failures": 0, "confirmations": [],
                   "inputTokens": 0, "outputTokens": 0, "billable": 0, "seconds": 0.0, "byTool": {}}
    conversation = None
    for turn, prompt in enumerate(prompts, 1):
        body = {
            "prompt": prompt,
            "office": {"port": port, "pid": bridge_pid(port), "app": spec["kind"], "family": "office"},
            "document": {"name": name},
            "options": {"interactive": True, "maxRounds": args.max_rounds, "maxTokens": args.max_tokens,
                        "maxSeconds": int(brief.get("timeout") or args.max_seconds)},
        }
        if conversation:
            body["conversationId"] = conversation
        started = post(core + "/v1/runs", body, args.token)
        if not started.get("ok"):
            raise RuntimeError("khong tao duoc luot: %s" % started)
        conversation = started["result"].get("conversationId") or conversation
        metrics = stream(core, started["result"]["runId"], args.token, os.path.join(folder, "events-%d.log" % turn))
        metrics["runId"] = started["result"]["runId"]
        total["turns"].append(metrics)
        total["outcome"] = metrics["outcome"]
        for key in ("rounds", "toolCalls", "failures", "inputTokens", "outputTokens", "billable"):
            total[key] += int(metrics.get(key) or 0)
        total["seconds"] = round(total["seconds"] + metrics.get("seconds", 0), 1)
        total["confirmations"] += metrics.get("confirmations") or []
        for tool, count in (metrics.get("byTool") or {}).items():
            total["byTool"][tool] = total["byTool"].get(tool, 0) + count
        total["model"] = metrics.get("model")
        if metrics["outcome"] != "run.completed":
            break
    total["label"], total["prompts"] = args.label, prompts
    target = {spec["target"]: name} if name else {}
    bridge(port, args.token, spec["save"], dict(target, path=os.path.join(folder, spec["file"])))
    for file, data in (("brief.json", brief), ("run.json", total)):
        with open(os.path.join(folder, file), "w", encoding="utf-8") as handle:
            json.dump(data, handle, ensure_ascii=False, indent=2)
    # Dong han app sau moi ca (tai lieu da luu): Excel ban Office khong co et.closeAll, so cu con mo thi saveAs
    # "result.xlsx" ca sau bi tu choi vi trung ten; 20 ca mo cung luc cung lam may 4 GB het RAM (bo slide 25/10/2026).
    live.close_app(bridge_pid(port))
    return total


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--core-port", type=int, default=47840)
    parser.add_argument("--app", choices=sorted(APPS), required=True)
    parser.add_argument("--label", required=True)
    parser.add_argument("--only", default="")
    parser.add_argument("--repeat", type=int, default=1)
    parser.add_argument("--skip-done", action="store_true")
    parser.add_argument("--max-rounds", type=int, default=60)
    parser.add_argument("--max-tokens", type=int, default=1500000)
    parser.add_argument("--max-seconds", type=int, default=900)
    parser.add_argument("--token", default="")
    parser.add_argument("--out", default=os.path.join(HERE, "results"))
    parser.add_argument("--allow-memory", action="store_true")
    args = parser.parse_args()
    args.token = args.token or registry_token()
    with urllib.request.urlopen("http://127.0.0.1:%d/health" % args.core_port, timeout=10) as response:
        memory = json.loads(response.read().decode("utf-8")).get("result", {}).get("memory")
    if memory == "on" and not args.allow_memory:
        print("Core dang bat bo nho agent: khoi dong Core voi AXIOM_MEMORY_ENABLED=false (hoac --allow-memory)", file=sys.stderr)
        return 2
    with open(os.path.join(HERE, "briefs.json"), encoding="utf-8") as handle:
        briefs = [b for b in json.load(handle)["briefs"] if b["app"] == args.app]
    if args.only:
        wanted = set(args.only.split(","))
        briefs = [b for b in briefs if b["id"] in wanted]
    keep_awake(True)
    try:
        for brief in briefs:
            for n in range(1, args.repeat + 1):
                folder = os.path.join(args.out, args.label, brief["id"], str(n))
                if args.skip_done and os.path.exists(os.path.join(folder, "run.json")):
                    continue
                print("== %s lan %d" % (brief["id"], n), flush=True)
                try:
                    total = run_once(args, brief, folder)
                    print("   %s rounds=%s tools=%s fail=%s confirm=%d billable=%s %ss" % (
                        total["outcome"], total["rounds"], total["toolCalls"], total["failures"],
                        len(total["confirmations"]), total["billable"], total["seconds"]), flush=True)
                except Exception as exc:  # noqa: BLE001 - mot brief loi khong dung ca bo
                    print("   LOI: %s: %s" % (type(exc).__name__, exc), flush=True)
    finally:
        keep_awake(False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
