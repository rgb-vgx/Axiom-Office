using System.Net;
using System.Net.Sockets;

namespace AxiomOffice.Core;

// Trang thai runtime cua Core: port dang nghe (co the la port he dieu hanh cap) va thoi diem bat dau.
public sealed class CoreRuntime
{
    public int Port { get; set; }

    public DateTime StartedUtc { get; } = DateTime.UtcNow;

    public double UptimeSeconds => (DateTime.UtcNow - StartedUtc).TotalSeconds;

    // Port trong (chi nghe 127.0.0.1): port dau tien con dung trong khoang [preferred, preferred+attempts).
    public static int? TryPickPort(int preferred, int attempts = 10)
    {
        for (int i = 0; i < attempts; i++)
        {
            int port = preferred + i;
            if (port is < 1 or > 65535)
            {
                break;
            }

            if (IsFree(port))
            {
                return port;
            }
        }

        return null;
    }

    public static bool IsFree(int port)
    {
        try
        {
            var probe = new TcpListener(IPAddress.Loopback, port);
            probe.Start();
            probe.Stop();
            return true;
        }
        catch (SocketException)
        {
            return false;
        }
    }

    // Port thuc sau khi Kestrel bind (CorePort = 0: he dieu hanh cap port nen phai doc lai).
    public static int? ResolveBoundPort(WebApplication app)
    {
        foreach (string url in app.Urls)
        {
            if (Uri.TryCreate(url, UriKind.Absolute, out Uri? uri) && uri.Port > 0)
            {
                return uri.Port;
            }
        }

        return null;
    }
}
