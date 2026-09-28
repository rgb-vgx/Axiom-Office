using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Web.Script.Serialization;

namespace WpsAiBridge.Bridge
{
    // Mỗi bridge đang sống ghi %LOCALAPPDATA%\WpsAiBridge\sessions\{pid}.json để agent/MCP
    // tìm được mọi instance (Office + WPS) mà không phải đoán port.
    internal sealed class SessionRegistry
    {
        private const int HeartbeatMs = 25000;
        private const int ProbeTimeoutMs = 2000;

        private static readonly string SessionsDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "WpsAiBridge", "sessions");

        private readonly IAppHost _host;
        private readonly int _port;
        private readonly int _pid;
        private readonly string _hostProcess;
        private readonly string _started;
        private readonly string _path;
        private readonly object _sync = new object();
        private readonly object _fileSync = new object();
        private readonly JavaScriptSerializer _json = new JavaScriptSerializer();
        private readonly ManualResetEvent _stop = new ManualResetEvent(false);
        private Thread _thread;
        private Task _probe;
        private string _document;
        private string _documentPath;

        public SessionRegistry(IAppHost host, int port)
        {
            _host = host;
            _port = port;
            Process process = Process.GetCurrentProcess();
            _pid = process.Id;
            _hostProcess = HostProcessName(process);
            _started = IsoNow();
            _path = Path.Combine(SessionsDir, _pid.ToString(CultureInfo.InvariantCulture) + ".json");
        }

        public static string Directory
        {
            get { return SessionsDir; }
        }

        public void Start()
        {
            WriteFile();
            _thread = new Thread(Heartbeat);
            _thread.IsBackground = true;
            _thread.Name = "WpsAiBridge.Session";
            _thread.Start();
            Logger.Info("Session registered: " + _path);
        }

        public void Stop()
        {
            _stop.Set();
            lock (_fileSync)
            {
                try
                {
                    if (File.Exists(_path))
                    {
                        File.Delete(_path);
                    }
                    Logger.Info("Session unregistered: " + _path);
                }
                catch (Exception ex)
                {
                    Logger.Error("Session file delete failed: " + _path, ex);
                }
            }
        }

        // Thông tin cho GET /session. live=true đọc tài liệu ngay lúc gọi (trên thread của request).
        public Dictionary<string, object> Describe(bool live)
        {
            if (live)
            {
                try
                {
                    // Host đang chạy lệnh khác quá 2s thì trả giá trị đã cache.
                    if (ComGate.TryRun(ProbeTimeoutMs, delegate { UpdateDocument(HostProbe.ActiveDocument(_host)); }))
                    {
                        // Giữ file registry khớp với lần đọc live mới nhất, không đợi heartbeat kế tiếp.
                        WriteFile();
                    }
                }
                catch (Exception ex)
                {
                    Logger.Info("Session: live document read failed: " + ex.GetType().Name + ": " + ex.Message);
                }
            }
            Dictionary<string, object> info = Snapshot();
            info["sessionFile"] = _path;
            return info;
        }

        private void Heartbeat()
        {
            while (!_stop.WaitOne(0))
            {
                RefreshDocument();
                WriteFile();
                if (_stop.WaitOne(HeartbeatMs))
                {
                    break;
                }
            }
        }

        // Đọc tên tài liệu trên threadpool, chờ tối đa ProbeTimeoutMs: host bận không làm trễ heartbeat
        // (lastSeen luôn được ghi đúng hạn). Lần đọc trước còn treo thì bỏ qua lần này.
        private void RefreshDocument()
        {
            Task pending = _probe;
            if (pending != null && !pending.IsCompleted)
            {
                return;
            }
            _probe = Task.Run(delegate
            {
                try
                {
                    // Host đang chạy lệnh của bridge thì bỏ qua, giữ tên tài liệu đã cache.
                    ComGate.TryRun(0, delegate { UpdateDocument(HostProbe.ActiveDocument(_host)); });
                }
                catch (COMException ex)
                {
                    if (!HostProbe.IsRetryable(ex))
                    {
                        UpdateDocument(null);
                    }
                }
                catch (Exception)
                {
                }
            });
            try
            {
                _probe.Wait(ProbeTimeoutMs);
            }
            catch (AggregateException)
            {
            }
        }

        private void UpdateDocument(Dictionary<string, object> doc)
        {
            lock (_sync)
            {
                _document = doc == null ? null : Convert.ToString(doc["name"]);
                _documentPath = doc == null ? null : Convert.ToString(doc["fullName"]);
            }
        }

        private Dictionary<string, object> Snapshot()
        {
            DateTime now = DateTime.UtcNow;
            var info = new Dictionary<string, object>
            {
                { "pid", _pid },
                { "app", _host.AppKind },
                { "family", _host.IsOfficeHost ? "office" : "wps" },
                { "port", _port },
                { "host", _hostProcess },
                { "version", CommandDispatcher.BridgeVersion },
                { "started", _started },
                { "lastSeen", now.ToString("yyyy-MM-ddTHH:mm:ss.fffZ", CultureInfo.InvariantCulture) },
                { "lastSeenEpoch", Math.Round((now - new DateTime(1970, 1, 1, 0, 0, 0, DateTimeKind.Utc)).TotalSeconds, 3) }
            };
            lock (_sync)
            {
                info["document"] = _document;
                info["documentPath"] = _documentPath;
            }
            return info;
        }

        private void WriteFile()
        {
            lock (_fileSync)
            {
                // Kiểm tra trong lock: Stop() đã xoá file thì không được ghi lại.
                if (_stop.WaitOne(0))
                {
                    return;
                }
                try
                {
                    System.IO.Directory.CreateDirectory(SessionsDir);
                    string json = _json.Serialize(Snapshot());
                    string temp = _path + ".tmp";
                    File.WriteAllText(temp, json, new UTF8Encoding(false));
                    if (File.Exists(_path))
                    {
                        File.Replace(temp, _path, null);
                    }
                    else
                    {
                        File.Move(temp, _path);
                    }
                }
                catch (Exception ex)
                {
                    Logger.Error("Session file write failed: " + _path, ex);
                }
            }
        }

        private static string IsoNow()
        {
            return DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ", CultureInfo.InvariantCulture);
        }

        private static string HostProcessName(Process process)
        {
            try
            {
                return process.MainModule.ModuleName;
            }
            catch
            {
                return process.ProcessName;
            }
        }
    }
}
