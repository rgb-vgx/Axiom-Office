# Component UNO: xu ly URL org.axiomoffice.bridge:... (menu trong Addons.xcu).
# Dang ky qua ProtocolHandler.xcu voi implementation org.axiomoffice.bridge.Dispatch.
import unohelper

IMPLEMENTATION = "org.axiomoffice.bridge.Dispatch"


def create_dispatch(ctx):
    from axiom.dispatch import Dispatcher

    return Dispatcher(ctx)


g_ImplementationHelper = unohelper.ImplementationHelper()
g_ImplementationHelper.addImplementation(create_dispatch, IMPLEMENTATION,
                                         ("com.sun.star.frame.ProtocolHandler",))
