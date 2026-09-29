using System;
using System.Runtime.InteropServices;

namespace AxiomOffice.Interop
{
    [ComImport]
    [Guid("000C0396-0000-0000-C000-000000000046")]
    [InterfaceType(ComInterfaceType.InterfaceIsIDispatch)]
    public interface IRibbonExtensibility
    {
        [DispId(1)]
        string GetCustomUI(string RibbonID);
    }
}
