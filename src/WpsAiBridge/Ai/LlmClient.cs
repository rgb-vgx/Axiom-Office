using System;
using System.Collections;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Text;
using System.Web.Script.Serialization;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
{
    internal sealed class LlmResult
    {
        public bool Ok;
        public string Text;
        public string Error;
        public double Seconds;
    }

    internal static class LlmClient
    {
        private const int TimeoutMs = 60000;

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
            error = null;
            try
            {
                var request = (HttpWebRequest)WebRequest.Create(url);
                request.Method = "POST";
                request.ContentType = "application/json";
                request.Timeout = TimeoutMs;
                request.ReadWriteTimeout = TimeoutMs;
                request.UserAgent = "WpsAiBridge";
                if (configure != null)
                {
                    configure(request);
                }
                byte[] data = Encoding.UTF8.GetBytes(json);
                request.ContentLength = data.Length;
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
            catch (WebException wex)
            {
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
    }
}
