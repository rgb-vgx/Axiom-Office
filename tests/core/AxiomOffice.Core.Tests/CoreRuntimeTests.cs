using System.Net;
using System.Net.Sockets;

namespace AxiomOffice.Core.Tests;

// Chon port: Core thu CorePort..CorePort+9 (New_arch.md muc 7.1).
public class CoreRuntimeTests
{
    [Fact]
    public void Port_dang_ban_thi_chon_port_ke_tiep()
    {
        int busy = FindFreePort();
        var listener = new TcpListener(IPAddress.Loopback, busy);
        listener.Start();
        try
        {
            int? picked = CoreRuntime.TryPickPort(busy, attempts: 3);

            Assert.NotNull(picked);
            Assert.NotEqual(busy, picked);
            Assert.InRange(picked!.Value, busy + 1, busy + 2);
            Assert.NotNull(CoreRuntime.TryPickPort(busy, attempts: 3)); // port ke tiep con trong
        }
        finally
        {
            listener.Stop();
        }
    }

    [Fact]
    public void Het_khoang_port_thi_tra_null()
    {
        int busy = FindFreePort();
        var listeners = new List<TcpListener>();
        try
        {
            for (int i = 0; i < 2; i++)
            {
                var listener = new TcpListener(IPAddress.Loopback, busy + i);
                listener.Start();
                listeners.Add(listener);
            }

            Assert.Null(CoreRuntime.TryPickPort(busy, attempts: 2));
        }
        finally
        {
            foreach (TcpListener listener in listeners)
            {
                listener.Stop();
            }
        }
    }

    [Fact]
    public void IsFree_phan_anh_dung_trang_thai()
    {
        int port = FindFreePort();
        Assert.True(CoreRuntime.IsFree(port));

        var listener = new TcpListener(IPAddress.Loopback, port);
        listener.Start();
        try
        {
            Assert.False(CoreRuntime.IsFree(port));
        }
        finally
        {
            listener.Stop();
        }

        Assert.True(CoreRuntime.IsFree(port));
    }

    private static int FindFreePort()
    {
        var probe = new TcpListener(IPAddress.Loopback, 0);
        probe.Start();
        int port = ((IPEndPoint)probe.LocalEndpoint).Port;
        probe.Stop();
        return port;
    }
}
