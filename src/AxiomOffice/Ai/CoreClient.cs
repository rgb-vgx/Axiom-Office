using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Text;
using System.Threading;
using System.Web.Script.Serialization;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ai
{
    // Mot su kien tien trinh tu Agent Core (SSE, New_arch.md muc 7.4).
    internal sealed class CoreEvent
    {
        public string Type;
        public long Seq;
        public string RunId;
        public string ConversationId;
        public string Action;
        public string ParamsPreview;
        public string ResultPreview;
        public string Reply;
        public bool Ok;
        public string Error;
        public string ErrorKind;
        public double Seconds;
        public int Rounds;
        public int InputTokens;
        public int OutputTokens;
    }

    // Client HTTP + SSE toi AxiomOffice.Core.exe: tim Core qua core.json, khoi dong khi can (muc 7.1),
    // chay luot agent qua Core API (7.3/7.4). Moi loi ket noi -> Run tra null de pane dung agent
    // in-process nhu cu (du phong). Khong bao gio nem ra ngoai.
    internal sealed class CoreClient
    {
        public const int Protocol = 1;

        private const int HealthTimeoutMs = 1500;
        private const int StartTimeoutMs = 12000;
        private const int StartConfirmMs = 30000;   // cache "Core dang song" trong 30s
        private const int RetryAfterFailureMs = 60000;

        private static readonly CoreClient Shared = new CoreClient();

        private readonly object _sync = new object();
        private string _baseUrl;
        private DateTime _baseUrlCheckedUtc = DateTime.MinValue;
        private DateTime _lastFailureUtc = DateTime.MinValue;

        public static CoreClient Instance
        {
            get { return Shared; }
        }

        public string LastError { get; private set; }

        public bool Enabled
        {
            get { return Config.CoreEnabled; }
        }

        public static string CoreJsonPath
        {
            get
            {
                return Path.Combine(
                    Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                    "AxiomOffice", "core.json");
            }
        }

        public static string CoreExePath
        {
            get
            {
                string dir = Path.GetDirectoryName(typeof(CoreClient).Assembly.Location);
                return Path.Combine(dir ?? "", "AxiomOffice.Core.exe");
            }
        }

        // Core dang song thi tra ve base URL; chua chay thi khoi dong (toi da StartTimeoutMs).
        public string EnsureBaseUrl()
        {
            if (!Enabled)
            {
                LastError = "Agent Core bi tat (CoreEnabled = 0)";
                return null;
            }

            lock (_sync)
            {
                if (_baseUrl != null && (DateTime.UtcNow - _baseUrlCheckedUtc).TotalMilliseconds < StartConfirmMs)
                {
                    return _baseUrl;
                }

                if ((DateTime.UtcNow - _lastFailureUtc).TotalMilliseconds < RetryAfterFailureMs)
                {
                    return null;
                }

                string url = ProbeExisting();
                if (url == null)
                {
                    url = StartAndProbe();
                }

                if (url == null)
                {
                    _lastFailureUtc = DateTime.UtcNow;
                    _baseUrl = null;
                    Logger.Info("CoreClient: Agent Core khong san sang (" + LastError + "); dung agent in-process");
                    return null;
                }

                _baseUrl = url;
                _baseUrlCheckedUtc = DateTime.UtcNow;
                LastError = null;
                return _baseUrl;
            }
        }

        // Chay mot luot qua Core. Tra null khi KHONG ket noi duoc (pane nen thu lai bang in-process);
        // loi nghiep vu cua luot chay van tra LlmResult voi Ok=false (khong chay lai de tranh ton token).
        public LlmResult Run(Connect host, string prompt, string conversationId, Action<CoreEvent> onEvent, CancellationToken cancel)
        {
            string baseUrl = EnsureBaseUrl();
            if (baseUrl == null)
            {
                return null;
            }

            string appKind = host != null ? host.AppKind : "wps";
            int bridgePid;
            int port = ResolvePort(host, out bridgePid);
            if (port <= 0)
            {
                LastError = "khong tim thay port cua bridge trong session registry";
                Logger.Info("CoreClient: fallback to in-process - " + LastError);
                return null;
            }

            var document = DocumentInfo(host);
            var request = new Dictionary<string, object>();
            request["prompt"] = prompt;
            if (!string.IsNullOrEmpty(conversationId))
            {
                request["conversationId"] = conversationId;
            }
            request["office"] = new Dictionary<string, object>
            {
                { "port", port },
                { "pid", bridgePid },
                { "app", appKind },
                { "family", host != null && host.IsOfficeHost ? "office" : "wps" }
            };
            request["document"] = document;
            request["options"] = new Dictionary<string, object> { { "maxSeconds", 300 }, { "maxTokens", 200000 } };

            string error;
            string responseText = PostJson(baseUrl + "/v1/runs", Serialize(request), 30000, cancel, out error);
            if (responseText == null)
            {
                LastError = error;
                Logger.Info("CoreClient: fallback to in-process - POST /v1/runs failed: " + error);
                return null;
            }

            // Core boc ket qua trong {"ok":true,"result":{runId, conversationId}} (giong bridge). Truoc day doc
            // runId o cap ngoai nen luon "khong tra runId" -> pane chay lai in-process trong khi Core van chay:
            // moi yeu cau bi lam HAI lan.
            Dictionary<string, object> reply = RunReply(responseText);
            string runId = reply != null ? Convert.ToString(reply.ContainsKey("runId") ? reply["runId"] : null) : null;
            if (string.IsNullOrEmpty(runId))
            {
                LastError = "Core khong tra runId: " + Truncate(responseText, 200);
                Logger.Info("CoreClient: fallback to in-process - " + LastError);
                return null;
            }

            var result = new LlmResult { ViaCore = true };
            result.ConversationId = reply != null && reply.ContainsKey("conversationId")
                ? Convert.ToString(reply["conversationId"])
                : null;

            var watch = Stopwatch.StartNew();
            using (cancel.Register(delegate { CancelRun(runId); }))
            {
                ReadEventStream(baseUrl, runId, result, onEvent, cancel);
            }
            watch.Stop();
            if (result.Seconds <= 0)
            {
                result.Seconds = watch.Elapsed.TotalSeconds;
            }

            if (!result.Ok && !result.Cancelled && !result.TimedOut && !result.Stopped)
            {
                // Da co runId thi Core co the da sua tai lieu: KHONG tra null (pane chay lai in-process se
                // lam hai lan). Bao loi cua Core cho nguoi dung.
                if (string.IsNullOrEmpty(result.Error))
                {
                    result.Error = "khong doc duoc ket qua tu Agent Core (run " + runId + ")";
                }
                Logger.Error("CoreClient: run " + runId + " failed: " + result.Error, null);
            }

            return result;
        }

        // Hoi thoai gan nhat cua tai lieu: pane dung de "lam tiep" sau khi mo lai (muc 9).
        public string FindConversationForDocument(string documentKey)
        {
            if (string.IsNullOrEmpty(documentKey))
            {
                return null;
            }

            string baseUrl = EnsureBaseUrl();
            if (baseUrl == null)
            {
                return null;
            }

            string error;
            string text = GetText(baseUrl + "/v1/conversations?limit=1&documentKey=" + Uri.EscapeDataString(documentKey), 5000, out error);
            if (text == null)
            {
                return null;
            }

            Dictionary<string, object> reply = Deserialize(text) as Dictionary<string, object>;
            object result;
            if (reply == null || !reply.TryGetValue("result", out result))
            {
                return null;
            }

            var payload = result as Dictionary<string, object>;
            object list;
            if (payload == null || !payload.TryGetValue("conversations", out list))
            {
                return null;
            }

            var items = list as object[];
            if (items == null || items.Length == 0)
            {
                return null;
            }

            var first = items[0] as Dictionary<string, object>;
            return first != null && first.ContainsKey("id") ? Convert.ToString(first["id"]) : null;
        }

        // Yeu cau Core dung luot chay (ngoai viec huy request SSE dang cho).
        public bool CancelRun(string runId)
        {
            if (string.IsNullOrEmpty(runId))
            {
                return false;
            }

            string baseUrl;
            lock (_sync)
            {
                baseUrl = _baseUrl;
            }

            if (baseUrl == null)
            {
                return false;
            }

            string error;
            return PostJson(baseUrl + "/v1/runs/" + runId + "/cancel", "{}", 5000, CancellationToken.None, out error) != null;
        }

        private string ProbeExisting()
        {
            int port;
            if (!TryReadCoreJson(out port))
            {
                LastError = "chua co core.json";
                return null;
            }

            string baseUrl = "http://127.0.0.1:" + port;
            if (!HealthOk(baseUrl))
            {
                LastError = "core.json co nhung /health khong tra loi (port " + port + ")";
                return null;
            }

            return baseUrl;
        }

        private string StartAndProbe()
        {
            string exe = CoreExePath;
            if (!File.Exists(exe))
            {
                LastError = "khong thay " + exe;
                return null;
            }

            try
            {
                Logger.Info("CoreClient: khoi dong Agent Core (" + exe + ")");
                var start = new ProcessStartInfo(exe)
                {
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    WorkingDirectory = Path.GetDirectoryName(exe) ?? "",
                };
                start.EnvironmentVariables["DOTNET_ROOT"] = Environment.GetEnvironmentVariable("DOTNET_ROOT") ?? "";
                Process.Start(start);
            }
            catch (Exception ex)
            {
                LastError = "khong khoi dong duoc Core: " + ex.Message;
                return null;
            }

            var watch = Stopwatch.StartNew();
            while (watch.ElapsedMilliseconds < StartTimeoutMs)
            {
                Thread.Sleep(300);
                int port;
                if (!TryReadCoreJson(out port))
                {
                    continue;
                }

                string baseUrl = "http://127.0.0.1:" + port;
                if (HealthOk(baseUrl))
                {
                    return baseUrl;
                }
            }

            LastError = "Core khong san sang sau " + (StartTimeoutMs / 1000) + "s";
            return null;
        }

        private static bool TryReadCoreJson(out int port)
        {
            port = 0;
            try
            {
                string path = CoreJsonPath;
                if (!File.Exists(path))
                {
                    return false;
                }

                var info = new JavaScriptSerializer().Deserialize<Dictionary<string, object>>(File.ReadAllText(path, Encoding.UTF8));
                if (info == null)
                {
                    return false;
                }

                int protocol = info.ContainsKey("protocol") ? Convert.ToInt32(info["protocol"]) : 0;
                if (protocol != Protocol)
                {
                    return false;
                }

                int pid = info.ContainsKey("pid") ? Convert.ToInt32(info["pid"]) : 0;
                if (!IsProcessAlive(pid))
                {
                    return false;
                }

                port = info.ContainsKey("port") ? Convert.ToInt32(info["port"]) : 0;
                return port > 0;
            }
            catch (Exception)
            {
                return false;
            }
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
                    return !process.HasExited;
                }
            }
            catch (Exception)
            {
                return false;
            }
        }

        private static bool HealthOk(string baseUrl)
        {
            string error;
            string text = GetText(baseUrl + "/health", HealthTimeoutMs, out error);
            if (text == null)
            {
                return false;
            }

            Dictionary<string, object> reply = Deserialize(text) as Dictionary<string, object>;
            if (reply == null || !(reply.ContainsKey("ok") && Convert.ToBoolean(reply["ok"])))
            {
                return false;
            }

            var result = reply.ContainsKey("result") ? reply["result"] as Dictionary<string, object> : null;
            if (result == null || !result.ContainsKey("protocol"))
            {
                return false;
            }

            return Convert.ToInt32(result["protocol"]) == Protocol;
        }

        // Doc SSE den khi co su kien ket thuc luot chay; do day la ket qua cuoi cung cho pane.
        private void ReadEventStream(string baseUrl, string runId, LlmResult result, Action<CoreEvent> onEvent, CancellationToken cancel)
        {
            HttpWebRequest request = null;
            try
            {
                request = (HttpWebRequest)WebRequest.Create(baseUrl + "/v1/runs/" + runId + "/events");
                request.Method = "GET";
                request.Accept = "text/event-stream";
                request.Timeout = 20000;
                request.ReadWriteTimeout = 600000;
                request.UserAgent = "AxiomOffice";
                if (!string.IsNullOrEmpty(Config.Token))
                {
                    request.Headers["X-Auth-Token"] = Config.Token;
                }

                using (cancel.Register(delegate { Abort(request); }))
                using (var response = (HttpWebResponse)request.GetResponse())
                using (var reader = new StreamReader(response.GetResponseStream(), Encoding.UTF8))
                {
                    string type = null;
                    while (!cancel.IsCancellationRequested)
                    {
                        string line = reader.ReadLine();
                        if (line == null)
                        {
                            break;
                        }

                        if (line.Length == 0)
                        {
                            continue;
                        }

                        if (line.StartsWith("event:", StringComparison.Ordinal))
                        {
                            type = line.Substring(6).Trim();
                            continue;
                        }

                        if (!line.StartsWith("data:", StringComparison.Ordinal))
                        {
                            continue;
                        }

                        string payload = line.Substring(5).Trim();
                        if (type == "ping")
                        {
                            continue;
                        }

                        CoreEvent item = ParseEvent(type, payload);
                        if (item == null)
                        {
                            continue;
                        }

                        if (item.ConversationId != null)
                        {
                            result.ConversationId = item.ConversationId;
                        }

                        onEvent(item);
                        ApplyEvent(result, item);

                        if (IsTerminal(item.Type))
                        {
                            return;
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                if (cancel.IsCancellationRequested)
                {
                    result.Cancelled = true;
                    return;
                }

                result.Error = ex is WebException ? "mat ket noi voi Agent Core: " + ex.Message : ex.GetType().Name + ": " + ex.Message;
                Logger.Info("CoreClient: doc su kien that bai: " + result.Error);
            }
            finally
            {
                if (result.Error == null && !result.Ok && !result.Cancelled && !result.TimedOut && !result.Stopped)
                {
                    result.Error = "Agent Core dong ket noi truoc khi tra loi";
                }
            }
        }

        private static void Abort(HttpWebRequest request)
        {
            try
            {
                request.Abort();
            }
            catch (Exception)
            {
            }
        }

        private static CoreEvent ParseEvent(string type, string payload)
        {
            try
            {
                var envelope = new JavaScriptSerializer().Deserialize<Dictionary<string, object>>(payload);
                if (envelope == null)
                {
                    return null;
                }

                var item = new CoreEvent { Type = type };
                if (envelope.ContainsKey("seq"))
                {
                    item.Seq = Convert.ToInt64(envelope["seq"]);
                }

                var data = envelope.ContainsKey("data") ? envelope["data"] as Dictionary<string, object> : null;
                if (data == null)
                {
                    return item;
                }

                item.RunId = Read(data, "runId");
                item.ConversationId = Read(data, "conversationId");
                item.Action = Read(data, "action");
                item.ParamsPreview = Read(data, "paramsPreview");
                item.ResultPreview = Read(data, "resultPreview");
                item.Reply = Read(data, "reply");
                item.Error = Read(data, "error");
                item.ErrorKind = Read(data, "kind");
                if (data.ContainsKey("ok"))
                {
                    item.Ok = Convert.ToBoolean(data["ok"]);
                }

                if (data.ContainsKey("seconds"))
                {
                    item.Seconds = Convert.ToDouble(data["seconds"]);
                }

                if (data.ContainsKey("rounds"))
                {
                    item.Rounds = Convert.ToInt32(data["rounds"]);
                }

                if (data.ContainsKey("inputTokens"))
                {
                    item.InputTokens = Convert.ToInt32(data["inputTokens"]);
                }

                if (data.ContainsKey("outputTokens"))
                {
                    item.OutputTokens = Convert.ToInt32(data["outputTokens"]);
                }

                return item;
            }
            catch (Exception ex)
            {
                Logger.Info("CoreClient: su kien hong (" + type + "): " + ex.Message);
                return null;
            }
        }

        private static string Read(Dictionary<string, object> data, string key)
        {
            object value;
            if (!data.TryGetValue(key, out value) || value == null)
            {
                return null;
            }

            return Convert.ToString(value);
        }

        private static bool IsTerminal(string type)
        {
            return type == "run.completed" || type == "run.failed" || type == "run.cancelled"
                || type == "run.timedout" || type == "run.stopped";
        }

        // Chuyen su kien ket thuc thanh ket qua cho pane (giong LlmResult cua che do in-process).
        private static void ApplyEvent(LlmResult result, CoreEvent item)
        {
            if (item.Seconds > 0)
            {
                result.Seconds = item.Seconds;
            }

            if (item.Rounds > 0)
            {
                result.Rounds = item.Rounds;
            }

            switch (item.Type)
            {
                case "run.completed":
                    result.Ok = true;
                    result.Text = item.Reply ?? result.Text;
                    break;
                case "run.cancelled":
                    result.Cancelled = true;
                    break;
                case "run.timedout":
                    result.TimedOut = true;
                    result.Error = item.Error ?? "agent timed out";
                    break;
                case "run.stopped":
                    result.Stopped = true;
                    result.Error = item.Error ?? "token budget exceeded";
                    break;
                case "run.failed":
                    result.Error = item.Error ?? "agent failed";
                    break;
            }
        }

        // Port + pid cua bridge trong TIEN TRINH NAY: doc tu session registry (dung khi port bi doi).
        // Core doi chieu pid voi bridge dang giu port nen gui pid ghi trong session file.
        private static int ResolvePort(Connect host, out int bridgePid)
        {
            bridgePid = Process.GetCurrentProcess().Id;
            try
            {
                string path = Path.Combine(SessionRegistry.Directory, bridgePid + ".json");
                if (File.Exists(path))
                {
                    var info = new JavaScriptSerializer().Deserialize<Dictionary<string, object>>(File.ReadAllText(path, Encoding.UTF8));
                    if (info != null && info.ContainsKey("port"))
                    {
                        int port = Convert.ToInt32(info["port"]);
                        if (port > 0)
                        {
                            if (info.ContainsKey("pid") && Convert.ToInt32(info["pid"]) > 0)
                            {
                                bridgePid = Convert.ToInt32(info["pid"]);
                            }
                            return port;
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                Logger.Info("CoreClient: doc session file that bai: " + ex.Message);
            }

            return host != null ? Config.PortForKind(host.AppKind, host.IsOfficeHost) : 0;
        }

        private static Dictionary<string, object> DocumentInfo(Connect host)
        {
            var document = new Dictionary<string, object>();
            try
            {
                Dictionary<string, object> info = HostProbe.ActiveDocument(host);
                if (info != null)
                {
                    document["name"] = info.ContainsKey("name") ? info["name"] : null;
                    document["fullName"] = info.ContainsKey("fullName") ? info["fullName"] : null;
                }
            }
            catch (Exception ex)
            {
                Logger.Info("CoreClient: doc thong tin tai lieu that bai: " + ex.Message);
            }

            return document;
        }

        private static string PostJson(string url, string json, int timeoutMs, CancellationToken cancel, out string error)
        {
            error = null;
            HttpWebRequest request = null;
            try
            {
                request = (HttpWebRequest)WebRequest.Create(url);
                request.Method = "POST";
                request.ContentType = "application/json";
                request.Timeout = timeoutMs;
                request.ReadWriteTimeout = timeoutMs;
                request.UserAgent = "AxiomOffice";
                if (!string.IsNullOrEmpty(Config.Token))
                {
                    request.Headers["X-Auth-Token"] = Config.Token;
                }

                byte[] data = Encoding.UTF8.GetBytes(json ?? "{}");
                request.ContentLength = data.Length;
                using (cancel.Register(delegate { Abort(request); }))
                {
                    using (Stream stream = request.GetRequestStream())
                    {
                        stream.Write(data, 0, data.Length);
                    }

                    using (var response = (HttpWebResponse)request.GetResponse())
                    using (var reader = new StreamReader(response.GetResponseStream(), Encoding.UTF8))
                    {
                        return reader.ReadToEnd();
                    }
                }
            }
            catch (WebException ex)
            {
                error = DescribeError(ex);
                return null;
            }
            catch (Exception ex)
            {
                error = ex.GetType().Name + ": " + ex.Message;
                return null;
            }
        }

        private static string GetText(string url, int timeoutMs, out string error)
        {
            error = null;
            try
            {
                var request = (HttpWebRequest)WebRequest.Create(url);
                request.Method = "GET";
                request.Timeout = timeoutMs;
                request.ReadWriteTimeout = timeoutMs;
                request.UserAgent = "AxiomOffice";
                if (!string.IsNullOrEmpty(Config.Token))
                {
                    request.Headers["X-Auth-Token"] = Config.Token;
                }

                using (var response = (HttpWebResponse)request.GetResponse())
                using (var reader = new StreamReader(response.GetResponseStream(), Encoding.UTF8))
                {
                    return reader.ReadToEnd();
                }
            }
            catch (WebException ex)
            {
                error = DescribeError(ex);
                return null;
            }
            catch (Exception ex)
            {
                error = ex.GetType().Name + ": " + ex.Message;
                return null;
            }
        }

        private static string DescribeError(WebException ex)
        {
            try
            {
                if (ex.Response != null)
                {
                    using (var reader = new StreamReader(ex.Response.GetResponseStream(), Encoding.UTF8))
                    {
                        string body = reader.ReadToEnd();
                        if (body.Length > 0)
                        {
                            return Truncate(body, 200);
                        }
                    }
                }
            }
            catch (Exception)
            {
            }

            return ex.Message;
        }

        private static string Serialize(object value)
        {
            return new JavaScriptSerializer().Serialize(value);
        }

        internal static Dictionary<string, object> RunReply(string responseText)
        {
            var reply = Deserialize(responseText) as Dictionary<string, object>;
            object inner;
            if (reply != null && !reply.ContainsKey("runId") && reply.TryGetValue("result", out inner) && inner is Dictionary<string, object>)
            {
                return (Dictionary<string, object>)inner;
            }
            return reply;
        }

        private static object Deserialize(string json)
        {
            try
            {
                return new JavaScriptSerializer().DeserializeObject(json);
            }
            catch (Exception)
            {
                return null;
            }
        }

        private static string Truncate(string value, int max)
        {
            if (string.IsNullOrEmpty(value) || value.Length <= max)
            {
                return value ?? "";
            }

            return value.Substring(0, max) + "...";
        }
    }
}
