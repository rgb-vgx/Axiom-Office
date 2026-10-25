"""Chay MOT viec that bang ai.ask tren PowerPoint (hoac Word/Excel) va thu lai ket qua de soat.

Tu mo app (bridge cong 47831-47833), tao tai lieu moi, gui prompt qua ai.ask (Core chay khong tuong tac - khong ai bam
xac nhan), roi: luu transcript, render tung slide ra PNG, chay checkLayout, luu tai lieu, va DONG app.

    python tests/live/run_ai_task.py tests/live/tasks/ngan-hang-ai-agent.md out/ngan-hang [--app ppt]

Sau khi chay, so do tung vong / tung lenh nam trong core.db (schema 4) - xem docs/plans/20-c0-ngu-canh-va-slide.md
muc "Cach do". Chi dung thu vien chuan + harness tests/live/test_live_commands.py.
"""
from __future__ import annotations

import argparse
import base64
import json
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_live_commands as t  # noqa: E402

NEW_DOCUMENT = {"ppt": "wpp.newPresentation", "word": "writer.newDocument", "excel": "et.newWorkbook"}
SAVE_AS = {"ppt": ("wpp.saveAs", "result.pptx"), "word": ("writer.saveAs", "result.docx"), "excel": ("et.saveAs", "result.xlsx")}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("prompt_file")
    parser.add_argument("out_dir")
    parser.add_argument("--app", default="ppt", choices=sorted(NEW_DOCUMENT))
    parser.add_argument("--timeout", type=int, default=900, help="giay cho ai.ask")
    args = parser.parse_args()

    prompt = open(args.prompt_file, encoding="utf-8").read()
    out = os.path.abspath(args.out_dir)
    os.makedirs(out, exist_ok=True)
    port, pid = t.launch(args.app, False)
    try:
        bridge = t.Bridge(port, out, args.app)
        bridge.cmd(NEW_DOCUMENT[args.app], record=False)
        started = time.time()
        status, reply = t.http(port, "POST", "/cmd", {"action": "ai.ask", "params": {"prompt": prompt}}, bridge.headers,
                               args.timeout)
        result = (reply or {}).get("result") or {}
        print("ai.ask %.0fs HTTP %s ok=%s rounds=%s error=%s" % (time.time() - started, status, result.get("ok"),
                                                               result.get("rounds"), result.get("error")))
        print("reply:", (result.get("reply") or "")[:1500])
        with open(os.path.join(out, "transcript.txt"), "w", encoding="utf-8") as handle:
            handle.write("\n".join(result.get("transcript") or []))
        if args.app == "ppt":
            count = (bridge.cmd("wpp.listSlides", record=False) or {}).get("slideCount", 0)
            print("slides:", count)
            for index in range(1, count + 1):
                shot = bridge.cmd("wpp.renderSlide", {"slide": index, "maxWidth": 960}, record=False) or {}
                if shot.get("base64"):
                    with open(os.path.join(out, "s%02d.png" % index), "wb") as handle:
                        handle.write(base64.b64decode(shot["base64"]))
            layout = bridge.cmd("wpp.checkLayout", record=False) or {}
            issues = [issue for slide in layout.get("slides", []) for issue in slide.get("issues", [])]
            print("checkLayout:", layout.get("issueCount"), json.dumps(issues, ensure_ascii=False)[:1500])
        action, name = SAVE_AS[args.app]
        bridge.cmd(action, {"path": os.path.join(out, name)}, record=False)
        print("da luu:", os.path.join(out, name))
    finally:
        t.close_app(pid)
    return 0


if __name__ == "__main__":
    sys.exit(main())
