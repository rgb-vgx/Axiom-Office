"""Xu ly URL org.axiomoffice.bridge:... tu menu (Addons.xcu -> ProtocolHandler.xcu -> component nay).

Hai URL: `:askpane` (mo pane Ask AI) va `:settings` (mo cua so Cai dat). Dispatch chay tren main thread
(menu cua LibreOffice), nen goi thang `panel.show_pane` / `dialogs.open_settings` ma khong qua UnoGate.
"""
from __future__ import annotations

import unohelper
from com.sun.star.frame import XDispatch, XDispatchProvider
from com.sun.star.lang import XInitialization

from . import log

ACTIONS = {"askpane", "settings"}


class Dispatcher(unohelper.Base, XDispatchProvider, XInitialization, XDispatch):
    """Vua la provider (LibreOffice goi queryDispatch) vua la dispatch (URL cua chung ta)."""

    def __init__(self, ctx):
        self.ctx = ctx
        self.frame = None

    # ---------------------------------------------------------------- XInitialization

    def initialize(self, args) -> None:
        # Phan tu dau la frame (xem tai lieu ProtocolHandler cua LibreOffice).
        try:
            self.frame = args[0]
        except Exception:  # noqa: BLE001
            self.frame = None

    # ---------------------------------------------------------------- XDispatchProvider

    def queryDispatch(self, url, target_name, flags):
        if self._action(url) is not None:
            return self
        return None

    def queryDispatches(self, requests):
        return tuple(self.queryDispatch(item.FeatureURL, item.FrameName, item.SearchFlags) for item in requests)

    # ---------------------------------------------------------------- XDispatch

    def dispatch(self, url, args) -> None:
        action = self._action(url)
        if action is None:
            return
        try:
            if action == "askpane":
                from . import panel

                panel.show_pane(self.ctx, self._frame())
            elif action == "settings":
                from . import dialogs

                dialogs.open_settings(_SettingsTarget(self.ctx, self._frame()))
        except Exception as exc:  # noqa: BLE001 - khong duoc lam sap LibreOffice tu menu
            log.error("dispatch %s failed: %s" % (action, exc))

    def addStatusListener(self, listener, url) -> None:
        pass

    def removeStatusListener(self, listener, url) -> None:
        pass

    # ---------------------------------------------------------------- noi bo

    def _frame(self):
        if self.frame is not None:
            return self.frame
        from . import documents

        return documents.desktop(self.ctx).getCurrentFrame()

    def _action(self, url):
        try:
            complete = str(url.Complete)
        except Exception:  # noqa: BLE001
            return None
        prefix = "org.axiomoffice.bridge:"
        if not complete.startswith(prefix):
            return None
        action = complete[len(prefix):]
        return action if action in ACTIONS else None


class _SettingsTarget:
    """Doi tuong toi thieu ma dialogs.open_settings can: ctx + frame."""

    def __init__(self, ctx, frame):
        self.ctx = ctx
        self.frame = frame
