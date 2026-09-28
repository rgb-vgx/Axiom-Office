using System;
using System.Runtime.InteropServices;
using WpsAiBridge.Bridge;

namespace WpsAiBridge
{
    [ComVisible(true)]
    [Guid("30F7166A-059E-484C-90C6-E1862DA76F50")]
    [ProgId("WpsAiBridge.Probe")]
    [ClassInterface(ClassInterfaceType.AutoDispatch)]
    public class Probe
    {
        public Probe()
        {
            Logger.Info("Probe constructor: instance created (no IDTExtensibility2)");
        }

        public string Ping()
        {
            Logger.Info("Probe.Ping called");
            return "pong";
        }
    }
}
