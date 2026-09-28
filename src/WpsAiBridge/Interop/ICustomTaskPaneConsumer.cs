using System;
using System.Runtime.InteropServices;

namespace WpsAiBridge.Interop
{
    [ComImport]
    [Guid("000C0397-0000-0000-C000-000000000046")]
    [InterfaceType(ComInterfaceType.InterfaceIsIDispatch)]
    public interface ICTPFactory
    {
        [DispId(1)]
        [return: MarshalAs(UnmanagedType.IDispatch)]
        object CreateCTP(string CTPProgId, string CTPTitle, [MarshalAs(UnmanagedType.Struct)] object ParentWindow);
    }

    [ComImport]
    [Guid("000C033E-0000-0000-C000-000000000046")]
    [InterfaceType(ComInterfaceType.InterfaceIsIDispatch)]
    public interface ICustomTaskPaneConsumer
    {
        [DispId(1)]
        void CTPFactoryAvailable([MarshalAs(UnmanagedType.IDispatch)] object CTPFactoryInst);
    }
}
