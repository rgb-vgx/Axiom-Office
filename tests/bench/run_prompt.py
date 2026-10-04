"""Giao mot de benchmark cho Axiom Core qua dung API ma pane dung, va ghi lai toan bo SSE.

    python tests/bench/run_prompt.py <core_port> <bridge_port> <bridge_pid> <prompt_file> <log_file> [token]

Axiom lam viec tren tai lieu DANG MO, nen phai mo LibreOffice that truoc (khong mo phong bridge):
    "C:\\Program Files\\LibreOffice\\program\\soffice.exe" --calc
roi lay pid + port tu GET /health cua bridge (mac dinh port 47852 cho Calc).

Man hinh in ra tung tool call va tung vong suy luan (neu bat LlmShowReasoning) de theo doi luc chay;
<log_file> giu nguyen van SSE de doi chieu sau nay.
"""
from __future__ import annotations

import json
import sys
import time
import urllib.request
from collections import Counter

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

TERMINAL = ("run.completed", "run.failed", "run.cancelled", "run.timedout", "run.stopped")


def post(url: str, body: dict, token: str) -> dict:
    request = urllib.request.Request(url, data=json.dumps(body, ensure_ascii=False).encode("utf-8"),
                                     method="POST")
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("X-Auth-Token", token)
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.loads(response.read().decode("utf-8"))


def save_document(bridge_port: str, token: str, path: str) -> str:
    """Luu tai lieu dang mo ra file. Phai lam TRUOC khi dong LibreOffice, khong thi ket qua bay mat:
    do tai lieu chua luu la "Untitled 1" nam trong bo nho - dong app la xong."""
    body = {"action": "et.saveAs", "params": {"path": path}}
    request = urllib.request.Request("http://127.0.0.1:%s/cmd" % bridge_port,
                                     data=json.dumps(body, ensure_ascii=False).encode("utf-8"), method="POST")
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("X-Auth-Token", token)
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            payload = json.loads(response.read().decode("utf-8"))
        return "da luu %s" % path if payload.get("ok") else "luu that bai: %s" % payload.get("error")
    except Exception as exc:  # noqa: BLE001
        return "luu that bai: %s: %s" % (type(exc).__name__, exc)


def main() -> int:
    if len(sys.argv) < 6:
        print(__doc__)
        return 2
    core_port, bridge_port, pid, prompt_file, log_path = sys.argv[1:6]
    token = sys.argv[6] if len(sys.argv) > 6 else ""
    # Tuy chon: <file_luu> <max_tokens>.
    #   <file_luu>  - luu tai lieu ra day sau khi luot chay ket thuc.
    #   <max_tokens> - ngan sach token PHAI TRA cho ca luot (mac dinh 1000000; Core chan tren o 1.000.000).
    #
    # MAC DINH 1.000.000, khong phai 400.000 nhu truoc. Ly do da do: bon luot o 400k deu dung vi ngan
    # sach, va luot chay lai Test 4 voi 1.000.000 thi XONG CA BAI (8/8 sheet, 5 chart, verified=true) ma
    # chi ton 375.806 token phai tra - IT HON 404.021 cua luot bi cat o 400k, du lam 3,7 lan so tool call.
    # Kich thuoc phan hoi cua model dao dong rat manh giua cac lan chay, nen mot ngan sach vua khit se cat
    # oan nhung luot le ra da xong. Xem tests/bench/results/con-lai.md muc 6.
    save_path = sys.argv[7] if len(sys.argv) > 7 else ""
    max_tokens = int(sys.argv[8]) if len(sys.argv) > 8 else 1000000
    # <max_rounds>: tran so vong. Mac dinh 300; Core chan tren o 1000.
    max_rounds = int(sys.argv[9]) if len(sys.argv) > 9 else 300
    with open(prompt_file, encoding="utf-8") as handle:
        prompt = handle.read()

    base = "http://127.0.0.1:%s" % core_port
    created = post(base + "/v1/runs", {
        "prompt": prompt,
        "office": {"port": int(bridge_port), "pid": int(pid), "app": "et", "family": "libreoffice"},
        "document": {"name": "Untitled 1"},
        # Ngan sach token ca luot, tinh theo token PHAI TRA THAT (cache khong tinh). Mac dinh 1.000.000
        # (xem chu thich o tren): de cung mot con so cho de so giua cac lan chay.
        # maxRounds 300 (Core chan tren o 1000): maxRounds 100 da la nut that that su o Test 1 (dung sau
        # 315 tool call), nen de bai khong bi cat vi so vong khi dang con tien.
        "options": {"maxRounds": max_rounds, "maxTokens": max_tokens},
    }, token)
    if not created.get("ok"):
        print("KHONG TAO DUOC LUOT CHAY:", created)
        return 1
    run_id = created["result"]["runId"]
    print("run:", run_id, "| conversation:", created["result"]["conversationId"], flush=True)

    request = urllib.request.Request(base + "/v1/runs/" + run_id + "/events")
    if token:
        request.add_header("X-Auth-Token", token)
    started = time.time()
    tools = 0
    # Buoc 0 roadmap hieu nang (02/10/2026): thu thap metric A/B ngay trong harness de moi lan chay
    # deu so duoc ma khong phai mo log SSE ra dem tay. Chi dem tu su kien SSE, khong doi Core/bridge.
    by_tool: Counter[str] = Counter()
    read_keys: list[str] = []       # khoa (action + paramsPreview) cua cac lenh doc de bat doc lap
    failures = 0                    # tong so tool loi
    repeat_fail = 0                 # so loi lap lai Y HET lien tiep (cung action + cung loi)
    repeat_fail_max = 0             # chuoi loi lap lai DAI NHAT trong ca luot
    last_fail_sig = ""
    first_check_ok_at: float | None = None  # time-to-first-valid-slice: checkRange ok dau tien
    # Byte ket qua DA VAO ngu canh model (resultBytes, Core tu 04/10/2026) - binary cu khong co truong nay.
    result_bytes: Counter[str] = Counter()
    result_max = 0
    result_max_tool = ""
    rounds_seen = 0
    with open(log_path, "w", encoding="utf-8") as log:
        log.write("run %s\n" % run_id)
        with urllib.request.urlopen(request, timeout=60 * 60) as response:
            event_type = None
            for raw in response:
                line = raw.decode("utf-8").rstrip("\r\n")
                log.write(line + "\n")
                if line.startswith("event:"):
                    event_type = line[6:].strip()
                    continue
                if not line.startswith("data:"):
                    continue
                try:
                    data = (json.loads(line[5:].strip()).get("data") or {})
                except json.JSONDecodeError:
                    continue
                elapsed = time.time() - started
                if event_type == "tool.finished":
                    tools += 1
                    action = str(data.get("action") or data.get("tool") or "?")
                    by_tool[action] += 1
                    ok = bool(data.get("ok"))
                    err = str(data.get("error") or "")[:90]
                    size = data.get("resultBytes")
                    if isinstance(size, int):
                        result_bytes[action] += size
                        if size > result_max:
                            result_max, result_max_tool = size, action
                    if action in ("et.readRange", "writer.getText", "wpp.listSlides"):
                        read_keys.append("%s|%s" % (action, data.get("paramsPreview")))
                    if not ok:
                        failures += 1
                        sig = "%s|%s" % (action, err)
                        repeat_fail = repeat_fail + 1 if sig == last_fail_sig else 1
                        repeat_fail_max = max(repeat_fail_max, repeat_fail)
                        last_fail_sig = sig
                    else:
                        last_fail_sig = ""
                        if action == "et.checkRange" and first_check_ok_at is None:
                            first_check_ok_at = elapsed
                    print("[%6.0fs] %3d %s %s %s" % (
                        elapsed, tools, "ok " if ok else "LOI", action, err), flush=True)
                elif event_type == "run.reasoning":
                    # Vong hien tai (phu du: event ket thuc thieu truong rounds - run.failed/cancelled).
                    try:
                        rounds_seen = max(rounds_seen, int(data.get("round") or 0))
                    except (TypeError, ValueError):
                        pass
                    print("[%6.0fs] ~~ vong %s: %s" % (
                        elapsed, data.get("round"), (data.get("text") or "")[:180].replace("\n", " ")), flush=True)
                elif event_type == "run.started":
                    print("[%6.0fs] bat dau (model %s)" % (elapsed, data.get("model")), flush=True)
                elif event_type in TERMINAL:
                    print("[%6.0fs] KET THUC: %s" % (elapsed, event_type), flush=True)
                    if event_type == "run.completed":
                        print("reply:", (data.get("reply") or "")[:600], flush=True)
                    else:
                        print("error:", data.get("error"), flush=True)
                    try:
                        in_tok = int(data.get("inputTokens") or 0)
                    except (TypeError, ValueError):
                        in_tok = 0
                    try:
                        out_tok = int(data.get("outputTokens") or 0)
                    except (TypeError, ValueError):
                        out_tok = 0
                    try:
                        cached = int(data.get("cachedTokens") or 0)
                    except (TypeError, ValueError):
                        cached = 0
                    bill_in = max(in_tok - cached, 0)
                    bill_total = bill_in + out_tok
                    hit = (cached / in_tok) if in_tok > 0 else 0.0
                    # Doc lap: cung mot khoa doc xuat hien > 1 lan (gioi han duoi - paramsPreview bi cat
                    # o 200 ky tu nen hai vung khac nhau van co the trung khoa; dung de soi, khong de ket an).
                    dup = 0
                    if read_keys:
                        seen: set[str] = set()
                        for key in read_keys:
                            if key in seen:
                                dup += 1
                            else:
                                seen.add(key)
                    dup_ratio = (dup / len(read_keys)) if read_keys else 0.0
                    print("rounds=%s toolCalls=%s in=%s out=%s cached=%s verified=%s" % (
                        data.get("rounds"), tools, data.get("inputTokens"), data.get("outputTokens"),
                        data.get("cachedTokens"), data.get("verified")), flush=True)
                    print("METRIC billable_in=%d billable_out=%d billable=%d cache_hit=%.3f" % (
                        bill_in, out_tok, bill_total, hit), flush=True)
                    print("METRIC by_tool: %s" % (
                        ", ".join("%s=%d" % item for item in by_tool.most_common())), flush=True)
                    print("METRIC reads=%d dup_reads=%d dup_ratio=%.3f failures=%d repeat_fail_max=%d" % (
                        len(read_keys), dup, dup_ratio, failures, repeat_fail_max), flush=True)
                    if result_bytes:
                        print("METRIC result_bytes=%d max=%d(%s) top: %s" % (
                            sum(result_bytes.values()), result_max, result_max_tool,
                            ", ".join("%s=%d" % item for item in result_bytes.most_common(5))), flush=True)
                    if first_check_ok_at is not None:
                        print("METRIC time_to_first_check_ok=%.0fs" % first_check_ok_at, flush=True)
                    print("METRIC wall=%.0fs rounds_seen=%s" % (
                        elapsed, data.get("rounds") or rounds_seen), flush=True)
                    log.flush()
                    break
    if save_path:
        # Luu NGAY sau khi luot chay xong va TRUOC khi ai do dong LibreOffice: tai lieu chua luu chi nam
        # trong bo nho, dong app la mat sach cong da lam.
        print(save_document(bridge_port, token, save_path), flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
