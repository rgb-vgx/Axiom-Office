# Component UNO cua extension Axiom Office: factory cho panel "Ask AI" trong sidebar.
# Sidebar.xcu tro ImplementationURL vao private:resource/toolpanel/AxiomOfficePanelFactory/AskAi, va sidebar
# tao factory theo implementation name org.axiomoffice.bridge.PanelFactory (khong qua ten service).
# Ham tao nay chi import axiom.panel khi sidebar thuc su can panel.
import unohelper

IMPLEMENTATION = "org.axiomoffice.bridge.PanelFactory"


def create_factory(ctx):
    from axiom.panel import PanelFactory

    return PanelFactory(ctx)


g_ImplementationHelper = unohelper.ImplementationHelper()
g_ImplementationHelper.addImplementation(create_factory, IMPLEMENTATION, ("com.sun.star.ui.UIElementFactory",))
