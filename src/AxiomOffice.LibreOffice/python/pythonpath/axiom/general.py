"""Lenh chung moi app (LibreOffice_arch.md muc 7.4): app.info, ai.ask (qua Agent Core), ui.askpane, app.screenshot."""
from __future__ import annotations

import base64
import json
import os
import tempfile
import time
import urllib.error
import urllib.request

from . import config, documents, log, values
from .commands import command, opt, req

SCREENSHOT_FILTERS = {"wps": "writer_png_Export", "et": "calc_png_Export", "wpp": "impress_png_Export"}


def _setup_value(ctx, name: str):
    try:
        provider = ctx.ServiceManager.createInstanceWithContext("com.sun.star.configuration.ConfigurationProvider", ctx)
        access = provider.createInstanceWithArguments(
            "com.sun.star.configuration.ConfigurationAccess",
            (documents.prop("nodepath", "/org.openoffice.Setup/Product"),))
        return access.getByName(name)
    except Exception:  # noqa: BLE001
        return None


def app_info(env, params):
    info = {
        "kind": env.kind,
        "family": "libreoffice",
        "name": _setup_value(env.ctx, "ooName") or "LibreOffice",
        "version": _setup_value(env.ctx, "ooSetupVersionAboutBox") or _setup_value(env.ctx, "ooSetupVersion"),
    }
    doc = documents.active(env.ctx, env.kind, required=False)
    key = {"wps": "activeDocument", "et": "activeWorkbook", "wpp": "activePresentation"}[env.kind]
    count_key = {"wps": "documents", "et": "workbooks", "wpp": "presentations"}[env.kind]
    count = 0
    enumeration = documents.desktop(env.ctx).getComponents().createEnumeration()
    while enumeration.hasMoreElements():
        if documents._is(enumeration.nextElement(), env.kind):  # noqa: SLF001
            count += 1
    info[count_key] = count
    if doc is not None:
        active = documents.describe(doc)
        if env.kind == "et":
            active["sheet"] = doc.getCurrentController().getActiveSheet().getName()
        elif env.kind == "wpp":
            active["slides"] = doc.getDrawPages().getCount()
        info[key] = active
    info["state"] = {"documents": count, "hasDocument": doc is not None}
    return info


# ---- ai.ask: chuyen sang Agent Core (interactive=false) ----

def _core_base_url():
    # Cung duong khoi dong voi pane (core.ensure): CoreExe hoac vi tri cai mac dinh tren Linux.
    from . import core

    try:
        return core.ensure()[0]
    except core.CoreError as exc:
        log.info("ai.ask: %s" % exc)
        return None


def _core_call(base: str, method: str, path: str, body=None, timeout: float = 30):
    data = None if body is None else json.dumps(body).encode("utf-8")
    request = urllib.request.Request(base + path, data=data, method=method)
    request.add_header("Content-Type", "application/json")
    token = config.token()
    if token:
        request.add_header("X-Auth-Token", token)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        payload = json.loads(exc.read().decode("utf-8") or "{}")
    if not payload.get("ok"):
        raise RuntimeError("Agent Core: %s" % payload.get("error"))
    return payload.get("result")


def ai_ask(env, params):
    prompt = values.string(params, "prompt")
    if not prompt:
        raise values.ParamError("'prompt' is required")
    base = _core_base_url()
    if base is None:
        raise RuntimeError("Agent Core is not running (start AxiomOffice.Core or set CoreExe)")
    from . import bridge

    port = next((p for k, p, _ in bridge._STATE.get("servers", []) if k == env.kind), 0)  # noqa: SLF001
    info = env.gate.run_quiet(lambda: documents.describe(documents.active(env.ctx, env.kind, required=False)), 10.0, {})
    started = _core_call(base, "POST", "/v1/runs", {
        "prompt": prompt,
        "office": {"port": port, "pid": os.getpid(), "app": env.kind, "family": "libreoffice"},
        "document": {"name": (info or {}).get("name"), "fullName": (info or {}).get("fullName")},
        "options": {"maxSeconds": 300, "maxTokens": 200000, "interactive": False},
    })
    run_id = started["runId"]
    deadline = time.time() + 330
    run = {}
    while time.time() < deadline:
        run = _core_call(base, "GET", "/v1/runs/" + run_id)
        if run.get("status") != "running":
            break
        time.sleep(1.0)
    ok = run.get("status") == "completed"
    reply = {"ok": ok, "transcript": run.get("transcript"), "seconds": run.get("seconds"),
             "rounds": run.get("rounds"), "viaCore": True, "runId": run_id, "status": run.get("status")}
    if ok:
        reply["reply"] = run.get("reply")
    else:
        reply["error"] = run.get("error") or ("run status " + str(run.get("status")))
    return reply


def ui_askpane(env, params):
    """Mo deck Axiom Office trong sidebar (LibreOffice_arch.md muc 10). Gated: dispatcher UI phai o main thread."""
    from . import panel

    frame = documents.desktop(env.ctx).getCurrentFrame()
    if frame is None:
        raise RuntimeError("ui.askpane: no visible LibreOffice window (headless)")
    result = panel.show_pane(env.ctx, frame)
    result["taskPane"] = True
    return result


def app_screenshot(env, params):
    """Xuat trang/slide hien tai ra PNG qua filter *_png_Export (khong can chup cua so)."""
    max_width = max(320, min(values.integer(params, "maxWidth", 1280), 2560))
    doc = env.document
    handle, path = tempfile.mkstemp(suffix=".png", prefix="axiom-shot-")
    os.close(handle)
    try:
        import uno

        data = uno.Any("[]com.sun.star.beans.PropertyValue",
                       documents.props(PixelWidth=max_width, PixelHeight=int(max_width * 1.414)))
        args = (documents.prop("FilterName", SCREENSHOT_FILTERS[env.kind]), documents.prop("FilterData", data),
                documents.prop("Overwrite", True))
        uno.invoke(doc, "storeToURL", (documents.to_url(path), args))
        with open(path, "rb") as stream:
            raw = stream.read()
    finally:
        try:
            os.remove(path)
        except OSError:
            pass
    width, height = _png_size(raw)
    return {"width": width, "height": height, "mime": "image/png", "base64": base64.b64encode(raw).decode("ascii")}


def _png_size(raw: bytes):
    if raw[:8] == b"\x89PNG\r\n\x1a\n" and len(raw) >= 24:
        return int.from_bytes(raw[16:20], "big"), int.from_bytes(raw[20:24], "big")
    return 0, 0


command("app.info", None, app_info, "Tên/version app, tài liệu đang mở, `state` (tài liệu, cửa sổ, visible)")
command("ai.ask", None, ai_ask, "Chạy AI agent trên tài liệu đang mở; trả `reply`, `transcript`, `seconds`, `rounds`",
        req("prompt"), gated=False)
command("ui.askpane", None, ui_askpane, "Mở panel Ask AI trong sidebar")
command("app.screenshot", None, app_screenshot,
        "Ảnh chụp trang/slide hiện tại (PNG base64, thu nhỏ theo `maxWidth`, mặc định 1280)", opt("maxWidth"))
