"""UnoGate: chay moi lenh UNO tren main thread cua LibreOffice, tuan tu (LibreOffice_arch.md muc 8).

Tuong duong ComGate cua add-in Windows. Thread HTTP dat viec qua com.sun.star.awt.AsyncCallback (hang doi
message cua VCL) roi cho ket qua; khoa giu tuan tu. Neu AsyncCallback khong chay (vd moi truong khong co
vong lap VCL), sau FALLBACK_AFTER giay goi truc tiep duoi khoa de khong treo mai.
"""
from __future__ import annotations

import threading
import time

import unohelper
from com.sun.star.awt import XCallback

from . import log


class BusyError(Exception):
    pass


class _Callback(unohelper.Base, XCallback):
    def __init__(self, job):
        self._job = job

    def notify(self, data):  # noqa: N802 - ten UNO
        self._job()


class UnoGate:
    def __init__(self, ctx, timeout: float = 60.0):
        self.ctx = ctx
        self.timeout = timeout
        self._lock = threading.Lock()
        self._async = ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.AsyncCallback", ctx)
        self._main_thread = threading.main_thread()
        self.direct_calls = 0
        # Thoi diem lenh gan nhat khong duoc main thread tra loi (thuong la mot hop thoai dang mo):
        # /health bao "stuck" de nguoi dung biet phai dong hop thoai thay vi doan mo.
        self.stuck_since: float | None = None

    @property
    def stuck(self) -> bool:
        return self.stuck_since is not None

    def run(self, fn, timeout: float | None = None):
        wait = self.timeout if timeout is None else timeout
        if not self._lock.acquire(timeout=wait):
            raise BusyError("LibreOffice is busy with another command")
        try:
            done = threading.Event()
            box: dict = {}
            self.stuck_since = None

            def job():
                if done.is_set():
                    return
                try:
                    box["result"] = fn()
                except BaseException as exc:  # noqa: BLE001 - tra nguyen loi cho thread goi
                    box["error"] = exc
                finally:
                    done.set()

            self._async.addCallback(_Callback(job), None)
            if not done.wait(wait):
                self.stuck_since = time.time()
                raise BusyError("LibreOffice did not answer within %.0fs - a dialog (or a whole-document "
                                "operation) is blocking the window; close it and retry" % wait)
            if "error" in box:
                raise box["error"]
            return box.get("result")
        finally:
            self._lock.release()

    def run_quiet(self, fn, timeout: float = 3.0, default=None):
        """Cho heartbeat/SSE: app ban thi bo luot, khong nem loi (khong danh dau stuck)."""
        try:
            return self.run(fn, timeout)
        except Exception as exc:  # noqa: BLE001
            log.info("gate skipped: %s" % exc)
            return default
