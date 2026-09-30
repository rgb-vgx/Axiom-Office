using System;
using System.Collections.Generic;
using System.Threading;
using System.Web.Script.Serialization;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ai
{
    // Ket qua mot lan thu ket noi LLM (/v1/llm/test): Ok=true kem Reply/Seconds, hoac Ok=false kem
    // Kind + Message + Hint (cau tieng Viet do Core dich san trong Setup/LlmErrors.cs).
    internal sealed class LlmProbe
    {
        public bool Ok;
        public string Kind;
        public string Message;
        public string Hint;
        public string Detail;
        public string Reply;
        public string Model;
        public double Seconds;
    }

    // Du lieu cua GET /v1/setup: preset nha cung cap, buoc, tinh nang, viec can lam va cau hinh dang chay.
    internal sealed class SetupInfo
    {
        public string Version;
        public List<Dictionary<string, object>> Presets = new List<Dictionary<string, object>>();
        public List<Dictionary<string, object>> Steps = new List<Dictionary<string, object>>();
        public List<Dictionary<string, object>> Features = new List<Dictionary<string, object>>();
        public List<Dictionary<string, object>> Problems = new List<Dictionary<string, object>>();
        public Dictionary<string, object> Current = new Dictionary<string, object>();
        public Dictionary<string, object> Core = new Dictionary<string, object>();

        public string ProviderId { get { return Text("providerId", Current, ""); } }

        internal static string Text(string key, Dictionary<string, object> from, string fallback)
        {
            if (from == null || !from.ContainsKey(key) || from[key] == null)
            {
                return fallback;
            }

            string value = Convert.ToString(from[key]);
            return string.IsNullOrEmpty(value) ? fallback : value;
        }

        internal static bool Flag(string key, Dictionary<string, object> from, bool fallback)
        {
            if (from == null || !from.ContainsKey(key) || from[key] == null)
            {
                return fallback;
            }

            try
            {
                return Convert.ToBoolean(from[key]);
            }
            catch
            {
                return fallback;
            }
        }
    }

    // Phan Core API danh cho wizard thiet lap (New_arch.md 7.3; Core: Api/SetupEndpoints.cs).
    // Tach khoi CoreClient.cs cho de doc; cung mot lop nho partial.
    internal sealed partial class CoreClient
    {
        private const int SetupTimeoutMs = 8000;
        private const int LlmTestTimeoutMs = 45000;   // Core tu dat han 30s cho mot lan thu

        // GET /v1/setup. Core chua chay -> null (wizard dung ban du phong tu SetupCatalog).
        public SetupInfo Setup(out string error)
        {
            error = null;
            string baseUrl = EnsureBaseUrl();
            if (baseUrl == null)
            {
                error = LastError;
                return null;
            }

            string text = GetText(baseUrl + "/v1/setup", SetupTimeoutMs, out error);
            if (text == null)
            {
                return null;
            }

            return ParseSetup(text, out error);
        }

        internal static SetupInfo ParseSetup(string text, out string error)
        {
            error = null;
            try
            {
                var serializer = new JavaScriptSerializer();
                var root = serializer.Deserialize<Dictionary<string, object>>(text);
                if (root == null || !root.ContainsKey("result") || root["result"] == null)
                {
                    error = "Core tra ve du lieu la";
                    return null;
                }

                var result = root["result"] as Dictionary<string, object>;
                if (result == null)
                {
                    error = "Core tra ve du lieu la";
                    return null;
                }

                var info = new SetupInfo();
                info.Version = SetupInfo.Text("version", result, "");
                info.Presets = ListOf(result, "presets");
                info.Steps = ListOf(result, "steps");
                info.Features = ListOf(result, "features");
                info.Problems = ListOf(result, "problems");
                info.Current = MapOf(result, "current");
                info.Core = MapOf(result, "core");
                return info;
            }
            catch (Exception ex)
            {
                error = ex.GetType().Name + ": " + ex.Message;
                return null;
            }
        }

        // POST /v1/llm/test: thu ket noi voi gia tri NGUOI DUNG VUA NHAP (chua luu). apiKey rong = dung khoa da luu.
        public LlmProbe TestLlm(string providerId, string codec, string endpoint, string model, string apiKey, CancellationToken cancel)
        {
            var probe = new LlmProbe { Kind = "internal", Message = "Không kiểm tra được kết nối." };
            string baseUrl = EnsureBaseUrl();
            if (baseUrl == null)
            {
                probe.Message = "Agent Core chưa chạy.";
                probe.Hint = "Mở lại ứng dụng hoặc bấm \"Khởi động lại Core\" rồi thử lại.";
                probe.Detail = LastError;
                return probe;
            }

            var body = new Dictionary<string, object>
            {
                { "providerId", providerId ?? "" },
                { "provider", codec ?? "" },
                { "endpoint", endpoint ?? "" },
                { "model", model ?? "" },
                { "apiKey", apiKey ?? "" }
            };

            string error;
            string text = PostJson(baseUrl + "/v1/llm/test", Serialize(body), LlmTestTimeoutMs, cancel, out error);
            if (text == null)
            {
                probe.Message = "Không gọi được Agent Core.";
                probe.Detail = error;
                return probe;
            }

            var serializer = new JavaScriptSerializer();
            try
            {
                var root = serializer.Deserialize<Dictionary<string, object>>(text);
                var result = MapOf(root, "result");
                if (result == null)
                {
                    probe.Detail = Truncate(text, 200);
                    return probe;
                }

                if (result.ContainsKey("seconds") || (result.ContainsKey("reply") && !result.ContainsKey("kind")))
                {
                    probe.Ok = true;
                    probe.Reply = SetupInfo.Text("reply", result, "OK");
                    probe.Model = SetupInfo.Text("model", result, model);
                    probe.Seconds = Number(result, "seconds");
                    return probe;
                }

                probe.Kind = SetupInfo.Text("kind", result, "internal");
                probe.Message = SetupInfo.Text("message", result, "Không kiểm tra được kết nối.");
                probe.Hint = SetupInfo.Text("hint", result, "");
                probe.Detail = SetupInfo.Text("detail", result, "");
                return probe;
            }
            catch (Exception ex)
            {
                probe.Detail = ex.GetType().Name + ": " + ex.Message;
                return probe;
            }
        }

        // GET /v1/llm/models: danh sach model cua may chu de nguoi dung chon thay vi tu go ten.
        public List<string> ListModels(string codec, string endpoint, string apiKey, out string error, out string hint)
        {
            error = null;
            hint = null;
            string baseUrl = EnsureBaseUrl();
            if (baseUrl == null)
            {
                error = "Agent Core chưa chạy.";
                return null;
            }

            string url = baseUrl + "/v1/llm/models?provider=" + Uri.EscapeDataString(codec ?? "")
                + "&endpoint=" + Uri.EscapeDataString(endpoint ?? "")
                + "&apiKey=" + Uri.EscapeDataString(apiKey ?? "");
            string text = GetText(url, LlmTestTimeoutMs, out error);
            if (text == null)
            {
                return null;
            }

            try
            {
                var serializer = new JavaScriptSerializer();
                var result = MapOf(serializer.Deserialize<Dictionary<string, object>>(text), "result");
                if (result == null)
                {
                    error = "Core tra ve du lieu la";
                    return null;
                }

                if (result.ContainsKey("kind"))
                {
                    error = SetupInfo.Text("message", result, "Không lấy được danh sách model.");
                    hint = SetupInfo.Text("hint", result, "");
                    return null;
                }

                var models = new List<string>();
                if (result.ContainsKey("models") && result["models"] is object[])
                {
                    foreach (object item in (object[])result["models"])
                    {
                        string name = Convert.ToString(item);
                        if (!string.IsNullOrEmpty(name))
                        {
                            models.Add(name);
                        }
                    }
                }

                return models;
            }
            catch (Exception ex)
            {
                error = ex.GetType().Name + ": " + ex.Message;
                return null;
            }
        }

        // Khoi dong lai Core (sau khi doi Token / CorePort): dung ban dang chay roi mo lai nhu binh thuong.
        public bool RestartCore(out string error)
        {
            error = null;
            string baseUrl = EnsureBaseUrl();
            if (baseUrl != null)
            {
                string shutdownError;
                PostJson(baseUrl + "/v1/admin/shutdown", "{}", 5000, CancellationToken.None, out shutdownError);
                // Cho tien trinh cu tra port + xoa core.json truoc khi khoi dong lai.
                for (int i = 0; i < 40; i++)
                {
                    Thread.Sleep(100);
                    if (!System.IO.File.Exists(CoreJsonPath))
                    {
                        break;
                    }
                }
            }

            lock (_sync)
            {
                _baseUrl = null;
                _baseUrlCheckedUtc = DateTime.MinValue;
                _lastFailureUtc = DateTime.MinValue;
            }

            string url = EnsureBaseUrl();
            if (url == null)
            {
                error = LastError;
                return false;
            }

            return true;
        }

        // Sinh Token moi (khoa bao ve giua add-in va Core) va tra ve gia tri da luu.
        public static string GenerateToken()
        {
            byte[] bytes = new byte[24];
            using (var random = System.Security.Cryptography.RandomNumberGenerator.Create())
            {
                random.GetBytes(bytes);
            }

            string token = "t-" + BitConverter.ToString(bytes).Replace("-", "").ToLowerInvariant();
            Config.WriteString("Token", token);
            return token;
        }

        internal static List<Dictionary<string, object>> ListOf(Dictionary<string, object> from, string key)
        {
            var list = new List<Dictionary<string, object>>();
            if (from == null || !from.ContainsKey(key) || !(from[key] is object[]))
            {
                return list;
            }

            foreach (object item in (object[])from[key])
            {
                var map = item as Dictionary<string, object>;
                if (map != null)
                {
                    list.Add(map);
                }
            }

            return list;
        }

        internal static Dictionary<string, object> MapOf(Dictionary<string, object> from, string key)
        {
            if (from == null || !from.ContainsKey(key))
            {
                return null;
            }

            return from[key] as Dictionary<string, object>;
        }

        internal static double Number(Dictionary<string, object> from, string key)
        {
            if (from == null || !from.ContainsKey(key) || from[key] == null)
            {
                return 0;
            }

            try
            {
                return Convert.ToDouble(from[key]);
            }
            catch
            {
                return 0;
            }
        }
    }
}
