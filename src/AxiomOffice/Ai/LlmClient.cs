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
        public double Seconds;
        public bool Cancelled;
        public bool TimedOut;
        public bool StepLimitReached;
        public bool Stopped;                 // dung vi tran token cua Agent Core
        public bool ViaCore;                 // luot chay qua Agent Core (khong phai in-process)
        public string ConversationId;        // hoi thoai ben Core (de "lam tiep")
        public int Rounds;
        public List<string> Transcript = new List<string>();
    }

    internal static class LlmClient
    {
        private const int TimeoutMs = 60000;

        // Trần thời gian cho cả một lượt agent (mọi vòng LLM + tool). Mỗi request HTTP vẫn có TimeoutMs riêng.
        public const int AgentTimeoutMs = 300000;

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

        public static LlmResult RunAgent(string systemPrompt, string userPrompt, List<LlmToolDef> tools,
            Func<string, string, string> executor, int maxIterations, Action<string> progress,
            CancellationToken cancel = default(CancellationToken))
        {
            var result = new LlmResult();
            var watch = Stopwatch.StartNew();
            var deadline = new CancellationTokenSource(AgentTimeoutMs);
            var linked = CancellationTokenSource.CreateLinkedTokenSource(cancel, deadline.Token);
            try
            {
                string endpoint = (Config.LlmEndpoint ?? "").Trim();
                if (endpoint.Length == 0)
                {
                    result.Error = "Endpoint is not configured";
                    return result;
                }
                if (endpoint.IndexOf("://") < 0)
                {
                    endpoint = "http://" + endpoint;
                }
                string model = Config.LlmModel;
                if (string.IsNullOrEmpty(model))
                {
                    result.Error = "Model is not configured";
                    return result;
                }
                string apiKey = Config.LlmApiKey;
                var serializer = new JavaScriptSerializer();

                if (Config.LlmProvider == "anthropic")
                {
                    RunAnthropicAgent(result, endpoint, apiKey, model, systemPrompt, userPrompt, tools, executor, maxIterations, progress, serializer, linked.Token);
                }
                else
                {
                    RunOpenAiAgent(result, endpoint, apiKey, model, systemPrompt, userPrompt, tools, executor, maxIterations, progress, serializer, linked.Token);
                }
            }
            catch (Exception ex)
            {
                result.Error = ex.Message;
            }
            finally
            {
                watch.Stop();
                result.Seconds = watch.Elapsed.TotalSeconds;
                if (linked.IsCancellationRequested)
                {
                    result.Ok = false;
                    if (cancel.IsCancellationRequested)
                    {
                        result.Cancelled = true;
                        result.Error = "cancelled by user";
                    }
                    else
                    {
                        result.TimedOut = true;
                        result.Error = "agent timed out after " + (AgentTimeoutMs / 1000) + "s";
                    }
                }
                linked.Dispose();
                deadline.Dispose();
            }
            return result;
        }

        private static void RunOpenAiAgent(LlmResult result, string endpoint, string apiKey, string model,
            string systemPrompt, string userPrompt, List<LlmToolDef> tools,
            Func<string, string, string> executor, int maxIterations, Action<string> progress,
            JavaScriptSerializer serializer, CancellationToken cancel)
        {
            var messages = new List<object>();
            messages.Add(new Dictionary<string, object> { { "role", "system" }, { "content", systemPrompt } });
            messages.Add(new Dictionary<string, object> { { "role", "user" }, { "content", userPrompt } });

            object[] toolDefs = BuildOpenAiTools(tools, serializer);
            bool toolsEnabled = true;
            string finalText = null;

            // maxIterations <= 0: không giới hạn số vòng (vẫn có trần AgentTimeoutMs và hủy từ người dùng).
            for (int iteration = 0; (maxIterations <= 0 || iteration < maxIterations) && finalText == null; iteration++)
            {
                if (cancel.IsCancellationRequested)
                {
                    return;
                }
                result.Rounds = iteration + 1;
                var body = new Dictionary<string, object>();
                body["model"] = model;
                body["messages"] = messages.ToArray();
                if (toolsEnabled)
                {
                    body["tools"] = toolDefs;
                }

                string error;
                string responseText = PostJson(BuildUrl(endpoint, "/chat/completions"), serializer.Serialize(body),
                    delegate(HttpWebRequest request)
                    {
                        if (!string.IsNullOrEmpty(apiKey))
                        {
                            request.Headers["Authorization"] = "Bearer " + apiKey;
                        }
                    }, cancel, out error);
                if (responseText == null)
                {
                    if (cancel.IsCancellationRequested)
                    {
                        return;
                    }
                    if (toolsEnabled && error != null && error.ToLowerInvariant().Contains("tool"))
                    {
                        toolsEnabled = false;
                        AddTranscript(result, progress, "(provider khong ho tro tools - chay che do thuong)");
                        continue;
                    }
                    result.Error = error;
                    return;
                }

                var parsed = Parse(responseText);
                var message = Dig(parsed, "choices", 0, "message") as Dictionary<string, object>;
                if (message == null)
                {
                    result.Error = "unexpected provider response";
                    return;
                }
                object toolCallsValue;
                object[] toolCalls = message.TryGetValue("tool_calls", out toolCallsValue) ? toolCallsValue as object[] : null;
                if (toolCalls == null || toolCalls.Length == 0)
                {
                    finalText = message.ContainsKey("content") ? message["content"] as string : null;
                    break;
                }

                messages.Add(message);
                foreach (object item in toolCalls)
                {
                    var call = item as Dictionary<string, object>;
                    if (call == null)
                    {
                        continue;
                    }
                    string id = Convert.ToString(call.ContainsKey("id") ? call["id"] : "");
                    var function = call.ContainsKey("function") ? call["function"] as Dictionary<string, object> : null;
                    string name = function != null ? Convert.ToString(function["name"]) : "";
                    string arguments = function != null && function.ContainsKey("arguments") ? Convert.ToString(function["arguments"]) : "{}";
                    if (cancel.IsCancellationRequested)
                    {
                        return;
                    }
                    string execResult = ExecuteTool(executor, name, arguments);
                    AddTranscript(result, progress, ToolLine(name, arguments, execResult));
                    messages.Add(new Dictionary<string, object>
                    {
                        { "role", "tool" },
                        { "tool_call_id", id },
                        { "content", execResult }
                    });
                }
            }
            FinishAgent(result, finalText, maxIterations);
        }

        private static void RunAnthropicAgent(LlmResult result, string endpoint, string apiKey, string model,
            string systemPrompt, string userPrompt, List<LlmToolDef> tools,
            Func<string, string, string> executor, int maxIterations, Action<string> progress,
            JavaScriptSerializer serializer, CancellationToken cancel)
        {
            var messages = new List<object>();
            messages.Add(new Dictionary<string, object> { { "role", "user" }, { "content", userPrompt } });

            object[] toolDefs = BuildAnthropicTools(tools, serializer);
            bool toolsEnabled = true;
            string finalText = null;

            // maxIterations <= 0: không giới hạn số vòng (vẫn có trần AgentTimeoutMs và hủy từ người dùng).
            for (int iteration = 0; (maxIterations <= 0 || iteration < maxIterations) && finalText == null; iteration++)
            {
                if (cancel.IsCancellationRequested)
                {
                    return;
                }
                result.Rounds = iteration + 1;
                var body = new Dictionary<string, object>();
                body["model"] = model;
                body["max_tokens"] = 4096;
                body["system"] = systemPrompt;
                body["messages"] = messages.ToArray();
                if (toolsEnabled)
                {
                    body["tools"] = toolDefs;
                }

                string error;
                string responseText = PostJson(BuildUrl(endpoint, "/messages"), serializer.Serialize(body),
                    delegate(HttpWebRequest request)
                    {
                        if (!string.IsNullOrEmpty(apiKey))
                        {
                            request.Headers["x-api-key"] = apiKey;
                        }
                        request.Headers["anthropic-version"] = "2023-06-01";
                    }, cancel, out error);
                if (responseText == null)
                {
                    if (cancel.IsCancellationRequested)
                    {
                        return;
                    }
                    if (toolsEnabled && error != null && error.ToLowerInvariant().Contains("tool"))
                    {
                        toolsEnabled = false;
                        AddTranscript(result, progress, "(provider khong ho tro tools - chay che do thuong)");
                        continue;
                    }
                    result.Error = error;
                    return;
                }

                var contentBlocks = Dig(Parse(responseText), "content") as object[];
                if (contentBlocks == null)
                {
                    result.Error = "unexpected provider response";
                    return;
                }
                var textParts = new List<string>();
                var toolUses = new List<Dictionary<string, object>>();
                foreach (object blockItem in contentBlocks)
                {
                    var block = blockItem as Dictionary<string, object>;
                    if (block == null)
                    {
                        continue;
                    }
                    string type = Convert.ToString(block.ContainsKey("type") ? block["type"] : "");
                    if (type == "text" && block.ContainsKey("text"))
                    {
                        textParts.Add(Convert.ToString(block["text"]));
                    }
                    else if (type == "tool_use")
                    {
                        toolUses.Add(block);
                    }
                }
                if (toolUses.Count == 0)
                {
                    finalText = string.Join("\n", textParts.ToArray());
                    break;
                }

                messages.Add(new Dictionary<string, object> { { "role", "assistant" }, { "content", contentBlocks } });
                var toolResults = new List<object>();
                foreach (var toolUse in toolUses)
                {
                    string id = Convert.ToString(toolUse.ContainsKey("id") ? toolUse["id"] : "");
                    string name = Convert.ToString(toolUse.ContainsKey("name") ? toolUse["name"] : "");
                    string inputJson = toolUse.ContainsKey("input") ? serializer.Serialize(toolUse["input"]) : "{}";
                    if (cancel.IsCancellationRequested)
                    {
                        return;
                    }
                    string execResult = ExecuteTool(executor, name, inputJson);
                    AddTranscript(result, progress, ToolLine(name, inputJson, execResult));
                    toolResults.Add(new Dictionary<string, object>
                    {
                        { "type", "tool_result" },
                        { "tool_use_id", id },
                        { "content", execResult }
                    });
                }
                messages.Add(new Dictionary<string, object> { { "role", "user" }, { "content", toolResults.ToArray() } });
            }
            FinishAgent(result, finalText, maxIterations);
        }

        private static void FinishAgent(LlmResult result, string finalText, int maxIterations)
        {
            if (finalText == null)
            {
                // Chỉ xảy ra khi bên gọi tự đặt giới hạn vòng: báo là chưa xong, không giả làm câu trả lời.
                result.Ok = false;
                result.StepLimitReached = true;
                result.Error = "agent stopped after " + maxIterations + " rounds without a final answer";
                return;
            }
            result.Text = StripThoughts(finalText);
            result.Ok = true;
        }

        // Một số model (Gemma 4 qua endpoint OpenAI-compatible của Google, DeepSeek/Qwen...) trả kèm phần suy
        // nghĩ <thought>...</thought> / <think>...</think> trong content: bỏ khỏi câu trả lời hiện cho người dùng.
        private static readonly System.Text.RegularExpressions.Regex ThoughtBlock = new System.Text.RegularExpressions.Regex(
            @"<(thought|think|thinking)>[\s\S]*?</\1>", System.Text.RegularExpressions.RegexOptions.IgnoreCase);

        internal static string StripThoughts(string text)
        {
            return string.IsNullOrEmpty(text) ? text : ThoughtBlock.Replace(text, "").Trim();
        }

        private static void AddTranscript(LlmResult result, Action<string> progress, string line)
        {
            result.Transcript.Add(line);
            if (progress != null)
            {
                try
                {
                    progress(line);
                }
                catch
                {
                }
            }
        }

        private static string ExecuteTool(Func<string, string, string> executor, string name, string arguments)
        {
            try
            {
                return executor(name, arguments);
            }
            catch (Exception ex)
            {
                return "{\"ok\":false,\"error\":\"" + ex.Message.Replace("\"", "'") + "\"}";
            }
        }

        private static string ToolLine(string name, string arguments, string execResult)
        {
            return name + " " + Truncate(arguments, 150) + " -> " + Truncate(execResult, 150);
        }

        private static object[] BuildOpenAiTools(List<LlmToolDef> tools, JavaScriptSerializer serializer)
        {
            var list = new List<object>();
            foreach (var tool in tools)
            {
                list.Add(new Dictionary<string, object>
                {
                    { "type", "function" },
                    { "function", new Dictionary<string, object>
                        {
                            { "name", tool.Name },
                            { "description", tool.Description },
                            { "parameters", serializer.DeserializeObject(tool.ParametersJson) }
                        }
                    }
                });
            }
            return list.ToArray();
        }

        private static object[] BuildAnthropicTools(List<LlmToolDef> tools, JavaScriptSerializer serializer)
        {
            var list = new List<object>();
            foreach (var tool in tools)
            {
                list.Add(new Dictionary<string, object>
                {
                    { "name", tool.Name },
                    { "description", tool.Description },
                    { "input_schema", serializer.DeserializeObject(tool.ParametersJson) }
                });
            }
            return list.ToArray();
        }
    }
}
