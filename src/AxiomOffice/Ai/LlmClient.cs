using System;
using System.Collections;
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
    internal sealed class LlmResult
    {
        public bool Ok;
        public string Text;
        public string Error;
        // ErrorKind: loai loi Core bao (provider | config | internal | office | budget ...) - phan biet loi may chu AI
        // voi viec Core chu dong dung luot (vd lap lai mot lenh hong). Rong khi khong qua Core.
        public string ErrorKind;
        public double Seconds;
        public bool Cancelled;
        public bool TimedOut;
        public bool Stopped;                 // dung vi tran token cua Agent Core
        public bool ViaCore;                 // luot chay qua Agent Core (khong phai in-process)
        public string ConversationId;        // hoi thoai ben Core (de "lam tiep")
        public string RunId;                 // luot chay ben Core (tim memory trich xuat nen cua luot)
        public int Rounds;
        // UndoSteps: so buoc hoan tac Word chac chan cua luot do Core tinh (da tru lan agent tu undo); -1 = Core khong bao
        // (Core cu, hoac luot in-process) -> pane tu dem nhu truoc.
        public int UndoSteps = -1;
        // EditSteps: so lenh sua tai lieu chac chan cua luot (moi app) do Core bao; -1 = khong biet.
        public int EditSteps = -1;
        // UndoMode: "revisions" = lượt sửa bằng Track Changes (hoàn tác = từ chối revision của lượt); "steps"/null = Ctrl+Z.
        public string UndoMode;
        public List<string> Transcript = new List<string>();
    }

    internal static class LlmClient
    {
        // 120s mỗi request (khớp Core ModelClient.DefaultRequestTimeoutMs): model free chậm có lúc cần hơn 60s.
        private const int TimeoutMs = 120000;

        public static LlmResult Chat(string systemPrompt, string userPrompt)
        {
            return Chat(systemPrompt, userPrompt, Config.LlmProvider, Config.LlmEndpoint, Config.LlmApiKey, Config.LlmModel);
        }

        public static LlmResult Chat(string systemPrompt, string userPrompt, string provider, string endpoint, string apiKey, string model)
        {
            var result = new LlmResult();
            var sw = Stopwatch.StartNew();
            try
            {
                endpoint = (endpoint ?? "").Trim();
                if (endpoint.Length == 0)
                {
                    result.Error = "Endpoint is not configured";
                    return result;
                }
                if (endpoint.IndexOf("://") < 0)
                {
                    endpoint = "http://" + endpoint;
                }
                if (string.IsNullOrEmpty(model))
                {
                    result.Error = "Model is not configured";
                    return result;
                }

                string responseText;
                string error;
                if (provider == "anthropic")
                {
                    responseText = CallAnthropic(endpoint, apiKey, model, systemPrompt, userPrompt, out error);
                    if (responseText == null)
                    {
                        result.Error = error;
                        return result;
                    }
                    var parsed = Parse(responseText);
                    result.Text = Dig(parsed, "content", 0, "text") as string;
                }
                else
                {
                    responseText = CallOpenAi(endpoint, apiKey, model, systemPrompt, userPrompt, out error);
                    if (responseText == null)
                    {
                        result.Error = error;
                        return result;
                    }
                    var parsed = Parse(responseText);
                    result.Text = Dig(parsed, "choices", 0, "message", "content") as string;
                }

                if (result.Text == null)
                {
                    result.Error = "Could not find text in provider response";
                    return result;
                }
                result.Text = StripThoughts(result.Text);
                result.Ok = true;
                return result;
            }
            catch (Exception ex)
            {
                result.Error = ex.Message;
                return result;
            }
            finally
            {
                sw.Stop();
                result.Seconds = sw.Elapsed.TotalSeconds;
            }
        }

        private static string CallOpenAi(string endpoint, string apiKey, string model, string systemPrompt, string userPrompt, out string error)
        {
            string url = BuildUrl(endpoint, "/chat/completions");
            var body = new Dictionary<string, object>();
            body["model"] = model;
            body["messages"] = new object[]
            {
                new Dictionary<string, object> { { "role", "system" }, { "content", systemPrompt } },
                new Dictionary<string, object> { { "role", "user" }, { "content", userPrompt } }
            };
            string json = new JavaScriptSerializer().Serialize(body);
            return PostJson(url, json, delegate(HttpWebRequest request)
            {
                if (!string.IsNullOrEmpty(apiKey))
                {
                    request.Headers["Authorization"] = "Bearer " + apiKey;
                }
            }, out error);
        }

        private static string CallAnthropic(string endpoint, string apiKey, string model, string systemPrompt, string userPrompt, out string error)
        {
            string url = BuildUrl(endpoint, "/messages");
            var body = new Dictionary<string, object>();
            body["model"] = model;
            body["max_tokens"] = 2048;
            body["system"] = systemPrompt;
            body["messages"] = new object[]
            {
                new Dictionary<string, object> { { "role", "user" }, { "content", userPrompt } }
            };
            string json = new JavaScriptSerializer().Serialize(body);
            return PostJson(url, json, delegate(HttpWebRequest request)
            {
                if (!string.IsNullOrEmpty(apiKey))
                {
                    request.Headers["x-api-key"] = apiKey;
                }
                request.Headers["anthropic-version"] = "2023-06-01";
            }, out error);
        }

        private static string BuildUrl(string endpoint, string suffix)
        {
            string baseUrl = endpoint.TrimEnd('/');
            if (baseUrl.EndsWith(suffix, StringComparison.OrdinalIgnoreCase))
            {
                return baseUrl;
            }
            return baseUrl + suffix;
        }

        private static string PostJson(string url, string json, Action<HttpWebRequest> configure, out string error)
        {
            return PostJson(url, json, configure, CancellationToken.None, out error);
        }

        // Lỗi tạm thời của nhà cung cấp (429/500/502/503/504, rớt mạng): thử lại có chờ tăng dần, giống Core
        // (ModelClient.PostAsync). Không thử lại khi hết giờ một request hay khi người dùng hủy.
        internal static readonly int[] RetryDelaysMs = { 1000, 2000, 4000 };
        private const int MaxRetryAfterMs = 10000;

        internal static bool IsTransient(int status, WebExceptionStatus network)
        {
            if (status == 429 || status == 500 || status == 502 || status == 503 || status == 504)
            {
                return true;
            }
            return status == 0 && (network == WebExceptionStatus.ConnectFailure
                || network == WebExceptionStatus.ConnectionClosed
                || network == WebExceptionStatus.ReceiveFailure
                || network == WebExceptionStatus.SendFailure
                || network == WebExceptionStatus.KeepAliveFailure
                || network == WebExceptionStatus.NameResolutionFailure);
        }

        private static string PostJson(string url, string json, Action<HttpWebRequest> configure, CancellationToken cancel, out string error)
        {
            for (int attempt = 0; ; attempt++)
            {
                int status;
                WebExceptionStatus network;
                int retryAfterMs;
                string text = PostJsonOnce(url, json, configure, cancel, out error, out status, out network, out retryAfterMs);
                if (text != null || cancel.IsCancellationRequested || attempt >= RetryDelaysMs.Length || !IsTransient(status, network))
                {
                    return text;
                }
                int delay = retryAfterMs > 0 ? Math.Min(retryAfterMs, MaxRetryAfterMs) : RetryDelaysMs[attempt];
                Logger.Info("LlmClient: " + (status > 0 ? "HTTP " + status : network.ToString()) + " - retry " + (attempt + 1) + "/" + RetryDelaysMs.Length + " in " + delay + "ms");
                if (cancel.WaitHandle.WaitOne(delay))
                {
                    error = "cancelled";
                    return null;
                }
            }
        }

        private static string PostJsonOnce(string url, string json, Action<HttpWebRequest> configure, CancellationToken cancel,
            out string error, out int status, out WebExceptionStatus network, out int retryAfterMs)
        {
            error = null;
            status = 0;
            network = WebExceptionStatus.Success;
            retryAfterMs = 0;
            if (cancel.IsCancellationRequested)
            {
                error = "cancelled";
                return null;
            }
            try
            {
                var request = (HttpWebRequest)WebRequest.Create(url);
                request.Method = "POST";
                request.ContentType = "application/json";
                request.Timeout = TimeoutMs;
                request.ReadWriteTimeout = TimeoutMs;
                request.UserAgent = "AxiomOffice";
                if (configure != null)
                {
                    configure(request);
                }
                byte[] data = Encoding.UTF8.GetBytes(json);
                request.ContentLength = data.Length;
                // Hủy / hết giờ tổng: Abort() làm request đang chờ ném WebException(RequestCanceled) ngay.
                using (cancel.Register(delegate { request.Abort(); }))
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
            catch (WebException wex)
            {
                if (cancel.IsCancellationRequested)
                {
                    error = "cancelled";
                    return null;
                }
                network = wex.Status;
                var http = wex.Response as HttpWebResponse;
                if (http != null)
                {
                    status = (int)http.StatusCode;
                    int seconds;
                    if (int.TryParse(http.Headers["Retry-After"], out seconds) && seconds > 0)
                    {
                        retryAfterMs = seconds * 1000;
                    }
                }
                string body = "";
                try
                {
                    if (wex.Response != null)
                    {
                        using (var reader = new StreamReader(wex.Response.GetResponseStream(), Encoding.UTF8))
                        {
                            body = reader.ReadToEnd();
                        }
                    }
                }
                catch
                {
                }
                error = wex.Message;
                if (body.Length > 0)
                {
                    error += " | " + Truncate(body, 400);
                }
                return null;
            }
            catch (Exception ex)
            {
                error = ex.Message;
                return null;
            }
        }

        private static object Parse(string json)
        {
            try
            {
                return new JavaScriptSerializer().DeserializeObject(json);
            }
            catch
            {
                return null;
            }
        }

        private static object Dig(object root, params object[] path)
        {
            object current = root;
            foreach (object step in path)
            {
                if (current == null)
                {
                    return null;
                }
                var dict = current as Dictionary<string, object>;
                if (dict != null)
                {
                    object next;
                    if (!dict.TryGetValue((string)step, out next))
                    {
                        return null;
                    }
                    current = next;
                    continue;
                }
                var array = current as object[];
                if (array != null && step is int)
                {
                    int index = (int)step;
                    if (index < 0 || index >= array.Length)
                    {
                        return null;
                    }
                    current = array[index];
                    continue;
                }
                var list = current as ArrayList;
                if (list != null && step is int)
                {
                    int index = (int)step;
                    if (index < 0 || index >= list.Count)
                    {
                        return null;
                    }
                    current = list[index];
                    continue;
                }
                return null;
            }
            return current;
        }

        private static string Truncate(string s, int max)
        {
            if (string.IsNullOrEmpty(s) || s.Length <= max)
            {
                return s ?? "";
            }
            return s.Substring(0, max) + "...";
        }

        // Một số model (Gemma 4 qua endpoint OpenAI-compatible của Google, DeepSeek/Qwen...) trả kèm phần suy
        // nghĩ <thought>...</thought> / <think>...</think> trong content: bỏ khỏi câu trả lời hiện cho người dùng.
        private static readonly System.Text.RegularExpressions.Regex ThoughtBlock = new System.Text.RegularExpressions.Regex(
            @"<(thought|think|thinking)>[\s\S]*?</\1>", System.Text.RegularExpressions.RegexOptions.IgnoreCase);

        internal static string StripThoughts(string text)
        {
            return string.IsNullOrEmpty(text) ? text : ThoughtBlock.Replace(text, "").Trim();
        }
    }
}
