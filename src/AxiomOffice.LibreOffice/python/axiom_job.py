# Component UNO cua extension Axiom Office: job chay khi LibreOffice khoi dong, bat bridge HTTP.
# Code that nam trong pythonpath/axiom (LibreOffice tu them thu muc pythonpath canh file component vao sys.path).
import traceback

import unohelper
from com.sun.star.task import XJob

IMPLEMENTATION = "org.axiomoffice.bridge.StartJob"


class StartJob(unohelper.Base, XJob):
    def __init__(self, ctx):
        self.ctx = ctx

    def execute(self, args):
        try:
            from axiom import bridge
            bridge.start(self.ctx)
        except Exception:  # noqa: BLE001 - khong duoc lam hong LibreOffice
            try:
                from axiom import log
                log.error("start job failed:\n" + traceback.format_exc())
            except Exception:  # noqa: BLE001
                pass
        return None


g_ImplementationHelper = unohelper.ImplementationHelper()
g_ImplementationHelper.addImplementation(StartJob, IMPLEMENTATION, (IMPLEMENTATION, "com.sun.star.task.Job"))
