using System;
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Text;
using System.Threading;
using System.Web.Script.Serialization;

namespace WpsAiBridge.Bridge
{
    internal sealed class HttpBridge
    {
        private readonly IAppHost _host;
        private readonly JavaScriptSerializer _json = new JavaScriptSerializer();
        private HttpListener _listener;
        private Thread _thread;
        private volatile bool _running;
        private int _port;

        public HttpBridge(IAppHost host)
        {
            _host = host;
        }

        public void Start()
        {
            if (!Config.Enabled)
            {
                Logger.Info("HttpBridge disabled by config (HKCU\\Software\\WpsAiBridge\\Enabled=0)");
                return;
            }

            _port = Config.PortForKind(_host.AppKind);
            _listener = new HttpListener();
            _listener.Prefixes.Add("http://localhost:" + _port + "/");
            _listener.Prefixes.Add("http://127.0.0.1:" + _port + "/");
            _listener.Start();

            _running = true;
            _thread = new Thread(Pump);
            _thread.IsBackground = true;
            _thread.Name = "WpsAiBridge.Http";
            _thread.SetApartmentState(ApartmentState.STA);
            _thread.Start();

            Logger.Info(string.Format("HttpBridge listening on http://127.0.0.1:{0}/ (kind={1})", _port, _host.AppKind));
        }

        public void Stop()
        {
            _running = false;
            try
            {
                if (_listener != null)
                {
                    _listener.Stop();
                    _listener.Close();
                    _listener = null;
                }
            }
            catch (Exception ex)
            {
                Logger.Error("HttpBridge.Stop failed", ex);
            }
            Logger.Info("HttpBridge stopped");
        }

        private void Pump()
        {
            while (_running)
            {
                HttpListenerContext context;
                try
                {
                    context = _listener.GetContext();
                }
                catch (Exception)
                {
                    if (!_running)
                    {
                        return;
                    }
                    Thread.Sleep(100);
                    continue;
                }

                try
                {
                    Handle(context);
                }
                catch (Exception ex)
                {
                    Logger.Error("Request handling failed", ex);
                    try
                    {
                        WriteJson(context, 500, Error("internal error: " + ex.Message));
                    }
                    catch
                    {
                    }
                }
            }
        }

        private void Handle(HttpListenerContext context)
        {
            string path = context.Request.Url.AbsolutePath.TrimEnd('/');
            if (path.Length == 0)
            {
                path = "/";
            }

            string token = Config.Token;
            if (!string.IsNullOrEmpty(token))
            {
                string provided = context.Request.Headers["X-Auth-Token"];
                if (provided != token)
                {
                    WriteJson(context, 401, Error("unauthorized"));
                    return;
                }
            }

            if (path == "/health" && context.Request.HttpMethod == "GET")
            {
                WriteJson(context, 200, CommandDispatcher.Health(_host, _port));
                return;
            }

            if (path == "/config" && context.Request.HttpMethod == "GET")
            {
                WriteJson(context, 200, CommandDispatcher.ConfigInfo());
                return;
            }

            if (path == "/cmd" && context.Request.HttpMethod == "POST")
            {
                string body = ReadBody(context);
                Dictionary<string, object> request;
                try
                {
                    request = _json.Deserialize<Dictionary<string, object>>(body);
                }
                catch (Exception)
                {
                    WriteJson(context, 400, Error("invalid JSON body"));
                    return;
                }

                if (request == null || !request.ContainsKey("action"))
                {
                    WriteJson(context, 400, Error("missing 'action' field"));
                    return;
                }

                string action = Convert.ToString(request["action"]);
                Dictionary<string, object> parameters = null;
                if (request.ContainsKey("params") && request["params"] != null)
                {
                    parameters = request["params"] as Dictionary<string, object>;
                }

                object result = CommandDispatcher.Execute(_host, action, parameters);
                WriteJson(context, 200, result);
                return;
            }

            WriteJson(context, 404, Error("not found: " + path));
        }

        private static string ReadBody(HttpListenerContext context)
        {
            using (var reader = new StreamReader(context.Request.InputStream, context.Request.ContentEncoding ?? Encoding.UTF8))
            {
                return reader.ReadToEnd();
            }
        }

        private void WriteJson(HttpListenerContext context, int statusCode, object payload)
        {
            string json;
            try
            {
                json = _json.Serialize(payload);
            }
            catch (Exception ex)
            {
                json = _json.Serialize(Error("serialization failed: " + ex.Message));
                statusCode = 500;
            }

            byte[] buffer = Encoding.UTF8.GetBytes(json);
            context.Response.StatusCode = statusCode;
            context.Response.ContentType = "application/json; charset=utf-8";
            context.Response.ContentLength64 = buffer.Length;
            context.Response.OutputStream.Write(buffer, 0, buffer.Length);
            context.Response.OutputStream.Close();
        }

        private static Dictionary<string, object> Error(string message)
        {
            return new Dictionary<string, object>
            {
                { "ok", false },
                { "error", message }
            };
        }
    }
}
