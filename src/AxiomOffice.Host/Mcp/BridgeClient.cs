using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Net;
using System.Text;
using AxiomOffice.Bridge;

namespace AxiomOffice.Host.Mcp
{
    // Gọi HTTP bridge của add-in (cùng cơ chế với tools/*-mcp/bridge.py cũ).
    internal static class BridgeClient
    {
        private const int CommandTimeoutMs = 330000; // ai.ask có trần 5 phút
        private const int StaleSeconds = 90;

        // app -> (loại logic, là Microsoft Office?)
        private static readonly Dictionary<string, KeyValuePair<string, bool>> Apps = new Dictionary<string, KeyValuePair<string, bool>>(StringComparer.OrdinalIgnoreCase)
        {
            { "wps", new KeyValuePair<string, bool>("wps", false) },
            { "et", new KeyValuePair<string, bool>("et", false) },
            { "wpp", new KeyValuePair<string, bool>("wpp", false) },
            { "word", new KeyValuePair<string, bool>("wps", true) },
            { "excel", new KeyValuePair<string, bool>("et", true) },
            { "ppt", new KeyValuePair<string, bool>("wpp", true) },
            { "powerpoint", new KeyValuePair<string, bool>("wpp", true) },
        };

        public static int ResolvePort(string app, int? port)
        {
            if (port.HasValue && port.Value > 0)
            {
                return port.Value;
            }
            KeyValuePair<string, bool> target;
            if (string.IsNullOrEmpty(app) || !Apps.TryGetValue(app, out target))
            {
                throw new ArgumentException("unknown app '" + app + "' (expected wps/et/wpp for WPS or word/excel/ppt for Microsoft Office, or pass port)");
            }
            return Config.PortForKind(target.Key, target.Value);
        }

        public static string BaseUrl(string app, int? port)
        {
            return "http://127.0.0.1:" + ResolvePort(app, port);
        }

        public static string Health(string app, int? port, int timeoutMs = 10000)
        {
            return Send("GET", BaseUrl(app, port) + "/health", null, timeoutMs, false);
        }

        public static string Command(string app, string action, Dictionary<string, object> parameters, int? port)
        {
            string body = Json.Serialize(new Dictionary<string, object>
            {
                { "action", action },
                { "params", parameters ?? new Dictionary<string, object>() }
            });
            return Send("POST", BaseUrl(app, port) + "/cmd", body, CommandTimeoutMs, true);
        }

        // Như bridge.sessions() bản Python: đọc registry, gọi /health, prune khi process chết hoặc
        // heartbeat quá 90s mà /health cũng không trả lời.
        public static List<Dictionary<string, object>> Sessions(bool prune = true, int timeoutMs = 1500, string directory = null)
        {
            directory = directory ?? SessionRegistry.Directory;
            var found = new List<Dictionary<string, object>>();
            if (!System.IO.Directory.Exists(directory))
            {
                return found;
            }
            var json = Json.Create();
            foreach (string path in System.IO.Directory.GetFiles(directory, "*.json"))
            {
                Dictionary<string, object> data;
                try
                {
                    data = json.DeserializeObject(File.ReadAllText(path, Encoding.UTF8)) as Dictionary<string, object>;
                }
                catch (Exception)
                {
                    continue; // đang được ghi lại
                }
                if (data == null)
                {
                    continue;
                }
                int pid = ToInt(data, "pid");
                int port = ToInt(data, "port");
                double lastSeen;
                if (!data.ContainsKey("lastSeenEpoch") || !double.TryParse(Convert.ToString(data["lastSeenEpoch"], CultureInfo.InvariantCulture),
                    NumberStyles.Float, CultureInfo.InvariantCulture, out lastSeen))
                {
                    lastSeen = (File.GetLastWriteTimeUtc(path) - new DateTime(1970, 1, 1, 0, 0, 0, DateTimeKind.Utc)).TotalSeconds;
                }
                double age = Math.Max(0, (DateTime.UtcNow - new DateTime(1970, 1, 1, 0, 0, 0, DateTimeKind.Utc)).TotalSeconds - lastSeen);
                bool alive = IsProcessAlive(pid);
                bool healthy = false;
                if (alive && port > 0)
                {
                    try
                    {
                        var reply = json.DeserializeObject(Health(null, port, timeoutMs)) as Dictionary<string, object>;
                        var result = reply != null && reply.ContainsKey("result") ? reply["result"] as Dictionary<string, object> : null;
                        healthy = reply != null && true.Equals(reply["ok"]) && result != null && ToInt(result, "pid") == pid;
                    }
                    catch (Exception)
                    {
                        healthy = false;
                    }
                }
                if (!alive || (age > StaleSeconds && !healthy))
                {
                    if (prune)
                    {
                        try
                        {
                            File.Delete(path);
                        }
                        catch (Exception)
                        {
                        }
                    }
                    continue;
                }
                data["healthy"] = healthy;
                data["ageSeconds"] = Math.Round(age, 1);
                data["file"] = path;
                found.Add(data);
            }
            found.Sort(delegate(Dictionary<string, object> a, Dictionary<string, object> b)
            {
                int byFamily = string.CompareOrdinal(Convert.ToString(a.ContainsKey("family") ? a["family"] : ""), Convert.ToString(b.ContainsKey("family") ? b["family"] : ""));
                return byFamily != 0 ? byFamily : ToInt(a, "port").CompareTo(ToInt(b, "port"));
            });
            return found;
        }

        private static bool IsProcessAlive(int pid)
        {
            if (pid <= 0)
            {
                return false;
            }
            try
            {
                using (Process process = Process.GetProcessById(pid))
                {
                    try
                    {
                        return !process.HasExited;
                    }
                    catch (System.ComponentModel.Win32Exception)
                    {
                        return true; // tồn tại nhưng không mở được (quyền)
                    }
                }
            }
            catch (ArgumentException)
            {
                return false;
            }
            catch (InvalidOperationException)
            {
                return false;
            }
        }

        private static int ToInt(Dictionary<string, object> data, string key)
        {
            object value;
            if (!data.TryGetValue(key, out value) || value == null)
            {
                return 0;
            }
            try
            {
                return Convert.ToInt32(value, CultureInfo.InvariantCulture);
            }
            catch (Exception)
            {
                return 0;
            }
        }

        // Trả nguyên body JSON của bridge (kể cả khi HTTP 4xx/5xx, bridge vẫn trả {"ok":false,...}).
        private static string Send(string method, string url, string body, int timeoutMs, bool auth)
        {
            var request = (HttpWebRequest)WebRequest.Create(url);
            request.Method = method;
            request.Timeout = timeoutMs;
            request.ReadWriteTimeout = timeoutMs;
            request.Proxy = null;
            if (auth)
            {
                string token = Config.Token;
                if (!string.IsNullOrEmpty(token))
                {
                    request.Headers["X-Auth-Token"] = token;
                }
            }
            if (body != null)
            {
                byte[] data = Encoding.UTF8.GetBytes(body);
                request.ContentType = "application/json; charset=utf-8";
                request.ContentLength = data.Length;
                using (Stream stream = request.GetRequestStream())
                {
                    stream.Write(data, 0, data.Length);
                }
            }
            try
            {
                using (var response = (HttpWebResponse)request.GetResponse())
                using (var reader = new StreamReader(response.GetResponseStream(), Encoding.UTF8))
                {
                    return reader.ReadToEnd();
                }
            }
            catch (WebException ex)
            {
                if (ex.Response != null)
                {
                    using (ex.Response)
                    using (var reader = new StreamReader(ex.Response.GetResponseStream(), Encoding.UTF8))
                    {
                        string text = reader.ReadToEnd();
                        if (text.Length > 0)
                        {
                            return text;
                        }
                    }
                }
                throw;
            }
        }
    }
}
