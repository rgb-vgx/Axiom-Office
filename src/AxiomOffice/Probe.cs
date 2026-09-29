using System;
using System.Runtime.InteropServices;
using AxiomOffice.Bridge;

namespace AxiomOffice
{
    [ComVisible(true)]
    [Guid("1BB15863-0453-416C-BABD-C97F929609C0")]
    [ProgId("AxiomOffice.Probe")]
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
